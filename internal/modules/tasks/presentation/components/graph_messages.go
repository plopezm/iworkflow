package components

import (
	"context"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Texts of the task graph presentation and loose tasks panel in 5 languages (en, es, zh, de, fr),
// mirroring the Rust Msg constants in src/modules/tasks/presentation/components/{graph_edge.rs, task_graph_canvas.rs, task_node.rs, loose_tasks_panel.rs, loose_task_card.rs}.

var (
	msgRemoveDependency = i18n.Msg{
		En: "Remove dependency",
		Es: "Quitar dependencia",
		Zh: "移除依赖",
		De: "Abhängigkeit entfernen",
		Fr: "Supprimer la dépendance",
	}

	msgAddNext = i18n.Msg{
		En: "Add next task",
		Es: "Añadir tarea siguiente",
		Zh: "添加后续任务",
		De: "Nächste Aufgabe hinzufügen",
		Fr: "Ajouter la tâche suivante",
	}

	msgAddParallel = i18n.Msg{
		En: "Add parallel task",
		Es: "Añadir tarea en paralelo",
		Zh: "添加并行任务",
		De: "Parallele Aufgabe hinzufügen",
		Fr: "Ajouter une tâche parallèle",
	}

	msgNextShort = i18n.Msg{
		En: "Next",
		Es: "Siguiente",
		Zh: "后续",
		De: "Nächste",
		Fr: "Suivante",
	}

	msgParallelShort = i18n.Msg{
		En: "Parallel",
		Es: "Paralela",
		Zh: "并行",
		De: "Parallel",
		Fr: "Parallèle",
	}

	msgDetachShort = i18n.Msg{
		En: "Remove from graph",
		Es: "Quitar del grafo",
		Zh: "移出图表",
		De: "Aus Graph entfernen",
		Fr: "Retirer du graphe",
	}

	msgDetachTitle = i18n.Msg{
		En: "Remove from graph: deletes all its connections and sends it back to the loose tasks",
		Es: "Quitar del grafo: elimina todas sus conexiones y la devuelve a las tareas sueltas",
		Zh: "移出图表：删除其所有连线，并将其放回零散任务",
		De: "Aus Graph entfernen: löscht alle Verbindungen und legt die Aufgabe zu den losen Aufgaben zurück",
		Fr: "Retirer du graphe : supprime toutes ses connexions et la renvoie aux tâches libres",
	}

	msgWaitingOne = i18n.Msg{
		En: "Waiting for 1 predecessor",
		Es: "Esperando 1 predecesora",
		Zh: "等待 1 个前置任务",
		De: "Wartet auf 1 Vorgänger",
		Fr: "En attente de 1 prédécesseur",
	}

	msgWaitingMany = i18n.Msg{
		En: "Waiting for {n} predecessors",
		Es: "Esperando {n} predecesoras",
		Zh: "等待 {n} 个前置任务",
		De: "Wartet auf {n} Vorgänger",
		Fr: "En attente de {n} prédécesseurs",
	}

	msgInputPort = i18n.Msg{
		En: "Input: drop a connection here",
		Es: "Entrada: suelta aquí una conexión",
		Zh: "输入：将连线拖到此处",
		De: "Eingang: Verbindung hier ablegen",
		Fr: "Entrée : déposez une connexion ici",
	}

	msgOutputPort = i18n.Msg{
		En: "Output: drag to another task to make it wait for this one",
		Es: "Salida: arrastra a otra tarea para que espere a esta",
		Zh: "输出：拖到另一个任务，让它等待此任务",
		De: "Ausgang: auf eine andere Aufgabe ziehen, damit sie auf diese wartet",
		Fr: "Sortie : glissez vers une autre tâche pour qu'elle attende celle-ci",
	}

	msgZoomIn = i18n.Msg{
		En: "Zoom in",
		Es: "Acercar",
		Zh: "放大",
		De: "Vergrößern",
		Fr: "Zoom avant",
	}

	msgZoomOut = i18n.Msg{
		En: "Zoom out",
		Es: "Alejar",
		Zh: "缩小",
		De: "Verkleinern",
		Fr: "Zoom arrière",
	}

	msgFitView = i18n.Msg{
		En: "Fit view",
		Es: "Ajustar vista",
		Zh: "适应视图",
		De: "Ansicht einpassen",
		Fr: "Ajuster la vue",
	}

	msgLooseTitle = i18n.Msg{
		En: "Loose tasks",
		Es: "Tareas sueltas",
		Zh: "零散任务",
		De: "Lose Aufgaben",
		Fr: "Tâches libres",
	}

	msgLooseExpand = i18n.Msg{
		En: "Show loose tasks",
		Es: "Mostrar tareas sueltas",
		Zh: "显示零散任务",
		De: "Lose Aufgaben anzeigen",
		Fr: "Afficher les tâches libres",
	}

	msgLooseCollapse = i18n.Msg{
		En: "Hide loose tasks",
		Es: "Ocultar tareas sueltas",
		Zh: "隐藏零散任务",
		De: "Lose Aufgaben ausblenden",
		Fr: "Masquer les tâches libres",
	}

	msgLooseEmptyTitle = i18n.Msg{
		En: "No loose tasks",
		Es: "No hay tareas sueltas",
		Zh: "没有零散任务",
		De: "Keine losen Aufgaben",
		Fr: "Aucune tâche libre",
	}

	msgLooseEmptyHint = i18n.Msg{
		En: "Tasks created with “New task” show up here. Run them on their own or drag them onto the canvas to chain them.",
		Es: "Las tareas creadas con «Nueva tarea» aparecen aquí. Ejecútalas por separado o arrástralas al lienzo para encadenarlas.",
		Zh: "通过“新建任务”创建的任务会显示在这里。可单独运行，或拖到画布上进行串联。",
		De: "Mit „Neue Aufgabe“ erstellte Aufgaben erscheinen hier. Führe sie einzeln aus oder ziehe sie auf die Zeichenfläche, um sie zu verketten.",
		Fr: "Les tâches créées avec « Nouvelle tâche » apparaissent ici. Exécutez-les seules ou glissez-les sur le canevas pour les enchaîner.",
	}

	msgAddToGraph = i18n.Msg{
		En: "Add to graph",
		Es: "Añadir al grafo",
		Zh: "添加到图",
		De: "Zum Graph hinzufügen",
		Fr: "Ajouter au graphe",
	}

	msgDragHint = i18n.Msg{
		En: "Drag onto the canvas to add it to the graph",
		Es: "Arrastra al lienzo para añadirla al grafo",
		Zh: "拖到画布上即可添加到图中",
		De: "Auf die Zeichenfläche ziehen, um sie zum Graph hinzuzufügen",
		Fr: "Glissez sur le canevas pour l'ajouter au graphe",
	}

	msgPendingDialogOne = i18n.Msg{
		En: "1 dialog",
		Es: "1 diálogo",
		Zh: "1 个对话框",
		De: "1 Dialog",
		Fr: "1 dialogue",
	}

	msgPendingDialogsMany = i18n.Msg{
		En: "{count} dialogs",
		Es: "{count} diálogos",
		Zh: "{count} 个对话框",
		De: "{count} Dialoge",
		Fr: "{count} dialogues",
	}

	msgTabLoose = i18n.Msg{
		En: "Loose ({n})",
		Es: "Sueltas ({n})",
		Zh: "零散 ({n})",
		De: "Lose ({n})",
		Fr: "Libres ({n})",
	}

	msgTabGraph = i18n.Msg{
		En: "Graph",
		Es: "Grafo",
		Zh: "图",
		De: "Graph",
		Fr: "Graphe",
	}

	msgTabsLabel = i18n.Msg{
		En: "Board view",
		Es: "Vista del tablero",
		Zh: "看板视图",
		De: "Board-Ansicht",
		Fr: "Vue du tableau",
	}
)

// TabOptions returns the sub-tab options for narrow screens: loose tasks and graph.
func TabOptions(ctx context.Context, looseCount int) []shared.SegmentOption {
	return []shared.SegmentOption{
		{
			Value: string(state.BoardTabLoose),
			Label: msgTabLoose.Fmt(ctx, map[string]string{"n": strconv.Itoa(looseCount)}),
			Attrs: templ.Attributes{
				"data-board-tab": string(state.BoardTabLoose),
			},
		},
		{
			Value: string(state.BoardTabGraph),
			Label: msgTabGraph.T(ctx),
			Attrs: templ.Attributes{
				"data-board-tab": string(state.BoardTabGraph),
			},
		},
	}
}

// WaitingLabel formats the predecessor waiting badge label. Returns nil if waiting <= 0.
func WaitingLabel(ctx context.Context, waiting int) *string {
	if waiting <= 0 {
		return nil
	}
	locale := i18n.En
	if ctx != nil {
		locale = i18n.StateFrom(ctx).Locale
	}
	if waiting == 1 {
		text := msgWaitingOne.In(locale)
		return &text
	}
	text := msgWaitingMany.In(locale)
	text = strings.ReplaceAll(text, "{n}", strconv.Itoa(waiting))
	return &text
}

func pendingDialogsLabel(ctx context.Context, count int) string {
	if count == 1 {
		return msgPendingDialogOne.T(ctx)
	}
	return msgPendingDialogsMany.Fmt(ctx, map[string]string{"count": strconv.Itoa(count)})
}

func priorityVariant(priority string) string {
	switch priority {
	case "low":
		return "muted"
	case "high":
		return "warn"
	case "urgent":
		return "error"
	default:
		return "muted"
	}
}
