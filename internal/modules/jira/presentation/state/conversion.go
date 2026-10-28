package state

import (
	"strings"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

// ConvertIssueView holds the state needed by the Jira issue conversion page.
type ConvertIssueView struct {
	Preparation dto.ConversionPreparationDTO
	Branches    []dto.ConversionBranchDTO
	Converted   *dto.ConvertedIssueDTO
	Error       string
}

// FormatBranch formats the branch name using this view's helper.
func (v ConvertIssueView) FormatBranch(branch dto.ConversionBranchDTO, locale i18n.Locale) string {
	return FormatBranch(branch, locale)
}

var msgRemoteBranch = i18n.Msg{
	En: "{name} (remote)",
	Es: "{name} (remota)",
	Zh: "{name}（远程）",
	De: "{name} (Remote)",
	Fr: "{name} (distante)",
}

// FormatBranch returns the display string for a branch: `{name} (remota)` if remote, or just `{name}`.
func FormatBranch(branch dto.ConversionBranchDTO, locale i18n.Locale) string {
	if !branch.IsRemote {
		return branch.Name
	}
	text := msgRemoteBranch.In(locale)
	return strings.ReplaceAll(text, "{name}", branch.Name)
}

// PriorityOption represents a selectable priority in the conversion form.
type PriorityOption struct {
	Value string
	Label i18n.Msg
}

// LabelText returns the localized label of the priority option.
func (p PriorityOption) LabelText(locale i18n.Locale) string {
	return p.Label.In(locale)
}

var (
	msgPriorityLow = i18n.Msg{
		En: "Low",
		Es: "Baja",
		Zh: "低",
		De: "Niedrig",
		Fr: "Basse",
	}
	msgPriorityNormal = i18n.Msg{
		En: "Normal",
		Es: "Normal",
		Zh: "普通",
		De: "Normal",
		Fr: "Normale",
	}
	msgPriorityHigh = i18n.Msg{
		En: "High",
		Es: "Alta",
		Zh: "高",
		De: "Hoch",
		Fr: "Haute",
	}
	msgPriorityUrgent = i18n.Msg{
		En: "Urgent",
		Es: "Urgente",
		Zh: "紧急",
		De: "Dringend",
		Fr: "Urgente",
	}
)

var priorityOptions = []PriorityOption{
	{Value: "low", Label: msgPriorityLow},
	{Value: "normal", Label: msgPriorityNormal},
	{Value: "high", Label: msgPriorityHigh},
	{Value: "urgent", Label: msgPriorityUrgent},
}

// PriorityOptions returns the list of supported task priorities for conversion.
func PriorityOptions() []PriorityOption {
	return priorityOptions
}

// PriorityLabel returns the localized label for a given priority value.
func PriorityLabel(value string, locale i18n.Locale) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return msgPriorityLow.In(locale)
	case "high":
		return msgPriorityHigh.In(locale)
	case "urgent":
		return msgPriorityUrgent.In(locale)
	default:
		return msgPriorityNormal.In(locale)
	}
}
