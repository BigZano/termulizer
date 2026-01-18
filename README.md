# Music Visualizer

A cross-platform terminal-based music visualizer inspired by **Windows Media Player (90s era)**. Captures real-time system audio and generates **9 vertical sine wave strands** with chaos-driven FBM distortion.

![Version](https://img.shields.io/badge/version-2.0-blue)
![Status](https://img.shields.io/badge/status-production--ready-success)
![Go](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey)

---

## Features

- **9 Vertical Sine Wave Strands** - One per frequency band (Sub-Bass → Air)
- **Real-Time Audio Capture** - FFT analysis at 30 FPS
- **Chaos-Driven Distortion** - FBM noise responds to music intensity
- **Cross-Platform** - Linux, macOS, and Windows support

---

## Quick Start

### Prerequisites

**Linux:**
```bash
sudo apt-get install portaudio19-dev
```

**macOS:**
```bash
brew install portaudio
```

**Windows:**  
No additional dependencies (PortAudio included)

### Build

```bash
git clone <your-repo-url>
cd music_visualizer
go build -o vis .
```

### Configure Audio Loopback

You need a loopback device to capture system audio, virtual or physical.

**Linux (PulseAudio):**
```bash
pactl load-module module-loopback
```

**Linux (PipeWire):**
Usually available by default. Verify with:
```bash
pw-cli list-objects Node | grep monitor
```

else you'll likely be using PulseAudio. Same command should confirm if you're using Pulse or PipeWire. 

**macOS:**
```bash
brew install blackhole-2ch
# Then configure in Audio MIDI Setup
```

**Windows:**  
Enable "Stereo Mix" in Sound Settings → Recording Devices

### Run

```bash
./vis
```

Press `q` to quit.

---

## Bands

Each band is targeted to represent a specific frequency range:

| # | Band        | Range (Hz)     | Target                   |
|---|-------------|----------------|--------------------------|
| 1 | Sub-Bass    | 20-60          | Deep rumble              |
| 2 | Bass        | 60-250         | Kick, bass               |
| 3 | Low Mids    | 250-500        | Warmth, snare            |
| 4 | Low-Mid     | 500-1000       | Guitar                   |
| 5 | Mids        | 1000-2000      | Vocals, guitar           |
| 6 | Upper Mids  | 2000-4000      | Clarity, bite            |
| 7 | Presence    | 4000-6000      | Articulation             |
| 8 | Highs       | 6000-12000     | Cymbals, brilliance      |
| 9 | Air         | 12000-20000    | Sparkle, airiness        |


---

## Built With

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** - TUI framework
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** - Terminal styling
- **[PortAudio](http://www.portaudio.com/)** - Cross-platform audio I/O
- **[GoNum](https://www.gonum.org/)** - FFT and DSP algorithms
- **[OpenSimplex](https://github.com/ojrac/opensimplex-go)** - FBM noise generation

-- ❤️ and Go

---

## Troubleshooting

### Waves don't move
- Verify audio loopback device is configured
- Check that music is playing
- Increase system volume

### ALSA warnings (Linux)
Harmless - ALSA scans for all possible device types.

### High CPU usage
- Resize terminal to 120x40 or smaller
- Check for other resource-intensive processes

### Colors look dull
Use a modern terminal with TrueColor support:
- Linux: Alacritty, Kitty
- macOS: iTerm2
- Windows: Windows Terminal

---

## Road Map

- [ ] API integration
- [ ] Configuration system (CLI flags, config file)
- [ ] Alternative color schemes
- [ ] Alternative visualization schemes

---

## Contact

For questions, issues, or suggestions, please open an issue on GitHub.
