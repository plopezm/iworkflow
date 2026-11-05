// Package components holds the templ components of the data_home module. They render the props they
// receive and contain no logic: the state package builds the props, and every text is already
// translated there.
package components

// DataHomeID is the element id of the card; the htmx requests swap the whole card.
const DataHomeID = "data-home-card"

// DataHomePath is the URL that loads the card. A restarting card polls it.
const DataHomePath = "/data-home/card"

// Routes the card controls post to. They are part of the contract with the inbound handlers.
const (
	dataHomeMovePath   = "/data-home/move"
	dataHomeCancelPath = "/data-home/cancel"
	// restartPollInterval is how often a restarting card asks whether the server is back.
	restartPollInterval = "every 2s"
)

// DataHomeCardProps configures the data folder card. Error replaces the whole card when the folder
// cannot be read. EnvLocked makes the card read-only and EnvNote explains why. The other texts are
// shown only when set; Notice reports a refused request above the controls.
type DataHomeCardProps struct {
	Loaded                 bool
	Title                  string
	Help                   string
	Error                  string
	Current                string
	CurrentLabel           string
	SourceText             string
	EnvLocked              bool
	EnvNote                string
	Notice                 string
	Restarting             bool
	RestartingText         string
	DestinationLabel       string
	DestinationPlaceholder string
	MoveLabel              string
	PendingText            string
	CancelLabel            string
	LastText               string
	LastFailed             bool
	Warnings               []string
	WarningsLabel          string
}
