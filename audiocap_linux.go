//go:build linux
// +build linux

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"unsafe"
)

// StartAudioCapture starts capturing system audio on Linux using PulseAudio/PipeWire
func StartAudioCapture(sampleRate, bufferSize int) (chan []float32, func() error, error) {
	// Get the default sink (playback device)
	defaultSink, err := getDefaultSink()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get default sink: %w", err)
	}

	monitorSource := defaultSink + ".monitor"
	log.Printf("Capturing from monitor source: %s", monitorSource)

	// Use parec to capture audio from the monitor source
	// Format: float32le (native endian float32), stereo (2 channels), specified sample rate
	cmd := exec.Command("parec",
		"--device="+monitorSource,
		"--format=float32le",
		"--channels=2",
		fmt.Sprintf("--rate=%d", sampleRate),
		"--latency-msec=20",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start parec: %w", err)
	}

	log.Printf("Successfully started parec capture: %dHz stereo (2ch), buffer=%d", sampleRate, bufferSize)

	audioChan := make(chan []float32, 8)
	done := make(chan struct{})

	// Reader goroutine
	go func() {
		defer func() {
			cmd.Process.Kill()
			cmd.Wait()
			close(audioChan)
		}()

		buffer := make([]byte, bufferSize*2*4) // stereo: 2 channels * 4 bytes per float32

		for {
			select {
			case <-done:
				return
			default:
				// Read raw bytes
				n, err := io.ReadFull(stdout, buffer)
				if err != nil {
					if err != io.EOF {
						LogError("Stream read error: %v", err)
					}
					return
				}

				if n > 0 && n%4 == 0 {
					// Convert bytes to float32 samples
					samples := make([]float32, n/4)
					for i := range samples {
						bits := binary.LittleEndian.Uint32(buffer[i*4 : (i+1)*4])
						samples[i] = float32frombits(bits)
					}

					select {
					case audioChan <- samples:
					default:
						// Channel full, skip this buffer
					}
				}
			}
		}
	}()

	stopFunc := func() error {
		close(done)
		return nil
	}

	return audioChan, stopFunc, nil
}

// getDefaultSink queries PulseAudio/PipeWire for the current default sink
func getDefaultSink() (string, error) {
	cmd := exec.Command("pactl", "info")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run pactl: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Default Sink:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("could not find default sink")
}

// float32frombits converts uint32 bits to float32
func float32frombits(b uint32) float32 {
	return *(*float32)(unsafe.Pointer(&b))
}
