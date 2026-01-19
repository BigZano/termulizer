package main

import (
	"math"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

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

// BeamRenderer creates horizontal plasma-like beams for each frequency band
type BeamRenderer struct {
	colors      [9]lipgloss.Color
	noiseGen    *NoiseGenerator
	smoothing   [9]float64 // Smoothed energy values
	chaosSmooth float64
	cache       *PerformanceCache
}

func NewBeamRenderer(noiseGen *NoiseGenerator) *BeamRenderer {
	return &BeamRenderer{
		colors:      defaultColors,
		noiseGen:    noiseGen,
		smoothing:   [9]float64{},
		chaosSmooth: 0.0,
		cache:       NewPerformanceCache(),
	}
}

func (br *BeamRenderer) SetColorScheme(scheme string) {
	switch scheme {
	case "retro":
		br.colors = retroColors
	default:
		br.colors = defaultColors
	}
}

// RenderPlasmaBeams creates vertical plasma beams for each frequency band
func (br *BeamRenderer) RenderPlasmaBeams(bands [9]float64, chaosLevel float64, width int, height int) string {
	// Smoother, more viscous transitions for "liquid" feel
	smoothFactor := 0.35 // 35% new, 65% old = smoother, more viscous
	for i := range bands {
		br.smoothing[i] = br.smoothing[i]*(1-smoothFactor) + bands[i]*smoothFactor
	}
	br.chaosSmooth = br.chaosSmooth*(1-smoothFactor) + chaosLevel*smoothFactor

	// DOUBLE height for Half-Block rendering (2 pixels per terminal row)
	renderHeight := height * 2

	// Calculate beam spacing - divide width by number of beams
	numBeams := 9
	padding := 2
	usableWidth := width - (padding * 2)
	beamSpacing := usableWidth / (numBeams + 1)

	// Use cache to get grids (reused memory) - using doubled height
	grid, colorGrid, intensityGrid := br.cache.GetGrids(renderHeight, width)
	defer br.cache.ReturnGrids(grid, colorGrid, intensityGrid)

	// Use a Mutex for thread-safe grid updates during parallel rendering
	var gridMu sync.Mutex

	// Render each vertical beam in parallel for performance
	var wg sync.WaitGroup
	for beamIdx := range numBeams {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			baseX := padding + (idx+1)*beamSpacing
			energy := br.smoothing[idx]
			color := br.colors[idx]

			br.renderVerticalBeamParallel(colorGrid, intensityGrid, baseX, renderHeight, width, energy, color, idx, &gridMu)
		}(beamIdx)
	}
	wg.Wait()

	return br.gridToStringHalfBlock(colorGrid, intensityGrid, width, height)
}

// renderVerticalBeamParallel is a thread-safe version of renderVerticalBeam
func (br *BeamRenderer) renderVerticalBeamParallel(
	colorGrid [][]lipgloss.Color,
	intensityGrid [][]float64,
	baseX int,
	height int,
	width int,
	energy float64,
	color lipgloss.Color,
	beamIdx int,
	mu *sync.Mutex,
) {
	// Focused beam: Thinner core, smaller overall footprint
	// coreWidth ranges from 0.8 to 3.0. This keeps them as "strands".
	coreWidth := 0.8 + (energy * 2.2)
	time := br.noiseGen.time

	// Vertical flow
	for y := range height {
		yNorm := float64(y) / float64(height)

		// Coherent liquid wobble
		noiseX := br.noiseGen.GenerateFBM(
			yNorm*1.8,                     // Slightly more frequency for "sinuous" look
			float64(beamIdx)*0.4+time*0.7, // Movement speed
			3,
			0.45,
		)

		// Horizontal "snaking" amplitude
		distortionAmp := (2.5 + energy*6.0) * (0.8 + br.chaosSmooth*1.2)
		xOffset := noiseX * distortionAmp

		// Add high-frequency jitter for "electricity" feel during high chaos/energy
		if br.chaosSmooth > 0.3 {
			jitter := br.noiseGen.Generate(float64(y)*0.8, time*15.0) * (br.chaosSmooth - 0.3) * 2.5
			xOffset += jitter
		}

		// Beam center X position
		beamCenterX := float64(baseX) + xOffset

		// Dynamic slice width for "pulsing" electricity effect
		pulse := math.Sin(yNorm*math.Pi*4.0 - time*(6.0+energy*10.0))
		currentWidth := coreWidth * (0.9 + pulse*0.1)

		// Reduced scan width for more focus
		scanWidth := int(currentWidth * 3.5)
		for dx := -scanWidth; dx <= scanWidth; dx++ {
			x := int(beamCenterX) + dx
			if x < 0 || x >= width {
				continue
			}

			// Distance from beam center
			dist := math.Abs(float64(dx)) / currentWidth

			// Sharper core and shorter-lived glow
			var falloff float64
			if dist < 0.3 {
				// Focused core: very high intensity, narrow
				falloff = 1.0 - (dist * 0.8)
			} else {
				// Diffuse aura: drops off very quickly
				falloff = 0.75 * math.Exp(-4.5*(dist-0.3))
			}

			if falloff < 0.05 {
				continue
			}

			// Final intensity influenced by energy
			intensity := energy * falloff

			// Electric flicker
			flicker := 0.92 + 0.16*math.Sin(time*25.0+float64(y)*0.6)
			intensity *= flicker

			// Apply gradient coloring (now includes heat shift)
			gradientColor := br.cache.ApplyGradient(color, intensity)

			mu.Lock()
			currentIntensity := intensityGrid[y][x]
			if intensity > 0.04 {
				if currentIntensity > 0.04 {
					// Blend colors for liquid overlap
					totalIntent := intensity + currentIntensity
					ratio := intensity / totalIntent

					blendedColor := br.cache.BlendColors(colorGrid[y][x], gradientColor, ratio)

					// Overlaps become hotter
					intensityGrid[y][x] = math.Max(currentIntensity, math.Min(1.0, currentIntensity+intensity*0.5))
					colorGrid[y][x] = blendedColor
				} else {
					colorGrid[y][x] = gradientColor
					intensityGrid[y][x] = intensity
				}
			}
			mu.Unlock()
		}
	}
}

func (br *BeamRenderer) gridToStringHalfBlock(
	colorGrid [][]lipgloss.Color,
	intensityGrid [][]float64,
	width int,
	height int,
) string {
	sb := br.cache.GetBuilder()
	defer br.cache.ReturnBuilder(sb)

	// We iterate by 2 in the renderHeight (which is height * 2)
	for y := 0; y < height*2; y += 2 {
		x := 0
		for x < width {
			// Check if this cell is empty
			if intensityGrid[y][x] < 0.05 && (y+1 >= height*2 || intensityGrid[y+1][x] < 0.05) {
				sb.WriteString(" ")
				x++
				continue
			}

			// Group horizontal runs with same FG/BG for efficiency
			start := x
			fg := colorGrid[y][x]
			var bg lipgloss.Color
			if y+1 < height*2 {
				bg = colorGrid[y+1][x]
			} else {
				bg = lipgloss.Color("") // Default background
			}

			x++
			for x < width {
				nextFG := colorGrid[y][x]
				var nextBG lipgloss.Color
				if y+1 < height*2 {
					nextBG = colorGrid[y+1][x]
				} else {
					nextBG = lipgloss.Color("")
				}

				if nextFG != fg || nextBG != bg {
					break
				}
				x++
			}

			// Render the run as Half-Blocks
			// Character is Upper Half Block '▀'
			// Foreground = Top pixel, Background = Bottom pixel
			style := br.cache.GetStyleFGBG(fg, bg)

			// Build the string of half-blocks for this run
			runLen := x - start
			runStr := strings.Repeat("▀", runLen)
			sb.WriteString(style.Render(runStr))
		}
		if y < (height*2)-2 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
