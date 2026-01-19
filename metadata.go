package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

// AudioMetadata represents currently playing media information
type AudioMetadata struct {
	AppName    string
	ArtistName string
	SongName   string
	AlbumName  string
	IsPlaying  bool
	Duration   int64 // Duration in seconds
}

// DefaultMetadata returns a placeholder when no media info is available
func DefaultMetadata() AudioMetadata {
	return AudioMetadata{
		AppName:    "No Media Playing",
		ArtistName: "Unknown Artist",
		SongName:   "Listening...",
		IsPlaying:  false,
	}
}

// RenderMetadata creates the top 30% metadata display section
func RenderMetadata(metadata AudioMetadata, width int, height int) string {
	var output strings.Builder

	// Title bar with retro aesthetic
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF00FF")).
		Background(lipgloss.Color("#1A0033")).
		Width(width).
		Align(lipgloss.Center)

	output.WriteString(titleStyle.Render("♪ MUSIC VISUALIZER ♪"))
	output.WriteString("\n\n")

	// App name
	appStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")).Bold(true)

	output.WriteString(appStyle.Render("▶ Source: "))
	output.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(metadata.AppName))
	output.WriteString("\n\n")

	// Artist
	if metadata.ArtistName != "" {
		artistStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true)

		output.WriteString(artistStyle.Render("♫ Artist: "))
		output.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB6C1")).Render(truncateString(metadata.ArtistName, width-15)))
		output.WriteString("\n")
	}

	// Song/Track
	if metadata.SongName != "" {
		songStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true)

		output.WriteString(songStyle.Render("♬ Track:  "))
		output.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFE0")).Render(truncateString(metadata.SongName, width-15)))
		output.WriteString("\n")
	}

	output.WriteString("\n")

	// Status indicator
	statusChar := "█"
	if !metadata.IsPlaying {
		statusChar = "▌▌"
	}

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render(fmt.Sprintf("[%s] %s", statusChar, getStatusText(metadata.IsPlaying)))

	output.WriteString(statusStyle)
	output.WriteString("\n\n")

	// Separator line
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))

	separator := strings.Repeat("═", width-2)
	output.WriteString(separatorStyle.Render(separator))
	output.WriteString("\n")

	return output.String()
}

func getStatusText(isPlaying bool) string {
	if isPlaying {
		return "PLAYING"
	}
	return "PAUSED"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func extractAppName(busName string) string {
	parts := strings.Split(busName, ".")
	if len(parts) < 4 {
		return "Unknown"
	}

	appName := parts[3]

	if len(appName) > 0 {
		appName = strings.ToUpper(appName[:1]) + appName[1:]
	}

	return appName
}

func extractString(metadata map[string]dbus.Variant, key string) string {
	variant, ok := metadata[key]
	if !ok {
		return ""
	}

	str, ok := variant.Value().(string)
	if !ok {
		return ""
	}

	return str
}

func extractStringArray(metadata map[string]dbus.Variant, key string) string {
	variant, ok := metadata[key]
	if !ok {
		return ""
	}

	strArray, ok := variant.Value().([]string)
	if ok && len(strArray) > 0 {
		if len(strArray) > 1 {
			return strings.Join(strArray, ", ")
		}
		return strArray[0]
	}

	str, ok := variant.Value().(string)
	if ok {
		return str
	}

	return ""
}

func (msp *MediaSessionProvider) ListAvailablePlayers() []string {
	var names []string
	if err := msp.conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return nil
	}

	var players []string
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			players = append(players, extractAppName(name))
		}
	}

	return players
}
