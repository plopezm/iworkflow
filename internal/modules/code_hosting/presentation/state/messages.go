package state

import "iworkflow/internal/core/i18n"

// User-facing texts of the code hosting screens. Each one copies the Rust Msg constant of the same
// name in src/modules/code_hosting/presentation/components/*.rs and provider_text.rs.

// Settings card.
var (
	msgTokenKeepPlaceholder = i18n.Msg{En: "•••••• (leave empty to keep it)", Es: "•••••• (déjalo vacío para conservarlo)", Zh: "••••••（留空以保留现有令牌）", De: "•••••• (leer lassen, um es beizubehalten)", Fr: "•••••• (laissez vide pour le conserver)"}
	msgURLOf                = i18n.Msg{En: "{provider} URL", Es: "URL de {provider}", Zh: "{provider} URL", De: "{provider}-URL", Fr: "URL de {provider}"}
	msgTokenLabel           = i18n.Msg{En: "Personal access token", Es: "Token de acceso personal", Zh: "个人访问令牌", De: "Persönliches Zugriffstoken", Fr: "Jeton d'accès personnel"}
	msgInsecureTLS          = i18n.Msg{En: "Accept untrusted TLS certificate (internal CA or self-signed)", Es: "Aceptar certificado TLS no confiable (CA interna o autofirmado)", Zh: "接受不受信任的 TLS 证书（内部 CA 或自签名）", De: "Nicht vertrauenswürdiges TLS-Zertifikat akzeptieren (interne CA oder selbstsigniert)", Fr: "Accepter un certificat TLS non approuvé (CA interne ou auto-signé)"}
	msgSave                 = i18n.Msg{En: "Save", Es: "Guardar", Zh: "保存", De: "Speichern", Fr: "Enregistrer"}
	msgTestConnection       = i18n.Msg{En: "Test connection", Es: "Probar conexión", Zh: "测试连接", De: "Verbindung testen", Fr: "Tester la connexion"}
	msgConnectedAs          = i18n.Msg{En: "Connected as {user}", Es: "Conectado como {user}", Zh: "已连接为 {user}", De: "Verbunden als {user}", Fr: "Connecté en tant que {user}"}
)

// Provider help texts.
var (
	msgHelpGitHub = i18n.Msg{En: "github.com or your GitHub Enterprise (base URL) and a personal token (repo scope; fine-grained: Pull requests and Contents read/write, Actions read). It is used to create pull requests, view CI and merge from each task; the repository is inferred from the origin remote. If the task branch is not on origin yet, the app pushes it before creating the request.", Es: "github.com o tu GitHub Enterprise (URL base) y un token personal (scope repo; fine-grained: Pull requests y Contents en lectura/escritura, Actions en lectura). Se usa para crear pull requests, ver el CI y mezclar desde cada tarea; el repositorio se deduce del remoto origin. Si la rama de la tarea aún no está en origin, la app la publica (push) antes de crear la solicitud.", Zh: "github.com 或你的 GitHub Enterprise（基础 URL）以及个人令牌（repo 范围；fine-grained：Pull requests 和 Contents 读写权限，Actions 读取权限）。用于在每个任务中创建 pull requests、查看 CI 和合并；仓库由 origin 远程地址推断。如果任务分支尚未在 origin 上，应用会在创建请求前先将其 push。", De: "github.com oder dein GitHub Enterprise (Basis-URL) und ein persönliches Token (Scope repo; fine-grained: Pull requests und Contents Lesen/Schreiben, Actions Lesen). Es wird verwendet, um Pull Requests zu erstellen, die CI einzusehen und aus jeder Aufgabe heraus zu mergen; das Repository wird aus dem Remote origin abgeleitet. Ist der Task-Branch noch nicht auf origin, pusht die App ihn vor dem Erstellen der Anfrage.", Fr: "github.com ou votre GitHub Enterprise (URL de base) et un jeton personnel (scope repo ; fine-grained : Pull requests et Contents en lecture/écriture, Actions en lecture). Il sert à créer des pull requests, consulter la CI et fusionner depuis chaque tâche ; le dépôt est déduit du remote origin. Si la branche de la tâche n'est pas encore sur origin, l'application la publie (push) avant de créer la demande."}
	msgHelpGitLab = i18n.Msg{En: "URL of your on-premise GitLab and a personal token (api scope). It is used to create merge requests, view CI and merge from each task; the project is inferred from the repository's origin remote. If the task branch is not on origin yet, the app pushes it before creating the request.", Es: "URL de tu GitLab on-premise y un token personal (scope api). Se usa para crear merge requests, ver el CI y mezclar desde cada tarea; el proyecto se deduce del remoto origin del repositorio. Si la rama de la tarea aún no está en origin, la app la publica (push) antes de crear la solicitud.", Zh: "你的本地部署 GitLab 的 URL 以及个人令牌（api 范围）。用于在每个任务中创建 merge requests、查看 CI 和合并；项目由仓库的 origin 远程地址推断。如果任务分支尚未在 origin 上，应用会在创建请求前先将其 push。", De: "URL deines On-Premise-GitLab und ein persönliches Token (Scope api). Es wird verwendet, um Merge Requests zu erstellen, die CI einzusehen und aus jeder Aufgabe heraus zu mergen; das Projekt wird aus dem Remote origin des Repositorys abgeleitet. Ist der Task-Branch noch nicht auf origin, pusht die App ihn vor dem Erstellen der Anfrage.", Fr: "URL de votre GitLab on-premise et un jeton personnel (scope api). Il sert à créer des merge requests, consulter la CI et fusionner depuis chaque tâche ; le projet est déduit du remote origin du dépôt. Si la branche de la tâche n'est pas encore sur origin, l'application la publie (push) avant de créer la demande."}
)

// Merge request panel.
var (
	msgNoPushNote         = i18n.Msg{En: "If the branch is not on origin yet, it is pushed automatically.", Es: "Si la rama aún no está en origin, se publica automáticamente (push).", Zh: "如果分支尚未在 origin 上，将自动执行 push 发布。", De: "Ist der Branch noch nicht auf origin, wird er automatisch gepusht.", Fr: "Si la branche n'est pas encore sur origin, elle est publiée automatiquement (push)."}
	msgHasConflicts       = i18n.Msg{En: "has conflicts", Es: "con conflictos", Zh: "存在冲突", De: "mit Konflikten", Fr: "avec conflits"}
	msgOpenIn             = i18n.Msg{En: "Open in {provider}", Es: "Abrir en {provider}", Zh: "在 {provider} 中打开", De: "In {provider} öffnen", Fr: "Ouvrir dans {provider}"}
	msgOpenRequestIn      = i18n.Msg{En: "Open {abbrev} in {provider}", Es: "Abrir {abbrev} en {provider}", Zh: "在 {provider} 中打开 {abbrev}", De: "{abbrev} in {provider} öffnen", Fr: "Ouvrir la {abbrev} dans {provider}"}
	msgMergeRequestButton = i18n.Msg{En: "Merge {abbrev}", Es: "Mezclar {abbrev}", Zh: "合并 {abbrev}", De: "{abbrev} mergen", Fr: "Fusionner la {abbrev}"}
	msgMergeConfirm       = i18n.Msg{En: "Merge the {abbrev} into {provider}? The target branch will be updated on the server.", Es: "¿Mezclar el {abbrev} en {provider}? La rama destino se actualizará en el servidor.", Zh: "要将 {abbrev} 合并到 {provider} 吗？目标分支将在服务器上更新。", De: "{abbrev} in {provider} mergen? Der Zielbranch wird auf dem Server aktualisiert.", Fr: "Fusionner la {abbrev} dans {provider} ? La branche cible sera mise à jour sur le serveur."}
	msgPushBranch         = i18n.Msg{En: "Push branch", Es: "Subir rama (push)", Zh: "推送分支", De: "Branch pushen", Fr: "Pousser la branche"}
	msgPushDone           = i18n.Msg{En: "Branch pushed to origin.", Es: "Rama subida a origin.", Zh: "分支已推送到 origin。", De: "Branch wurde zu origin gepusht.", Fr: "Branche poussée vers origin."}
	msgNoPipeline         = i18n.Msg{En: "No CI pipeline.", Es: "Sin pipeline de CI.", Zh: "没有 CI 流水线。", De: "Keine CI-Pipeline.", Fr: "Aucun pipeline de CI."}
	msgMerge              = i18n.Msg{En: "Merge", Es: "Mezclar", Zh: "合并", De: "Mergen", Fr: "Fusionner"}
	msgNoMRYet            = i18n.Msg{En: "This branch has no merge/pull request yet.", Es: "Esta rama aún no tiene merge/pull request.", Zh: "此分支还没有 merge/pull request。", De: "Dieser Branch hat noch keinen Merge/Pull Request.", Fr: "Cette branche n'a pas encore de merge/pull request."}
	msgTitleOptional      = i18n.Msg{En: "Title (optional, defaults to the task's)", Es: "Título (opcional, por defecto el de la tarea)", Zh: "标题（可选，默认为任务标题）", De: "Titel (optional, standardmäßig der der Aufgabe)", Fr: "Titre (facultatif, par défaut celui de la tâche)"}
	msgCreateMR           = i18n.Msg{En: "Create merge/pull request", Es: "Crear merge/pull request", Zh: "创建 merge/pull request", De: "Merge/Pull Request erstellen", Fr: "Créer une merge/pull request"}
	msgPanelTitle         = i18n.Msg{En: "Merge / Pull request", Es: "Merge / Pull request", Zh: "Merge / Pull request", De: "Merge / Pull Request", Fr: "Merge / Pull request"}
	msgRefresh            = i18n.Msg{En: "Refresh", Es: "Actualizar", Zh: "刷新", De: "Aktualisieren", Fr: "Actualiser"}
)

// Request, pipeline and job states.
var (
	msgStatusSuccess     = i18n.Msg{En: "successful", Es: "correcto", Zh: "成功", De: "erfolgreich", Fr: "réussi"}
	msgStatusFailed      = i18n.Msg{En: "failed", Es: "fallido", Zh: "失败", De: "fehlgeschlagen", Fr: "échoué"}
	msgStatusRunning     = i18n.Msg{En: "running", Es: "en curso", Zh: "运行中", De: "läuft", Fr: "en cours"}
	msgStatusPending     = i18n.Msg{En: "pending", Es: "pendiente", Zh: "等待中", De: "ausstehend", Fr: "en attente"}
	msgStatusCreated     = i18n.Msg{En: "created", Es: "creado", Zh: "已创建", De: "erstellt", Fr: "créé"}
	msgStatusCanceled    = i18n.Msg{En: "canceled", Es: "cancelado", Zh: "已取消", De: "abgebrochen", Fr: "annulé"}
	msgStatusSkipped     = i18n.Msg{En: "skipped", Es: "omitido", Zh: "已跳过", De: "übersprungen", Fr: "ignoré"}
	msgStatusManual      = i18n.Msg{En: "manual", Es: "manual", Zh: "手动", De: "manuell", Fr: "manuel"}
	msgStatusOpened      = i18n.Msg{En: "open", Es: "abierto", Zh: "打开", De: "offen", Fr: "ouvert"}
	msgStatusMerged      = i18n.Msg{En: "merged", Es: "mezclado", Zh: "已合并", De: "gemergt", Fr: "fusionné"}
	msgStatusClosed      = i18n.Msg{En: "closed", Es: "cerrado", Zh: "已关闭", De: "geschlossen", Fr: "fermé"}
	msgStatusMergeable   = i18n.Msg{En: "can be merged", Es: "se puede mezclar", Zh: "可以合并", De: "kann gemergt werden", Fr: "peut être fusionné"}
	msgStatusCannotMerge = i18n.Msg{En: "cannot be merged", Es: "no se puede mezclar", Zh: "无法合并", De: "kann nicht gemergt werden", Fr: "ne peut pas être fusionné"}
	msgStatusConflict    = i18n.Msg{En: "has conflicts", Es: "con conflictos", Zh: "存在冲突", De: "mit Konflikten", Fr: "avec conflits"}
	msgStatusCIRunning   = i18n.Msg{En: "CI running", Es: "CI en curso", Zh: "CI 运行中", De: "CI läuft", Fr: "CI en cours"}
	msgStatusCIMustPass  = i18n.Msg{En: "CI must pass", Es: "el CI debe pasar", Zh: "CI 必须通过", De: "CI muss bestehen", Fr: "la CI doit réussir"}
	msgStatusNotApproved = i18n.Msg{En: "pending approval", Es: "pendiente de aprobación", Zh: "等待批准", De: "Freigabe ausstehend", Fr: "en attente d'approbation"}
	msgStatusDiscussions = i18n.Msg{En: "unresolved discussions", Es: "discusiones sin resolver", Zh: "讨论未解决", De: "ungelöste Diskussionen", Fr: "discussions non résolues"}
	msgStatusChecking    = i18n.Msg{En: "checking", Es: "comprobando", Zh: "检查中", De: "wird geprüft", Fr: "vérification"}
)
