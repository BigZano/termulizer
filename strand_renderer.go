package main

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StrandRenderer handles vertical sine wave visualization
type StrandRenderer struct {
	colors      [9]lipgloss.Color
	noiseGen    *NoiseGenerator
	smoothing   [9]float64 // Smoothed energy values for less jitter
	chaosSmooth float64    // Smoothed chaos level
}

func NewStrandRenderer(noiseGen *NoiseGenerator) *StrandRenderer {
	// Original color gradient (dark red → bright pink)
	colors := [9]lipgloss.Color{
		lipgloss.Color("#5A0000"), // Dark red
		lipgloss.Color("#E10600"), // Red
		lipgloss.Color("#FF7A00"), // Orange
		lipgloss.Color("#FFD400"), // Yellow
		lipgloss.Color("#3DFF4E"), // Green
		lipgloss.Color("#00E5FF"), // Cyan
		lipgloss.Color("#2F5BFF"), // Blue
		lipgloss.Color("#6A00FF"), // Purple
		lipgloss.Color("#FF00C8"), // Magenta
	}

	return &StrandRenderer{
		colors:      colors,
		noiseGen:    noiseGen,
		smoothing:   [9]float64{},
		chaosSmooth: 0.0,
	}
}

// RenderVerticalWaves creates 9 vertical sine wave strands
func (sr *StrandRenderer) RenderVerticalWaves(bands [9]float64, chaosLevel float64, width int, height int) string {
	// Smooth the values to reduce jitter (exponential smoothing)
	smoothFactor := 0.3
	for i := range bands {
		sr.smoothing[i] = sr.smoothing[i]*(1-smoothFactor) + bands[i]*smoothFactor
	}
	sr.chaosSmooth = sr.chaosSmooth*(1-smoothFactor) + chaosLevel*smoothFactor

	// Calculate strand spacing (divide width by number of strands + padding)
	numStrands := 9
	padding := 2
	usableWidth := width - (padding * 2)
	strandSpacing := usableWidth / (numStrands + 1)

	// Create a 2D grid for rendering
	grid := make([][]rune, height)
	colorGrid := make([][]lipgloss.Color, height)
	intensityGrid := make([][]float64, height)

	for y := range height {
		grid[y] = make([]rune, width)
		colorGrid[y] = make([]lipgloss.Color, width)
		intensityGrid[y] = make([]float64, width)
		for x := range width {
			grid[y][x] = ' '
		}
	}

	// Render each strand
	for strandIdx := 0; strandIdx < numStrands; strandIdx++ {
		baseX := padding + (strandIdx+1)*strandSpacing
		energy := sr.smoothing[strandIdx]
		color := sr.colors[strandIdx]

		// Render this strand's wave
		sr.renderStrand(grid, colorGrid, intensityGrid, baseX, height, energy, color, strandIdx)
	}

	// Convert grid to string with colors
	return sr.gridToString(grid, colorGrid, intensityGrid, width, height)
}

// renderStrand draws a single vertical sine wave strand
func (sr *StrandRenderer) renderStrand(
	grid [][]rune,
	colorGrid [][]lipgloss.Color,
	intensityGrid [][]float64,
	baseX int,
	height int,
	energy float64,
	color lipgloss.Color,
	strandIdx int,
) {
	// Wave parameters
	amplitude := energy * 8.0                              // How far from center the wave moves (in characters)
	frequency := 2.0 + (energy * 3.0)                      // How many complete cycles
	phase := sr.noiseGen.time * (1.0 + sr.chaosSmooth*2.0) // Animation speed based on chaos

	// Apply FBM distortion based on chaos
	octaves := 2 + int(sr.chaosSmooth*4)
	persistence := 0.4 + (sr.chaosSmooth * 0.3)

	// Draw the wave from top to bottom
	for y := 0; y < height; y++ {
		// Normalize y to 0-1 then to phase angle
		yNorm := float64(y) / float64(height)
		angle := yNorm * math.Pi * 2.0 * frequency

		// Base sine wave
		sineWave := math.Sin(angle + phase + float64(strandIdx)*0.3)

		// Add FBM noise distortion
		noise := sr.noiseGen.GenerateFBM(
			float64(strandIdx)*0.5,
			yNorm*3.0+sr.noiseGen.time*0.5,
			octaves,
			persistence,
		)

		// Blend sine and noise
		blendFactor := 0.2 + (sr.chaosSmooth * 0.3)
		waveValue := sineWave*(1-blendFactor) + noise*blendFactor

		// Calculate x offset
		xOffset := int(waveValue * amplitude)
		x := baseX + xOffset

		// Clamp to grid bounds
		if x >= 0 && x < len(grid[0]) {
			// Calculate intensity based on energy
			intensity := 0.5 + (energy * 0.5)

			// Add glow effect - draw multiple characters around the center
			sr.drawPixelWithGlow(grid, colorGrid, intensityGrid, x, y, color, intensity)
		}
	}
}

// drawPixelWithGlow adds a pixel with optional neighboring pixels for glow effect
func (sr *StrandRenderer) drawPixelWithGlow(
	grid [][]rune,
	colorGrid [][]lipgloss.Color,
	intensityGrid [][]float64,
	x int, y int,
	color lipgloss.Color,
	intensity float64,
) {
	// Main pixel
	if intensity > intensityGrid[y][x] {
		grid[y][x] = getCharForIntensity(intensity)
		colorGrid[y][x] = color
		intensityGrid[y][x] = intensity
	}

	// Add horizontal glow for high energy
	if intensity > 0.7 {
		glowIntensity := intensity * 0.6
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 {
				continue
			}
			gx := x + dx
			if gx >= 0 && gx < len(grid[0]) && glowIntensity > intensityGrid[y][gx] {
				grid[y][gx] = getCharForIntensity(glowIntensity)
				colorGrid[y][gx] = color
				intensityGrid[y][gx] = glowIntensity
			}
		}
	}
}

// gridToString converts the grid to a styled string
func (sr *StrandRenderer) gridToString(
	grid [][]rune,
	colorGrid [][]lipgloss.Color,
	intensityGrid [][]float64,
	width int,
	height int,
) string {
	var output strings.Builder

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			char := grid[y][x]
			if char != ' ' && intensityGrid[y][x] > 0 {
				styled := lipgloss.NewStyle().
					Foreground(colorGrid[y][x]).
					Render(string(char))
				output.WriteString(styled)
			} else {
				output.WriteString(" ")
			}
		}
		if y < height-1 {
			output.WriteString("\n")
		}
	}

	return output.String()
}

// getCharForIntensity returns a character based on intensity level
func getCharForIntensity(intensity float64) rune {
	if intensity > 0.85 {
		return '█' // Full block
	} else if intensity > 0.65 {
		return '▓' // Dark shade
	} else if intensity > 0.45 {
		return '▒' // Medium shade
	} else if intensity > 0.25 {
		return '░' // Light shade
	}
	return '·' // Dot
}
