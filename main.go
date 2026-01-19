package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gordonklaus/portaudio"
	"github.com/mdlayher/waveform"
)

func mustGetWd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "<unknown>"
	}
	return wd
}

// detectTerminalCapabilities checks if the terminal supports alt-screen and returns appropriate options
// Defaults to inline mode for maximum compatibility across all shells
func detectTerminalCapabilities() []tea.ProgramOption {
	// Check TERM environment variable
	termType := os.Getenv("TERM")
	shell := os.Getenv("SHELL")

	LogInfo("Terminal detection: TERM=%s, SHELL=%s", termType, shell)
	fmt.Fprintf(os.Stderr, "[DEBUG] Terminal: TERM=%s, SHELL=%s\n", termType, shell)

	// Default to inline mode for compatibility with all shells (starship, zsh, fish, etc.)
	// User can opt-in to alt-screen mode via MUSIC_VIS_MODE=altscreen if their terminal supports it
	hasAltScreen := false

	// Allow user to override and enable alt-screen mode explicitly
	if forceMode := os.Getenv("MUSIC_VIS_MODE"); forceMode != "" {
		LogInfo("User override via MUSIC_VIS_MODE=%s", forceMode)
		if forceMode == "altscreen" || forceMode == "alt-screen" {
			LogInfo("User enabled alt-screen mode")
			fmt.Fprintln(os.Stderr, "[INFO] Running in alt-screen mode (user override)")
			hasAltScreen = true
		}
	}

	// Build options list
	opts := []tea.ProgramOption{
		tea.WithMouseCellMotion(),
	}

	if hasAltScreen {
		LogInfo("Enabling alt-screen mode")
		opts = append(opts, tea.WithAltScreen())
	} else {
		LogInfo("Running in inline mode (compatible with all shells)")
		fmt.Fprintln(os.Stderr, "[INFO] Running in inline mode (compatible with all shells)")
	}

	return opts
}

// isTerminalInteractive checks if we're running in an interactive terminal
func isTerminalInteractive() bool {
	// Check if stdin is a terminal
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

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
		fmt.Println("\nconnected and configured to display playback.")
	}

	return nil
}

// Include metadata from OS media session
// (populated on platforms that support it)
type AudioMessage struct {
	Bands      [9]float64
	ChaosLevel float64
	Timestamp  time.Time
	Metadata   AudioMetadata
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

var (
	fps         = flag.Int("fps", 60, "Frames per second(10-120)")
	sensitivity = flag.Float64("sensitivity", 1.0, "Audio sensitivity multiplier(0.5-2.0)")
	colorScheme = flag.String("colors", "vibrant", "Color scheme ( vibrant, retro, pastel, mono)")
	deviceName  = flag.String("device", "", "Audio device name (empty = auto)")
)

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

	// Get the default sink name from PulseAudio/PipeWire on Linux
	var defaultSinkMonitor string
	if runtime.GOOS == "linux" {
		cmd := exec.Command("pactl", "info")
		if output, err := cmd.Output(); err == nil {
			// Extract default sink name
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Default Sink:") {
					parts := strings.Split(line, ":")
					if len(parts) == 2 {
						sinkName := strings.TrimSpace(parts[1])
						defaultSinkMonitor = sinkName + ".monitor"
						log.Printf("Found default sink: %s (looking for monitor: %s)", sinkName, defaultSinkMonitor)
						break
					}
				}
			}
		}
	}

	// First pass: Try to find the monitor of the default sink
	// Extract key parts of the sink name to match against PortAudio device names
	if defaultSinkMonitor != "" {
		// The monitor name often differs slightly from PulseAudio/PipeWire naming
		// Extract meaningful parts (e.g., "SteelSeries_Arctis" from full name)
		sinkParts := strings.FieldsFunc(defaultSinkMonitor, func(r rune) bool {
			return r == '.' || r == '-' || r == '_'
		})

		for _, device := range devices {
			if device.MaxInputChannels == 0 {
				continue
			}
			name := strings.ToLower(device.Name)

			// Try to match multiple parts of the sink name
			matchCount := 0
			for _, part := range sinkParts {
				if len(part) > 3 && strings.Contains(name, strings.ToLower(part)) {
					matchCount++
				}
			}

			// If we match at least 2 significant parts, it's likely our device
			if matchCount >= 2 {
				log.Printf("✓ Selected monitor of default sink: %s (matched %d parts of %s)",
					device.Name, matchCount, defaultSinkMonitor)
				return device, nil
			}
		}
	}

	// Second pass: Try to find any monitor device (more specific than loopback)
	for _, device := range devices {
		if device.MaxInputChannels == 0 {
			continue
		}

		name := strings.ToLower(device.Name)

		// Skip JACK devices that cause crashes
		if strings.Contains(name, "jack") {
			continue
		}

		// Prioritize monitor devices
		if strings.Contains(name, "monitor") {
			log.Printf("✓ Selected monitor device: %s", device.Name)
			return device, nil
		}
	}

	// Third pass: Try loopback devices (less reliable)
	for _, device := range devices {
		if device.MaxInputChannels == 0 {
			continue // Skip output-only devices
		}

		name := strings.ToLower(device.Name)

		// Skip JACK devices that cause crashes
		if strings.Contains(name, "jack") {
			continue
		}

		// macOS: BlackHole driver
		// Windows: Stereo Mix, loopback
		if strings.Contains(name, "blackhole") ||
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
	// Force ALSA to avoid JACK backend issues on Linux
	os.Setenv("PA_ALSA_PLUGHW", "1")
	os.Setenv("SDL_AUDIODRIVER", "alsa")

	// select device - but we'll use OpenDefaultStream for now to test
	device, err := selectCaptureDevice()
	if err != nil {
		return nil, nil, fmt.Errorf("device selection failed: %w", err)
	}

	LogInfo("Selected device: %s", device.Name)

	// Create Int32 buffer for reading audio
	bufferInt32 := make([]int32, framesPerBuf)

	// Use OpenDefaultStream with the buffer - simpler and more reliable
	stream, err := portaudio.OpenDefaultStream(inChannels, 0, float64(sampleRate), len(bufferInt32), bufferInt32)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return nil, nil, fmt.Errorf("failed to start stream: %w", err)
	}

	ch := make(chan []float32, 8)
	done := make(chan struct{})
	doneDone := make(chan struct{})

	// reader loop
	go func() {
		defer func() {
			stream.Stop()
			stream.Close()
			close(ch)
			close(doneDone)
		}()

		for {
			select {
			case <-done:
				return
			default:
				// Read audio data
				if err := stream.Read(); err != nil {
					LogError("Stream read error: %v", err)
					return
				}

				// Convert Int32 to Float32 normalized to [-1.0, 1.0]
				buf := make([]float32, len(bufferInt32))
				for i, sample := range bufferInt32 {
					buf[i] = float32(sample) / 2147483648.0 // 2^31
				}

				select {
				case ch <- buf:
				default:
					// Channel full, skip
				}
			}
		}
	}()

	stop := func() error {
		close(done)
		<-doneDone
		return nil
	}

	return ch, stop, nil
}

func main() {
	// Write to stderr immediately so we know the binary runs
	fmt.Fprintln(os.Stderr, "[DEBUG] Binary starting...")

	// Initialize logger first - use current directory
	logPath := "./log.log"
	fmt.Fprintf(os.Stderr, "[DEBUG] Initializing logger at %s\n", logPath)
	if err := InitLogger(logPath); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Failed to initialize logger: %v\n", err)
		wd, _ := os.Getwd()
		fmt.Fprintf(os.Stderr, "[DEBUG] Working directory: %s\n", wd)
		os.Exit(1)
	}
	defer CloseLogger()

	LogInfo("Application starting")
	fmt.Fprintf(os.Stderr, "[DEBUG] Logger initialized at %s\n", logPath)

	// Ensure terminal is restored even on panic
	defer func() {
		if r := recover(); r != nil {
			LogPanic(r, "main function")
			// Aggressively restore terminal state
			fmt.Print("\033[?25h")   // Show cursor
			fmt.Print("\033[?1049l") // Exit alt screen
			fmt.Print("\033[2J")     // Clear screen
			fmt.Print("\033[H")      // Home
			fmt.Print("\033c")       // Full reset
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintf(os.Stderr, "\n[FATAL] CRASH - check log: %s\n", logPath)
			fmt.Fprintf(os.Stderr, "[FATAL] Panic: %v\n", r)
			os.Exit(1)
		}
	}()

	fmt.Fprintln(os.Stderr, "[DEBUG] Calling runVisualizer()")
	runVisualizer()
	LogInfo("Application exiting normally")
	fmt.Fprintln(os.Stderr, "[DEBUG] Exiting normally")
}

func runVisualizer() {
	fmt.Fprintln(os.Stderr, "[DEBUG] runVisualizer() started")

	// init port audio
	const sampleRate = 44100
	const framesPerBuffer = 2048
	const channels = 1

	flag.Parse()

	LogInfo("Initializing PortAudio (rate=%d, buffer=%d, channels=%d)", sampleRate, framesPerBuffer, channels)
	fmt.Fprintln(os.Stderr, "[DEBUG] About to initialize PortAudio")

	// Initialize PortAudio early so device queries work (used by detectAudioSetup)
	if err := portaudio.Initialize(); err != nil {
		LogError("Failed to initialize PortAudio: %v", err)
		log.Fatal("failed to initialize PortAudio:", err)
	}
	defer portaudio.Terminate()

	LogInfo("PortAudio initialized successfully")

	if err := detectAudioSetup(); err != nil {
		LogError("Audio setup detection failed: %v", err)
		log.Fatal(err)
	}

	LogInfo("Audio setup detected")

	LogInfo("Starting audio capture")
	audioChan, stopAudio, audioErr := StartAudioCapture(sampleRate, framesPerBuffer)
	if audioErr != nil {
		LogError("Failed to start audio capture: %v", audioErr)
		log.Fatal(audioErr)
	}
	defer stopAudio()

	LogInfo("PortAudio stream started successfully")

	LogInfo("Creating audio processor")
	processor, procErr := NewAudioProcessor(sampleRate, framesPerBuffer)
	if procErr != nil {
		LogError("Failed to create audio processor: %v", procErr)
		log.Fatal(procErr)
	}

	LogInfo("Audio processor created successfully")

	tuiChan := make(chan AudioMessage, 10)
	quitAudio := make(chan struct{})
	var wg sync.WaitGroup

	LogInfo("Starting audio processing goroutine")

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				LogPanic(r, "audio goroutine")
			}
			LogInfo("Audio goroutine shutting down")
			close(tuiChan)
		}()

		LogDebug("Audio goroutine running")

		for {
			select {
			case buffer, ok := <-audioChan:
				if !ok {
					return
				}
				// Safely process buffer with recovery
				func() {
					defer func() {
						if r := recover(); r != nil {
							LogPanic(r, "ProcessBuffer")
						}
					}()
					msg := processor.ProcessBuffer(buffer)
					select {
					case tuiChan <- msg:
					case <-quitAudio:
						return
					}
				}()
			case <-quitAudio:
				return
			}
		}
	}()

	LogInfo("Initializing TUI")
	fmt.Fprintln(os.Stderr, "[INFO] Creating Bubbletea TUI...")

	// Check if we're in an interactive terminal
	if !isTerminalInteractive() {
		LogError("Not running in an interactive terminal")
		fmt.Fprintln(os.Stderr, "[ERROR] This program requires an interactive terminal")
		log.Fatal("Not an interactive terminal")
	}

	// Detect terminal capabilities and get appropriate options
	terminOptions := detectTerminalCapabilities()

	// Create Bubbletea program with detected options
	p := tea.NewProgram(
		initialModel(tuiChan),
		terminOptions...,
	)

	if p == nil {
		LogError("Failed to create Bubbletea program")
		fmt.Fprintln(os.Stderr, "[FATAL] Failed to create Bubbletea program")
		log.Fatal("Failed to create TUI")
	}

	// Ensure terminal restoration on panic (shell-agnostic)
	defer func() {
		if r := recover(); r != nil {
			LogPanic(r, "TUI panic")
			// Universal terminal cleanup sequences that work across shells

			// Final cleanup - ensure terminal is in good state
			fmt.Print("\033[?25h")   // Show cursor
			fmt.Print("\r\n")        // Carriage return + newline
			os.Stdout.Sync()         // Flush output
			fmt.Print("\033[?25h")   // Show cursor
			fmt.Print("\033[?1049l") // Exit alt screen
			fmt.Print("\033[2J")     // Clear screen
			fmt.Print("\033[H")      // Move to home
			fmt.Print("\r\n")        // Newline with carriage return
			os.Stdout.Sync()         // Flush output
			fmt.Fprintf(os.Stderr, "\n[FATAL] TUI panic: %v\nCheck log.log for details\n", r)
			panic(r) // Re-panic after cleanup
		}
	}()

	// Setup signal handler for graceful shutdown
	LogInfo("Setting up signal handlers")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run TUI in goroutine so we can monitor signals
	LogInfo("Starting TUI")
	fmt.Fprintln(os.Stderr, "[INFO] Starting TUI - entering alt screen mode")
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				LogPanic(r, "TUI goroutine panic")
				fmt.Fprintf(os.Stderr, "[PANIC] TUI goroutine: %v\n", r)
				done <- fmt.Errorf("panic: %v", r)
				return
			}
		}()

		LogInfo("Calling p.Run()")
		fmt.Fprintln(os.Stderr, "[DEBUG] Calling Bubbletea p.Run()")
		_, err := p.Run()
		LogInfo("TUI p.Run() returned with error: %v", err)
		fmt.Fprintf(os.Stderr, "[DEBUG] TUI exited, error=%v\n", err)
		done <- err
	}()

	// Wait for TUI to finish or signal
	var runErr error
	select {
	case <-sigChan:
		LogInfo("Signal received, initiating shutdown")
		fmt.Fprintln(os.Stderr, "[INFO] Signal received, quitting TUI")
		p.Quit()
		runErr = <-done
	case runErr = <-done:
		LogInfo("TUI finished normally")
		fmt.Fprintln(os.Stderr, "[INFO] TUI finished")
	}

	if runErr != nil {
		LogError("TUI error: %v", runErr)
		fmt.Fprintf(os.Stderr, "[ERROR] TUI error: %v\n", runErr)
	}

	// Signal audio goroutine to stop
	LogInfo("Signaling audio goroutine to stop")
	close(quitAudio)

	LogInfo("Waiting for audio goroutine to finish")
	wg.Wait()

	LogInfo("Draining audio channel")
	drainCount := 0
	for range tuiChan {
		drainCount++
	}
	LogInfo("Drained %d messages from channel", drainCount)
	LogInfo("Shutdown complete")
}
