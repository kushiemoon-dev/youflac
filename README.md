<div align="center">

# YouFLAC

[![Download](https://img.shields.io/badge/Download-Latest-blue?style=for-the-badge&logo=github)](https://github.com/kushiemoon-dev/youflac/releases)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://github.com/kushiemoon-dev/youflac#docker)

**YouTube Video + Lossless FLAC Audio = Perfect MKV**

*Create high-quality music videos by combining YouTube video with lossless FLAC audio from Tidal, Qobuz & Amazon Music*

![Windows 10+](https://img.shields.io/badge/Windows-10+-0078D6?style=flat-square&logo=windows)
![macOS 10.13+](https://img.shields.io/badge/macOS-10.13+-000000?style=flat-square&logo=apple)
![Linux](https://img.shields.io/badge/Linux-Any-FCC624?style=flat-square&logo=linux&logoColor=black)
![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat-square&logo=docker&logoColor=white)

</div>

---

## Features

- **4K Video** — Downloads best quality video from YouTube (up to 4K AV1/VP9)
- **Lossless Audio** — Fetches FLAC from Tidal, Qobuz, or Amazon Music
- **Smart Matching** — Auto-matches video to correct audio track via song.link
- **MKV Output** — Combines video + FLAC into a single MKV container
- **Media Server Ready** — Generates NFO files for Jellyfin/Plex/Kodi
- **Queue System** — Process multiple videos with concurrent downloads
- **Lyrics Support** — Fetches synced lyrics from LRCLIB (embed or .lrc file)
- **Download History** — Track and re-download previous items
- **Audio Analyzer** — Visualize spectrogram and waveform
- **Cross-Platform** — Desktop app or Docker container

---

## Download

**[⬇️ Download Latest Release](https://github.com/kushiemoon-dev/youflac/releases)**

---

## Docker

Run YouFLAC as a web application with Docker:

```bash
# Quick start
docker run -d \
  --name youflac \
  -p 8080:8080 \
  -v ./config:/config \
  -v ./downloads:/downloads \
  youflac:latest

# Or with docker-compose
git clone https://github.com/kushiemoon-dev/youflac.git
cd youflac
docker compose up -d
```

Access the web UI at `http://localhost:8080`

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `OUTPUT_DIR` | `/downloads` | Download directory |
| `CONFIG_DIR` | `/config` | Config directory |
| `VIDEO_QUALITY` | `best` | `best`, `1080p`, `720p`, `480p` |
| `CONCURRENT_DOWNLOADS` | `2` | Parallel downloads (1-5) |
| `NAMING_TEMPLATE` | `jellyfin` | `jellyfin`, `plex`, `flat`, `album`, `year` |
| `GENERATE_NFO` | `true` | Create NFO metadata files |
| `EMBED_COVER_ART` | `true` | Embed cover art in MKV |
| `LYRICS_ENABLED` | `false` | Fetch lyrics automatically |
| `LYRICS_EMBED_MODE` | `lrc` | `lrc`, `embed`, `both` |
| `AUDIO_SOURCE_PRIORITY` | `tidal,qobuz,amazon` | Audio source order |
| `COOKIES_BROWSER` | `` | Browser for YouTube cookies |
| `THEME` | `dark` | `dark`, `light`, `system` |
| `ACCENT_COLOR` | `pink` | UI accent color |

See [.env.example](.env.example) for full configuration.

---

## Screenshot

<div align="center">
<img src="screenshot.png" alt="YouFLAC Screenshot" width="800">
</div>

---

## Requirements (Desktop)

| Dependency | Installation |
|------------|--------------|
| **FFmpeg** | Must be in PATH |
| **yt-dlp** | Must be in PATH |

```bash
# Arch Linux
sudo pacman -S ffmpeg yt-dlp

# Ubuntu/Debian
sudo apt install ffmpeg && pip install yt-dlp

# macOS
brew install ffmpeg yt-dlp

# Windows (Chocolatey)
choco install ffmpeg yt-dlp
```

> **Note:** Docker image includes FFmpeg and yt-dlp — no manual installation needed.

---

## Build from Source

### Desktop App (Wails)

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone & build
git clone https://github.com/kushiemoon-dev/youflac.git
cd youflac
cd frontend && pnpm install && cd ..
wails build
```

### Docker Image

```bash
git clone https://github.com/kushiemoon-dev/youflac.git
cd youflac
docker compose build
```

---

## How It Works

```
YouTube URL
     │
     ▼
┌──────────────┐     ┌──────────────┐
│   yt-dlp     │     │  song.link   │
│  Video DL    │     │   Resolve    │
└──────┬───────┘     └──────┬───────┘
       │                    │
       │              ┌─────▼─────┐
       │              │   Tidal   │
       │              │   FLAC    │
       │              └─────┬─────┘
       │                    │
       ▼                    ▼
┌────────────────────────────────┐
│      FFmpeg Mux → MKV          │
└────────────────────────────────┘
```

---

## Output Structure

```
~/MusicVideos/
└── Artist Name/
    └── Song Title/
        ├── Song Title.mkv
        ├── Song Title.nfo
        ├── Song Title.lrc        (if lyrics enabled)
        └── Song Title-poster.jpg
```

---

## Configuration

Settings location:
- **Linux**: `~/.config/youflac/config.json`
- **macOS**: `~/Library/Application Support/youflac/config.json`
- **Windows**: `%APPDATA%\youflac\config.json`
- **Docker**: `/config/config.json`

| Setting | Default |
|---------|---------|
| Output Directory | `~/MusicVideos` |
| Video Quality | `best` |
| Audio Sources | `tidal, qobuz, amazon` |
| Generate NFO | `true` |
| Embed Cover Art | `true` |
| Lyrics | `disabled` |

---

## Credits

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — Video downloading
- [FFmpeg](https://ffmpeg.org/) — Media muxing
- [Wails](https://wails.io/) — Desktop framework
- [Fiber](https://gofiber.io/) — HTTP framework (Docker)
- [song.link](https://song.link/) — Cross-platform music linking
- [LRCLIB](https://lrclib.net/) — Synced lyrics database
- Inspired by [SpotiFLAC](https://github.com/afkarxyz/SpotiFLAC)

---

## Disclaimer

1. **YouFLAC** is intended for **educational and private use only**.

2. This is a **third-party tool** and is **not affiliated with, endorsed by, or connected to** YouTube, Tidal, Qobuz, Amazon Music, or any other streaming service.

3. By using this tool, **you agree** to:
   - Use it in compliance with all applicable laws in your jurisdiction
   - Respect the Terms of Service of the platforms involved
   - Take full responsibility for how you use downloaded content

4. The developers of YouFLAC **assume no liability** for any misuse of this software or any violations of third-party terms or copyrights.

---

<div align="center">

**MIT License** — see [LICENSE](LICENSE)

</div>
