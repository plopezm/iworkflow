package models_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	transcriptionerrors "iworkflow/internal/modules/transcription/domain/errors"
	"iworkflow/internal/modules/transcription/domain/models"
)

// wav builds a RIFF/WAVE file with the given format and 16-bit samples.
func wav(format, channels uint16, rate uint32, bits uint16, samples []int16) []byte {
	data := make([]byte, 0, len(samples)*2)
	for _, s := range samples {
		data = binary.LittleEndian.AppendUint16(data, uint16(s))
	}
	out := []byte("RIFF")
	out = binary.LittleEndian.AppendUint32(out, uint32(36+len(data)))
	out = append(out, "WAVEfmt "...)
	out = binary.LittleEndian.AppendUint32(out, 16)
	out = binary.LittleEndian.AppendUint16(out, format)
	out = binary.LittleEndian.AppendUint16(out, channels)
	out = binary.LittleEndian.AppendUint32(out, rate)
	out = binary.LittleEndian.AppendUint32(out, rate*uint32(channels)*uint32(bits)/8)
	out = binary.LittleEndian.AppendUint16(out, channels*bits/8)
	out = binary.LittleEndian.AppendUint16(out, bits)
	out = append(out, "data"...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(data)))
	return append(out, data...)
}

func TestParseWAVNormalizesSamplesToFloats(t *testing.T) {
	audio, err := models.ParseWAV(wav(1, 1, 16000, 16, []int16{0, 16384, -16384, 32767, -32768}))

	require.NoError(t, err)
	require.Equal(t, models.SampleRate, audio.SampleRate())
	require.InDeltaSlice(t, []float32{0, 0.5, -0.5, 0.99997, -1}, audio.Samples(), 1e-4)
}

func TestParseWAVSamplesAreACopy(t *testing.T) {
	audio, err := models.ParseWAV(wav(1, 1, 16000, 16, []int16{16384}))
	require.NoError(t, err)

	audio.Samples()[0] = 9

	require.InDelta(t, 0.5, audio.Samples()[0], 1e-6)
}

func TestParseWAVSkipsUnknownChunksBeforeData(t *testing.T) {
	plain := wav(1, 1, 16000, 16, []int16{16384})
	// Insert a LIST chunk between fmt and data.
	extra := append([]byte("LIST"), 3, 0, 0, 0, 'a', 'b', 'c', 0) // odd size: padded to even
	withList := append(append(append([]byte{}, plain[:36]...), extra...), plain[36:]...)

	audio, err := models.ParseWAV(withList)

	require.NoError(t, err)
	require.Len(t, audio.Samples(), 1)
}

func TestParseWAVRejectsWhatIsNotMono16kPCM16(t *testing.T) {
	good := []int16{1, 2, 3}
	tests := []struct {
		name string
		data []byte
	}{
		{"empty body", nil},
		{"not riff", []byte("this is not a wav file at all, just text")},
		{"truncated header", wav(1, 1, 16000, 16, good)[:30]},
		{"stereo", wav(1, 2, 16000, 16, good)},
		{"44.1 kHz", wav(1, 1, 44100, 16, good)},
		{"float format", wav(3, 1, 16000, 16, good)},
		{"8 bits", wav(1, 1, 16000, 8, good)},
		{"no samples", wav(1, 1, 16000, 16, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := models.ParseWAV(tt.data)

			require.ErrorIs(t, err, transcriptionerrors.ErrInvalidAudio)
		})
	}
}

func TestParseWAVRejectsAudioLongerThanTheLimit(t *testing.T) {
	tooLong := make([]int16, models.SampleRate*models.MaxSeconds+1)
	atLimit := make([]int16, models.SampleRate*models.MaxSeconds)

	_, err := models.ParseWAV(wav(1, 1, 16000, 16, tooLong))
	require.True(t, errors.Is(err, transcriptionerrors.ErrAudioTooLong))

	audio, err := models.ParseWAV(wav(1, 1, 16000, 16, atLimit))
	require.NoError(t, err)
	require.Len(t, audio.Samples(), len(atLimit))
}
