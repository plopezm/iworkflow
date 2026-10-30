package inbound

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

// maxImportBytes bounds an uploaded import file. A profile is a few kilobytes; the limit only guards
// against an accidental huge file.
const maxImportBytes = 1 << 20

// rowsFromForm reads the routing rows of the editor. The editor sends one agent, model and thinking
// field per row, in the same order, so the i-th value of each belongs to the same row.
func rowsFromForm(r *http.Request) []state.RoutingRow {
	agents := r.PostForm["agent"]
	models := r.PostForm["model"]
	thinking := r.PostForm["thinking"]
	rows := make([]state.RoutingRow, 0, len(agents))
	for index, agent := range agents {
		row := state.RoutingRow{Agent: agent}
		if index < len(models) {
			row.Model = models[index]
		}
		if index < len(thinking) {
			row.Thinking = thinking[index]
		}
		rows = append(rows, row)
	}
	return rows
}

// applyRowAction applies the row action the editor sent. Add takes the agent from the "add_agent"
// select; remove takes it from "target".
func applyRowAction(r *http.Request, rows []state.RoutingRow) []state.RoutingRow {
	switch r.PostFormValue("action") {
	case state.ActionAddRow:
		return state.ApplyRowAction(rows, state.ActionAddRow, r.PostFormValue("add_agent"))
	case state.ActionRemoveRow:
		return state.ApplyRowAction(rows, state.ActionRemoveRow, r.PostFormValue("target"))
	default:
		return rows
	}
}

// importInput is the content of an import request: the uploaded file when there is one, else the
// pasted text, and the optional new name.
type importInput struct {
	Raw      string
	RenameTo string
}

// readImportForm reads a multipart import form (the dialog sends multipart so the file can travel).
// A plain form is accepted too, with the pasted text only.
func readImportForm(r *http.Request) (importInput, error) {
	if err := r.ParseMultipartForm(maxImportBytes); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		return importInput{}, fmt.Errorf("read import form: %w", err)
	}
	input := importInput{Raw: r.FormValue("raw"), RenameTo: r.FormValue("rename_to")}
	if r.MultipartForm == nil {
		return input, nil
	}
	// Browsers send the file part even when no file was chosen: only a named file replaces the text.
	headers := r.MultipartForm.File["file"]
	if len(headers) == 0 || headers[0].Filename == "" {
		return input, nil
	}
	file, err := headers[0].Open()
	if err != nil {
		return importInput{}, fmt.Errorf("open import file: %w", err)
	}
	defer func() { _ = file.Close() }() // read-only file: a close error changes nothing
	content, err := io.ReadAll(io.LimitReader(file, maxImportBytes))
	if err != nil {
		return importInput{}, fmt.Errorf("read import file: %w", err)
	}
	input.Raw = string(content)
	return input, nil
}
