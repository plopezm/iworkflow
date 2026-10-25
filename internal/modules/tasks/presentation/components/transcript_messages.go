package components

import "iworkflow/internal/core/i18n"

// Texts of the transcript, copied from the Rust transcript_view.rs and transcript.rs Msg constants.
var (
	msgTranscriptLaunchDetails  = i18n.Msg{En: "Launch details", Es: "Detalles de arranque", Zh: "启动详情", De: "Startdetails", Fr: "Détails de démarrage"}
	msgTranscriptRoleUser       = i18n.Msg{En: "You", Es: "Tú", Zh: "你", De: "Du", Fr: "Vous"}
	msgTranscriptRoleAgent      = i18n.Msg{En: "Agent", Es: "Agente", Zh: "智能体", De: "Agent", Fr: "Agent"}
	msgTranscriptReasoning      = i18n.Msg{En: "Reasoning", Es: "Razonamiento", Zh: "推理过程", De: "Überlegungen", Fr: "Raisonnement"}
	msgTranscriptToolRunning    = i18n.Msg{En: "running", Es: "en curso", Zh: "进行中", De: "läuft", Fr: "en cours"}
	msgTranscriptToolOK         = i18n.Msg{En: "ok", Es: "ok", Zh: "成功", De: "ok", Fr: "ok"}
	msgTranscriptToolError      = i18n.Msg{En: "error", Es: "error", Zh: "错误", De: "Fehler", Fr: "erreur"}
	msgTranscriptDialogAnswered = i18n.Msg{En: "answered", Es: "respondido", Zh: "已回答", De: "beantwortet", Fr: "répondu"}
	msgTranscriptDialogPending  = i18n.Msg{En: "pending", Es: "pendiente", Zh: "待处理", De: "ausstehend", Fr: "en attente"}
	msgTranscriptStoppedCode    = i18n.Msg{En: "Agent stopped (code {code})", Es: "Agente detenido (código {code})", Zh: "智能体已停止（代码 {code}）", De: "Agent angehalten (Code {code})", Fr: "Agent arrêté (code {code})"}
	msgTranscriptStoppedNoCode  = i18n.Msg{En: "Agent stopped (no code)", Es: "Agente detenido (sin código)", Zh: "智能体已停止（无代码）", De: "Agent angehalten (kein Code)", Fr: "Agent arrêté (sans code)"}
	msgTranscriptNoActivity     = i18n.Msg{En: "No activity yet.", Es: "Sin actividad todavía.", Zh: "暂无活动。", De: "Noch keine Aktivität.", Fr: "Aucune activité pour l’instant."}
	msgTranscriptWriting        = i18n.Msg{En: "Agent · writing…", Es: "Agente · escribiendo…", Zh: "智能体 · 正在输入…", De: "Agent · schreibt …", Fr: "Agent · en train d’écrire…"}
	msgTranscriptOneImage       = i18n.Msg{En: "📎 1 image", Es: "📎 1 imagen", Zh: "📎 1 张图片", De: "📎 1 Bild", Fr: "📎 1 image"}
	msgTranscriptManyImages     = i18n.Msg{En: "📎 {n} images", Es: "📎 {n} imágenes", Zh: "📎 {n} 张图片", De: "📎 {n} Bilder", Fr: "📎 {n} images"}
)
