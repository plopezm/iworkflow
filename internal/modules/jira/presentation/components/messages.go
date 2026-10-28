package components

import (
	"context"
	"strings"

	"iworkflow/internal/core/i18n"
)

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

	msgImportedIssues = i18n.Msg{
		En: "Imported issues",
		Es: "Tareas importadas",
		Zh: "已导入的任务",
		De: "Importierte Aufgaben",
		Fr: "Tâches importées",
	}

	msgSyncing = i18n.Msg{
		En: "Syncing…",
		Es: "Sincronizando…",
		Zh: "同步中…",
		De: "Wird synchronisiert…",
		Fr: "Synchronisation…",
	}

	msgSync = i18n.Msg{
		En: "Sync",
		Es: "Sincronizar",
		Zh: "同步",
		De: "Synchronisieren",
		Fr: "Synchroniser",
	}

	msgNewLane = i18n.Msg{
		En: "+ New lane",
		Es: "+ Nuevo carril",
		Zh: "+ 新建泳道",
		De: "+ Neue Lane",
		Fr: "+ Nouvelle voie",
	}

	msgSearch = i18n.Msg{
		En: "Search",
		Es: "Buscar",
		Zh: "搜索",
		De: "Suchen",
		Fr: "Rechercher",
	}

	msgSearchPlaceholder = i18n.Msg{
		En: "Key, summary, assignee or label",
		Es: "Clave, resumen, responsable o etiqueta",
		Zh: "键、摘要、负责人或标签",
		De: "Schlüssel, Zusammenfassung, Bearbeiter oder Label",
		Fr: "Clé, résumé, responsable ou étiquette",
	}

	msgJiraStatus = i18n.Msg{
		En: "Status in Jira",
		Es: "Estado en Jira",
		Zh: "Jira 中的状态",
		De: "Status in Jira",
		Fr: "Statut dans Jira",
	}

	msgLocalStatus = i18n.Msg{
		En: "Local status",
		Es: "Estado local",
		Zh: "本地状态",
		De: "Lokaler Status",
		Fr: "Statut local",
	}

	msgAll = i18n.Msg{
		En: "All",
		Es: "Todos",
		Zh: "全部",
		De: "Alle",
		Fr: "Tous",
	}

	msgAssignedToMe = i18n.Msg{
		En: "Assigned to me · not Done",
		Es: "Asignadas a mí · sin Done",
		Zh: "分配给我 · 未完成",
		De: "Mir zugewiesen · nicht Done",
		Fr: "Qui me sont assignées · hors Done",
	}

	msgEditLane = i18n.Msg{
		En: "Edit lane",
		Es: "Editar carril",
		Zh: "编辑泳道",
		De: "Lane bearbeiten",
		Fr: "Modifier la voie",
	}

	msgEditShort = i18n.Msg{
		En: "Edit",
		Es: "Editar",
		Zh: "编辑",
		De: "Bearbeiten",
		Fr: "Modifier",
	}

	msgDeleteLane = i18n.Msg{
		En: "Delete lane",
		Es: "Eliminar carril",
		Zh: "删除泳道",
		De: "Lane löschen",
		Fr: "Supprimer la voie",
	}

	msgDeleteShort = i18n.Msg{
		En: "Delete",
		Es: "Eliminar",
		Zh: "删除",
		De: "Löschen",
		Fr: "Supprimer",
	}

	msgDeleteNotice = i18n.Msg{
		En: "The lane is deleted; issues already converted or ignored are kept.",
		Es: "Se elimina el carril; las tareas ya convertidas o ignoradas se conservan.",
		Zh: "将删除该泳道；已转换或已忽略的任务会保留。",
		De: "Die Lane wird gelöscht; bereits konvertierte oder ignorierte Aufgaben bleiben erhalten.",
		Fr: "La voie est supprimée ; les tâches déjà converties ou ignorées sont conservées.",
	}

	msgViewTask = i18n.Msg{
		En: "View task",
		Es: "Ver tarea",
		Zh: "查看任务",
		De: "Aufgabe ansehen",
		Fr: "Voir la tâche",
	}

	msgConvert = i18n.Msg{
		En: "Convert to task",
		Es: "Convertir en tarea",
		Zh: "转换为任务",
		De: "In Aufgabe umwandeln",
		Fr: "Convertir en tâche",
	}

	msgIgnore = i18n.Msg{
		En: "Ignore",
		Es: "Ignorar",
		Zh: "忽略",
		De: "Ignorieren",
		Fr: "Ignorer",
	}

	msgRestore = i18n.Msg{
		En: "Restore",
		Es: "Restaurar",
		Zh: "恢复",
		De: "Wiederherstellen",
		Fr: "Restaurer",
	}

	msgOpenInJira = i18n.Msg{
		En: "Open in Jira",
		Es: "Abrir en Jira",
		Zh: "在 Jira 中打开",
		De: "In Jira öffnen",
		Fr: "Ouvrir dans Jira",
	}

	msgLaneName = i18n.Msg{
		En: "Name",
		Es: "Nombre",
		Zh: "名称",
		De: "Name",
		Fr: "Nom",
	}

	msgLaneNamePlaceholder = i18n.Msg{
		En: "Backend, Critical bugs, Pending reviews…",
		Es: "Backend, Bugs críticos, Revisiones pendientes…",
		Zh: "后端、严重缺陷、待审核…",
		De: "Backend, Kritische Bugs, Ausstehende Reviews…",
		Fr: "Backend, Bugs critiques, Revues en attente…",
	}

	msgJqlLabel = i18n.Msg{
		En: "JQL query",
		Es: "Consulta JQL",
		Zh: "JQL 查询",
		De: "JQL-Abfrage",
		Fr: "Requête JQL",
	}

	msgJqlHint = i18n.Msg{
		En: "The lane's issues are fetched from Jira with this query every time you sync.",
		Es: "Las tareas del carril se traen de Jira con esta consulta cada vez que sincronizas.",
		Zh: "每次同步时，都会使用此查询从 Jira 获取该泳道的任务。",
		De: "Die Aufgaben der Lane werden bei jeder Synchronisierung mit dieser Abfrage aus Jira geladen.",
		Fr: "Les tâches de la voie sont récupérées depuis Jira avec cette requête à chaque synchronisation.",
	}

	msgSavingAndSyncing = i18n.Msg{
		En: "Saving and syncing…",
		Es: "Guardando y sincronizando…",
		Zh: "正在保存并同步…",
		De: "Wird gespeichert und synchronisiert…",
		Fr: "Enregistrement et synchronisation…",
	}

	msgSaveChanges = i18n.Msg{
		En: "Save changes",
		Es: "Guardar cambios",
		Zh: "保存更改",
		De: "Änderungen speichern",
		Fr: "Enregistrer les modifications",
	}

	msgCreateLane = i18n.Msg{
		En: "Create lane",
		Es: "Crear carril",
		Zh: "创建泳道",
		De: "Lane erstellen",
		Fr: "Créer la voie",
	}

	msgLaneFailed = i18n.Msg{
		En: "Lane {lane}",
		Es: "Carril {lane}",
		Zh: "泳道 {lane}",
		De: "Lane {lane}",
		Fr: "Voie {lane}",
	}
)

func formatLaneFailed(ctx context.Context, lane string) string {
	return strings.ReplaceAll(msgLaneFailed.T(ctx), "{lane}", lane)
}
