package inbound

import "iworkflow/internal/core/i18n"

// User-facing texts of the transcription endpoint.
var (
	msgInvalidAudio = i18n.Msg{En: "The recording is not valid.", Es: "La grabación no es válida.", Zh: "录音无效。", De: "Die Aufnahme ist ungültig.", Fr: "L'enregistrement n'est pas valide."}
	msgTooLong      = i18n.Msg{En: "The recording is too long (60 seconds at most).", Es: "La grabación es demasiado larga (máximo 60 segundos).", Zh: "录音太长（最长 60 秒）。", De: "Die Aufnahme ist zu lang (höchstens 60 Sekunden).", Fr: "L'enregistrement est trop long (60 secondes au maximum)."}
	msgNoSpeech     = i18n.Msg{En: "No speech was recognized in the recording.", Es: "No se ha reconocido ninguna voz en la grabación.", Zh: "录音中没有识别到语音。", De: "In der Aufnahme wurde keine Sprache erkannt.", Fr: "Aucune parole n'a été reconnue dans l'enregistrement."}
	msgUnavailable  = i18n.Msg{En: "Transcription is not available on this server.", Es: "La transcripción no está disponible en este servidor.", Zh: "此服务器不支持转写。", De: "Die Transkription ist auf diesem Server nicht verfügbar.", Fr: "La transcription n'est pas disponible sur ce serveur."}
	msgFailed       = i18n.Msg{En: "The recording could not be transcribed.", Es: "No se ha podido transcribir la grabación.", Zh: "无法转写录音。", De: "Die Aufnahme konnte nicht transkribiert werden.", Fr: "L'enregistrement n'a pas pu être transcrit."}
	msgBusy         = i18n.Msg{En: "Another recording is being transcribed. Try again in a moment.", Es: "Se está transcribiendo otra grabación. Inténtalo de nuevo en un momento.", Zh: "正在转写另一段录音。请稍后再试。", De: "Eine andere Aufnahme wird gerade transkribiert. Versuche es gleich noch einmal.", Fr: "Un autre enregistrement est en cours de transcription. Réessayez dans un instant."}
)
