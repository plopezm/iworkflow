package components

import "iworkflow/internal/core/i18n"

// Texts of the create dialog and the edit page, copied from the Rust Msg constants they replace
// (src/modules/tasks/presentation/components/task_modal.rs, pages/task_edit_page.rs, components/image_picker.rs,
// components/existing_attachments.rs).
var (
	msgRemoteBranch        = i18n.Msg{En: "{name} (remote)", Es: "{name} (remota)", Zh: "{name}（远程）", De: "{name} (remote)", Fr: "{name} (distante)"}
	msgBaseBranch          = i18n.Msg{En: "Base branch", Es: "Rama base", Zh: "基础分支", De: "Basis-Branch", Fr: "Branche de base"}
	msgRelatedProjects     = i18n.Msg{En: "Related projects", Es: "Proyectos relacionados", Zh: "关联项目", De: "Verwandte Projekte", Fr: "Projets liés"}
	msgRelatedProjectsHint = i18n.Msg{En: "The agent can read their files and, when available, their engram and codegraph. @ also completes their files.", Es: "El agente puede leer sus ficheros y, si están disponibles, su engram y su codegraph. @ también completa sus ficheros.", Zh: "智能体可以读取这些项目的文件，若可用还可读取其 engram 和 codegraph。输入 @ 时也会补全其文件。", De: "Der Agent kann deren Dateien lesen und, falls verfügbar, auch deren engram und codegraph. @ vervollständigt außerdem deren Dateien.", Fr: "L’agent peut lire leurs fichiers et, si disponibles, leur engram et leur codegraph. @ complète aussi leurs fichiers."}
	msgChooseProjects      = i18n.Msg{En: "Choose projects", Es: "Elegir proyectos", Zh: "选择项目", De: "Projekte wählen", Fr: "Choisir des projets"}
	msgFilterProjects      = i18n.Msg{En: "Filter projects", Es: "Filtrar proyectos", Zh: "筛选项目", De: "Projekte filtern", Fr: "Filtrer les projets"}
	msgNoProjects          = i18n.Msg{En: "No project matches.", Es: "Ningún proyecto coincide.", Zh: "没有匹配的项目。", De: "Kein Projekt passt.", Fr: "Aucun projet ne correspond."}
	msgDone                = i18n.Msg{En: "Done", Es: "Hecho", Zh: "完成", De: "Fertig", Fr: "Terminé"}
	msgWorkspaceBranch     = i18n.Msg{En: "Workspace branch ({branch})", Es: "Rama del workspace ({branch})", Zh: "工作区分支（{branch}）", De: "Branch des Workspace ({branch})", Fr: "Branche de l’espace de travail ({branch})"}
	msgDerivedFromPrompt   = i18n.Msg{En: "Derived from the prompt", Es: "Se deriva del prompt", Zh: "由提示词生成", De: "Wird aus dem Prompt abgeleitet", Fr: "Déduit du prompt"}
	msgAdvancedOptions     = i18n.Msg{En: "Advanced options", Es: "Opciones avanzadas", Zh: "高级选项", De: "Erweiterte Optionen", Fr: "Options avancées"}
	msgMaxRAM              = i18n.Msg{En: "Max. RAM", Es: "Memoria RAM máx.", Zh: "最大内存", De: "Max. RAM", Fr: "RAM max."}
	msgMaxCPU              = i18n.Msg{En: "Max. CPU", Es: "CPU máx.", Zh: "最大 CPU", De: "Max. CPU", Fr: "CPU max."}
	msgPercentOfCore       = i18n.Msg{En: "% of one core", Es: "% de un núcleo", Zh: "占一个核心的百分比", De: "% eines Kerns", Fr: "% d’un cœur"}
	msgDefaultFromSet      = i18n.Msg{En: "Default (settings)", Es: "Por defecto (ajustes)", Zh: "默认（设置）", De: "Standard (Einstellungen)", Fr: "Par défaut (paramètres)"}
	msgLinkFailedCreate    = i18n.Msg{En: "Task created, but its dependency could not be added: {error}", Es: "Tarea creada, pero no se pudo añadir su dependencia: {error}", Zh: "任务已创建，但无法添加其依赖：{error}", De: "Aufgabe erstellt, aber die Abhängigkeit konnte nicht hinzugefügt werden: {error}", Fr: "Tâche créée, mais sa dépendance n'a pas pu être ajoutée : {error}"}
	msgEditTask            = i18n.Msg{En: "Edit task", Es: "Editar tarea", Zh: "编辑任务", De: "Aufgabe bearbeiten", Fr: "Modifier la tâche"}
	msgTaskNotEditable     = i18n.Msg{En: "This task can no longer be edited.", Es: "Esta tarea ya no se puede editar.", Zh: "此任务无法再编辑。", De: "Diese Aufgabe kann nicht mehr bearbeitet werden.", Fr: "Cette tâche ne peut plus être modifiée."}
	msgAttachImages        = i18n.Msg{En: "Attach images (max. {mib} MiB each)", Es: "Adjuntar imágenes (máx. {mib} MiB cada una)", Zh: "附加图片（每张最大 {mib} MiB）", De: "Bilder anhängen (max. {mib} MiB pro Bild)", Fr: "Joindre des images ({mib} Mio max. chacune)"}
	msgAttach              = i18n.Msg{En: "Attach", Es: "Adjuntar", Zh: "附加", De: "Anhängen", Fr: "Joindre"}
	msgAttachedImages      = i18n.Msg{En: "Attached images", Es: "Imágenes adjuntas", Zh: "已附加的图片", De: "Angehängte Bilder", Fr: "Images jointes"}
	msgRemoveImage         = i18n.Msg{En: "Remove image {name}", Es: "Quitar imagen {name}", Zh: "移除图片 {name}", De: "Bild {name} entfernen", Fr: "Retirer l’image {name}"}
	msgStart               = i18n.Msg{En: "Start", Es: "Iniciar", Zh: "开始", De: "Starten", Fr: "Démarrer"}
	msgEdit                = i18n.Msg{En: "Edit", Es: "Editar", Zh: "编辑", De: "Bearbeiten", Fr: "Modifier"}
	msgDelete              = i18n.Msg{En: "Delete", Es: "Borrar", Zh: "删除", De: "Löschen", Fr: "Supprimer"}
	msgConfirmDelete       = i18n.Msg{En: "Delete this task from the pending list?", Es: "¿Borrar esta tarea de la lista de pendientes?", Zh: "要从待办列表中删除此任务吗？", De: "Diese Aufgabe aus der Liste der offenen Aufgaben löschen?", Fr: "Supprimer cette tâche de la liste des tâches en attente ?"}
	msgAgent               = i18n.Msg{En: "Agent", Es: "Agente", Zh: "智能体", De: "Agent", Fr: "Agent"}
	msgModel               = i18n.Msg{En: "Model", Es: "Modelo", Zh: "模型", De: "Modell", Fr: "Modèle"}
	msgThinking            = i18n.Msg{En: "Reasoning effort", Es: "Esfuerzo de razonamiento", Zh: "推理级别", De: "Denkaufwand", Fr: "Effort de réflexion"}
	msgProfile             = i18n.Msg{En: "Profile", Es: "Perfil", Zh: "配置文件", De: "Profil", Fr: "Profil"}
	msgDefault             = i18n.Msg{En: "Default", Es: "Por defecto", Zh: "默认", De: "Standard", Fr: "Par défaut"}
	msgDefaultRuntime      = i18n.Msg{En: "Default ({runtime})", Es: "Por defecto ({runtime})", Zh: "默认（{runtime}）", De: "Standard ({runtime})", Fr: "Par défaut ({runtime})"}
	msgThinkingOff         = i18n.Msg{En: "off", Es: "desactivado", Zh: "关闭", De: "aus", Fr: "désactivé"}
	msgThinkingMinimal     = i18n.Msg{En: "minimal", Es: "mínimo", Zh: "极低", De: "minimal", Fr: "minimal"}
	msgThinkingLow         = i18n.Msg{En: "low", Es: "bajo", Zh: "低", De: "niedrig", Fr: "bas"}
	msgThinkingMedium      = i18n.Msg{En: "medium", Es: "medio", Zh: "中", De: "mittel", Fr: "moyen"}
	msgThinkingHigh        = i18n.Msg{En: "high", Es: "alto", Zh: "高", De: "hoch", Fr: "élevé"}
	msgThinkingXHigh       = i18n.Msg{En: "xhigh", Es: "muy alto", Zh: "超高", De: "sehr hoch", Fr: "très élevé"}
	msgThinkingMax         = i18n.Msg{En: "max", Es: "máximo", Zh: "最大", De: "maximal", Fr: "maximal"}
	msgSavingTask          = i18n.Msg{En: "Creating your task", Es: "Creando tu tarea", Zh: "正在创建任务", De: "Aufgabe wird erstellt", Fr: "Création de la tâche"}
	msgSavingChanges       = i18n.Msg{En: "Saving your changes", Es: "Guardando los cambios", Zh: "正在保存更改", De: "Änderungen werden gespeichert", Fr: "Enregistrement des modifications"}
	msgStepSave            = i18n.Msg{En: "Saving the task", Es: "Guardando la tarea", Zh: "保存任务", De: "Aufgabe speichern", Fr: "Enregistrement de la tâche"}
	msgStepWorktree        = i18n.Msg{En: "Preparing the worktree and its branch", Es: "Preparando el worktree y su rama", Zh: "准备工作树及其分支", De: "Worktree und Branch vorbereiten", Fr: "Préparation du worktree et de sa branche"}
	msgStepHooks           = i18n.Msg{En: "Running the start hooks", Es: "Ejecutando los hooks de inicio", Zh: "运行启动钩子", De: "Start-Hooks ausführen", Fr: "Exécution des hooks de démarrage"}
	msgStepAgent           = i18n.Msg{En: "Starting the agent and sending the prompt", Es: "Arrancando el agente y enviando el prompt", Zh: "启动智能体并发送提示词", De: "Agent starten und Prompt senden", Fr: "Démarrage de l’agent et envoi du prompt"}
	msgStepImages          = i18n.Msg{En: "Storing the images", Es: "Guardando las imágenes", Zh: "保存图片", De: "Bilder speichern", Fr: "Enregistrement des images"}
	msgStepBoard           = i18n.Msg{En: "Updating the board", Es: "Actualizando el tablero", Zh: "更新看板", De: "Board aktualisieren", Fr: "Mise à jour du tableau"}
	msgSubmitSlow          = i18n.Msg{En: "This is taking longer than usual: a slow git remote or a start hook may be the cause. You can keep waiting.", Es: "Está tardando más de lo habitual: puede deberse a un remoto git lento o a un hook de inicio. Puedes seguir esperando.", Zh: "耗时比平时长：可能是 git 远程较慢或启动钩子所致。请继续等待。", De: "Das dauert länger als üblich: Ein langsames Git-Remote oder ein Start-Hook könnte die Ursache sein. Du kannst weiter warten.", Fr: "Cela prend plus de temps que d’habitude : un dépôt git distant lent ou un hook de démarrage peut en être la cause. Vous pouvez continuer à attendre."}
)
