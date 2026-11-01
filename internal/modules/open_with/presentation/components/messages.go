package components

import "iworkflow/internal/core/i18n"

var (
	msgTitle       = i18n.Msg{En: "Open with", Es: "Abrir con", Zh: "打开方式", De: "Öffnen mit", Fr: "Ouvrir avec"}
	msgEmpty       = i18n.Msg{En: "No applications configured yet.", Es: "Aún no hay aplicaciones configuradas.", Zh: "尚未配置应用程序。", De: "Noch keine Anwendungen konfiguriert.", Fr: "Aucune application configurée."}
	msgOpen        = i18n.Msg{En: "Open", Es: "Abrir", Zh: "打开", De: "Öffnen", Fr: "Ouvrir"}
	msgDelete      = i18n.Msg{En: "Delete", Es: "Eliminar", Zh: "删除", De: "Löschen", Fr: "Supprimer"}
	msgConfirmDel  = i18n.Msg{En: "Delete this application from the list?", Es: "¿Eliminar esta aplicación de la lista?", Zh: "从列表中删除此应用程序？", De: "Diese Anwendung aus der Liste löschen?", Fr: "Supprimer cette application de la liste ?"}
	msgAddTitle    = i18n.Msg{En: "Add application", Es: "Añadir aplicación", Zh: "添加应用程序", De: "Anwendung hinzufügen", Fr: "Ajouter une application"}
	msgNameLabel   = i18n.Msg{En: "Name", Es: "Nombre", Zh: "名称", De: "Name", Fr: "Nom"}
	msgPathLabel   = i18n.Msg{En: "Executable path", Es: "Ruta del ejecutable", Zh: "可执行文件路径", De: "Pfad der Programmdatei", Fr: "Chemin de l'exécutable"}
	msgPathHint    = i18n.Msg{En: "/usr/bin/code", Es: "/usr/bin/code", Zh: "/usr/bin/code", De: "/usr/bin/code", Fr: "/usr/bin/code"}
	msgArgsLabel   = i18n.Msg{En: "Extra arguments (optional, one per line)", Es: "Parámetros extra (opcional, uno por línea)", Zh: "额外参数（可选，每行一个）", De: "Zusätzliche Argumente (optional, eines pro Zeile)", Fr: "Arguments supplémentaires (facultatif, un par ligne)"}
	msgEnvLabel    = i18n.Msg{En: "Environment variables (optional, NAME=VALUE per line)", Es: "Variables de entorno (opcional, NOMBRE=VALOR por línea)", Zh: "环境变量（可选，每行 NAME=VALUE）", De: "Umgebungsvariablen (optional, NAME=WERT pro Zeile)", Fr: "Variables d'environnement (facultatif, NOM=VALEUR par ligne)"}
	msgAddNew      = i18n.Msg{En: "Add new +", Es: "Añadir nuevo +", Zh: "添加新应用 +", De: "Neu hinzufügen +", Fr: "Ajouter +"}
	msgAdd         = i18n.Msg{En: "Add", Es: "Añadir", Zh: "添加", De: "Hinzufügen", Fr: "Ajouter"}
	msgStartedPref = i18n.Msg{En: "Started", Es: "Lanzada", Zh: "已启动", De: "Gestartet", Fr: "Lancée"}
	msgBrowse      = i18n.Msg{En: "Browse…", Es: "Examinar…", Zh: "浏览…", De: "Durchsuchen…", Fr: "Parcourir…"}
	msgPickerTitle = i18n.Msg{En: "Choose the executable (files of the server)", Es: "Elige el ejecutable (ficheros del servidor)", Zh: "选择可执行文件（服务器上的文件）", De: "Programmdatei wählen (Dateien des Servers)", Fr: "Choisir l'exécutable (fichiers du serveur)"}
	msgUp          = i18n.Msg{En: "Up", Es: "Subir", Zh: "上一级", De: "Nach oben", Fr: "Dossier parent"}
	msgCancel      = i18n.Msg{En: "Cancel", Es: "Cancelar", Zh: "取消", De: "Abbrechen", Fr: "Annuler"}
	msgFilter      = i18n.Msg{En: "Filter by name", Es: "Filtrar por nombre", Zh: "按名称筛选", De: "Nach Name filtern", Fr: "Filtrer par nom"}
	msgFolderEmpty = i18n.Msg{En: "Nothing to show here.", Es: "No hay nada que mostrar aquí.", Zh: "这里没有内容。", De: "Hier gibt es nichts anzuzeigen.", Fr: "Rien à afficher ici."}
	msgTruncated   = i18n.Msg{En: "Showing the first entries only; use the filter to narrow the list.", Es: "Solo se muestran las primeras entradas; usa el filtro para acotar la lista.", Zh: "仅显示前面的条目；请使用筛选缩小范围。", De: "Nur die ersten Einträge werden angezeigt; verwenden Sie den Filter.", Fr: "Seules les premières entrées sont affichées ; utilisez le filtre."}
	msgWithArgs    = i18n.Msg{En: "args", Es: "parámetros", Zh: "参数", De: "Argumente", Fr: "arguments"}
	msgWithEnv     = i18n.Msg{En: "env", Es: "entorno", Zh: "环境", De: "Umgebung", Fr: "environnement"}
)
