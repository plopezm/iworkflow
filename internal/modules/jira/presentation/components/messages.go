package components

import "iworkflow/internal/core/i18n"

var (
	msgCardTitle = i18n.Msg{
		En: "Jira connection",
		Es: "Conexión con Jira",
		Zh: "Jira 连接",
		De: "Jira-Verbindung",
		Fr: "Connexion à Jira",
	}

	msgServerURL = i18n.Msg{
		En: "Server URL",
		Es: "URL del servidor",
		Zh: "服务器 URL",
		De: "Server-URL",
		Fr: "URL du serveur",
	}

	msgServerURLPlaceholder = i18n.Msg{
		En: "https://jira.mycompany.com",
		Es: "https://jira.miempresa.com",
		Zh: "https://jira.mycompany.com",
		De: "https://jira.meinefirma.com",
		Fr: "https://jira.monentreprise.com",
	}

	msgTokenLabel = i18n.Msg{
		En: "User token (PAT)",
		Es: "Token de usuario (PAT)",
		Zh: "用户令牌 (PAT)",
		De: "Benutzer-Token (PAT)",
		Fr: "Jeton utilisateur (PAT)",
	}

	msgTokenPlaceholder = i18n.Msg{
		En: "Personal access token",
		Es: "Token de acceso personal",
		Zh: "个人访问令牌",
		De: "Persönliches Zugriffstoken",
		Fr: "Jeton d'accès personnel",
	}

	msgTokenKeepPlaceholder = i18n.Msg{
		En: "Empty = keep the stored token",
		Es: "Vacío = conservar el token guardado",
		Zh: "留空 = 保留已保存的令牌",
		De: "Leer = gespeichertes Token behalten",
		Fr: "Vide = conserver le jeton enregistré",
	}

	msgAcceptInvalidCerts = i18n.Msg{
		En: "Accept unverified certificate (self-signed or internal CA)",
		Es: "Aceptar certificado no verificado (autofirmado o CA interna)",
		Zh: "接受未验证的证书（自签名或内部 CA）",
		De: "Nicht verifiziertes Zertifikat akzeptieren (selbstsigniert oder interne CA)",
		Fr: "Accepter un certificat non vérifié (auto-signé ou AC interne)",
	}

	msgConnect = i18n.Msg{
		En: "Connect",
		Es: "Conectar",
		Zh: "连接",
		De: "Verbinden",
		Fr: "Connecter",
	}

	msgChecking = i18n.Msg{
		En: "Checking…",
		Es: "Comprobando…",
		Zh: "检查中…",
		De: "Wird geprüft…",
		Fr: "Vérification…",
	}

	msgCancel = i18n.Msg{
		En: "Cancel",
		Es: "Cancelar",
		Zh: "取消",
		De: "Abbrechen",
		Fr: "Annuler",
	}

	msgEdit = i18n.Msg{
		En: "Edit connection",
		Es: "Editar conexión",
		Zh: "编辑连接",
		De: "Verbindung bearbeiten",
		Fr: "Modifier la connexion",
	}

	msgDisconnect = i18n.Msg{
		En: "Disconnect",
		Es: "Desconectar",
		Zh: "断开连接",
		De: "Trennen",
		Fr: "Déconnecter",
	}

	msgServer = i18n.Msg{
		En: "Server: ",
		Es: "Servidor: ",
		Zh: "服务器：",
		De: "Server: ",
		Fr: "Serveur : ",
	}

	msgLastSync = i18n.Msg{
		En: "Last sync: ",
		Es: "Última sincronización: ",
		Zh: "上次同步：",
		De: "Letzte Synchronisierung: ",
		Fr: "Dernière synchronisation : ",
	}

	msgNever = i18n.Msg{
		En: "never",
		Es: "nunca",
		Zh: "从未",
		De: "nie",
		Fr: "jamais",
	}

	msgLastFailure = i18n.Msg{
		En: "Last failure: {error}",
		Es: "Último fallo: {error}",
		Zh: "上次失败：{error}",
		De: "Letzter Fehler: {error}",
		Fr: "Dernier échec : {error}",
	}

	msgForgetNotice = i18n.Msg{
		En: "The token and imported issues will be forgotten.",
		Es: "Se olvidan el token y las tareas importadas.",
		Zh: "令牌和已导入的任务将被清除。",
		De: "Token und importierte Aufgaben werden vergessen.",
		Fr: "Le jeton et les tâches importées seront oubliés.",
	}
)
