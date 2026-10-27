package state

// ReportRequest is what the user asked the printable diff report for.
type ReportRequest int

// The requests the diff report can be opened with.
const (
	ReportRequestNone ReportRequest = iota
	ReportRequestPlain
	ReportRequestExplained
)

// ReportPhaseKind is the screen state of the diff report. The Go screen state derives it from the request, the
// answer of the agent and the error of the send, and holds the content of a complete report back as Preparing
// until it is rendered.
type ReportPhaseKind int

// The phases of the printable diff report.
const (
	ReportPhaseIdle ReportPhaseKind = iota
	ReportPhasePreparing
	ReportPhasePlain
	ReportPhaseWaiting
	ReportPhaseReady
	ReportPhaseFailed
)

// ReportPhase is what the screen must show for the current report request.
type ReportPhase struct {
	Kind ReportPhaseKind
	// Explanation is the agent answer, set when Kind is ReportPhaseReady.
	Explanation string
	// Error is why the report could not be rendered, set when Kind is ReportPhaseFailed.
	Error string
}

// IsWaiting reports whether the agent is still producing the explanation.
func (p ReportPhase) IsWaiting() bool { return p.Kind == ReportPhaseWaiting }

// IsPreparing reports whether the content of the report is complete but not rendered yet.
func (p ReportPhase) IsPreparing() bool { return p.Kind == ReportPhasePreparing }

// IsBusy reports whether the progress dialog must be shown: waiting for the agent or preparing the render.
func (p ReportPhase) IsBusy() bool { return p.IsWaiting() || p.IsPreparing() }

// HasReport reports whether the content of the report is complete and can be rendered: the plain report and the
// explained one that holds an answer.
func (p ReportPhase) HasReport() bool {
	return p.Kind == ReportPhasePlain || p.Kind == ReportPhaseReady
}

// GateRender holds a complete report back as Preparing until it is rendered. The server renders as soon as it can,
// so a rendered content passes the gate.
func GateRender(phase ReportPhase, rendered bool) ReportPhase {
	if phase.HasReport() && !rendered {
		return ReportPhase{Kind: ReportPhasePreparing}
	}
	return phase
}

// PhaseFor maps a report request, the agent answer found in the transcript and the error of the send to the phase
// to render. An answer found for a request wins over a send error, and the plain report ignores both.
func PhaseFor(request ReportRequest, explanation *string, sendError *string) ReportPhase {
	switch request {
	case ReportRequestPlain:
		return ReportPhase{Kind: ReportPhasePlain}
	case ReportRequestExplained:
		if explanation != nil {
			return ReportPhase{Kind: ReportPhaseReady, Explanation: *explanation}
		}
		if sendError != nil {
			return ReportPhase{Kind: ReportPhaseFailed, Error: *sendError}
		}
		return ReportPhase{Kind: ReportPhaseWaiting}
	default:
		return ReportPhase{Kind: ReportPhaseIdle}
	}
}

// SendErrorSince returns the error of a send whose version is newer than baseline, or nil. The sends that ran at or
// before the baseline were superseded, so their results are discarded.
func SendErrorSince(version int, baseline int, err *string) *string {
	if version <= baseline {
		return nil
	}
	return err
}

// DiffReport is the screen state of the printable diff report of a task: what was asked for, and the conversation
// that answers it. The phase is derived from the request, the answer found in the transcript and the error of the
// send that asked for it.
type DiffReport struct {
	TaskID string
	// Request is what the user asked for; ReportRequestNone means no report is open.
	Request ReportRequest
	// Since is the length of the transcript when the explanation was requested; the answers before it are ignored.
	Since int
	// Transcript is the conversation of the task the explanation is searched in.
	Transcript Transcript
	// Version is the version of the send the report reads, as the Rust version reads the action it waits on.
	Version int
	// Baseline is the version of the send that asked for the explanation; the results before it are discarded.
	Baseline int
	// SendError is the error of the send that asked for the explanation, when it failed.
	SendError *string
}

// Phase returns the phase the screen must show: the content of a complete report passes the render gate, because
// the server renders as soon as it can.
func (r DiffReport) Phase() ReportPhase {
	return GateRender(r.contentPhase(), true)
}

// contentPhase derives the phase from the request, the answer and the error of the send.
func (r DiffReport) contentPhase() ReportPhase {
	switch r.Request {
	case ReportRequestPlain:
		return ReportPhase{Kind: ReportPhasePlain}
	case ReportRequestExplained:
		if answer, ok := FindExplanation(r.Transcript, ExplanationPrompt, r.Since); ok {
			return ReportPhase{Kind: ReportPhaseReady, Explanation: answer}
		}
		if err := SendErrorSince(r.Version, r.Baseline, r.SendError); err != nil {
			return ReportPhase{Kind: ReportPhaseFailed, Error: *err}
		}
		return ReportPhase{Kind: ReportPhaseWaiting}
	default:
		return PhaseFor(ReportRequestNone, nil, nil)
	}
}

// OpenPlain opens the plain report, behind the progress dialog of the server render.
func (r DiffReport) OpenPlain() DiffReport {
	r.Request = ReportRequestPlain
	return r
}

// Close closes the report or stops waiting for the agent.
func (r DiffReport) Close() DiffReport {
	r.Request = ReportRequestNone
	r.SendError = nil
	return r
}

// RequestExplanation asks the agent to explain the changes and waits for its answer. The answers before the
// current conversation are ignored, as the answers of the previous request were.
func (r DiffReport) RequestExplanation(transcript Transcript) DiffReport {
	r.Request = ReportRequestExplained
	r.Since = len(transcript.Items)
	r.Baseline = r.Version
	r.SendError = nil
	return r
}

// WithSendError records the error of the send that asked for the explanation, so the report fails with it.
func (r DiffReport) WithSendError(err string) DiffReport {
	r.SendError = &err
	return r
}
