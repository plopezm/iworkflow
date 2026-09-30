# dark-mode-gentle-theme

## Objective
Add a dark mode using the gentle-ai "Gentle" palette, next to the existing cream/purple light theme,
with a user toggle (light / dark / system) persisted in `localStorage`, applied before first paint
(no flash) and hydration-safe.

## Palette source
`/home/pablolm/.npm-global/lib/node_modules/gentle-pi/themes/Gentle.json` (read `vars`). Key values:
bg `#06080f`, subtle surface `#0d0f14`, border `#313342`, borderSubtle `#232A40`, selection `#232A40`,
text `#F3F6F9`, muted `#5C6170`, accent (gold) `#E0C15A`, blue `#7FB4CA`, secondary `#A3B5D6`,
heading `#B5B2D0`, green `#B7CC85`, warning `#DEBA87`, red `#CB7C94`, purple `#B99BF2`,
magenta `#C99AD6`, toolSuccessBg `#1a2e1a`, toolErrorBg `#2e1a1a`.

## Design decisions
- Keep the current token names (`--color-bg`, `surface`, `surface-2/3`, `border`, `fg`, `muted`,
  `accent`, `ok`, `warn`, `error`, `purple`, `pink`, `stage-*`). Make Tailwind utilities theme-aware by
  having `@theme` tokens reference runtime CSS variables (e.g. `--color-bg: var(--c-bg)`; light values
  on `:root`, Gentle values on `:root[data-theme="dark"]`) so no component class changes are needed.
  Map: accent -> blue `#7FB4CA` (primary interactive) or gold `#E0C15A` where it reads better as
  brand/highlight; pick one consistently and document it. `color-scheme` switches with the theme.
- Replace hardcoded colors (rgba glows, shadows, `text-white`, hex in `.rs` files or CSS) with tokens
  so dark looks right. Audit diff view colors, badges, toasts, modal backdrops, scrollbars, inputs.
- Toggle: generic `ThemeToggle` component in `src/core/components/` (no imports from `modules/`).
  State = `RwSignal<ThemeMode>` provided by context from `app.rs`; pure helpers (mode <-> string,
  resolved theme) unit-tested. Persist via `src/core/browser.rs` helpers (localStorage), cfg-gated for
  hydrate. Place it in the sidebar footer/header area next to existing controls.
- No flash: inline blocking `<script>` in `shell()` `<head>` sets `data-theme` on `<html>` from
  localStorage / `prefers-color-scheme` before paint. SSR default markup must match hydration (do not
  render toggle state differing between server and client; initialize from a neutral default and
  update in an `Effect` after hydration).
- Follow AGENTS.md strictly (explicit `let` types, no unwrap/expect, no business logic in components).

## Allowed edit surfaces
src/app.rs
src/core/components/**
src/core/browser.rs
src/modules/tasks/presentation/**
src/modules/workspaces/presentation/**
src/modules/agent_settings/presentation/**
style/tailwind.css
odd/tasks/dark-mode-gentle-theme.md

## Checks
`cargo fmt --all -- --check`, `cargo clippy --features ssr --tests -- -D warnings`,
`cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings`,
`cargo test --features ssr`, `cargo leptos build`. Visual check (Playwright/Chromium) of board,
workspaces and task detail in both themes at 1440x900 and 390px; reload persistence; no console
errors; no hydration warnings. Save screenshots under `/tmp/dark-mode/` for the human review.

## Tasks
- [x] T1 Theme tokens (light + Gentle dark), hardcoded color audit, toggle + persistence + no-flash.

## Progress / evidence

### T1 (done)
- Tokens: `@theme` colors now reference `--c-*` runtime vars (`:root` light, `:root[data-theme="dark"]` Gentle). Accent = blue `#7FB4CA`
  in dark (single interactive color); new `--color-on-accent` (white light / `#06080f` dark) replaces `text-white` on primary/danger buttons.
  Deviation: dark `muted`/`stage-backlog` = `#8b91a5` (Gentle `#5C6170` too low-contrast for 13px text); `surface-2` `#151926`, `surface-3` `#232a40`.
- Audit: glows/dialog gradient now `color-mix` on tokens; modal/panel scrims keep `bg-black/*` (theme-neutral); no hex left in `.rs`.
- Toggle: `src/core/components/theme_toggle.rs` (`ThemeMode`, `provide_theme_mode`, `ThemeToggle`, `THEME_INIT_SCRIPT`), in sidebar footer.
  Persistence + DOM via `src/core/browser.rs` (wasm-bindgen `inline_js`, no Cargo change; key `pi-theme`). Mode initialized to System, synced from
  localStorage in an `Effect` after hydration; inline `<script>` in `shell()` head sets `data-theme` before paint. OS changes are not tracked live in "Auto".
- Checks: fmt OK; clippy ssr `--tests` OK; clippy hydrate wasm OK; `cargo test --features ssr` all pass (222 lib tests + integration); `cargo leptos build` OK.
- Visual (Playwright/Chromium, isolated server on :3010): board, workspaces, task detail (Por hacer state) in light+dark at 1440x900 and 390x844;
  reload keeps dark (`localStorage=dark`); first-response `data-theme=dark` with system dark and empty storage; 0 console errors/warnings.
  Screenshots: `/tmp/dark-mode/*.png`. Not covered: live transcript/diff/dialog panels and toasts (token-only changes), OS theme change while in Auto.
