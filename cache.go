package main

import (
	"math"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

const sineTableSize = 8192

type SineTable struct {
	table []float64
	size  int
}

func NewSineTable() *SineTable {
	table := make([]float64, sineTableSize)
	for i := 0; i < sineTableSize; i++ {
		angle := float64(i) / float64(sineTableSize) * 2 * math.Pi
		table[i] = math.Sin(angle)
	}
	return &SineTable{table: table, size: sineTableSize}
}

func (st *SineTable) Sin(angle float64) float64 {
	normalized := math.Mod(angle, 2*math.Pi)
	if normalized < 0 {
		normalized += 2 * math.Pi
	}
	tablePos := (normalized / (2 * math.Pi)) * float64(st.size)
	index := int(tablePos)
	fraction := tablePos - float64(index)
	val1 := st.table[index%st.size]
	val2 := st.table[(index+1)%st.size]
	return val1 + fraction*(val2-val1)
}

type PerformanceCache struct {
	sineTable     *SineTable
	gridPool      sync.Pool
	colorPool     sync.Pool
	intensityPool sync.Pool
	styleCache    map[string]lipgloss.Style
	styleMu       sync.RWMutex
	builderPool   sync.Pool
}

func NewPerformanceCache() *PerformanceCache {
	return &PerformanceCache{
		sineTable:  NewSineTable(),
		styleCache: make(map[string]lipgloss.Style, 3000),
		gridPool: sync.Pool{
			New: func() interface{} {
				return make([][]rune, 0)
			},
		},
		colorPool: sync.Pool{
			New: func() interface{} {
				return make([][]lipgloss.Color, 0)
			},
		},
		intensityPool: sync.Pool{
			New: func() interface{} {
				return make([][]float64, 0)
			},
		},
		builderPool: sync.Pool{
			New: func() interface{} {
				return new(strings.Builder)
			},
		},
	}
}

func (pc *PerformanceCache) GetGrids(height, width int) ([][]rune, [][]lipgloss.Color, [][]float64) {
	grid := pc.gridPool.Get().([][]rune)
	colorGrid := pc.colorPool.Get().([][]lipgloss.Color)
	intensityGrid := pc.intensityPool.Get().([][]float64)

	if len(grid) < height {
		grid = make([][]rune, height)
		colorGrid = make([][]lipgloss.Color, height)
		intensityGrid = make([][]float64, height)
	}

	for y := 0; y < height; y++ {
		if len(grid[y]) < width {
			grid[y] = make([]rune, width)
			colorGrid[y] = make([]lipgloss.Color, width)
			intensityGrid[y] = make([]float64, width)
		}
		for x := 0; x < width; x++ {
			grid[y][x] = ' '
			colorGrid[y][x] = lipgloss.Color("")
			intensityGrid[y][x] = 0
		}
	}
	return grid[:height], colorGrid[:height], intensityGrid[:height]
}

func (pc *PerformanceCache) ReturnGrids(grid [][]rune, colorGrid [][]lipgloss.Color, intensityGrid [][]float64) {
	pc.gridPool.Put(grid)
	pc.colorPool.Put(colorGrid)
	pc.intensityPool.Put(intensityGrid)
}

func (pc *PerformanceCache) GetStyle(fg lipgloss.Color) lipgloss.Style {
	key := string(fg)
	pc.styleMu.RLock()
	style, ok := pc.styleCache[key]
	pc.styleMu.RUnlock()
	if ok {
		return style
	}

	pc.styleMu.Lock()
	defer pc.styleMu.Unlock()
	if style, ok = pc.styleCache[key]; ok {
		return style
	}
	style = lipgloss.NewStyle().Foreground(fg)
	pc.styleCache[key] = style
	return style
}

func (pc *PerformanceCache) GetStyleFGBG(fg, bg lipgloss.Color) lipgloss.Style {
	key := string(fg) + "," + string(bg)
	pc.styleMu.RLock()
	style, ok := pc.styleCache[key]
	pc.styleMu.RUnlock()
	if ok {
		return style
	}

	pc.styleMu.Lock()
	defer pc.styleMu.Unlock()
	if style, ok = pc.styleCache[key]; ok {
		return style
	}
	style = lipgloss.NewStyle().Foreground(fg).Background(bg)
	pc.styleCache[key] = style
	return style
}

func (pc *PerformanceCache) GetBuilder() *strings.Builder {
	sb := pc.builderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

func (pc *PerformanceCache) ReturnBuilder(sb *strings.Builder) {
	pc.builderPool.Put(sb)
}

// ApplyGradient handles both brightness and "Heat Heat" shift towards white for high intensity
func (pc *PerformanceCache) ApplyGradient(baseColor lipgloss.Color, intensity float64) lipgloss.Color {
	r, g, b, ok := parseHex(string(baseColor))
	if !ok {
		return baseColor
	}

	// Linear-to-power gradient for brightness
	brightnessFactor := 0.2 + math.Pow(intensity, 0.7)*0.8

	// Heat shift: very high intensity makes the core turn white-ish
	heatFactor := 0.0
	if intensity > 0.7 {
		heatFactor = (intensity - 0.7) / 0.3 // 0.0 to 1.0
	}

	rf := float64(r) * brightnessFactor
	gf := float64(g) * brightnessFactor
	bf := float64(b) * brightnessFactor

	// Blend with white for the "hot" core
	rf = rf*(1-heatFactor) + 255*heatFactor
	gf = gf*(1-heatFactor) + 255*heatFactor
	bf = bf*(1-heatFactor) + 255*heatFactor

	return uint8ToHex(uint8(rf), uint8(gf), uint8(bf))
}

func (pc *PerformanceCache) BlendColors(c1, c2 lipgloss.Color, ratio float64) lipgloss.Color {
	r1, g1, b1, ok1 := parseHex(string(c1))
	r2, g2, b2, ok2 := parseHex(string(c2))

	if !ok1 {
		return c2
	}
	if !ok2 {
		return c1
	}

	// Simple linear blending to preserve the "heat map" overlap without it getting muddy
	r := uint8(float64(r1)*(1-ratio) + float64(r2)*ratio)
	g := uint8(float64(g1)*(1-ratio) + float64(g2)*ratio)
	b := uint8(float64(b1)*(1-ratio) + float64(b2)*ratio)

	return uint8ToHex(r, g, b)
}

func uint8ToHex(r, g, b uint8) lipgloss.Color {
	const hex = "0123456789ABCDEF"
	var res [7]byte
	res[0] = '#'
	res[1] = hex[r>>4]
	res[2] = hex[r&0x0F]
	res[3] = hex[g>>4]
	res[4] = hex[g&0x0F]
	res[5] = hex[b>>4]
	res[6] = hex[b&0x0F]
	return lipgloss.Color(string(res[:]))
}

func parseHex(hex string) (uint8, uint8, uint8, bool) {
	if len(hex) != 7 || hex[0] != '#' {
		return 0, 0, 0, false
	}

	r := hexToUint8(hex[1], hex[2])
	g := hexToUint8(hex[3], hex[4])
	b := hexToUint8(hex[5], hex[6])

	return r, g, b, true
}

func hexToUint8(h, l byte) uint8 {
	return (unhex(h) << 4) | unhex(l)
}

func unhex(b byte) uint8 {
	switch {
	case '0' <= b && b <= '9':
		return b - '0'
	case 'a' <= b && b <= 'f':
		return b - 'a' + 10
	case 'A' <= b && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}
