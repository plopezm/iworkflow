package components

// ProgressStep is one line of the submit progress panel. Phase groups the steps a script can hide: a step with the phase
// "start" is dropped when the form does not start the task. An empty Phase is always shown.
type ProgressStep struct {
	Label string
	Phase string
}

// SubmitProgressProps configures the panel shown over a form while its request waits for the server.
type SubmitProgressProps struct {
	// Title is what the server is doing, in one short sentence.
	Title string
	// Steps are the stages of the wait, in order. They show an estimate: the server answers once, at the end.
	Steps []ProgressStep
	// SlowHint is shown once the wait is longer than usual.
	SlowHint string
}
