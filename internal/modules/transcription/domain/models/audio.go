// Package models holds the entities and value objects of the transcription module.
package models

import (
	"encoding/binary"
	"fmt"

	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
)

const (
	// SampleRate is the only sample rate accepted: the clients resample before sending.
	SampleRate int = 16000
	// MaxSeconds is the longest recording accepted.
	MaxSeconds int = 60

	pcmFormat      uint16 = 1
	bitsPerSample  uint16 = 16
	riffHeaderLen         = 12
	chunkHeaderLen        = 8
	fmtChunkMin           = 16
)

// Audio is a validated mono recording at SampleRate, as floats in [-1, 1].
type Audio struct {
	samples []float32
}

// SampleRate is the sample rate of the audio.
func (Audio) SampleRate() int { return SampleRate }

// Samples returns a copy of the samples, so callers cannot change the audio.
func (a Audio) Samples() []float32 {
	return append([]float32(nil), a.samples...)
}

// ParseWAV validates a RIFF/WAVE file (mono, 16 kHz, 16-bit PCM, 1 sample to MaxSeconds) and decodes its samples.
// Chunks other than "fmt " and "data" are skipped. A data size larger than the body (a streamed file) is clamped.
func ParseWAV(data []byte) (Audio, error) {
	if len(data) < riffHeaderLen || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return Audio{}, fmt.Errorf("not a RIFF/WAVE file: %w", transcriptionerrors.ErrInvalidAudio)
	}
	var formatOK bool
	rest := data[riffHeaderLen:]
	for len(rest) >= chunkHeaderLen {
		id := string(rest[0:4])
		size := int(min(uint64(binary.LittleEndian.Uint32(rest[4:8])), uint64(len(rest)-chunkHeaderLen)))
		body := rest[chunkHeaderLen : chunkHeaderLen+size]
		switch id {
		case "fmt ":
			if err := checkFormat(body); err != nil {
				return Audio{}, err
			}
			formatOK = true
		case "data":
			if !formatOK {
				return Audio{}, fmt.Errorf("data before fmt chunk: %w", transcriptionerrors.ErrInvalidAudio)
			}
			return decodeSamples(body)
		}
		advance := chunkHeaderLen + size + size%2 // chunks are padded to an even size
		if advance >= len(rest) {
			break
		}
		rest = rest[advance:]
	}
	return Audio{}, fmt.Errorf("no data chunk: %w", transcriptionerrors.ErrInvalidAudio)
}

func checkFormat(body []byte) error {
	if len(body) < fmtChunkMin {
		return fmt.Errorf("short fmt chunk: %w", transcriptionerrors.ErrInvalidAudio)
	}
	format := binary.LittleEndian.Uint16(body[0:2])
	channels := binary.LittleEndian.Uint16(body[2:4])
	rate := binary.LittleEndian.Uint32(body[4:8])
	bits := binary.LittleEndian.Uint16(body[14:16])
	if format != pcmFormat || channels != 1 || int(rate) != SampleRate || bits != bitsPerSample {
		return fmt.Errorf("format %d, %d channels, %d Hz, %d bits: %w", format, channels, rate, bits, transcriptionerrors.ErrInvalidAudio)
	}
	return nil
}

func decodeSamples(body []byte) (Audio, error) {
	count := len(body) / 2
	if count == 0 {
		return Audio{}, fmt.Errorf("no samples: %w", transcriptionerrors.ErrInvalidAudio)
	}
	if count > SampleRate*MaxSeconds {
		return Audio{}, fmt.Errorf("%d samples: %w", count, transcriptionerrors.ErrAudioTooLong)
	}
	samples := make([]float32, count)
	for i := range samples {
		samples[i] = float32(int16(binary.LittleEndian.Uint16(body[i*2:]))) / 32768
	}
	return Audio{samples: samples}, nil
}
