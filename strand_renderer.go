package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StrandRenderer handles vertical sine wave visualization
type StrandRenderer struct {
	colors           [9]lipgloss.Color
	noiseGen         *NoiseGenerator
	smoothedEnergies [9]float64
	previousEnergies [9]float64
	bandPhysics      [9]BandPhysics
	chaosSmooth      float64
}

var defaultColors = [9]lipgloss.Color{
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

var retroColors = [9]lipgloss.Color{
	lipgloss.Color("#FF0080"),
	lipgloss.Color("#FF0099"),
	lipgloss.Color("#FF00CC"),
	lipgloss.Color("#FF00FF"),
	lipgloss.Color("#CC00FF"),
	lipgloss.Color("#9900FF"),
	lipgloss.Color("#6600FF"),
	lipgloss.Color("#3300FF"),
	lipgloss.Color("#FF00CC"),
}

func NewStrandRenderer(noiseGen *NoiseGenerator) *StrandRenderer {
	return &StrandRenderer{
		colors:           defaultColors,
		noiseGen:         noiseGen,
		smoothedEnergies: [9]float64{},
		previousEnergies: [9]float64{},
		bandPhysics: [9]BandPhysics{
			lowFreqPhysics,
			lowFreqPhysics,
			lowFreqPhysics,
			midFreqPhysics,
			midFreqPhysics,
			midFreqPhysics,
			highFreqPhysics,
			highFreqPhysics,
			highFreqPhysics,
		},
		chaosSmooth: 0.0,
	}
}

func (sr *StrandRenderer) SetColorScheme(scheme string) {
	switch scheme {
	case "retro":
		sr.colors = retroColors
	default:
		sr.colors = defaultColors
	}
}

// RenderVerticalWaves creates 9 vertical sine wave strands
func (sr *StrandRenderer) RenderVerticalWaves(bands [9]float64, chaosLevel float64, width int, height int) string {
	// Smooth the values to reduce jitter (exponential smoothing)
	smoothFactor := 0.3
	for i := range bands {
		// Use actual band energies from audio processing
		sr.smoothedEnergies[i] = sr.smoothedEnergies[i]*(1-smoothFactor) + bands[i]*smoothFactor
	}
	sr.chaosSmooth = sr.chaosSmooth*(1-smoothFactor) + chaosLevel*smoothFactor

	LogDebug("Rendering: bands=[%.3f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f]",
		sr.smoothedEnergies[0], sr.smoothedEnergies[1], sr.smoothedEnergies[2], sr.smoothedEnergies[3],
		sr.smoothedEnergies[4], sr.smoothedEnergies[5], sr.smoothedEnergies[6], sr.smoothedEnergies[7], sr.smoothedEnergies[8])

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

	// Render each strand with the same test energy
	for strandIdx := range numStrands {
		baseX := padding + (strandIdx+1)*strandSpacing
		energy := sr.smoothedEnergies[strandIdx] // All should be the same now
		color := sr.colors[strandIdx]

		// Render this strand's wave
		sr.renderStrand(grid, colorGrid, intensityGrid, baseX, height, energy, color, strandIdx)
	}

	// Convert grid to string with colors
	return sr.gridToString(grid, colorGrid, intensityGrid, width, height)
}

// renderStrand draws a single vertical sine wave strand with smooth animation and diffusion
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
	// Wave parameters with smooth, clean sine behavior
	baselineAmp := 3.0                                     // Increased from 1.5 for better visibility
	amplitude := baselineAmp + (energy * 12.0)             // Increased multiplier from 8.0 to 12.0
	frequency := 1.5 + (energy * 2.5)                      // How many complete cycles
	phase := sr.noiseGen.time * (0.8 + sr.chaosSmooth*1.5) // Smooth animation speed

	LogDebug("Strand %d: energy=%.6f, amplitude=%.2f", strandIdx, energy, amplitude)

	// Apply subtle FBM distortion only during high chaos
	octaves := 1 + int(sr.chaosSmooth*3)
	persistence := 0.3 + (sr.chaosSmooth * 0.2)

	// Draw the wave from top to bottom with smooth interpolation
	for y := range height {
		// Normalize y to 0-1 then to phase angle
		yNorm := float64(y) / float64(height)
		angle := yNorm * math.Pi * 2.0 * frequency

		// Pure smooth sine wave as base (traditional and responsive)
		sineWave := math.Sin(angle + phase + float64(strandIdx)*0.2)

		// Create gradient color based on position along the strand
		// Color transitions from darker at edges to brighter in middle
		gradientFactor := math.Sin(yNorm * math.Pi) // 0 at top/bottom, 1 at middle

		// Add subtle FBM only for chaos/variation
		var waveValue float64
		if sr.chaosSmooth > 0.1 {
			noise := sr.noiseGen.GenerateFBM(
				float64(strandIdx)*0.3,
				yNorm*2.0+sr.noiseGen.time*0.3,
				octaves,
				persistence,
			)
			// Keep distortion subtle to maintain clean wave
			blendFactor := 0.1 + (sr.chaosSmooth * 0.15)
			waveValue = sineWave*(1-blendFactor) + noise*blendFactor
		} else {
			waveValue = sineWave
		}

		// Calculate x offset with sub-pixel smoothing
		xOffsetFloat := waveValue * amplitude
		xOffset := int(xOffsetFloat)
		fractional := xOffsetFloat - float64(xOffset)
		x := baseX + xOffset

		// Calculate intensity with fractional sub-pixel contribution
		baseIntensity := 0.6 + (energy * 0.4)
		intensity := baseIntensity * (1.0 - math.Abs(sineWave)*0.3) * gradientFactor // Use gradientFactor

		// Main pixel
		if x >= 0 && x < len(grid[0]) {
			sr.drawPixelWithDiffusion(grid, colorGrid, intensityGrid, x, y, color, intensity, fractional, baseX, amplitude)
		}
	}
}

// drawPixelWithDiffusion adds a pixel with vertical and horizontal diffusion aura for smooth color blending
func (sr *StrandRenderer) drawPixelWithDiffusion(
	grid [][]rune,
	colorGrid [][]lipgloss.Color,
	intensityGrid [][]float64,
	x int, y int,
	color lipgloss.Color,
	intensity float64,
	_ float64,
	_ int,
	_ float64,
) {
	// Apply gradient to color based on intensity
	gradientColor := applyGradient(color, intensity)

	// Main pixel
	currentIntensity := intensityGrid[y][x]
	if intensity > currentIntensity {
		grid[y][x] = getCharForIntensity(intensity)
		colorGrid[y][x] = gradientColor
		intensityGrid[y][x] = intensity
	} else if currentIntensity > 0.15 && intensity > 0.15 {
		// Blend colors when overlapping
		blendedColor := blendColors(colorGrid[y][x], gradientColor, 0.5)
		colorGrid[y][x] = blendedColor
		intensityGrid[y][x] = math.Max(currentIntensity, intensity)
	}

	// Enhanced vertical diffusion aura (above and below for smooth color blending)
	for dy := -3; dy <= 3; dy++ { // Increased from -2 to -3
		if dy == 0 {
			continue
		}
		gy := y + dy
		if gy >= 0 && gy < len(grid) {
			// Gentler falloff for more diffusion
			diffusionIntensity := intensity * math.Pow(0.65, float64(int(math.Abs(float64(dy)))))
			currentDiffIntensity := intensityGrid[gy][x]

			if diffusionIntensity > currentDiffIntensity {
				grid[gy][x] = getCharForIntensity(diffusionIntensity)
				colorGrid[gy][x] = applyGradient(color, diffusionIntensity)
				intensityGrid[gy][x] = diffusionIntensity
			} else if currentDiffIntensity > 0.1 && diffusionIntensity > 0.1 {
				// Blend overlapping diffusion
				blendedColor := blendColors(colorGrid[gy][x], applyGradient(color, diffusionIntensity), 0.3)
				colorGrid[gy][x] = blendedColor
			}
		}
	}

	// Enhanced horizontal diffusion aura for ALL energy levels (not just high)
	maxHDiffusion := 2
	if intensity > 0.65 {
		maxHDiffusion = 4 // More spread for high energy
	}

	for dx := -maxHDiffusion; dx <= maxHDiffusion; dx++ {
		if dx == 0 {
			continue
		}
		gx := x + dx
		if gx >= 0 && gx < len(grid[0]) {
			diffusionIntensity := intensity * math.Pow(0.5, float64(int(math.Abs(float64(dx)))))
			currentDiffIntensity := intensityGrid[y][gx]

			if diffusionIntensity > 0.08 { // Lower threshold for more spread
				if diffusionIntensity > currentDiffIntensity {
					grid[y][gx] = getCharForIntensity(diffusionIntensity)
					colorGrid[y][gx] = applyGradient(color, diffusionIntensity)
					intensityGrid[y][gx] = diffusionIntensity
				} else if currentDiffIntensity > 0.08 && diffusionIntensity > 0.08 {
					// Blend overlapping diffusion - this creates color mixing
					blendedColor := blendColors(colorGrid[y][gx], applyGradient(color, diffusionIntensity), 0.4)
					colorGrid[y][gx] = blendedColor
				}
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
// Refined for "liquid plasma" beams to be more defined and less blocky
func getCharForIntensity(intensity float64) rune {
	if intensity > 0.85 {
		return '█' // Solid
	} else if intensity > 0.65 {
		return '▓' // Dense
	} else if intensity > 0.45 {
		return '▒' // Medium
	} else if intensity > 0.25 {
		return '░' // Light
	} else if intensity > 0.10 {
		return '·' // Dot
	}
	return ' ' // Too dim
}

// applyGradient creates a gradient version of a color based on intensity
func applyGradient(baseColor lipgloss.Color, intensity float64) lipgloss.Color {
	hexStr := string(baseColor)
	if len(hexStr) != 7 || hexStr[0] != '#' {
		return baseColor
	}

	var r, g, b uint8
	fmt.Sscanf(hexStr, "#%02x%02x%02x", &r, &g, &b)

	factor := 0.3 + (intensity * 0.7)
	r = uint8(float64(r) * factor)
	g = uint8(float64(g) * factor)
	b = uint8(float64(b) * factor)

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b))
}

// blendColors mixes two colors together
func blendColors(c1, c2 lipgloss.Color, ratio float64) lipgloss.Color {
	hex1, hex2 := string(c1), string(c2)
	if len(hex1) != 7 || len(hex2) != 7 || hex1[0] != '#' || hex2[0] != '#' {
		return c1
	}

	var r1, g1, b1, r2, g2, b2 uint8
	fmt.Sscanf(hex1, "#%02x%02x%02x", &r1, &g1, &b1)
	fmt.Sscanf(hex2, "#%02x%02x%02x", &r2, &g2, &b2)

	r := uint8(float64(r1)*(1-ratio) + float64(r2)*ratio)
	g := uint8(float64(g1)*(1-ratio) + float64(g2)*ratio)
	b := uint8(float64(b1)*(1-ratio) + float64(b2)*ratio)

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b))
}
