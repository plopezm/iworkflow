package components

// CodeHostSettingsCardProps configures the settings card of one provider.
//
// LoadError, when set, replaces the form: the stored settings could not be read. SaveError is the
// message of a failed save; the form then keeps the URL the user typed (BaseURL, InsecureTLS). The
// token is never a prop: the field is always empty, and TokenPlaceholder says whether one is stored.
type CodeHostSettingsCardProps struct {
	ID                 string
	Title              string
	Help               string
	LoadError          string
	SaveError          string
	SaveURL            string
	TestURL            string
	TestResultID       string
	BaseURLLabel       string
	BaseURL            string
	BaseURLPlaceholder string
	TokenLabel         string
	TokenPlaceholder   string
	TokenConfigured    bool
	InsecureTLS        bool
	InsecureTLSLabel   string
	SaveLabel          string
	TestLabel          string
}

// ConnectionResultProps is the answer of the connection test: the authenticated user (OK) or the
// error message. Text is already translated.
type ConnectionResultProps struct {
	Text string
	OK   bool
}

// TaskMergeRequestPanelProps configures the merge request panel that a task screen embeds. The panel
// is a lazy container: it loads its body from MergeRequestPath(TaskID) when it appears, so the
// embedding screen needs no data of its own.
type TaskMergeRequestPanelProps struct {
	TaskID string
}

// TaskMergeRequestBodyProps is the content of the panel after a read or an action.
//
// Error and Notice are shown above the content. Create is set when the task has no request yet (the
// form that opens one); Status is set when it has one. Both are nil when the read failed.
type TaskMergeRequestBodyProps struct {
	TaskID       string
	Title        string
	RefreshLabel string
	Error        string
	Notice       string
	Create       *CreateRequestProps
	Status       *MergeRequestStatusProps
}

// CreateRequestProps configures the form that opens a merge request. Title keeps the typed title when
// the creation fails.
type CreateRequestProps struct {
	Intro      string
	TitleLabel string
	Title      string
	Note       string
	Action     string
}

// BadgeProps is a semantic label: Variant is one of ok, warn, error, info, muted.
type BadgeProps struct {
	Text    string
	Variant string
}

// PipelineProps is the latest CI run of a request with its jobs.
type PipelineProps struct {
	Title     string
	Status    BadgeProps
	WebURL    string
	OpenLabel string
	Jobs      []PipelineJobProps
}

// PipelineJobProps is one CI job.
type PipelineJobProps struct {
	Status     BadgeProps
	Name       string
	StageLabel string
	WebURL     string
	LogLabel   string
}

// MergeRequestStatusProps configures an existing request.
//
// Open enables the actions (merge and push). ShowMergeStatus is set for open requests only, as in the
// Rust version. Confirm replaces the merge button with the confirmation panel. NoPipeline is the text
// shown when Pipeline is nil.
type MergeRequestStatusProps struct {
	TaskID          string
	Heading         string
	Branches        string
	State           BadgeProps
	MergeStatus     BadgeProps
	ShowMergeStatus bool
	HasConflicts    bool
	ConflictLabel   string
	WebURL          string
	OpenLabel       string
	Pipeline        *PipelineProps
	NoPipeline      string
	Open            bool
	Confirm         bool
	ConfirmMessage  string
	MergeLabel      string
	MergeButton     string
	PushLabel       string
}
