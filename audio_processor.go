package main

import (
	"math"
	"math/cmplx"
	"time"

	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/gonum/dsp/window"
)

type AudioProcessor struct {
	sampleRate int
	fft        *fourier.FFT
	window     []float64
}

func NewAudioProcessor(sampleRate, bufferSize int) *AudioProcessor {
	return &AudioProcessor{
		sampleRate: sampleRate,
		fft:        fourier.NewFFT(bufferSize),
		window:     window.Hann(make([]float64, bufferSize)),
	}
}

// Builds buffer to perform FFT and calculates band "energy"
func (ap *AudioProcessor) ProcessBuffer(buffer []float32) AudioMessage {
	// Convert to float64 and apply window function
	windowedData := make([]float64, len(buffer))
	for i, sample := range buffer {
		windowedData[i] = float64(sample) * ap.window[i]
	}

	// FFT math
	coeffs := ap.fft.Coefficients(nil, windowedData)

	binWidth := float64(ap.sampleRate) / float64(len(buffer))

	// pull energy from bands
	var bandEnergies [9]float64
	var totalEnergy float64

	for i, band := range bands {
		minBin := int(band.MinFreq / binWidth)
		maxBin := int(band.MaxFreq / binWidth)

		// sum magnitudes in frequency range
		var bandSum float64
		for j := minBin; j < maxBin && j < len(coeffs); j++ {
			magnitude := cmplx.Abs(coeffs[j])
			bandSum += magnitude * magnitude
		}

		bandEnergies[i] = math.Sqrt(bandSum / float64(maxBin-minBin))
		totalEnergy += bandEnergies[i]
	}

	chaosLevel := calculateChaos(bandEnergies[:], totalEnergy)

	return AudioMessage{
		Bands:      bandEnergies,
		ChaosLevel: chaosLevel,
		Timestamp:  time.Now(),
	}
}

func calculateChaos(energies []float64, total float64) float64 {
	if total < 0.0001 {
		return 0.0
	}

	normalized := make([]float64, len(energies))
	for i, e := range energies {
		normalized[i] = e / total
	}

	// calculate variance
	mean := 1.0 / float64(len(energies))
	variance := 0.0
	for _, n := range normalized {
		diff := n - mean
		variance += diff * diff
	}

	// include total energy
	energyFactor := math.Tanh(total * 10)

	chaos := (variance*5.0)*0.6 + energyFactor*0.4

	return math.Max(0, math.Min(1, chaos))
}
