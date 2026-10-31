package sherpa

// HoldForTest takes the recognizer's slot, as a running transcription does, and returns the function that frees it.
func (r *Recognizer) HoldForTest() func() {
	r.slot <- struct{}{}
	return r.release
}
