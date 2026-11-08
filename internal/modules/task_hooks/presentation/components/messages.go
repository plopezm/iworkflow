package components

import "iworkflow/internal/core/i18n"

var (
	msgTitle        = i18n.Msg{En: "Hooks", Es: "Hooks", Zh: "钩子", De: "Hooks", Fr: "Hooks"}
	msgIntro        = i18n.Msg{En: "Actions that run before or after a task starts or finishes in this workspace.", Es: "Acciones que se ejecutan antes o después de empezar o terminar una tarea de este workspace.", Zh: "在此工作区中任务开始或结束之前或之后运行的操作。", De: "Aktionen, die vor oder nach dem Start oder Abschluss einer Aufgabe in diesem Workspace laufen.", Fr: "Actions exécutées avant ou après le démarrage ou la fin d'une tâche de cet espace de travail."}
	msgEventStart   = i18n.Msg{En: "When a task starts", Es: "Al empezar una tarea", Zh: "任务开始时", De: "Wenn eine Aufgabe startet", Fr: "Au démarrage d'une tâche"}
	msgEventFinish  = i18n.Msg{En: "When a task finishes", Es: "Al terminar una tarea", Zh: "任务结束时", De: "Wenn eine Aufgabe endet", Fr: "À la fin d'une tâche"}
	msgBefore       = i18n.Msg{En: "Before", Es: "Antes de", Zh: "之前", De: "Vorher", Fr: "Avant"}
	msgAfter        = i18n.Msg{En: "After", Es: "Después de", Zh: "之后", De: "Nachher", Fr: "Après"}
	msgNoHooks      = i18n.Msg{En: "No hooks yet.", Es: "Ningún hook todavía.", Zh: "还没有 hook。", De: "Noch keine Hooks.", Fr: "Aucun hook pour l'instant."}
	msgAddHook      = i18n.Msg{En: "Add hook +", Es: "Añadir hook +", Zh: "添加 hook +", De: "Hook hinzufügen +", Fr: "Ajouter un hook +"}
	msgKindScript   = i18n.Msg{En: "Script", Es: "Script", Zh: "脚本", De: "Skript", Fr: "Script"}
	msgKindAgent    = i18n.Msg{En: "Agent", Es: "Agente", Zh: "智能体", De: "Agent", Fr: "Agent"}
	msgActive       = i18n.Msg{En: "On", Es: "Activo", Zh: "已启用", De: "Aktiv", Fr: "Actif"}
	msgInactive     = i18n.Msg{En: "Off", Es: "Inactivo", Zh: "已停用", De: "Inaktiv", Fr: "Inactif"}
	msgEnable       = i18n.Msg{En: "Enable", Es: "Activar", Zh: "启用", De: "Aktivieren", Fr: "Activer"}
	msgDisable      = i18n.Msg{En: "Disable", Es: "Desactivar", Zh: "停用", De: "Deaktivieren", Fr: "Désactiver"}
	msgEdit         = i18n.Msg{En: "Edit", Es: "Editar", Zh: "编辑", De: "Bearbeiten", Fr: "Modifier"}
	msgDelete       = i18n.Msg{En: "Delete", Es: "Eliminar", Zh: "删除", De: "Löschen", Fr: "Supprimer"}
	msgConfirmDel   = i18n.Msg{En: "Delete this hook?", Es: "¿Eliminar este hook?", Zh: "删除此 hook？", De: "Diesen Hook löschen?", Fr: "Supprimer ce hook ?"}
	msgTest         = i18n.Msg{En: "Test", Es: "Probar", Zh: "测试", De: "Testen", Fr: "Tester"}
	msgMoveUp       = i18n.Msg{En: "Move up", Es: "Subir", Zh: "上移", De: "Nach oben", Fr: "Monter"}
	msgMoveDown     = i18n.Msg{En: "Move down", Es: "Bajar", Zh: "下移", De: "Nach unten", Fr: "Descendre"}
	msgTimeout      = i18n.Msg{En: "Timeout", Es: "Tiempo máximo", Zh: "超时", De: "Zeitlimit", Fr: "Délai max."}
	msgOnFailure    = i18n.Msg{En: "On failure", Es: "Si falla", Zh: "失败时", De: "Bei Fehler", Fr: "En cas d'échec"}
	msgFailBlock    = i18n.Msg{En: "Block the action", Es: "Bloquea la acción", Zh: "阻止操作", De: "Aktion blockieren", Fr: "Bloque l'action"}
	msgFailWarn     = i18n.Msg{En: "Warn and continue", Es: "Avisa y continúa", Zh: "警告并继续", De: "Warnen und fortfahren", Fr: "Avertir et continuer"}
	msgFailIgnore   = i18n.Msg{En: "Ignore", Es: "Ignora", Zh: "忽略", De: "Ignorieren", Fr: "Ignorer"}
	msgFormNew      = i18n.Msg{En: "New hook", Es: "Nuevo hook", Zh: "新 hook", De: "Neuer Hook", Fr: "Nouveau hook"}
	msgFormEdit     = i18n.Msg{En: "Edit hook", Es: "Editar hook", Zh: "编辑 hook", De: "Hook bearbeiten", Fr: "Modifier le hook"}
	msgEnabledLabel = i18n.Msg{En: "Enabled", Es: "Habilitado", Zh: "启用", De: "Aktiviert", Fr: "Activé"}
	msgKindLabel    = i18n.Msg{En: "Type", Es: "Tipo", Zh: "类型", De: "Typ", Fr: "Type"}
	msgPhaseLabel   = i18n.Msg{En: "Moment", Es: "Momento", Zh: "时机", De: "Zeitpunkt", Fr: "Moment"}
	msgEventLabel   = i18n.Msg{En: "Event", Es: "Evento", Zh: "事件", De: "Ereignis", Fr: "Événement"}
	msgCommand      = i18n.Msg{En: "Command", Es: "Comando", Zh: "命令", De: "Befehl", Fr: "Commande"}
	msgCommandHint  = i18n.Msg{En: "./scripts/check.sh", Es: "./scripts/check.sh", Zh: "./scripts/check.sh", De: "./scripts/check.sh", Fr: "./scripts/check.sh"}
	msgArgs         = i18n.Msg{En: "Arguments (optional, one per line; {{.Branch}} is expanded)", Es: "Argumentos (opcional, uno por línea; {{.Branch}} se expande)", Zh: "参数（可选，每行一个；{{.Branch}} 会被展开）", De: "Argumente (optional, eines pro Zeile; {{.Branch}} wird ersetzt)", Fr: "Arguments (facultatif, un par ligne ; {{.Branch}} est remplacé)"}
	msgEnv          = i18n.Msg{En: "Environment variables (optional, NAME=VALUE per line)", Es: "Variables de entorno (opcional, NOMBRE=VALOR por línea)", Zh: "环境变量（可选，每行 NAME=VALUE）", De: "Umgebungsvariablen (optional, NAME=WERT pro Zeile)", Fr: "Variables d'environnement (facultatif, NOM=VALEUR par ligne)"}
	msgUseShell     = i18n.Msg{En: "Run through a shell", Es: "Ejecutar con shell", Zh: "通过 shell 运行", De: "Über eine Shell ausführen", Fr: "Exécuter via un shell"}
	msgPrompt       = i18n.Msg{En: "Instruction for the agent", Es: "Instrucción para el agente", Zh: "给智能体的指令", De: "Anweisung für den Agenten", Fr: "Consigne pour l'agent"}
	msgRuntime      = i18n.Msg{En: "Runtime (optional, workspace default)", Es: "Runtime (opcional, por defecto del workspace)", Zh: "运行时（可选，默认为工作区设置）", De: "Laufzeit (optional, Standard des Workspaces)", Fr: "Runtime (facultatif, celui de l'espace)"}
	msgModel        = i18n.Msg{En: "Model (optional, provider/model)", Es: "Modelo (opcional, proveedor/modelo)", Zh: "模型（可选，提供方/模型）", De: "Modell (optional, anbieter/modell)", Fr: "Modèle (facultatif, fournisseur/modèle)"}
	msgThinking     = i18n.Msg{En: "Thinking (optional)", Es: "Razonamiento (opcional)", Zh: "思考强度（可选）", De: "Denkstufe (optional)", Fr: "Réflexion (facultatif)"}
	msgProfile      = i18n.Msg{En: "Profile (optional)", Es: "Perfil (opcional)", Zh: "配置文件（可选）", De: "Profil (optional)", Fr: "Profil (facultatif)"}
	msgTimeoutHint  = i18n.Msg{En: "Whole seconds, at most %d.", Es: "Segundos enteros, como máximo %d.", Zh: "整数秒，最多 %d。", De: "Ganze Sekunden, höchstens %d.", Fr: "Secondes entières, %d au maximum."}
	msgSave         = i18n.Msg{En: "Save", Es: "Guardar", Zh: "保存", De: "Speichern", Fr: "Enregistrer"}
	msgCancel       = i18n.Msg{En: "Cancel", Es: "Cancelar", Zh: "取消", De: "Abbrechen", Fr: "Annuler"}
	msgNoticeSaved  = i18n.Msg{En: "Hook saved.", Es: "Hook guardado.", Zh: "Hook 已保存。", De: "Hook gespeichert.", Fr: "Hook enregistré."}
	msgNoticeUpd    = i18n.Msg{En: "Hook updated.", Es: "Hook actualizado.", Zh: "Hook 已更新。", De: "Hook aktualisiert.", Fr: "Hook mis à jour."}
	msgNoticeDel    = i18n.Msg{En: "Hook deleted.", Es: "Hook eliminado.", Zh: "Hook 已删除。", De: "Hook gelöscht.", Fr: "Hook supprimé."}
	msgNoticeOrder  = i18n.Msg{En: "Order saved.", Es: "Orden guardado.", Zh: "顺序已保存。", De: "Reihenfolge gespeichert.", Fr: "Ordre enregistré."}
	msgActionFailed = i18n.Msg{En: "The action failed:", Es: "La acción falló:", Zh: "操作失败：", De: "Die Aktion ist fehlgeschlagen:", Fr: "L'action a échoué :"}
	msgResultOK     = i18n.Msg{En: "Passed", Es: "Correcto", Zh: "通过", De: "Erfolgreich", Fr: "Réussi"}
	msgResultFailed = i18n.Msg{En: "Failed", Es: "Fallido", Zh: "失败", De: "Fehlgeschlagen", Fr: "Échec"}
	msgResultTO     = i18n.Msg{En: "Timed out", Es: "Tiempo agotado", Zh: "超时", De: "Zeitüberschreitung", Fr: "Délai dépassé"}
	msgResultError  = i18n.Msg{En: "Could not run", Es: "No se pudo ejecutar", Zh: "无法运行", De: "Konnte nicht ausgeführt werden", Fr: "Impossible à exécuter"}
	msgExitCode     = i18n.Msg{En: "exit code", Es: "código de salida", Zh: "退出码", De: "Exitcode", Fr: "code de sortie"}
	msgNoOutput     = i18n.Msg{En: "(no output)", Es: "(sin salida)", Zh: "（无输出）", De: "(keine Ausgabe)", Fr: "(aucune sortie)"}
)
