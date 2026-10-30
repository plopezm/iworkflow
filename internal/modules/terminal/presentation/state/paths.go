package state

// TerminalsPath is the address of the sessions list.
const TerminalsPath string = "/terminals"

// SessionPath is the address of one session. It is the same path the Rust version used.
func SessionPath(id string) string {
	return TerminalsPath + "/" + id
}
