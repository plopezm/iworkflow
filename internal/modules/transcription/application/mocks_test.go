package application_test

import (
	"context"
	"encoding/binary"
)

// fakeRecognizer is a Recognizer that answers a fixed text or error and records what it received.
type fakeRecognizer struct {
	text    string
	err     error
	calls   int
	samples []float32
	rate    int
}

func (f *fakeRecognizer) Recognize(_ context.Context, samples []float32, sampleRate int) (string, error) {
	f.calls++
	f.samples = samples
	f.rate = sampleRate
	return f.text, f.err
}

// validWAV is 0.1 s of silence as mono 16 kHz 16-bit PCM.
func validWAV() []byte {
	const count = 1600
	data := make([]byte, count*2)
	out := []byte("RIFF")
	out = binary.LittleEndian.AppendUint32(out, uint32(36+len(data)))
	out = append(out, "WAVEfmt "...)
	out = binary.LittleEndian.AppendUint32(out, 16)
	out = binary.LittleEndian.AppendUint16(out, 1)
	out = binary.LittleEndian.AppendUint16(out, 1)
	out = binary.LittleEndian.AppendUint32(out, 16000)
	out = binary.LittleEndian.AppendUint32(out, 32000)
	out = binary.LittleEndian.AppendUint16(out, 2)
	out = binary.LittleEndian.AppendUint16(out, 16)
	out = append(out, "data"...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(data)))
	return append(out, data...)
}
