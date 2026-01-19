# Termulizer

Termulizer is a high-performance, real-time music visualizer built for the terminal. Inspired by the classic media players of the 90s, it uses nine vertical sine wave strands that react dynamically to your system audio. It's designed to be lightweight, responsive, and visually engaging, bringing a bit of nostalgic flair to your shell.

---

## Features

We've focused on making Termulizer as smooth and responsive as possible. Here is what's under the hood:

- Real-Time FFT Analysis: The audio processor captures system sound at 60 FPS, ensuring that every beat and frequency change is reflected instantly in the visualization.
- Nine Vertical Strands: We've mapped Nine independent strands to specific frequency bands, ranging from deep sub-bass to the highest air frequencies.
- Chaos-Driven Distortion: A custom FBM noise generator adds organic, fluid motion to the strands, making them look more like liquid than static waves as the music intensity increases.
- Performance First: With a custom double-buffering system and a dedicated grid-based rendering engine, we've eliminated flickering and kept CPU usage low.
- Interactivity: You can switch between different color palettes on the fly to match your terminal's theme or your current mood.
- Multiplatform: Whether you are on Linux, macOS, or Windows, Termulizer works across all major operating systems.

---

## Video Demonstration

Check out Termulizer in action:

[Watch the Demonstration Video](assets/demo.mp4)

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
Enable "Stereo Mix" in Sound Settings -> Recording Devices

### Run

```bash
./vis
```

Press 'q' to quit.

---

## Roadmap

(Roadmap section to be filled by the user)

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

## Shell Compatibility

The visualizer works with **all shells** (bash, zsh, fish, starship, and custom prompts).

**If you experience terminal corruption with custom shells:**

```bash
# Option 1: Set environment variable
export MUSIC_VIS_MODE=inline
./music_visualizer

# Option 2: Use the provided script  
./run_inline.sh
```

Inline mode disables alternate screen buffer for better compatibility with custom prompts and terminal multiplexers.

**Make it permanent:** Add `export MUSIC_VIS_MODE=inline` to your `.bashrc`, `.zshrc`, or shell config.

---

## Built With

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** - TUI framework
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** - Terminal styling
- **[PortAudio](http://www.portaudio.com/)** - Cross-platform audio I/O
- **[GoNum](https://www.gonum.org/)** - FFT and DSP algorithms
- **[OpenSimplex](https://github.com/ojrac/opensimplex-go)** - FBM noise generation


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

## Contact

For questions, issues, or suggestions, please open an issue on GitHub.
