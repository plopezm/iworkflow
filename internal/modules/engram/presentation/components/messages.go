// Package components holds the templ components of the Engram settings card.
package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// User-facing texts of the Engram server card.
var (
	msgLoading     = i18n.Msg{En: "Loading…", Es: "Cargando…", Zh: "加载中…", De: "Wird geladen…", Fr: "Chargement…"}
	msgCardTitle   = i18n.Msg{En: "Engram memory server", Es: "Servidor de memoria Engram", Zh: "Engram 记忆服务器", De: "Engram-Gedächtnisserver", Fr: "Serveur de mémoire Engram"}
	msgCardHelp    = i18n.Msg{En: "Address of the Engram HTTP API. If nothing answers there and the address is on this machine, iworkflow starts Engram with its default settings.", Es: "Dirección de la API HTTP de Engram. Si no responde nadie y la dirección es de esta máquina, iworkflow inicia Engram con sus datos por defecto.", Zh: "Engram HTTP API 的地址。如果没有响应且地址在本机，iworkflow 会使用默认设置启动 Engram。", De: "Adresse der Engram-HTTP-API. Antwortet dort nichts und liegt die Adresse auf diesem Rechner, startet iworkflow Engram mit seinen Standardwerten.", Fr: "Adresse de l'API HTTP d'Engram. Si rien ne répond et que l'adresse est locale, iworkflow démarre Engram avec ses réglages par défaut."}
	msgFieldLabel  = i18n.Msg{En: "Server address", Es: "Dirección del servidor", Zh: "服务器地址", De: "Serveradresse", Fr: "Adresse du serveur"}
	msgSave        = i18n.Msg{En: "Save and apply", Es: "Guardar y aplicar", Zh: "保存并应用", De: "Speichern und anwenden", Fr: "Enregistrer et appliquer"}
	msgReset       = i18n.Msg{En: "Restore default", Es: "Restaurar por defecto", Zh: "恢复默认", De: "Standard wiederherstellen", Fr: "Rétablir la valeur par défaut"}
	msgReachable   = i18n.Msg{En: "Engram is answering", Es: "Engram está respondiendo", Zh: "Engram 正在响应", De: "Engram antwortet", Fr: "Engram répond"}
	msgUnreachable = i18n.Msg{En: "Engram is not answering", Es: "Engram no responde", Zh: "Engram 没有响应", De: "Engram antwortet nicht", Fr: "Engram ne répond pas"}
	msgFromEnv     = i18n.Msg{En: "The address comes from IWF_ENGRAM_URL. Saving here overrides it; iworkflow does not start Engram for an address set that way.", Es: "La dirección viene de IWF_ENGRAM_URL. Guardar aquí la sustituye; iworkflow no inicia Engram para una dirección definida así.", Zh: "地址来自 IWF_ENGRAM_URL。在此保存会覆盖它；iworkflow 不会为这样设置的地址启动 Engram。", De: "Die Adresse stammt aus IWF_ENGRAM_URL. Speichern überschreibt sie; für so gesetzte Adressen startet iworkflow Engram nicht.", Fr: "L'adresse vient de IWF_ENGRAM_URL. L'enregistrer ici la remplace ; iworkflow ne démarre pas Engram pour une adresse définie ainsi."}
	msgRunning     = i18n.Msg{En: "Engram was already running; nothing was started.", Es: "Engram ya estaba en marcha; no se ha iniciado nada.", Zh: "Engram 已在运行，未启动任何内容。", De: "Engram lief bereits; nichts wurde gestartet.", Fr: "Engram tournait déjà ; rien n'a été démarré."}
	msgStarted     = i18n.Msg{En: "Engram was off and has been started.", Es: "Engram estaba apagado y se ha iniciado.", Zh: "Engram 原本未运行，现已启动。", De: "Engram war aus und wurde gestartet.", Fr: "Engram était arrêté et a été démarré."}
	msgSkipped     = i18n.Msg{En: "Engram is not answering and will not be started here: the address is remote, has no port, or comes from IWF_ENGRAM_URL.", Es: "Engram no responde y no se iniciará desde aquí: la dirección es remota, no tiene puerto o viene de IWF_ENGRAM_URL.", Zh: "Engram 没有响应，且不会在此启动：地址是远程的、没有端口，或来自 IWF_ENGRAM_URL。", De: "Engram antwortet nicht und wird hier nicht gestartet: Die Adresse ist entfernt, hat keinen Port oder stammt aus IWF_ENGRAM_URL.", Fr: "Engram ne répond pas et ne sera pas démarré ici : l'adresse est distante, sans port, ou vient de IWF_ENGRAM_URL."}
	msgStartFailed = i18n.Msg{En: "Engram could not be started. Check that the engram program is installed and in the PATH.", Es: "No se ha podido iniciar Engram. Comprueba que el programa engram está instalado y en el PATH.", Zh: "无法启动 Engram。请确认 engram 程序已安装并在 PATH 中。", De: "Engram konnte nicht gestartet werden. Prüfe, ob das Programm engram installiert und im PATH ist.", Fr: "Engram n'a pas pu être démarré. Vérifiez que le programme engram est installé et dans le PATH."}
	msgInvalidURL  = i18n.Msg{En: "The address must be an http or https URL with a host, for example http://127.0.0.1:7437.", Es: "La dirección debe ser una URL http o https con host, por ejemplo http://127.0.0.1:7437.", Zh: "地址必须是带主机的 http 或 https URL，例如 http://127.0.0.1:7437。", De: "Die Adresse muss eine http- oder https-URL mit Host sein, z. B. http://127.0.0.1:7437.", Fr: "L'adresse doit être une URL http ou https avec un hôte, par exemple http://127.0.0.1:7437."}
	msgSaveFailed  = i18n.Msg{En: "The address could not be saved.", Es: "No se ha podido guardar la dirección.", Zh: "无法保存地址。", De: "Die Adresse konnte nicht gespeichert werden.", Fr: "L'adresse n'a pas pu être enregistrée."}
)

// InvalidURLText is the message for an address that does not validate.
func InvalidURLText(ctx context.Context) string { return msgInvalidURL.T(ctx) }

// SaveFailedText is the message for a storage failure.
func SaveFailedText(ctx context.Context) string { return msgSaveFailed.T(ctx) }
