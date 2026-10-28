package state

import (
	"fmt"
	"strings"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

var (
	msgSyncEmpty = i18n.Msg{
		En: "Synced: Jira returned no issues.",
		Es: "Sincronizado: Jira no devolvió ninguna tarea.",
		Zh: "已同步：Jira 没有返回任何任务。",
		De: "Synchronisiert: Jira hat keine Aufgaben zurückgegeben.",
		Fr: "Synchronisé : Jira n'a renvoyé aucune tâche.",
	}
	msgSyncUnchanged = i18n.Msg{
		En: "Synced: {total} issues (nothing new).",
		Es: "Sincronizado: {total} tareas (sin novedades).",
		Zh: "已同步：{total} 个任务（无新增）。",
		De: "Synchronisiert: {total} Aufgaben (keine Neuigkeiten).",
		Fr: "Synchronisé : {total} tâches (rien de nouveau).",
	}
	msgSyncWithNew = i18n.Msg{
		En: "Synced: {total} issues ({added} new).",
		Es: "Sincronizado: {total} tareas ({added} nuevas).",
		Zh: "已同步：{total} 个任务（{added} 个新增）。",
		De: "Synchronisiert: {total} Aufgaben ({added} neu).",
		Fr: "Synchronisé : {total} tâches ({added} nouvelles).",
	}
	msgSyncOneLaneFailed = i18n.Msg{
		En: "{base} One lane failed.",
		Es: "{base} Un carril falló.",
		Zh: "{base} 有 1 个泳道失败。",
		De: "{base} Eine Lane ist fehlgeschlagen.",
		Fr: "{base} Une voie a échoué.",
	}
	msgSyncLanesFailed = i18n.Msg{
		En: "{base} {n} lanes failed.",
		Es: "{base} {n} carriles fallaron.",
		Zh: "{base} 有 {n} 个泳道失败。",
		De: "{base} {n} Lanes sind fehlgeschlagen.",
		Fr: "{base} {n} voies ont échoué.",
	}

	msgStateNew = i18n.Msg{
		En: "New",
		Es: "Nueva",
		Zh: "新建",
		De: "Neu",
		Fr: "Nouvelle",
	}
	msgStateConverted = i18n.Msg{
		En: "Converted",
		Es: "Convertida",
		Zh: "已转换",
		De: "Konvertiert",
		Fr: "Convertie",
	}
	msgStateIgnored = i18n.Msg{
		En: "Ignored",
		Es: "Ignorada",
		Zh: "已忽略",
		De: "Ignoriert",
		Fr: "Ignorée",
	}

	msgCountFiltered = i18n.Msg{
		En: "{shown} of {total}",
		Es: "{shown} de {total}",
		Zh: "{shown} / {total}",
		De: "{shown} von {total}",
		Fr: "{shown} sur {total}",
	}

	msgEmptyLane = i18n.Msg{
		En: "No issues in this lane. Press “Sync” to fetch them from Jira.",
		Es: "Sin tareas en este carril. Pulsa «Sincronizar» para traerlas de Jira.",
		Zh: "此泳道中没有任务。点击“同步”从 Jira 获取。",
		De: "Keine Aufgaben in dieser Lane. Klicke auf „Synchronisieren“, um sie aus Jira zu laden.",
		Fr: "Aucune tâche dans cette voie. Cliquez sur « Synchroniser » pour les récupérer depuis Jira.",
	}
	msgEmptyFiltered = i18n.Msg{
		En: "No issue in this lane matches the filters.",
		Es: "Ninguna tarea de este carril coincide con los filtros.",
		Zh: "此泳道中没有符合筛选条件的任务。",
		De: "Keine Aufgabe in dieser Lane entspricht den Filtern.",
		Fr: "Aucune tâche de cette voie ne correspond aux filtres.",
	}

	msgLaneSyncFailed = i18n.Msg{
		En: "Lane “{lane}” was saved, but it could not be synced: {error}",
		Es: "El carril «{lane}» se guardó, pero no se pudo sincronizar: {error}",
		Zh: "泳道“{lane}”已保存，但无法同步：{error}",
		De: "Die Lane „{lane}“ wurde gespeichert, konnte aber nicht synchronisiert werden: {error}",
		Fr: "La voie « {lane} » a été enregistrée, mais n'a pas pu être synchronisée : {error}",
	}
)

// SyncMessage returns a human-readable summary of a sync outcome.
func SyncMessage(outcome dto.SyncOutcomeDTO, locale i18n.Locale) string {
	var base string
	switch {
	case outcome.Total == 0:
		base = msgSyncEmpty.In(locale)
	case outcome.Added == 0:
		base = strings.ReplaceAll(msgSyncUnchanged.In(locale), "{total}", fmt.Sprintf("%d", outcome.Total))
	default:
		base = strings.ReplaceAll(msgSyncWithNew.In(locale), "{total}", fmt.Sprintf("%d", outcome.Total))
		base = strings.ReplaceAll(base, "{added}", fmt.Sprintf("%d", outcome.Added))
	}

	switch len(outcome.FailedLanes) {
	case 0:
		return base
	case 1:
		return strings.ReplaceAll(msgSyncOneLaneFailed.In(locale), "{base}", base)
	default:
		msg := strings.ReplaceAll(msgSyncLanesFailed.In(locale), "{base}", base)
		return strings.ReplaceAll(msg, "{n}", fmt.Sprintf("%d", len(outcome.FailedLanes)))
	}
}

// LocalStateLabel returns the localized label of a local issue state ("new", "converted", "ignored").
func LocalStateLabel(state string, locale i18n.Locale) string {
	switch strings.ToLower(state) {
	case "converted":
		return msgStateConverted.In(locale)
	case "ignored":
		return msgStateIgnored.In(locale)
	default:
		return msgStateNew.In(locale)
	}
}

// LocalStateVariant returns the badge variant corresponding to the local state.
func LocalStateVariant(state string) string {
	switch strings.ToLower(state) {
	case "converted":
		return "ok"
	case "ignored":
		return "muted"
	default:
		return "info"
	}
}

// LaneCountLabel formats the issue count indicator (e.g. "3" or "2 de 5").
func LaneCountLabel(shown, total int, locale i18n.Locale) string {
	if shown == total {
		return fmt.Sprintf("%d", total)
	}
	text := strings.ReplaceAll(msgCountFiltered.In(locale), "{shown}", fmt.Sprintf("%d", shown))
	return strings.ReplaceAll(text, "{total}", fmt.Sprintf("%d", total))
}

// EmptyLaneMessage returns the message for an expanded lane with no visible issues.
func EmptyLaneMessage(total int, locale i18n.Locale) string {
	if total == 0 {
		return msgEmptyLane.In(locale)
	}
	return msgEmptyFiltered.In(locale)
}

// LaneSyncNotice returns a formatted warning when a lane failed its initial sync.
func LaneSyncNotice(saved dto.SavedLaneDTO, locale i18n.Locale) *string {
	if saved.SyncError == nil || *saved.SyncError == "" {
		return nil
	}
	text := strings.ReplaceAll(msgLaneSyncFailed.In(locale), "{lane}", saved.Lane.Name)
	text = strings.ReplaceAll(text, "{error}", *saved.SyncError)
	return &text
}
