package main

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gordonklaus/portaudio"
	"github.com/mdlayher/waveform"
)

func detectAudioSetup() error {
	// check for audio loopback device or default input
	devices, err := portaudio.Devices()
	if err != nil {
		return err
	}

	hasLoopback := false
	for _, device := range devices {
		name := strings.ToLower(device.Name)
		// Linux: monitor, Mac: BlackHole, Windows: Stereo Mix / loopback audio devices
		if strings.Contains(name, "monitor") ||
			strings.Contains(name, "blackhole") ||
			strings.Contains(name, "stereo mix") ||
			strings.Contains(name, "loopback") {
			hasLoopback = true
			break
		}
	}
	if !hasLoopback {
		fmt.Println("Warning: System audio device not detected.")
		fmt.Println("\nPlease ensure you have an audio device")
		fmt.Println("\nconnected and cofigured to display playback.")
	}

	return nil
}

type AudioMessage struct {
	Bands      [9]float64 // line bands to mirror parametric eq bands
	ChaosLevel float64    // Overal distortion level, 0.0 - 1.0
	Timestamp  time.Time
}

type FrequencyBand struct {
	Name    string
	MinFreq float64
	MaxFreq float64
	Energy  float64
	Color   lipgloss.Color
}

var bands = []FrequencyBand{
	{Name: "Sub-Bass", MinFreq: 20, MaxFreq: 60},       // Deep low-end rumble
	{Name: "Bass", MinFreq: 60, MaxFreq: 250},          // Kick drum, bass guitar fundamentals
	{Name: "Low Mids", MinFreq: 250, MaxFreq: 500},     // Lower warmth, snare body
	{Name: "Low-Mid", MinFreq: 500, MaxFreq: 1000},     // Boxiness control
	{Name: "Mids", MinFreq: 1000, MaxFreq: 2000},       // Vocal presence, guitar attack
	{Name: "Upper Mids", MinFreq: 2000, MaxFreq: 4000}, // Clarity, bite, harshness
	{Name: "Presence", MinFreq: 4000, MaxFreq: 6000},   // Forwardness, articulation
	{Name: "Highs", MinFreq: 6000, MaxFreq: 12000},     // Brilliance, cymbals
	{Name: "Air", MinFreq: 12000, MaxFreq: 20000},      // Sparkle, openness
}

func generateWaveform(inputPath, outputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}
	defer f.Close()

	// OptionsFunc, not Option; Scale takes two uints.
	opts := []waveform.OptionsFunc{
		waveform.Resolution(1024),
		waveform.Scale(2, 2), // x, y are uint
		waveform.FGColorFunction(
			waveform.SolidColor(color.RGBA{255, 0, 0, 255}),
		),
		waveform.BGColorFunction(
			waveform.SolidColor(color.White),
		),
	}

	// Create waveform
	wf, err := waveform.New(f, opts...)
	if err != nil {
		return fmt.Errorf("failed to create waveform: %w", err)
	}

	// Compute values and draw image
	vals, err := wf.Compute()
	if err != nil {
		return fmt.Errorf("failed to compute waveform: %w", err)
	}
	img := wf.Draw(vals) // img is image.Image[web:28]

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	return nil
}

func selectCaptureDevice() (*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate devices: %w", err)
	}

	// Try to find loopback/monitor device (system audio output)
	for _, device := range devices {
		if device.MaxInputChannels == 0 {
			continue // Skip output-only devices
		}

		name := strings.ToLower(device.Name)

		// Linux: PulseAudio monitor, PipeWire virtual sink
		// macOS: BlackHole driver
		// Windows: Stereo Mix, loopback
		if strings.Contains(name, "monitor") ||
			strings.Contains(name, "blackhole") ||
			strings.Contains(name, "stereo mix") ||
			strings.Contains(name, "loopback") ||
			strings.Contains(name, "what u hear") { // Creative Sound Blaster
			log.Printf("✓ Selected audio device: %s", device.Name)
			return device, nil
		}
	}

	// Fall back to default input device
	defaultInput, err := portaudio.DefaultInputDevice()
	if err != nil {
		return nil, fmt.Errorf("no suitable audio device found: %w", err)
	}

	log.Printf("⚠ Using default input device: %s (may not capture system audio)", defaultInput.Name)
	return defaultInput, nil
}

func startPortAudio(sampleRate int, framesPerBuf int, inChannels int) (chan []float32, func() error, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, nil, err
	}

	// stream buffer used for stream.Read
	buffer := make([]float32, framesPerBuf*inChannels)

	// select device
	device, err := selectCaptureDevice()
	if err != nil {
		portaudio.Terminate()
		return nil, nil, fmt.Errorf("device selection failed: %w", err)
	}

	// Prefer the device's default sample rate when available to avoid
	// "Invalid sample rate" errors from PortAudio/ALSA.
	sr := sampleRate
	if device != nil && device.DefaultSampleRate > 0 {
		sr = int(device.DefaultSampleRate)
	}

	// Configure stream with selected device
	streamParams := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   device,
			Channels: inChannels,
			Latency:  device.DefaultLowInputLatency,
		},
		SampleRate:      float64(sr),
		FramesPerBuffer: len(buffer),
	}

	stream, err := portaudio.OpenStream(streamParams, buffer)
	if err != nil {
		portaudio.Terminate()
		return nil, nil, err
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		portaudio.Terminate()
		return nil, nil, err
	}

	ch := make(chan []float32, 8)
	done := make(chan struct{})
	doneDone := make(chan struct{})

	// reader loop
	go func() {
		defer stream.Stop()
		defer stream.Close()
		defer close(doneDone)

		for {
			select {
			case <-done:
				close(ch)
				return
			default:
				if err := stream.Read(); err != nil {
					// stop on read error
					close(ch)
					return
				}
				buf := make([]float32, len(buffer))
				copy(buf, buffer)

				select {
				case ch <- buf:
				case <-done:
					close(ch)
					return
				}
			}
		}
	}()

	// stop function: signal goroutine, wait for finish and terminate PortAudio
	stop := func() error {
		close(done)
		<-doneDone
		return portaudio.Terminate()
	}
	return ch, stop, nil
}

func main() {
	// init port audio
	const sampleRate = 44100
	const framesPerBuffer = 2048
	const channels = 1

	audioChan, stopAudio, err := startPortAudio(sampleRate, framesPerBuffer, channels)
	if err != nil {
		log.Fatal(err)
	}
	defer stopAudio()

	processor := NewAudioProcessor(sampleRate, framesPerBuffer)

	tuiChan := make(chan AudioMessage, 10)

	go func() {
		for buffer := range audioChan {
			msg := processor.ProcessBuffer(buffer)
			select {
			case tuiChan <- msg:
			default:
				// skip if TUI is behind
			}
		}
		close(tuiChan)
	}()

	p := tea.NewProgram(initialModel(tuiChan), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
