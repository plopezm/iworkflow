// Package app is the composition root: it builds the feature modules, wires their ports to each other
// and assembles the HTTP router (the dock shell, every module's routes and the shared middlewares).
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"iworkflow/internal/core/config"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/listen"
	"iworkflow/internal/core/restart"
	"iworkflow/internal/core/web"
	agentprofilesdi "iworkflow/internal/modules/agent_profiles/infrastructure/di"
	agentprofilespages "iworkflow/internal/modules/agent_profiles/presentation/pages"
	agentsettingsdi "iworkflow/internal/modules/agent_settings/infrastructure/di"
	agentsettingspages "iworkflow/internal/modules/agent_settings/presentation/pages"
	agentsfilesdi "iworkflow/internal/modules/agents_files/infrastructure/di"
	agentsfilespages "iworkflow/internal/modules/agents_files/presentation/pages"
	codehostingapp "iworkflow/internal/modules/code_hosting/application"
	codehostingmodels "iworkflow/internal/modules/code_hosting/domain/models"
	codehostingdi "iworkflow/internal/modules/code_hosting/infrastructure/di"
	prefsinbound "iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
	prefssystem "iworkflow/internal/modules/preferences/infrastructure/adapters/outbound/system"
	prefsdi "iworkflow/internal/modules/preferences/infrastructure/di"
	prefspages "iworkflow/internal/modules/preferences/presentation/pages"
	projecttreedi "iworkflow/internal/modules/project_tree/infrastructure/di"
	tasksdi "iworkflow/internal/modules/tasks/infrastructure/di"
	terminaldi "iworkflow/internal/modules/terminal/infrastructure/di"
	terminalpages "iworkflow/internal/modules/terminal/presentation/pages"
	workspacesdi "iworkflow/internal/modules/workspaces/infrastructure/di"
	workspacespages "iworkflow/internal/modules/workspaces/presentation/pages"
)

// Deps are the process-level inputs of the composition root.
type Deps struct {
	DB     *sql.DB
	Config config.Config
	Logger *slog.Logger
	// StaticDir holds the built assets served under /static/.
	StaticDir string
	// Listen describes the address the server listens on and whether it demands the token.
	Listen listen.Info
	// Preferences are the saved settings read before the server starts (see LoadPreferences).
	Preferences Preferences
	// Restart is the handle the settings screen uses to ask for a restart.
	Restart *restart.Handle
}

// App is the assembled server: the router and the modules it mounts.
type App struct {
	logger     *slog.Logger
	staticDir  string
	guard      *prefsinbound.AccessGuard
	prefs      *prefsdi.Module
	workspaces *workspacesdi.Module
	terminal   *terminaldi.Module
	tasks      *tasksdi.Module
	loops      *taskLoops
	modules    []web.Module
	dock       *dock.Handlers
}

// New composes the modules. The order matters only for the ports the modules share: the workspaces
// and agents_files modules need each other, so the project lookup is bound after both are built, and the
// workspaces and tasks modules need each other, so the workspace cascade is bound once tasks exists.
// New starts nothing: Start runs the tasks startup steps and the background loops.
func New(deps Deps) (*App, error) {
	guard := prefsinbound.NewAccessGuard(deps.Preferences.Token, deps.Listen.TokenRequired)

	projects := &projectLookup{}
	agentsFiles := agentsfilesdi.NewModule(agentsfilesdi.Deps{
		DB:         deps.DB,
		StorageDir: deps.Config.AgentsFilesDir,
		Projects:   projects,
		Logger:     deps.Logger,
	})
	workspaceTasks := &workspaceTasksBridge{}
	workspaces := workspacesdi.NewModule(workspacesdi.Deps{
		DB:          deps.DB,
		HomeDir:     deps.Config.HomeDir,
		Tasks:       workspaceTasks,
		AgentsFiles: &agentsFilesBridge{module: agentsFiles},
	})
	projects.list = workspaces.List
	projects.get = workspaces.Get

	prefs := prefsdi.NewModule(prefsdi.Deps{
		DB:        deps.DB,
		Logger:    deps.Logger,
		Guard:     guard,
		Network:   prefssystem.NewNetworkDetector(deps.Logger),
		Runtime:   prefssystem.NewServerRuntime(deps.Listen),
		Restarter: prefssystem.NewServerRestarter(deps.Restart),
	})
	settings := agentsettingsdi.NewModule(agentsettingsdi.Dependencies{
		DB:                 deps.DB,
		GentleShellBin:     deps.Config.GentleShellBin,
		PiBin:              deps.Config.PiBin,
		CursorBin:          deps.Config.CursorBin,
		GentlePiConfigHome: deps.Config.GentlePiConfigHome,
	})

	// The remote merge is shared: the tasks module merges through it and code_hosting binds its use case to it.
	remoteMerge := codehostingapp.NewRemoteMerge()
	tasks, err := tasksdi.NewModule(tasksdi.Deps{
		DB:             deps.DB,
		Workspaces:     &taskWorkspaceLookup{get: workspaces.Get, list: workspaces.List},
		AttachmentsDir: deps.Config.AttachmentsDir,
		WorktreesDir:   deps.Config.WorktreesDir,
		Binaries: tasksdi.RuntimeBinaries{
			GentleShell:       deps.Config.GentleShellBin,
			Pi:                deps.Config.PiBin,
			Claude:            deps.Config.ClaudeBin,
			Cursor:            deps.Config.CursorBin,
			CursorSessionsDir: deps.Config.CursorSessionsDir,
		},
		LaunchSettings: &launchSettings{prepareLaunch: settings.PrepareLaunch},
		Remote:         &remoteMergeBridge{inner: remoteMerge},
	})
	if err != nil {
		return nil, fmt.Errorf("compose tasks: %w", err)
	}
	workspaceTasks.deleteAll = tasks.DeleteWorkspace

	projectTree := projecttreedi.NewModule(projecttreedi.Deps{
		Workspaces: &workspaceRoots{get: workspaces.Get},
		Tasks:      &taskRootLookup{get: tasks.Get},
	})
	profiles := agentprofilesdi.NewModule(agentprofilesdi.Deps{
		ProfilesDir:  deps.Config.GentlePiConfigHome,
		AgentsDir:    deps.Config.PiAgentsDir,
		ModelOptions: &modelOptions{listModels: settings.ListModels},
	})
	codeHosting := codehostingdi.NewModule(codehostingdi.Deps{
		DB:          deps.DB,
		Tasks:       &taskSourceLookup{get: tasks.Get, workspaces: workspaces.Get},
		RemoteMerge: remoteMerge,
	})
	terminal := terminaldi.NewModule(terminaldi.Dependencies{
		WorkspaceResolver: workspaceTargets(workspaces.Get),
		TaskResolver:      taskTargets(tasks.Get),
		HomeDir:           deps.Config.HomeDir,
		Logger:            deps.Logger,
	})

	routes := make([]dock.Route, 0, 24)
	// The board owns the home address, so it precedes the not-migrated catch-alls below.
	routes = append(routes, tasks.Tabs()...)
	routes = append(routes, agentsFiles.Tabs()...)
	routes = append(routes, workspaces.Tabs()...)
	routes = append(routes, projectTree.Tabs()...)
	routes = append(routes, profiles.Tabs()...)
	routes = append(routes, settings.Tabs(settingsSections(settings, codeHosting))...)
	// The terminal tabs must precede the not-migrated catch-alls: the first matching pattern wins.
	routes = append(routes, terminal.Tabs()...)
	routes = append(routes, notMigratedRoutes()...)
	registry, err := dock.NewRegistry(routes...)
	if err != nil {
		return nil, fmt.Errorf("register dock tabs: %w", err)
	}

	return &App{
		logger:     deps.Logger,
		staticDir:  deps.StaticDir,
		guard:      guard,
		prefs:      prefs,
		workspaces: workspaces,
		terminal:   terminal,
		tasks:      tasks,
		loops:      newTaskLoops(),
		modules:    []web.Module{prefs, agentsFiles, workspaces, settings, projectTree, profiles, codeHosting, terminal, tasks},
		dock:       dock.NewHandlers(registry, launcherMenu(), emptyGroup()),
	}, nil
}

// Shutdown releases what the modules hold while the server runs, in the order of the Rust main. The caller
// stops the HTTP server first, so no request starts work meanwhile. It stops the background loops and waits for
// them, stops every live agent, waits for the worktree removals, persists the pending agent events, and last kills
// the live terminal sessions, so no shell outlives the process. Each step has its own budget of shutdownStepTimeout,
// so a slow step cannot starve the ones after it; ctx is the global upper bound. A step that runs out of its budget is
// reported and the next one still runs.
func (a *App) Shutdown(ctx context.Context) error {
	return runShutdownSteps(ctx, shutdownStepTimeout, a.shutdownSteps())
}

// shutdownSteps are the steps of Shutdown, in order.
func (a *App) shutdownSteps() []shutdownStep {
	return []shutdownStep{
		{name: "stop the background loops", run: a.stopLoops},
		{name: "stop the agents", run: func(ctx context.Context) error { a.tasks.StopAgents(ctx); return nil }},
		{name: "wait for the worktree removals", run: func(ctx context.Context) error { a.tasks.WaitWorktreeCleanup(ctx); return nil }},
		{name: "persist the agent events", run: func(context.Context) error { a.tasks.FlushEvents(); return nil }},
		{name: "close the agent event queue", run: func(context.Context) error { a.tasks.ShutdownEvents(); return nil }},
		{name: "shutdown terminals", run: a.terminal.Shutdown},
	}
}

// ShutdownBudget is the global upper bound the caller gives Shutdown: every step may use its whole budget and there
// is still slack for the last one, so a run of slow steps never cuts the terminals short.
func (a *App) ShutdownBudget() time.Duration {
	return time.Duration(len(a.shutdownSteps()))*shutdownStepTimeout + shutdownSlack
}

// Router builds the HTTP router. Middlewares run in the order of the Rust server: the cross-site
// guard and the access check wrap everything, including the login route and the static assets.
func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(prefsinbound.RejectCrossSiteWrites, prefsinbound.RequireAccess(a.guard))
	r.Use(
		middleware.RequestID,
		web.RequestLog(a.logger),
		web.Recover(a.logger),
		web.BodyLimit(web.DefaultBodyBytes),
		web.Locale,
		prefsinbound.SyncLanguageCookie(a.logger, a.prefs.GetLanguage, a.prefs.SaveLanguage),
	)
	r.Handle("/static/*", web.StaticHandler(a.staticDir))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Handle(prefsinbound.LoginPath, prefsinbound.AccessRouter(a.guard))
	a.dock.Routes(r)
	for _, module := range a.modules {
		module.Routes(r)
	}
	// Every other GET is a page visit: the shell loads, and dock.js opens the path as a tab.
	r.Get("/", a.page)
	r.NotFound(a.page)
	return r
}

// page renders the shell document. Pages are fragments the dock loads; the shell only hosts them.
//
//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
func (a *App) page(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, shellPage(a.tasks.Sidebar(r.Context())))
}

// settingsSections are the sections of the settings screen, in display order.
func settingsSections(settings *agentsettingsdi.Module, codeHosting *codehostingdi.Module) []agentsettingspages.SettingsSection {
	return []agentsettingspages.SettingsSection{
		{
			ID:    "language",
			Label: msgSettingsLang.T,
			Content: func(ctx context.Context) templ.Component {
				return prefspages.LanguageSection(i18n.StateFrom(ctx).Locale)
			},
		},
		settings.ResourcesSection(msgResources.T),
		{
			ID:    "network",
			Label: msgSettingsNet.T,
			Content: func(context.Context) templ.Component {
				return prefspages.NetworkSection()
			},
		},
		notMigratedSection("jira", "Jira"),
		{
			ID:      "gitlab",
			Label:   func(context.Context) string { return "GitLab" },
			Content: codeHosting.SettingsContent(codehostingmodels.GitLab),
		},
		{
			ID:      "github",
			Label:   func(context.Context) string { return "GitHub" },
			Content: codeHosting.SettingsContent(codehostingmodels.GitHub),
		},
	}
}

// notMigratedSection is a settings section whose module is not migrated yet.
func notMigratedSection(id string, label string) agentsettingspages.SettingsSection {
	return agentsettingspages.SettingsSection{
		ID:      id,
		Label:   func(context.Context) string { return label },
		Content: func(context.Context) templ.Component { return notMigratedPane() },
	}
}

// Dock paths of the migrated screens. The modules own the same paths for their tabs.
const (
	boardPath       = "/"
	workspacesPath  = "/workspaces"
	agentsFilesPath = "/agents-files"
	profilesPath    = "/profiles"
	modelsPath      = "/models"
	terminalsPath   = "/terminals"
)

// navEntry is a top-level destination of the sidebar and of the "+" launcher. Only migrated screens
// are listed; the others are reachable by address and show the not-migrated pane.
type navEntry struct {
	Href  string
	Icon  string
	Title func(ctx context.Context) string
}

// navEntries are the migrated top-level destinations, in display order.
var navEntries = []navEntry{
	{Href: boardPath, Icon: "▦", Title: msgTitleBoard.T},
	{Href: workspacesPath, Icon: "▤", Title: workspacespages.ListTitle},
	{Href: agentsFilesPath, Icon: "§", Title: agentsfilespages.TabTitle.T},
	{Href: profilesPath, Icon: "☰", Title: agentprofilespages.TabTitle.T},
	{Href: modelsPath, Icon: "◇", Title: agentprofilespages.ModelsTitle.T},
	{Href: terminalsPath, Icon: "❯", Title: terminalpages.TerminalsTitle},
	{Href: agentsettingspages.SettingsPath, Icon: "⚙", Title: agentsettingspages.SettingsTitle},
}

// notMigratedRoutes are the screens that still belong to the Rust version. They keep their addresses
// (so a bookmark opens a tab) and explain the state instead of a 404.
func notMigratedRoutes() []dock.Route {
	return []dock.Route{
		notMigratedRoute("/jira", "◈", msgTitleJira),
		notMigratedRoute("/jira/{rest...}", "◈", msgTitleJira),
		notMigratedRoute("/tasks/{rest...}", "●", msgTitleTasks),
	}
}

func notMigratedRoute(pattern string, icon string, title i18n.Msg) dock.Route {
	return dock.Route{
		Pattern: pattern,
		Icon:    icon,
		Title: func(ctx context.Context, _ dock.Params) string {
			return title.T(ctx)
		},
		Render: func(context.Context, dock.Params) templ.Component {
			return notMigratedPane()
		},
	}
}

// launcherMenu is the entry list of the "+" menu: the migrated destinations.
func launcherMenu() templ.Component {
	return launcherEntries()
}

// emptyGroup is the body of a group without tabs.
func emptyGroup() templ.Component {
	return emptyGroupBody()
}
