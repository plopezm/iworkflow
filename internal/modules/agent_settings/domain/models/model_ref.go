package models

// ModelRef is an orchestrator model, selectable through `--provider`/`--model` of pi.
type ModelRef struct {
	Provider string
	Model    string
}
