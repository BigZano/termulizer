package main

import (
	"log"
	"math"
	"math/cmplx"
	"time"

	"github.com/gordonklaus/portaudio"
	"gonum.org/v1/gonum/dsp/fourier"
)

type AudioProcessor struct {
	stream          *portaudio.Stream
	buffer          []float32
	analysisBuffer  []float32
	sampleRate      int
	bufferSize      int
	noiseGen        *NoiseGenerator
	bands           [9]float64
	smoothedBands   [9]float64
	smoothingFactor float64
	chaosLevel      float64
	fft             *fourier.FFT
	mediaProvider   *MediaSessionProvider
}

func NewAudioProcessor(sampleRate, bufferSize int) (*AudioProcessor, error) {
	mediaProvider, err := NewMediaSessionProvider()
	if err != nil {
		log.Printf("Error loading metadata provider: %v", err)
		mediaProvider = nil
	}

	return &AudioProcessor{
		sampleRate:     sampleRate,
		bufferSize:     bufferSize,
		buffer:         make([]float32, bufferSize),
		analysisBuffer: make([]float32, bufferSize),
		fft:            fourier.NewFFT(bufferSize),
		mediaProvider:  mediaProvider,
	}, nil
}

// Builds buffer to perform FFT and calculates band "energy"
func (ap *AudioProcessor) ProcessBuffer(buffer []float32) AudioMessage {
	defer func() {
		if r := recover(); r != nil {
			LogPanic(r, "ProcessBuffer internal")
		}
	}()

	// Validate input
	if len(buffer) == 0 {
		LogError("ProcessBuffer received empty buffer")
		return AudioMessage{
			Bands:      [9]float64{},
			ChaosLevel: 0,
			Timestamp:  time.Now(),
			Metadata:   DefaultMetadata(),
		}
	}

	// Split stereo into L and R channels
	// Stereo format: [L, R, L, R, L, R, ...]
	var leftChannel, rightChannel []float64
	if len(buffer)%2 == 0 {
		// Stereo input
		leftChannel = make([]float64, len(buffer)/2)
		rightChannel = make([]float64, len(buffer)/2)
		for i := 0; i < len(buffer); i += 2 {
			leftChannel[i/2] = float64(buffer[i])
			rightChannel[i/2] = float64(buffer[i+1])
		}
	} else {
		// Mono fallback
		leftChannel = make([]float64, len(buffer))
		rightChannel = make([]float64, len(buffer))
		for i, sample := range buffer {
			leftChannel[i] = float64(sample)
			rightChannel[i] = float64(sample)
		}
	}

	// Run FFT on both channels independently in parallel
	type fftResult struct {
		coeffs []complex128
		err    error
	}

	leftResult := make(chan fftResult, 1)
	rightResult := make(chan fftResult, 1)

	// Left channel FFT
	go func() {
		coeffs := ap.fft.Coefficients(nil, leftChannel)
		leftResult <- fftResult{coeffs: coeffs, err: nil}
	}()

	// Right channel FFT
	go func() {
		coeffs := ap.fft.Coefficients(nil, rightChannel)
		rightResult <- fftResult{coeffs: coeffs, err: nil}
	}()

	// Wait for both FFTs to complete
	leftFFT := <-leftResult
	rightFFT := <-rightResult

	if len(leftFFT.coeffs) == 0 || len(rightFFT.coeffs) == 0 {
		LogError("FFT returned empty coefficients")
		return AudioMessage{
			Bands:      [9]float64{},
			ChaosLevel: 0,
			Timestamp:  time.Now(),
			Metadata:   DefaultMetadata(),
		}
	}

	// Debug: Log FFT magnitudes
	if len(leftFFT.coeffs) > 500 {
		LogDebug("FFT: L_coeffs=%d, L_mag[100]=%.8f, R_mag[100]=%.8f",
			len(leftFFT.coeffs), cmplx.Abs(leftFFT.coeffs[100]), cmplx.Abs(rightFFT.coeffs[100]))
	}

	binWidth := float64(ap.sampleRate) / float64(len(leftChannel))

	// Extract energy from bands using BOTH channels
	var bandEnergies [9]float64
	var totalEnergy float64

	// Use package-level `bands` (defined in main.go) for frequency ranges.
	for i := range bandEnergies {
		if i >= len(bands) {
			break
		}
		fb := bands[i]
		minBin := int(fb.MinFreq / binWidth)
		maxBin := int(fb.MaxFreq / binWidth)

		// clamp and validate
		if minBin < 0 {
			minBin = 0
		}
		if maxBin > len(leftFFT.coeffs) {
			maxBin = len(leftFFT.coeffs)
		}
		if maxBin <= minBin {
			bandEnergies[i] = 0
			continue
		}

		// Process LEFT and RIGHT channels separately, then combine
		var leftBandSum, rightBandSum float64

		for j := minBin; j < maxBin && j < len(leftFFT.coeffs); j++ {
			// Left channel
			leftMag := cmplx.Abs(leftFFT.coeffs[j])
			weightedLeft := leftMag * math.Log1p(leftMag)
			leftBandSum += weightedLeft * weightedLeft

			// Right channel
			rightMag := cmplx.Abs(rightFFT.coeffs[j])
			weightedRight := rightMag * math.Log1p(rightMag)
			rightBandSum += weightedRight * weightedRight
		}

		denom := float64(maxBin - minBin)
		if denom <= 0 {
			bandEnergies[i] = 0
			continue
		}

		// Combine L+R with stereo width calculation
		// Use RMS of both channels plus a stereo width factor
		leftEnergy := math.Sqrt(leftBandSum / denom)
		rightEnergy := math.Sqrt(rightBandSum / denom)

		// Stereo width: difference between L and R adds variation
		stereoWidth := math.Abs(leftEnergy-rightEnergy) * 0.3

		// Combined energy: average of L+R plus stereo width bonus
		bandEnergy := (leftEnergy + rightEnergy) / 2.0
		bandEnergy += stereoWidth // Stereo differences add energy

		// Apply power curve to enhance mid-range response
		bandEnergy = math.Pow(bandEnergy, 0.8)

		// Per-band scaling to compensate for natural frequency roll-off
		// Balanced for "Liquid" movement: responsive but not jittery
		scaleFactors := []float64{
			10.0, // Sub-Bass (Thump)
			12.0, // Bass
			14.0, // Low Mids
			16.0, // Low-Mid
			18.0, // Mids
			22.0, // Upper Mids
			26.0, // Presence
			30.0, // Highs
			35.0, // Air
		}

		if i < len(scaleFactors) {
			bandEnergy *= scaleFactors[i]
		} else {
			bandEnergy *= 100000.0
		}

		LogDebug("Band %d: L=%.8f, R=%.8f, combined=%.6f, stereoWidth=%.6f (bins %d-%d)",
			i, leftEnergy, rightEnergy, bandEnergy, stereoWidth, minBin, maxBin)

		// Ensure no NaN or Inf
		if !math.IsNaN(bandEnergy) && !math.IsInf(bandEnergy, 0) && bandEnergy > 0 {
			bandEnergies[i] = bandEnergy
			totalEnergy += bandEnergy
		} else {
			bandEnergies[i] = 0
		}
	}

	// Gentler normalization - preserve relative differences between bands
	// Instead of forcing max to 1.0, just clamp outliers
	for i := range bandEnergies {
		if bandEnergies[i] > 1.0 {
			bandEnergies[i] = 1.0
		}
		// Apply a slight boost to very quiet bands to make them visible
		if bandEnergies[i] < 0.1 && bandEnergies[i] > 0.001 {
			bandEnergies[i] = 0.1 + (bandEnergies[i] * 2.0)
		}
	}

	LogDebug("Energy stats: total=%.6f, bands=[%.3f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f,%.3f]",
		totalEnergy, bandEnergies[0], bandEnergies[1], bandEnergies[2],
		bandEnergies[3], bandEnergies[4], bandEnergies[5], bandEnergies[6], bandEnergies[7], bandEnergies[8])

	chaosLevel := calculateChaos(bandEnergies[:], totalEnergy)

	// attach metadata if available
	var metadata AudioMetadata
	if ap.mediaProvider != nil {
		metadata = ap.mediaProvider.GetCurrentMedia()
	} else {
		metadata = DefaultMetadata()
	}

	return AudioMessage{
		Bands:      bandEnergies,
		ChaosLevel: chaosLevel,
		Timestamp:  time.Now(),
		Metadata:   metadata,
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

func (ap *AudioProcessor) Close() error {
	if ap.mediaProvider != nil {
		ap.mediaProvider.Close()
	}
	return nil
}
