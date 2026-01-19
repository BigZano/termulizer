package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	width        int
	height       int
	bands        [9]float64
	chaosLevel   float64
	audioChan    <-chan AudioMessage
	noiseGen     *NoiseGenerator
	beamRenderer *BeamRenderer
	metadata     AudioMetadata
	colorScheme  string
	ready        bool
}

type (
	tickMsg  time.Time
	audioMsg AudioMessage
)

func initialModel(audioChan <-chan AudioMessage) model {
	noiseGen := NewNoiseGenerator(time.Now().UnixNano())

	LogInfo("Creating initial TUI model")

	return model{
		audioChan:    audioChan,
		noiseGen:     noiseGen,
		beamRenderer: NewBeamRenderer(noiseGen),
		metadata:     DefaultMetadata(),
		colorScheme:  "original",
		ready:        false,
	}
}

func (m model) Init() tea.Cmd {
	LogInfo("TUI Init() called")
	return tea.Batch(
		tickCmd(),
		waitForAudio(m.audioChan),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second/60, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForAudio(ch <-chan AudioMessage) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			LogInfo("Audio channel closed, sending tea.Quit")
			return tea.Quit
		}
		return audioMsg(msg)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			LogInfo("User requested quit via key: %s", msg.String())
			return m, tea.Quit
		case " ": // spacebar
			// Toggle between color schemes
			if m.colorScheme == "original" {
				m.colorScheme = "retro"
			} else {
				m.colorScheme = "original"
			}
			m.beamRenderer.SetColorScheme(m.colorScheme)
			LogDebug("Color scheme changed to: %s", m.colorScheme)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		LogInfo("Window resized: %dx%d", m.width, m.height)

	case tickMsg:
		// update noise animation (60 FPS)
		m.noiseGen.Update(1.0 / 60.0)
		return m, tickCmd()

	case audioMsg:
		// updates with new audio data
		m.bands = msg.Bands
		m.chaosLevel = msg.ChaosLevel
		m.metadata = msg.Metadata

		return m, waitForAudio(m.audioChan)

	case tea.QuitMsg:
		LogInfo("Received tea.QuitMsg")
		return m, tea.Quit
	}

	return m, nil
}

func (m model) View() string {
	if !m.ready || m.width == 0 {
		return "Initializing visualizer..."
	}

	// Calculate section heights
	metadataHeight := int(float64(m.height) * 0.3)
	waveHeight := m.height - metadataHeight - 2 // -2 for footer

	// Render metadata section (top 30%)
	metadata := RenderMetadata(m.metadata, m.width, metadataHeight)

	// Render horizontal plasma beams (bottom 70%)
	waves := m.beamRenderer.RenderPlasmaBeams(m.bands, m.chaosLevel, m.width, waveHeight)

	// Footer
	schemeLabel := "Original"
	if m.colorScheme == "retro" {
		schemeLabel = "Retro"
	}

	footer := lipgloss.NewStyle().
		Faint(true).
		Foreground(lipgloss.Color("#888888")).
		Render("\nPress 'q' to quit | SPACE to change colors | 60 FPS | " + schemeLabel)

	return fmt.Sprintf("%s\n%s%s", metadata, waves, footer)
}
