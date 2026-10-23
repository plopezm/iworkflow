package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// User-facing texts of the workspaces screens. Each one copies the Rust `Msg` constant it replaces
// (src/modules/workspaces/presentation/**), so every language keeps the original wording.
var (
	msgNoWorkspaces  = i18n.Msg{En: "No workspaces registered yet.", Es: "Aún no hay workspaces registrados.", Zh: "还没有已注册的工作区。", De: "Noch keine Workspaces registriert.", Fr: "Aucun espace de travail enregistré pour le moment."}
	msgEdit          = i18n.Msg{En: "Edit", Es: "Editar", Zh: "编辑", De: "Bearbeiten", Fr: "Modifier"}
	msgOpenTerminal  = i18n.Msg{En: "Open terminal", Es: "Abrir terminal", Zh: "打开终端", De: "Terminal öffnen", Fr: "Ouvrir un terminal"}
	msgRemove        = i18n.Msg{En: "Remove", Es: "Quitar", Zh: "移除", De: "Entfernen", Fr: "Retirer"}
	msgConfirmRemove = i18n.Msg{En: "Remove the workspace \"{name}\"?", Es: "¿Quitar el workspace \"{name}\"?", Zh: "要移除工作区“{name}”吗？", De: "Workspace „{name}“ entfernen?", Fr: "Retirer l'espace de travail « {name} » ?"}

	msgName                   = i18n.Msg{En: "Name", Es: "Nombre", Zh: "名称", De: "Name", Fr: "Nom"}
	msgNamePlaceholder        = i18n.Msg{En: "my-project", Es: "mi-proyecto", Zh: "我的项目", De: "mein-projekt", Fr: "mon-projet"}
	msgPath                   = i18n.Msg{En: "Path", Es: "Ruta", Zh: "路径", De: "Pfad", Fr: "Chemin"}
	msgAbsolutePath           = i18n.Msg{En: "Absolute path (created if it does not exist)", Es: "Ruta absoluta (se crea si no existe)", Zh: "绝对路径（不存在时会创建）", De: "Absoluter Pfad (wird erstellt, falls nicht vorhanden)", Fr: "Chemin absolu (créé s'il n'existe pas)"}
	msgProjectPathPlaceholder = i18n.Msg{En: "/home/user/projects/my-project", Es: "/home/usuario/proyectos/mi-proyecto", Zh: "/home/用户/项目/我的项目", De: "/home/benutzer/projekte/mein-projekt", Fr: "/home/utilisateur/projets/mon-projet"}
	msgChoose                 = i18n.Msg{En: "Choose…", Es: "Elegir…", Zh: "选择…", De: "Auswählen…", Fr: "Choisir…"}
	msgBaseBranch             = i18n.Msg{En: "Base branch (empty = current branch)", Es: "Rama base (vacío = rama actual)", Zh: "基础分支（留空 = 当前分支）", De: "Basis-Branch (leer = aktueller Branch)", Fr: "Branche de base (vide = branche actuelle)"}
	msgInitialAgents          = i18n.Msg{En: "Initial AGENTS.md (optional)", Es: "AGENTS.md inicial (opcional)", Zh: "初始 AGENTS.md（可选）", De: "Initiale AGENTS.md (optional)", Fr: "AGENTS.md initial (facultatif)"}
	msgNoAgents               = i18n.Msg{En: "No AGENTS.md", Es: "Sin AGENTS.md", Zh: "无 AGENTS.md", De: "Keine AGENTS.md", Fr: "Sans AGENTS.md"}
	msgRegister               = i18n.Msg{En: "Register", Es: "Registrar", Zh: "注册", De: "Registrieren", Fr: "Enregistrer"}

	msgMissingFolder       = i18n.Msg{En: "The folder {path} does not exist. Create it and initialize a git repository?", Es: "La carpeta {path} no existe. ¿Crearla e inicializar un repositorio git?", Zh: "文件夹 {path} 不存在。是否创建并初始化 git 仓库？", De: "Der Ordner {path} existiert nicht. Erstellen und ein Git-Repository initialisieren?", Fr: "Le dossier {path} n'existe pas. Le créer et initialiser un dépôt git ?"}
	msgNotAGitRepository   = i18n.Msg{En: "{path} is not a git repository. Initialize it?", Es: "{path} no es un repositorio git. ¿Inicializarlo?", Zh: "{path} 不是 git 仓库。是否初始化？", De: "{path} ist kein Git-Repository. Initialisieren?", Fr: "{path} n'est pas un dépôt git. L'initialiser ?"}
	msgNoCommits           = i18n.Msg{En: "The repository {path} has no commits. Create an empty initial commit?", Es: "El repositorio {path} no tiene commits. ¿Crear un commit inicial vacío?", Zh: "仓库 {path} 没有提交。是否创建一个空的初始提交？", De: "Das Repository {path} hat keine Commits. Einen leeren initialen Commit erstellen?", Fr: "Le dépôt {path} n'a aucun commit. Créer un commit initial vide ?"}
	msgCreateAndInitialize = i18n.Msg{En: "Create and initialize", Es: "Crear e inicializar", Zh: "创建并初始化", De: "Erstellen und initialisieren", Fr: "Créer et initialiser"}
	msgInitialize          = i18n.Msg{En: "Initialize", Es: "Inicializar", Zh: "初始化", De: "Initialisieren", Fr: "Initialiser"}
	msgCreateCommit        = i18n.Msg{En: "Create commit", Es: "Crear commit", Zh: "创建提交", De: "Commit erstellen", Fr: "Créer un commit"}

	msgChooseFolder          = i18n.Msg{En: "Choose folder", Es: "Elegir carpeta", Zh: "选择文件夹", De: "Ordner auswählen", Fr: "Choisir un dossier"}
	msgFolderPathPlaceholder = i18n.Msg{En: "/path/to/folder", Es: "/ruta/a/la/carpeta", Zh: "/路径/到/文件夹", De: "/pfad/zum/ordner", Fr: "/chemin/vers/le/dossier"}
	msgGo                    = i18n.Msg{En: "Go", Es: "Ir", Zh: "前往", De: "Los", Fr: "Aller"}
	msgUp                    = i18n.Msg{En: "↑ Up", Es: "↑ Subir", Zh: "↑ 上一级", De: "↑ Hoch", Fr: "↑ Remonter"}
	msgUseThisFolder         = i18n.Msg{En: "Use this folder", Es: "Usar esta carpeta", Zh: "使用此文件夹", De: "Diesen Ordner verwenden", Fr: "Utiliser ce dossier"}
	msgNewFolderName         = i18n.Msg{En: "New folder name", Es: "Nombre de la nueva carpeta", Zh: "新文件夹名称", De: "Name des neuen Ordners", Fr: "Nom du nouveau dossier"}
	msgNewFolderHere         = i18n.Msg{En: "New folder here", Es: "Nueva carpeta aquí", Zh: "在此新建文件夹", De: "Neuer Ordner hier", Fr: "Nouveau dossier ici"}
	msgCreate                = i18n.Msg{En: "Create", Es: "Crear", Zh: "创建", De: "Erstellen", Fr: "Créer"}
	msgFilterFolders         = i18n.Msg{En: "Filter folders…", Es: "Filtrar carpetas…", Zh: "筛选文件夹…", De: "Ordner filtern…", Fr: "Filtrer les dossiers…"}
	msgFilterFoldersLabel    = i18n.Msg{En: "Filter folders", Es: "Filtrar carpetas", Zh: "筛选文件夹", De: "Ordner filtern", Fr: "Filtrer les dossiers"}
	msgNoSubfolders          = i18n.Msg{En: "No subfolders", Es: "Sin subcarpetas", Zh: "没有子文件夹", De: "Keine Unterordner", Fr: "Aucun sous-dossier"}

	msgAgentsLoadFailed = i18n.Msg{En: "Could not load the AGENTS.md files: {error}", Es: "No se pudieron cargar los ficheros AGENTS.md: {error}", Zh: "无法加载 AGENTS.md 文件：{error}", De: "Die AGENTS.md-Dateien konnten nicht geladen werden: {error}", Fr: "Impossible de charger les fichiers AGENTS.md : {error}"}
)

// AgentsLoadFailedText is the notice shown when the stored AGENTS.md files cannot be listed.
func AgentsLoadFailedText(ctx context.Context, err error) string {
	return msgAgentsLoadFailed.Fmt(ctx, map[string]string{"error": err.Error()})
}
