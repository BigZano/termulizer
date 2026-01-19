package main

import (
	"math"

	"github.com/ojrac/opensimplex-go"
)

type NoiseGenerator struct {
	noise opensimplex.Noise
	time  float64
}

func NewNoiseGenerator(seed int64) *NoiseGenerator {
	return &NoiseGenerator{
		noise: opensimplex.New(seed),
		time:  0.0,
	}
}

// FBM generator, octaves based on chaos level and persistence also based on chaos level. Crazier the input, crazier the visual output.
func (ng *NoiseGenerator) GenerateFBM(x, y float64, octaves int, persistence float64) float64 {
	var total, frequency, amplitude, maxValue float64 = 0, 1, 1, 0

	for i := 0; i < octaves; i++ {
		total += ng.noise.Eval2(x*frequency, y*frequency) * amplitude
		maxValue += amplitude
		amplitude *= persistence
		frequency *= 2
	}

	return total / maxValue
}

// apply noise-based distortion to sine wave
func (ng *NoiseGenerator) DistortWave(x, amplitude, chaosLevel float64) float64 {
	// Base sine wave
	baseWave := math.Sin(x)

	// Dynamic octives
	octaves := 2 + int(chaosLevel*6)

	// Dynamic persistence
	persistence := 0.3 + (chaosLevel * 0.5)

	// Apply distortion
	distortion := ng.GenerateFBM(x*0.1, ng.time*0.1, octaves, persistence)

	// Blend it all together
	blendFactor := 0.1 + (chaosLevel * 0.4)

	return (baseWave * (1 - blendFactor)) + (distortion * blendFactor * amplitude)
}

func (ng *NoiseGenerator) Update(delta float64) {
	ng.time += delta
}

// Generate simple 2D noise
func (ng *NoiseGenerator) Generate(x, y float64) float64 {
	return ng.noise.Eval2(x, y)
}
