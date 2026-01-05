<div align="center">

# YouFLAC

[![Download](https://img.shields.io/badge/Download-Latest-blue?style=for-the-badge&logo=github)](https://github.com/kushiemoon-dev/youflac/releases)

**YouTube Video + Lossless FLAC Audio = Perfect MKV**

*Create high-quality music videos by combining YouTube video with lossless FLAC audio from Tidal, Qobuz & Amazon Music*

![Windows 10+](https://img.shields.io/badge/Windows-10+-0078D6?style=flat-square&logo=windows)
![macOS 10.13+](https://img.shields.io/badge/macOS-10.13+-000000?style=flat-square&logo=apple)
![Linux](https://img.shields.io/badge/Linux-Any-FCC624?style=flat-square&logo=linux&logoColor=black)

</div>

---

## Features

- **4K Video** — Downloads best quality video from YouTube (up to 4K AV1/VP9)
- **Lossless Audio** — Fetches FLAC from Tidal, Qobuz, or Amazon Music
- **Smart Matching** — Auto-matches video to correct audio track via song.link
- **MKV Output** — Combines video + FLAC into a single MKV container
- **Media Server Ready** — Generates NFO files for Jellyfin/Plex/Kodi
- **Queue System** — Process multiple videos with concurrent downloads
- **Cross-Platform** — Runs on Windows, Linux, and macOS

---

## Download

**[⬇️ Download Latest Release](https://github.com/kushiemoon-dev/youflac/releases)**

---

## Screenshot

<div align="center">
<img src="screenshot.png" alt="YouFLAC Screenshot" width="800">
</div>

---

## Requirements

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

---

## Build from Source

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone & build
git clone https://github.com/kushiemoon-dev/youflac.git
cd youflac
cd frontend && pnpm install && cd ..
wails build
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
        └── Song Title-poster.jpg
```

---

## Configuration

Settings location:
- **Linux**: `~/.config/youflac/config.json`
- **macOS**: `~/Library/Application Support/youflac/config.json`
- **Windows**: `%APPDATA%\youflac\config.json`

| Setting | Default |
|---------|---------|
| Output Directory | `~/MusicVideos` |
| Video Quality | `best` |
| Audio Sources | `tidal, qobuz, amazon` |
| Generate NFO | `true` |
| Embed Cover Art | `true` |

---

## Credits

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — Video downloading
- [FFmpeg](https://ffmpeg.org/) — Media muxing
- [Wails](https://wails.io/) — Desktop framework
- [song.link](https://song.link/) — Cross-platform music linking
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
