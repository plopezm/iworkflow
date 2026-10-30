# Dock: tabbed workspace shell

The dock is the single-page shell of iworkflow. The page holds a tree of split groups; each group
has a tab bar and a body. Every tab shows a server-rendered page addressed by its URL path
(`/board`, `/workspaces`, `/tasks/{id}/files/{path...}`, ...). The browser keeps the layout; the
server renders it.

## How it works

- **Layout state lives in the browser**, in `localStorage` under `iwf-dock-layout`. It is the JSON
  of `dock.Layout`, identical to the Rust version's format, so an existing saved layout keeps working.
- **Every change is an htmx request.** `web/static/js/dock.js` posts the saved layout plus the action
  fields to `/dock/action/{op}`. The server applies the action (`dock.Apply`) and returns the whole
  `#dock` fragment, which htmx swaps in. The fragment carries the new layout in `data-layout`, which
  the script stores.
- **Tab content is kept alive.** Panes stay mounted (hidden) after their first activation, so unsaved
  form text survives tab switches. Before each swap, `dock.js` lifts the panes out of the DOM and
  puts them back afterwards. A pane's content is loaded with `GET /dock/pane?path=...` the first time
  the tab becomes visible.
- **The address bar** shows the focused tab path (`history.replaceState`). A direct link or reload
  reopens that path as a tab (`POST /dock/layout` with `boot`). `/` opens nothing, so the dock starts
  empty, as in the Rust version.

## Endpoints

| Method | Path                    | Form fields                                   | Response                         |
|--------|-------------------------|-----------------------------------------------|----------------------------------|
| POST   | `/dock/layout`          | `layout` (saved JSON), `boot` (optional path) | the `#dock` fragment             |
| POST   | `/dock/action/{op}`     | `layout` + the fields of the operation        | the `#dock` fragment             |
| GET    | `/dock/pane?path=...`   | `path` (query)                                | the page of the tab              |

Operations (`dock.Apply`): `open`, `open_beside`, `open_or_reuse`, `activate`, `focus`, `close_tab`,
`close_others`, `close_group`, `split_group`, `move_tab`, `reorder_tab`, `move_to_new_split`,
`set_ratio`, `set_title`, `replace`, `close_and_open`. An unknown operation returns 400.

## Registering a tab page (module contract)

A module does not touch the layout. It declares routes and lets the registry pick them:

```go
registry, err := dock.NewRegistry(
    dock.Route{
        Name:    "task",                     // reuse name, optional
        Pattern: "/tasks/{id}",              // {name} captures one segment, {name...} the rest
        Icon:    "●",                        // glyph on the tab button
        Title:   func(ctx context.Context, p dock.Params) string { return msgTaskTitle.T(ctx) },
        Render:  func(ctx context.Context, p dock.Params) templ.Component { return pages.TaskDetail(p["id"]) },
    },
)
```

- `Title` is the default tab title. A page can change it at runtime by rendering an element with
  `data-dock-set-title="New title"` inside its pane.
- A page can replace its own tab (for example after a redirect) by rendering
  `data-dock-redirect="/tasks/abc"` inside its pane.
- A route that matches nothing renders the built-in "Not found" page.
- `NewRegistry` fails on a bad pattern or a duplicated `Name`: check its error at start-up.

Pass the registry to `dock.NewHandlers(registry, launcher, empty)` and mount it with `Routes(r)`.
`launcher` is the entry list of the `+` menu (build its entries with `dock/link.Tab`, see below). `empty` is the body of an empty group. Both may be nil.

## Shell markup (the page that hosts the dock)

The shell renders `dock.Placeholder()` where the dock goes, and loads, in this order, after htmx:

```html
<script src="/static/vendor/htmx.min.js"></script>
<script type="module" src="/static/js/ui.js"></script>
<script type="module" src="/static/js/dock.js"></script>
<script type="module" src="/static/js/code.js"></script>
```

`dock.js` needs `htmx` as a global, so it must load after htmx. The `/static/js/` files are ES
modules; `package.json` in that folder marks them as such for Node tests.

## Links into tabs

Use `dock/link.Tab` for any link that should open a tab (sidebar, launcher, buttons):

```go
@link.Tab(link.Props{Href: "/jira", Class: "menu-item", Title: "Jira"}) { Jira }
```

- A plain click opens or activates the tab. Ctrl, Cmd, Shift or the middle button keep the native
  browser behaviour.
- `Beside: true` opens it in the group on the right, and only on wide viewports.
- `Reuse: "<route name>"` activates an already open tab of that route instead of opening `Href`.
- The fragment of `Href` is dropped for the tab (`/#lane-1` opens `/`).

## Components used by the dock

`internal/core/components` provides the shared pieces: `DropdownMenu` (the `+` and `⋯` menus),
`KeepAlive` (the pane wrapper), `SidebarSection`, `SidebarItem`, `AppShell`, `CollapsePanel`,
`Modal`, `ToastStack`, and the form controls. The behaviour of each is in `web/static/js/ui.js` and
is driven by `data-*` attributes documented in the templ files.

## Testing

- Go: `go test -p 2 ./internal/core/dock/...` (layout model, JSON compatibility with the Rust format,
  registry matching, actions, HTTP handlers, link markup).
- Browser helpers: `node --test web/static/js/` (dock, ui, notifications, code, follow-bottom).
- End to end: the parity scripts `end2end/dock_smoke.cjs` and `end2end/tab_state_smoke.cjs` must pass
  against the Go server once the shell and the module pages are wired.
