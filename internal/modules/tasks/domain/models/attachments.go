package models

// StoredAttachment is the metadata of an image stored for a task. The bytes live in the attachment storage.
type StoredAttachment struct {
	ID        string
	Name      string
	MimeType  string
	Path      string
	SizeBytes uint64
}

// TaskAttachmentContent is the content of a stored image, ready to be served.
type TaskAttachmentContent struct {
	MimeType string
	Bytes    []byte
}

// TaskFileContent is the text of a file inside a task worktree.
type TaskFileContent struct {
	Path    string
	Content string
}

// MaxTaskFileBytes is the largest worktree file that can be read or saved from the diff view: 1 MiB.
const MaxTaskFileBytes = 1024 * 1024
