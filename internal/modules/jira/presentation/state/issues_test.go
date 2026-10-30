package state_test

import (
	"testing"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/state"
)

func TestCanSaveLane(t *testing.T) {
	if state.CanSaveLane("", "") {
		t.Errorf("empty name and jql should not be savable")
	}
	if state.CanSaveLane("My Lane", "   ") {
		t.Errorf("blank jql should not be savable")
	}
	if state.CanSaveLane("  ", "project = ABC") {
		t.Errorf("blank name should not be savable")
	}
	if !state.CanSaveLane("My Lane", "project = ABC") {
		t.Errorf("valid name and jql should be savable")
	}
}

func TestBuildFilter(t *testing.T) {
	f := state.BuildFilter("  search text  ", " In Progress ", "new")
	if f.Search != "search text" {
		t.Errorf("expected trimmed search, got %q", f.Search)
	}
	if f.Status == nil || *f.Status != "In Progress" {
		t.Errorf("expected status 'In Progress', got %v", f.Status)
	}
	if f.LocalState == nil || *f.LocalState != "new" {
		t.Errorf("expected local state 'new', got %v", f.LocalState)
	}

	blank := state.BuildFilter("", "", "")
	if blank.Search != "" {
		t.Errorf("expected empty search, got %q", blank.Search)
	}
	if blank.Status != nil {
		t.Errorf("expected nil status, got %v", blank.Status)
	}
	if blank.LocalState != nil {
		t.Errorf("expected nil local state, got %v", blank.LocalState)
	}
}

func TestFilterIssues(t *testing.T) {
	issues := []dto.IssueDTO{
		{
			Key:         "ABC-1",
			Summary:     "Falla el login",
			Description: "Error al entrar",
			Status:      "To Do",
			LocalState:  "new",
			Assignee:    stringPtr("Ana"),
		},
		{
			Key:         "ABC-2",
			Summary:     "Mejorar onboarding",
			Description: "Pantallas iniciales",
			Status:      "In Progress",
			LocalState:  "converted",
			Assignee:    stringPtr("Carlos"),
		},
		{
			Key:         "ABC-3",
			Summary:     "Documentar API",
			Description: "Swagger y OpenAPI",
			Status:      "Done",
			LocalState:  "ignored",
			Assignee:    nil,
		},
	}

	// 1. Sin filtros
	all := state.FilterIssues(issues, dto.IssueFilterDTO{})
	if len(all) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(all))
	}

	// 2. Filtro de búsqueda por texto en Summary
	bySummary := state.FilterIssues(issues, dto.IssueFilterDTO{Search: "login"})
	if len(bySummary) != 1 || bySummary[0].Key != "ABC-1" {
		t.Errorf("expected ABC-1 by summary, got %v", bySummary)
	}

	// 3. Filtro de búsqueda por Clave
	byKey := state.FilterIssues(issues, dto.IssueFilterDTO{Search: "abc-2"})
	if len(byKey) != 1 || byKey[0].Key != "ABC-2" {
		t.Errorf("expected ABC-2 by key, got %v", byKey)
	}

	// 4. Filtro por Estado de Jira
	statusProgress := "In Progress"
	byStatus := state.FilterIssues(issues, dto.IssueFilterDTO{Status: &statusProgress})
	if len(byStatus) != 1 || byStatus[0].Key != "ABC-2" {
		t.Errorf("expected ABC-2 by status, got %v", byStatus)
	}

	// 5. Filtro por Estado Local
	localIgnored := "ignored"
	byLocal := state.FilterIssues(issues, dto.IssueFilterDTO{LocalState: &localIgnored})
	if len(byLocal) != 1 || byLocal[0].Key != "ABC-3" {
		t.Errorf("expected ABC-3 by local state, got %v", byLocal)
	}
}

func TestSyncMessageFormatting(t *testing.T) {
	emptyOutcome := dto.SyncOutcomeDTO{Total: 0, Added: 0, FailedLanes: nil}
	if msg := state.SyncMessage(emptyOutcome, i18n.Es); msg != "Sincronizado: Jira no devolvió ninguna tarea." {
		t.Errorf("unexpected empty sync message in Es: %q", msg)
	}
	if msg := state.SyncMessage(emptyOutcome, i18n.En); msg != "Synced: Jira returned no issues." {
		t.Errorf("unexpected empty sync message in En: %q", msg)
	}

	unchangedOutcome := dto.SyncOutcomeDTO{Total: 3, Added: 0, FailedLanes: nil}
	if msg := state.SyncMessage(unchangedOutcome, i18n.Es); msg != "Sincronizado: 3 tareas (sin novedades)." {
		t.Errorf("unexpected unchanged sync message: %q", msg)
	}

	newOutcome := dto.SyncOutcomeDTO{Total: 5, Added: 2, FailedLanes: nil}
	if msg := state.SyncMessage(newOutcome, i18n.Es); msg != "Sincronizado: 5 tareas (2 nuevas)." {
		t.Errorf("unexpected new sync message: %q", msg)
	}

	failedLaneOutcome := dto.SyncOutcomeDTO{Total: 4, Added: 1, FailedLanes: []string{"lane-1"}}
	if msg := state.SyncMessage(failedLaneOutcome, i18n.Es); msg != "Sincronizado: 4 tareas (1 nuevas). Un carril falló." {
		t.Errorf("unexpected 1 failed lane message: %q", msg)
	}

	multipleFailedOutcome := dto.SyncOutcomeDTO{Total: 4, Added: 0, FailedLanes: []string{"lane-1", "lane-2"}}
	if msg := state.SyncMessage(multipleFailedOutcome, i18n.Es); msg != "Sincronizado: 4 tareas (sin novedades). 2 carriles fallaron." {
		t.Errorf("unexpected multiple failed lanes message: %q", msg)
	}
}

func TestLocalStateLabelAndVariant(t *testing.T) {
	if state.LocalStateLabel("new", i18n.Es) != "Nueva" {
		t.Errorf("expected 'Nueva', got %q", state.LocalStateLabel("new", i18n.Es))
	}
	if state.LocalStateLabel("converted", i18n.Es) != "Convertida" {
		t.Errorf("expected 'Convertida', got %q", state.LocalStateLabel("converted", i18n.Es))
	}
	if state.LocalStateLabel("ignored", i18n.Es) != "Ignorada" {
		t.Errorf("expected 'Ignorada', got %q", state.LocalStateLabel("ignored", i18n.Es))
	}

	if state.LocalStateVariant("new") != "info" {
		t.Errorf("expected 'info', got %q", state.LocalStateVariant("new"))
	}
	if state.LocalStateVariant("converted") != "ok" {
		t.Errorf("expected 'ok', got %q", state.LocalStateVariant("converted"))
	}
	if state.LocalStateVariant("ignored") != "muted" {
		t.Errorf("expected 'muted', got %q", state.LocalStateVariant("ignored"))
	}
}

func TestLaneCountLabel(t *testing.T) {
	if state.LaneCountLabel(3, 3, i18n.Es) != "3" {
		t.Errorf("expected '3', got %q", state.LaneCountLabel(3, 3, i18n.Es))
	}
	if state.LaneCountLabel(2, 5, i18n.Es) != "2 de 5" {
		t.Errorf("expected '2 de 5', got %q", state.LaneCountLabel(2, 5, i18n.Es))
	}
	if state.LaneCountLabel(2, 5, i18n.En) != "2 of 5" {
		t.Errorf("expected '2 of 5', got %q", state.LaneCountLabel(2, 5, i18n.En))
	}
}

func stringPtr(s string) *string {
	return &s
}
