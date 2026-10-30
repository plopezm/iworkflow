package components

import (
	"context"
	"encoding/json"

	"iworkflow/internal/core/i18n"
)

// Texts of the composer, copied from the Rust composer.rs and composer_completion.rs Msg constants.
var (
	msgComposerPlaceholder = i18n.Msg{En: "Write a message for the agent… (@ for files, / for commands, Ctrl+Enter to send)", Es: "Escribe un mensaje para el agente… (@ ficheros, / comandos, Ctrl+Enter para enviar)", Zh: "给智能体写一条消息…（@ 引用文件，/ 命令，Ctrl+Enter 发送）", De: "Schreibe eine Nachricht an den Agent … (@ Dateien, / Befehle, Strg+Enter zum Senden)", Fr: "Écrivez un message pour l’agent… (@ fichiers, / commandes, Ctrl+Entrée pour envoyer)"}
	msgModeLabel           = i18n.Msg{En: "Agent mode", Es: "Modo del agente", Zh: "智能体模式", De: "Agent-Modus", Fr: "Mode de l’agent"}
	msgModeAgent           = i18n.Msg{En: "Agent", Es: "Agente", Zh: "智能体", De: "Agent", Fr: "Agent"}
	msgModePlan            = i18n.Msg{En: "Plan", Es: "Plan", Zh: "计划", De: "Plan", Fr: "Plan"}
	msgSend                = i18n.Msg{En: "Send", Es: "Enviar", Zh: "发送", De: "Senden", Fr: "Envoyer"}
	msgNoFiles             = i18n.Msg{En: "No matching files", Es: "Ningún fichero coincide", Zh: "没有匹配的文件", De: "Keine passenden Dateien", Fr: "Aucun fichier correspondant"}
	msgSuggestions         = i18n.Msg{En: "Suggestions", Es: "Sugerencias", Zh: "建议", De: "Vorschläge", Fr: "Suggestions"}
	msgRemoveReference     = i18n.Msg{En: "Remove reference {name}", Es: "Quitar la referencia {name}", Zh: "移除引用 {name}", De: "Verweis {name} entfernen", Fr: "Retirer la référence {name}"}
)

// composerCommand is a slash command the menu offers. It is sent to the agent as plain text; /plan and /agent switch
// the mode instead, when the runtime reports one.
type composerCommand struct {
	name        string
	description i18n.Msg
}

// composerCommands are the commands of the Rust composer, in the same order.
var composerCommands = []composerCommand{
	{name: "plan", description: i18n.Msg{En: "Switch to plan mode", Es: "Cambiar al modo plan", Zh: "切换到计划模式", De: "In den Planmodus wechseln", Fr: "Passer en mode plan"}},
	{name: "agent", description: i18n.Msg{En: "Switch to agent mode", Es: "Cambiar al modo agente", Zh: "切换到智能体模式", De: "In den Agent-Modus wechseln", Fr: "Passer en mode agent"}},
	{name: "compact", description: i18n.Msg{En: "Summarize the conversation to free context", Es: "Resumir la conversación para liberar contexto", Zh: "总结对话以释放上下文", De: "Unterhaltung zusammenfassen, um Kontext freizugeben", Fr: "Résumer la conversation pour libérer du contexte"}},
	{name: "clear", description: i18n.Msg{En: "Clear the conversation", Es: "Limpiar la conversación", Zh: "清除对话", De: "Unterhaltung löschen", Fr: "Effacer la conversation"}},
	{name: "model", description: i18n.Msg{En: "Change the model", Es: "Cambiar el modelo", Zh: "更换模型", De: "Modell wechseln", Fr: "Changer de modèle"}},
	{name: "review", description: i18n.Msg{En: "Review the current changes", Es: "Revisar los cambios actuales", Zh: "审查当前更改", De: "Aktuelle Änderungen prüfen", Fr: "Relire les modifications en cours"}},
	{name: "help", description: i18n.Msg{En: "List the agent's commands", Es: "Listar los comandos del agente", Zh: "列出智能体的命令", De: "Befehle des Agents auflisten", Fr: "Lister les commandes de l’agent"}},
}

// composerCommandOffer is one command as the menu script reads it from the composer.
type composerCommandOffer struct {
	Insert string `json:"insert"`
	Label  string `json:"label"`
	Hint   string `json:"hint"`
}

// composerCommandsJSON lists the commands with their descriptions in the language of the request. The menu script
// filters them by the typed prefix.
func composerCommandsJSON(ctx context.Context) string {
	offers := make([]composerCommandOffer, 0, len(composerCommands))
	for _, command := range composerCommands {
		offers = append(offers, composerCommandOffer{
			Insert: "/" + command.name,
			Label:  "/" + command.name,
			Hint:   command.description.T(ctx),
		})
	}
	encoded, err := json.Marshal(offers)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// modeValuesJSON is the body a mode button posts: the wire name of the mode it names.
func modeValuesJSON(mode string) string {
	encoded, err := json.Marshal(map[string]string{"mode": mode})
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// composerModeClass is the class of one mode of the switch; the active one is marked.
func composerModeClass(active bool) string {
	if active {
		return "composer-mode-option is-active"
	}
	return "composer-mode-option"
}

// ariaBool renders a flag as the value aria-pressed takes.
func ariaBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
