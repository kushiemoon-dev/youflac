package backend

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// processItem runs the full download pipeline for a single queue item.
// Called by worker goroutines.
func (q *Queue) processItem(id string) {
	// Create cancellable context for this item
	itemCtx, cancel := context.WithCancel(q.ctx)

	// Store cancel func
	q.mutex.Lock()
	for i := range q.items {
		if q.items[i].ID == id {
			// Skip if not pending
			if q.items[i].Status != StatusPending {
				q.mutex.Unlock()
				cancel()
				return
			}
			q.items[i].cancelFunc = cancel
			q.items[i].Status = StatusFetchingInfo
			q.items[i].StartedAt = time.Now()
			q.items[i].Stage = "Fetching video info..."
			break
		}
	}
	q.mutex.Unlock()

	defer cancel()

	// Get item info
	item := q.GetItem(id)
	if item == nil {
		return
	}

	// Emit started event
	q.emit(QueueEvent{
		Type:   "updated",
		ItemID: id,
		Item:   item,
	})

	// Load config
	q.mutex.RLock()
	config := q.config
	q.mutex.RUnlock()

	if config == nil {
		config = &defaultConfig
	}

	// Create temp directory for this download
	tempDir := filepath.Join(os.TempDir(), "youflac", id)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		q.SetItemError(id, fmt.Errorf("failed to create temp dir: %w", err))
		return
	}
	defer os.RemoveAll(tempDir) // Cleanup on completion

	// ==========================================================================
	// Stage 1: Fetch Video Info (if not already present)
	// ==========================================================================
	var videoInfo *VideoInfo
	var videoID string
	var err error

	if item.Title == "" {
		// Normal flow: fetch from YouTube
		q.UpdateStatus(id, StatusFetchingInfo, 5, "Parsing URL...")

		videoID, err = ParseYouTubeURL(item.VideoURL)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("invalid YouTube URL: %w", err))
			return
		}

		select {
		case <-itemCtx.Done():
			return
		default:
		}

		videoInfo, err = GetVideoMetadata(videoID)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("failed to fetch video info: %w", err))
			return
		}

		// Update item with video info
		q.updateItem(id, func(item *QueueItem) {
			item.Title = videoInfo.Title
			item.Artist = videoInfo.Artist
			item.Thumbnail = videoInfo.Thumbnail
			item.Duration = videoInfo.Duration
		})
	} else {
		// Already have info (from import or previous fetch)
		if item.VideoURL != "" {
			videoID, _ = ParseYouTubeURL(item.VideoURL)
		}
		videoInfo = &VideoInfo{
			Title:     item.Title,
			Artist:    item.Artist,
			Thumbnail: item.Thumbnail,
			Duration:  item.Duration,
		}
	}

	// ==========================================================================
	// Stage 1.5: Check for Existing File (Skip Detection)
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.mutex.RLock()
	fileIndex := q.fileIndex
	q.mutex.RUnlock()

	if fileIndex != nil && videoInfo.Title != "" {
		existingFile := fileIndex.FindMatch(videoInfo.Title, videoInfo.Artist)
		if existingFile != nil {
			q.UpdateStatus(id, StatusOrganizing, 80, "Found existing file...")

			// Determine target path
			outputDir := config.OutputDirectory
			if outputDir == "" {
				outputDir = GetDefaultOutputDirectory()
			}

			// Get current item for playlist info
			item = q.GetItem(id)
			if item != nil && item.PlaylistName != "" {
				playlistFolder := SanitizeFileName(item.PlaylistName)
				outputDir = filepath.Join(outputDir, playlistFolder)
			}

			muxMetadata := &Metadata{
				Title:  videoInfo.Title,
				Artist: videoInfo.Artist,
				Track:  item.PlaylistPosition,
			}

			// Use original file extension when copying
			existingExt := filepath.Ext(existingFile.Path)
			if existingExt == "" {
				existingExt = ".mkv"
			}

			var targetPath string
			if item.PlaylistPosition > 0 {
				targetPath = GeneratePlaylistFilePath(muxMetadata, outputDir, existingExt)
			} else {
				targetPath = GenerateFilePath(muxMetadata, config.NamingTemplate, outputDir, existingExt)
			}

			// Check if it's the same path (already in correct location)
			if existingFile.Path == targetPath {
				// Already in correct location, just mark complete
				q.updateItem(id, func(item *QueueItem) {
					item.Status = StatusComplete
					item.Progress = 100
					item.Stage = "Skipped (already exists)"
					item.OutputPath = existingFile.Path
					item.CompletedAt = time.Now()
				})
				q.emit(QueueEvent{
					Type:     "completed",
					ItemID:   id,
					Progress: 100,
					Status:   StatusComplete,
				})
				slog.Info("skipped, already exists", "path", existingFile.Path)
				return
			}

			// Copy file to new location
			q.UpdateStatus(id, StatusOrganizing, 90, "Copying existing file...")
			if err := copyFile(existingFile.Path, targetPath); err == nil {
				// Update file index with new entry
				fileIndex.AddEntry(FileIndexEntry{
					Path:      targetPath,
					Title:     videoInfo.Title,
					Artist:    videoInfo.Artist,
					Duration:  existingFile.Duration,
					Size:      existingFile.Size,
					IndexedAt: time.Now(),
				})
				go fileIndex.Save()

				q.updateItem(id, func(item *QueueItem) {
					item.Status = StatusComplete
					item.Progress = 100
					item.Stage = "Copied from existing"
					item.OutputPath = targetPath
					item.CompletedAt = time.Now()
				})
				q.emit(QueueEvent{
					Type:     "completed",
					ItemID:   id,
					Progress: 100,
					Status:   StatusComplete,
				})
				slog.Info("copied from existing", "src", existingFile.Path, "dst", targetPath)
				return
			}
			// If copy fails, continue with normal download
			slog.Warn("copy failed, proceeding with download")
		}
	}

	// ==========================================================================
	// Stage 2: Download Video
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	var videoPath string
	audioOnly := false

	// Download video from YouTube
	q.UpdateStatus(id, StatusDownloadingVideo, 10, "Downloading video...")

	videoPath, err = DownloadVideo(videoID, config.VideoQuality, tempDir, config.CookiesBrowser)
	if err != nil {
		// Don't fail immediately - try audio-only fallback
		slog.Warn("video download failed, trying audio-only fallback", "err", err)
		q.UpdateStatus(id, StatusDownloadingAudio, 40, "Video unavailable, downloading audio only...")
		audioOnly = true
		videoPath = ""

		q.updateItem(id, func(item *QueueItem) {
			item.AudioOnly = true
		})
	} else {
		q.UpdateStatus(id, StatusDownloadingVideo, 40, "Video downloaded")
		slog.Debug("video downloaded", "path", videoPath)

		q.updateItem(id, func(item *QueueItem) {
			item.VideoPath = videoPath
		})
	}

	// ==========================================================================
	// Stage 3: Find and Download Audio
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.UpdateStatus(id, StatusDownloadingAudio, 40, "Finding audio match...")

	audioPath := ""

	// Create metadata for NFO
	metadata := &Metadata{
		Title:    videoInfo.Title,
		Artist:   videoInfo.Artist,
		Duration: videoInfo.Duration,
	}

	// Try to find and download FLAC audio using multi-service cascade
	audioDownloaded := false

	// Initialize download services
	tidalHifiService := NewTidalHifiService()
	lucidaService := NewLucidaService()
	orpheusService := NewOrpheusDLService()

	// Get audio links via songlink
	if item.SpotifyURL != "" || item.VideoURL != "" {
		q.UpdateStatus(id, StatusDownloadingAudio, 45, "Resolving audio sources...")
		slog.Debug("resolving audio sources", "url", item.VideoURL)

		sourceURL := item.VideoURL
		if item.SpotifyURL != "" {
			sourceURL = item.SpotifyURL
		}

		slog.Debug("calling ResolveMusicURL", "url", sourceURL)
		links, err := ResolveMusicURL(sourceURL)
		slog.Debug("ResolveMusicURL result", "err", err, "hasLinks", links != nil)
		if err == nil && links != nil {
			// Try each audio source in priority order
			for _, source := range config.AudioSourcePriority {
				select {
				case <-itemCtx.Done():
					return
				default:
				}

				var downloadURL string
				switch source {
				case "tidal":
					downloadURL = links.URLs.TidalURL
				case "qobuz":
					downloadURL = links.URLs.QobuzURL
				case "amazon":
					downloadURL = links.URLs.AmazonURL
				case "deezer":
					downloadURL = links.URLs.DeezerURL
				}

				if downloadURL == "" {
					continue
				}

				slog.Debug("trying audio source", "source", source, "url", downloadURL)
				q.UpdateStatus(id, StatusDownloadingAudio, 50, fmt.Sprintf("Downloading from %s...", source))

				// Service cascade for FLAC download
				var result *AudioDownloadResult
				var downloadErr error

				// 1. Try TidalHifiService FIRST for Tidal URLs (vogel.qqdl.site - works!)
				if source == "tidal" && tidalHifiService.IsAvailable() {
					slog.Debug("trying TidalHifi API", "source", source)
					q.UpdateStatus(id, StatusDownloadingAudio, 51, "Downloading FLAC from Tidal...")
					result, downloadErr = tidalHifiService.Download(downloadURL, tempDir, "flac")
					if downloadErr != nil {
						slog.Debug("TidalHifi failed", "err", downloadErr)
					}
				}

				// 2. Try Lucida (web API) if TidalHifi failed or not Tidal
				if result == nil {
					slog.Debug("trying Lucida", "source", source)
					result, downloadErr = lucidaService.Download(downloadURL, tempDir, "flac")
					if downloadErr != nil {
						slog.Debug("Lucida failed", "err", downloadErr)
					}
				}

				// 3. Try OrpheusDL/Streamrip (Python subprocess) as last resort
				if result == nil && orpheusService.IsAvailable() {
					slog.Debug("trying OrpheusDL/Streamrip", "source", source)
					q.UpdateStatus(id, StatusDownloadingAudio, 52, fmt.Sprintf("Trying OrpheusDL for %s...", source))
					result, downloadErr = orpheusService.Download(downloadURL, tempDir, "flac")
					if downloadErr != nil {
						slog.Debug("OrpheusDL failed", "err", downloadErr)
					}
				}

				// Success!
				if result != nil {
					slog.Info("FLAC downloaded", "source", source, "path", result.FilePath)
					audioDownloaded = true
					audioPath = result.FilePath
					q.updateItem(id, func(item *QueueItem) {
						item.AudioSource = source
						item.AudioPath = audioPath
					})
					break
				}
			}
		}
	}

	// If songlink resolution failed or no FLAC sources found, try TidalHifi search
	if !audioDownloaded && videoInfo.Artist != "" && videoInfo.Title != "" {
		slog.Debug("trying TidalHifi search", "artist", videoInfo.Artist, "title", videoInfo.Title)
		q.UpdateStatus(id, StatusDownloadingAudio, 55, "Searching Tidal for track...")

		if tidalHifiService.IsAvailable() {
			result, err := tidalHifiService.DownloadBySearch(videoInfo.Artist, videoInfo.Title, tempDir)
			if err == nil && result != nil {
				slog.Info("FLAC found via Tidal search", "path", result.FilePath)
				audioDownloaded = true
				audioPath = result.FilePath
				q.updateItem(id, func(item *QueueItem) {
					item.AudioSource = "tidal-search"
					item.AudioPath = audioPath
				})
			} else {
				slog.Warn("Tidal search failed", "err", err)
			}
		}
	}

	if !audioDownloaded {
		// Fallback: extract audio from video (only if video exists)
		if videoPath != "" {
			q.UpdateStatus(id, StatusDownloadingAudio, 55, "Extracting audio from video...")
			// Use .mka (Matroska audio) which supports any codec (opus, aac, etc.)
			audioPath = filepath.Join(tempDir, "audio.mka")

			err = ExtractAudioFromVideo(videoPath, audioPath)
			if err != nil {
				q.SetItemError(id, fmt.Errorf("failed to extract audio: %w", err))
				return
			}

			q.updateItem(id, func(item *QueueItem) {
				item.AudioSource = "extracted"
				item.AudioPath = audioPath
			})
		} else {
			// Audio-only mode but no audio was downloaded from services
			q.SetItemError(id, fmt.Errorf("failed to download audio: no audio source available and video unavailable"))
			return
		}
	}

	// ==========================================================================
	// Stage 4: Mux Video + Audio
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.UpdateStatus(id, StatusMuxing, 70, "Muxing video and audio...")

	// Determine output path
	outputDir := config.OutputDirectory
	if outputDir == "" {
		outputDir = GetDefaultOutputDirectory()
	}

	// Get current item for updated paths
	item = q.GetItem(id)

	// If item is part of a playlist, create playlist subfolder
	if item.PlaylistName != "" {
		playlistFolder := SanitizeFileName(item.PlaylistName)
		outputDir = filepath.Join(outputDir, playlistFolder)
	}

	// Create metadata for muxing
	muxMetadata := &Metadata{
		Title:     videoInfo.Title,
		Artist:    videoInfo.Artist,
		Album:     item.Album,
		Thumbnail: videoInfo.Thumbnail,
		Duration:  videoInfo.Duration,
		Track:     item.PlaylistPosition, // Use playlist position as track number
	}

	// Generate output path using naming template
	// Use .flac extension for audio-only, .mkv for video+audio
	outputExt := ".mkv"
	if audioOnly {
		outputExt = ".flac"
	}

	var outputPath string
	if item.PlaylistPosition > 0 {
		// Playlist item: use track number prefix format "01 - Artist - Title"
		outputPath = GeneratePlaylistFilePath(muxMetadata, outputDir, outputExt)
	} else {
		// Regular item: use configured naming template
		outputPath = GenerateFilePath(muxMetadata, config.NamingTemplate, outputDir, outputExt)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		q.SetItemError(id, fmt.Errorf("failed to create output directory: %w", err))
		return
	}

	// Check for conflicts
	if exists, _ := CheckFileConflict(outputPath); exists {
		outputPath = ResolveConflict(outputPath)
	}

	// Download cover if embedding
	var coverPath string
	if config.EmbedCoverArt && videoInfo.Thumbnail != "" {
		coverPath = filepath.Join(tempDir, "cover.jpg")
		if err := DownloadPoster(videoInfo.Thumbnail, coverPath); err != nil {
			coverPath = "" // Failed to download, proceed without cover
		}
	}

	var result *MuxResult
	if audioOnly {
		// Audio-only fallback: create FLAC file
		q.UpdateStatus(id, StatusMuxing, 80, "Creating FLAC file...")
		result, err = CreateFLACWithMetadata(item.AudioPath, outputPath, muxMetadata, coverPath)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("failed to create FLAC: %w", err))
			return
		}
	} else {
		// Normal case: mux video + audio into MKV
		q.UpdateStatus(id, StatusMuxing, 80, "Creating MKV file...")
		result, err = MuxVideoWithFLAC(item.VideoPath, item.AudioPath, outputPath, muxMetadata, coverPath, nil)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("failed to mux: %w", err))
			return
		}
	}

	// ==========================================================================
	// Stage 4.5: Fetch and Embed Lyrics (if enabled)
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	if config.LyricsEnabled && videoInfo.Artist != "" && videoInfo.Title != "" {
		q.UpdateStatus(id, StatusOrganizing, 85, "Fetching lyrics...")

		lyrics, lyricsErr := FetchLyrics(videoInfo.Artist, videoInfo.Title)
		if lyricsErr == nil && lyrics != nil {
			embedMode := LyricsEmbedMode(config.LyricsEmbedMode)
			if embedMode == "" {
				embedMode = LyricsEmbedLRC // Default to LRC file
			}

			switch embedMode {
			case LyricsEmbedFile:
				if err := EmbedLyricsInFile(result.OutputPath, lyrics); err != nil {
					slog.Warn("failed to embed lyrics", "err", err)
				} else {
					slog.Debug("lyrics embedded in file")
				}
			case LyricsEmbedLRC:
				if lyrics.HasSync {
					if lrcPath, err := SaveLRCFile(lyrics, result.OutputPath); err != nil {
						slog.Warn("failed to save LRC file", "err", err)
					} else {
						slog.Debug("LRC file saved", "path", lrcPath)
					}
				} else if lyrics.PlainText != "" {
					if txtPath, err := SavePlainLyricsFile(lyrics, result.OutputPath); err != nil {
						slog.Warn("failed to save lyrics file", "err", err)
					} else {
						slog.Debug("lyrics file saved", "path", txtPath)
					}
				}
			case LyricsEmbedBoth:
				// Save LRC/TXT file
				if lyrics.HasSync {
					if lrcPath, err := SaveLRCFile(lyrics, result.OutputPath); err == nil {
						slog.Debug("LRC file saved", "path", lrcPath)
					}
				} else if lyrics.PlainText != "" {
					if txtPath, err := SavePlainLyricsFile(lyrics, result.OutputPath); err == nil {
						slog.Debug("lyrics file saved", "path", txtPath)
					}
				}
				// Also embed in file
				if err := EmbedLyricsInFile(result.OutputPath, lyrics); err != nil {
					slog.Warn("failed to embed lyrics", "err", err)
				} else {
					slog.Debug("lyrics embedded in file")
				}
			}
		} else if lyricsErr != nil {
			slog.Debug("lyrics not found", "err", lyricsErr)
		}
	}

	// ==========================================================================
	// Stage 5: Organize and Generate NFO
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.UpdateStatus(id, StatusOrganizing, 90, "Organizing files...")

	// Generate NFO if enabled
	if config.GenerateNFO {
		nfoPath := outputPath[:len(outputPath)-4] + ".nfo"
		nfoOpts := &NFOOptions{
			IncludeFileInfo: true,
		}

		// Get file info for NFO
		if mediaInfo, err := GetMediaInfo(result.OutputPath); err == nil {
			nfoOpts.MediaInfo = mediaInfo
		}

		if err := WriteNFO(metadata, nfoPath, nfoOpts); err != nil {
			// Non-fatal, just log
			slog.Warn("failed to write NFO", "err", err)
		}
	}

	// Download poster alongside MKV
	if videoInfo.Thumbnail != "" {
		posterPath := outputPath[:len(outputPath)-4] + "-poster.jpg"
		DownloadPoster(videoInfo.Thumbnail, posterPath) // Ignore error, non-fatal
	}

	// ==========================================================================
	// Complete
	// ==========================================================================

	// Add completed file to index for future duplicate detection
	q.mutex.RLock()
	fi := q.fileIndex
	q.mutex.RUnlock()
	if fi != nil && videoInfo != nil {
		stat, _ := os.Stat(result.OutputPath)
		var fileSize int64
		if stat != nil {
			fileSize = stat.Size()
		}
		fi.AddEntry(FileIndexEntry{
			Path:      result.OutputPath,
			Title:     videoInfo.Title,
			Artist:    videoInfo.Artist,
			Duration:  videoInfo.Duration,
			Size:      fileSize,
			IndexedAt: time.Now(),
		})
		go fi.Save()
	}

	// Get file size for history
	var fileSize int64
	if stat, err := os.Stat(result.OutputPath); err == nil {
		fileSize = stat.Size()
	}

	q.updateItem(id, func(item *QueueItem) {
		item.Status = StatusComplete
		item.Progress = 100
		item.Stage = "Complete"
		item.OutputPath = result.OutputPath
		item.FileSize = fileSize
		item.CompletedAt = time.Now()
	})

	// Save to history
	q.mutex.RLock()
	history := q.history
	q.mutex.RUnlock()
	if history != nil {
		item = q.GetItem(id)
		if item != nil {
			history.AddFromQueueItem(item, "complete", "")
		}
	}

	q.emit(QueueEvent{
		Type:     "completed",
		ItemID:   id,
		Progress: 100,
		Status:   StatusComplete,
	})
}

// ExtractAudioFromVideo extracts the audio track from a video file.
func ExtractAudioFromVideo(videoPath, audioPath string) error {
	return ExtractAudioStream(videoPath, audioPath)
}
