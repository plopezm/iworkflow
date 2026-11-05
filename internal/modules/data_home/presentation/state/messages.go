// Package state builds the view-models of the data_home card from the data the handlers read. It
// holds no business logic: the checks live in the use cases.
package state

import "iworkflow/internal/core/i18n"

// User-facing texts of the data folder card. Each one is written in every supported language.
var (
	msgTitle = i18n.Msg{En: "Data folder", Es: "Carpeta de datos", Zh: "数据目录", De: "Datenordner", Fr: "Dossier de données"}
	msgHelp  = i18n.Msg{
		En: "Moves the iWork Flow data folder to another directory. The server restarts to apply the move, and a large folder takes a while. Stop the tasks that are starting, running or finishing before moving.",
		Es: "Mueve la carpeta de datos de iWork Flow a otro directorio. El servidor se reinicia para aplicar el cambio y una carpeta grande tarda un rato. Detén las tareas que se están iniciando, ejecutando o finalizando antes de mover.",
		Zh: "将 iWork Flow 数据目录移动到另一个目录。服务器会重启以应用此更改，大型目录需要一些时间。移动前请停止正在启动、运行或完成中的任务。",
		De: "Verschiebt den iWork-Flow-Datenordner in ein anderes Verzeichnis. Der Server startet neu, um die Änderung zu übernehmen; ein großer Ordner braucht etwas Zeit. Beende vorher Aufgaben, die gestartet werden, laufen oder abgeschlossen werden.",
		Fr: "Déplace le dossier de données d'iWork Flow vers un autre répertoire. Le serveur redémarre pour appliquer le changement ; un gros dossier prend du temps. Arrêtez les tâches en cours de démarrage, d'exécution ou de finalisation avant le déplacement.",
	}
	msgCurrentFolder = i18n.Msg{En: "Current folder", Es: "Carpeta actual", Zh: "当前目录", De: "Aktueller Ordner", Fr: "Dossier actuel"}
	msgSourceEnv     = i18n.Msg{
		En: "set by the IWF_HOME environment variable",
		Es: "definida por la variable de entorno IWF_HOME",
		Zh: "由环境变量 IWF_HOME 设置",
		De: "durch die Umgebungsvariable IWF_HOME festgelegt",
		Fr: "définie par la variable d'environnement IWF_HOME",
	}
	msgSourcePointer = i18n.Msg{
		En: "saved in the per-user settings",
		Es: "guardada en la configuración del usuario",
		Zh: "保存在用户设置中",
		De: "in den Benutzereinstellungen gespeichert",
		Fr: "enregistrée dans les paramètres de l'utilisateur",
	}
	msgSourceDefault = i18n.Msg{En: "default folder", Es: "carpeta predeterminada", Zh: "默认目录", De: "Standardordner", Fr: "dossier par défaut"}
	msgEnvNote       = i18n.Msg{
		En: "IWF_HOME is set, so the environment decides the folder and this screen cannot move it. Unset IWF_HOME to move the folder from here.",
		Es: "IWF_HOME está definida, así que el entorno decide la carpeta y esta pantalla no puede moverla. Quita IWF_HOME para mover la carpeta desde aquí.",
		Zh: "已设置 IWF_HOME，因此目录由环境决定，此界面无法移动。取消设置 IWF_HOME 后可在此移动。",
		De: "IWF_HOME ist gesetzt, daher bestimmt die Umgebung den Ordner und dieser Bildschirm kann ihn nicht verschieben. Hebe IWF_HOME auf, um den Ordner hier zu verschieben.",
		Fr: "IWF_HOME est définie : c'est l'environnement qui fixe le dossier, et cet écran ne peut pas le déplacer. Supprimez IWF_HOME pour déplacer le dossier ici.",
	}
	msgDestinationLabel = i18n.Msg{
		En: "New folder (absolute path)",
		Es: "Nueva carpeta (ruta absoluta)",
		Zh: "新目录（绝对路径）",
		De: "Neuer Ordner (absoluter Pfad)",
		Fr: "Nouveau dossier (chemin absolu)",
	}
	msgMove   = i18n.Msg{En: "Move folder", Es: "Mover carpeta", Zh: "移动目录", De: "Ordner verschieben", Fr: "Déplacer le dossier"}
	msgCancel = i18n.Msg{
		En: "Cancel the pending move",
		Es: "Cancelar el movimiento pendiente",
		Zh: "取消待执行的移动",
		De: "Ausstehendes Verschieben abbrechen",
		Fr: "Annuler le déplacement en attente",
	}
	msgPending = i18n.Msg{
		En: "A move to {to} waits for the next start. The server applies it then.",
		Es: "Un movimiento a {to} espera al próximo inicio. El servidor lo aplica entonces.",
		Zh: "移动到 {to} 的操作等待下次启动时执行。",
		De: "Ein Verschieben nach {to} wartet auf den nächsten Start. Der Server führt es dann aus.",
		Fr: "Un déplacement vers {to} attend le prochain démarrage, qui l'appliquera.",
	}
	msgRestarting = i18n.Msg{
		En: "Restarting the server to move the data folder. This card reconnects when the server is back.",
		Es: "Reiniciando el servidor para mover la carpeta de datos. Esta tarjeta se reconecta cuando el servidor vuelve.",
		Zh: "正在重启服务器以移动数据目录。服务器恢复后此卡片会自动重连。",
		De: "Der Server startet neu, um den Datenordner zu verschieben. Diese Karte verbindet sich wieder, sobald der Server zurück ist.",
		Fr: "Redémarrage du serveur pour déplacer le dossier de données. Cette carte se reconnecte à son retour.",
	}
	msgLastOK = i18n.Msg{
		En: "Last move finished: {from} → {to}.",
		Es: "Último movimiento terminado: {from} → {to}.",
		Zh: "上次移动已完成：{from} → {to}。",
		De: "Letztes Verschieben abgeschlossen: {from} → {to}.",
		Fr: "Dernier déplacement terminé : {from} → {to}.",
	}
	msgLastFailed = i18n.Msg{
		En: "The last move failed: {error}. The old folder stays in use.",
		Es: "El último movimiento falló: {error}. La carpeta anterior sigue en uso.",
		Zh: "上次移动失败：{error}。仍使用原目录。",
		De: "Das letzte Verschieben ist fehlgeschlagen: {error}. Der alte Ordner bleibt in Verwendung.",
		Fr: "Le dernier déplacement a échoué : {error}. L'ancien dossier reste utilisé.",
	}
	msgWarnings = i18n.Msg{
		En: "The move finished with notes:",
		Es: "El movimiento terminó con avisos:",
		Zh: "移动已完成，但有以下提示：",
		De: "Das Verschieben ist mit Hinweisen abgeschlossen:",
		Fr: "Le déplacement s'est terminé avec des remarques :",
	}
	msgNoticeEnvLocked = i18n.Msg{
		En: "The folder is set by IWF_HOME, so it cannot be moved here.",
		Es: "La carpeta la define IWF_HOME, así que no se puede mover aquí.",
		Zh: "目录由 IWF_HOME 设置，无法在此移动。",
		De: "Der Ordner wird durch IWF_HOME festgelegt und kann hier nicht verschoben werden.",
		Fr: "Le dossier est défini par IWF_HOME et ne peut pas être déplacé ici.",
	}
	msgNoticeActiveTasks = i18n.Msg{
		En: "Stop the tasks that are starting, running or finishing, then try again.",
		Es: "Detén las tareas que se están iniciando, ejecutando o finalizando, y vuelve a intentarlo.",
		Zh: "请停止正在启动、运行或完成中的任务，然后重试。",
		De: "Beende die Aufgaben, die gestartet werden, laufen oder abgeschlossen werden, und versuche es erneut.",
		Fr: "Arrêtez les tâches en cours de démarrage, d'exécution ou de finalisation, puis réessayez.",
	}
	msgNoticePending = i18n.Msg{
		En: "A move is already pending. It runs at the next start.",
		Es: "Ya hay un movimiento pendiente. Se ejecuta en el próximo inicio.",
		Zh: "已有待执行的移动，将在下次启动时执行。",
		De: "Ein Verschieben steht bereits aus. Es läuft beim nächsten Start.",
		Fr: "Un déplacement est déjà en attente. Il s'exécute au prochain démarrage.",
	}
	msgUnavailable = i18n.Msg{
		En: "The data folder cannot be managed on this machine.",
		Es: "La carpeta de datos no se puede gestionar en esta máquina.",
		Zh: "无法在此机器上管理数据目录。",
		De: "Der Datenordner kann auf diesem Rechner nicht verwaltet werden.",
		Fr: "Le dossier de données ne peut pas être géré sur cette machine.",
	}
	msgReadFailed = i18n.Msg{
		En: "The data folder could not be read: {error}",
		Es: "No se pudo leer la carpeta de datos: {error}",
		Zh: "无法读取数据目录：{error}",
		De: "Der Datenordner konnte nicht gelesen werden: {error}",
		Fr: "Le dossier de données n'a pas pu être lu : {error}",
	}
	msgNoticeFailed = i18n.Msg{
		En: "The data folder could not be moved: {error}",
		Es: "No se pudo mover la carpeta de datos: {error}",
		Zh: "无法移动数据目录：{error}",
		De: "Der Datenordner konnte nicht verschoben werden: {error}",
		Fr: "Le dossier de données n'a pas pu être déplacé : {error}",
	}
)

// reasonTexts translates the reasons a move is refused, keyed by the reason of the domain.
var reasonTexts = map[string]i18n.Msg{
	"relative": {
		En: "The destination must be an absolute path.",
		Es: "El destino debe ser una ruta absoluta.",
		Zh: "目标必须是绝对路径。",
		De: "Das Ziel muss ein absoluter Pfad sein.",
		Fr: "La destination doit être un chemin absolu.",
	},
	"same": {
		En: "The destination is the current folder.",
		Es: "El destino es la carpeta actual.",
		Zh: "目标就是当前目录。",
		De: "Das Ziel ist der aktuelle Ordner.",
		Fr: "La destination est le dossier actuel.",
	},
	"nested": {
		En: "The destination and the current folder cannot contain each other.",
		Es: "El destino y la carpeta actual no pueden contenerse mutuamente.",
		Zh: "目标与当前目录不能互相包含。",
		De: "Das Ziel und der aktuelle Ordner dürfen einander nicht enthalten.",
		Fr: "La destination et le dossier actuel ne peuvent pas se contenir.",
	},
	"not_empty": {
		En: "The destination must be a new or empty folder.",
		Es: "El destino debe ser una carpeta nueva o vacía.",
		Zh: "目标必须是新建或空的目录。",
		De: "Das Ziel muss ein neuer oder leerer Ordner sein.",
		Fr: "La destination doit être un dossier neuf ou vide.",
	},
	"not_writable": {
		En: "The destination is not writable.",
		Es: "No se puede escribir en el destino.",
		Zh: "目标不可写。",
		De: "In das Ziel kann nicht geschrieben werden.",
		Fr: "La destination n'est pas accessible en écriture.",
	},
	"no_space": {
		En: "There is not enough free space at the destination.",
		Es: "No hay suficiente espacio libre en el destino.",
		Zh: "目标磁盘空间不足。",
		De: "Am Ziel ist nicht genug freier Speicher.",
		Fr: "Espace libre insuffisant à la destination.",
	},
	"source_unsafe": {
		En: "The current folder is too broad to move safely.",
		Es: "La carpeta actual es demasiado amplia para moverla con seguridad.",
		Zh: "当前目录范围过大，无法安全移动。",
		De: "Der aktuelle Ordner ist zu umfassend, um sicher verschoben zu werden.",
		Fr: "Le dossier actuel est trop large pour être déplacé en toute sécurité.",
	},
	"source_missing": {
		En: "The current folder is not an iWork Flow data folder.",
		Es: "La carpeta actual no es una carpeta de datos de iWork Flow.",
		Zh: "当前目录不是 iWork Flow 数据目录。",
		De: "Der aktuelle Ordner ist kein iWork-Flow-Datenordner.",
		Fr: "Le dossier actuel n'est pas un dossier de données d'iWork Flow.",
	},
}
