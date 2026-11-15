// Package dto holds the plain data the usage_limits HTTP adapters exchange. It keeps the domain models
// out of the presentation layer.
package dto

import (
	"time"

	"iworkflow/internal/modules/usage_limits/domain/models"
)

// WindowDTO is one rate-limit window as shown to the user.
type WindowDTO struct {
	Name    string
	UsedPct float64
	Level   string
	// ResetsAt is the zero time when the provider does not say when the window renews.
	ResetsAt time.Time
}

// ProviderDTO is one provider that answered, with its windows.
type ProviderDTO struct {
	ID      string
	Stale   bool
	Windows []WindowDTO
}

// ReportDTO is the outcome of asking every provider.
type ReportDTO struct {
	Providers   []ProviderDTO
	Unavailable []string
}

// FromReport maps the domain report to its DTO.
func FromReport(report models.Report) ReportDTO {
	out := ReportDTO{}
	for _, usage := range report.Usages {
		provider := ProviderDTO{ID: string(usage.Provider()), Stale: usage.Stale()}
		for _, window := range usage.Windows() {
			provider.Windows = append(provider.Windows, WindowDTO{
				Name:     string(window.Name()),
				UsedPct:  window.UsedPct(),
				Level:    string(window.Level()),
				ResetsAt: window.ResetsAt(),
			})
		}
		out.Providers = append(out.Providers, provider)
	}
	for _, id := range report.Unavailable {
		out.Unavailable = append(out.Unavailable, string(id))
	}
	return out
}
