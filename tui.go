package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	width          int
	height         int
	bands          [9]float64
	chaosLevel     float64
	audioChan      <-chan AudioMessage
	noiseGen       *NoiseGenerator
	strandRenderer *StrandRenderer
	metadata       AudioMetadata
	history        []AudioMessage
	maxHistory     int
}

type (
	tickMsg  time.Time
	audioMsg AudioMessage
)

func initialModel(audioChan <-chan AudioMessage) model {
	noiseGen := NewNoiseGenerator(time.Now().UnixNano())

	return model{
		audioChan:      audioChan,
		noiseGen:       noiseGen,
		strandRenderer: NewStrandRenderer(noiseGen),
		metadata:       DefaultMetadata(),
		maxHistory:     30, // Reduced for 30 FPS
		history:        make([]AudioMessage, 0, 30),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.waitForAudio(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) waitForAudio() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.audioChan
		if !ok {
			return tea.Quit() // Channel closed, exit gracefully
		}
		return audioMsg(msg)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		// update noise animation (30 FPS)
		m.noiseGen.Update(1.0 / 30.0)
		return m, tickCmd()

	case audioMsg:
		// updates with new audio data
		m.bands = msg.Bands
		m.chaosLevel = msg.ChaosLevel
		m.history = append(m.history, AudioMessage(msg))
		if len(m.history) > m.maxHistory {
			m.history = m.history[1:]
		}

		return m, m.waitForAudio()
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var output strings.Builder

	// Calculate section heights
	metadataHeight := int(float64(m.height) * 0.3)
	waveHeight := m.height - metadataHeight - 2 // -2 for footer

	// Render metadata section (top 30%)
	metadata := RenderMetadata(m.metadata, m.width, metadataHeight)
	output.WriteString(metadata)
	output.WriteString("\n")

	// Render vertical sine wave strands (bottom 70%)
	waves := m.strandRenderer.RenderVerticalWaves(m.bands, m.chaosLevel, m.width, waveHeight)
	output.WriteString(waves)

	// Footer
	footer := lipgloss.NewStyle().
		Faint(true).
		Foreground(lipgloss.Color("#888888")).
		Render("\nPress 'q' to quit | 30 FPS")
	output.WriteString(footer)

	return output.String()
}
