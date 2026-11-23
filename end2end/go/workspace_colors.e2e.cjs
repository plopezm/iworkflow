// end2end/go/workspace_colors.e2e.cjs
// Color of a workspace, end to end in the browser: the radio picker of the registration and edit forms,
// the pill (`.ws-chip.ws-<key>`) in the workspaces list, the board sidebar, the dashboard task badge, and the
// tint of the pill in light, dark and a non-default palette. Every check reads an observable effect: a DOM class or
// data attribute, a computed background color, or the value persisted by the server (read again after a reload).
// Screenshots go to SMOKE_SCREENSHOT_DIR (set by run_go.sh to <run dir>/screenshots).
// The workspace is private to this module: run_go.sh creates REPO_WORKSPACE_COLORS and the server refuses a second
// workspace on the same root. The task of step 3 is created in backlog (not started), so no agent runs.
const fs = require('fs');
const path = require('path');

const tag = Date.now().toString();
const REPO = process.env.REPO_WORKSPACE_COLORS;
const NAME = `colores-${tag}`;
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';
const TRANSPARENT = 'rgba(0, 0, 0, 0)';

// The card of the workspace in the list (article.card, one per workspace).
const card = (page) => page.locator('article.card', { hasText: NAME }).first();
// The picker of the open form (registration or edit): the radio inputs are named "color".
const picker = (page) => page.locator('[data-testid="workspace-color-picker"]');
const checkedColor = (page) =>
  page.locator('[data-testid="workspace-color-picker"] input[name="color"]:checked').evaluate((el) => el.value);
const pickColor = (page, key) =>
  page.locator(`[data-testid="workspace-color-picker"] input[name="color"][value="${key}"]`).check();
const sidebarChip = (page) => page.locator('#board-sidebar [data-workspace-color]', { hasText: NAME }).first();

// Polls `read()` until it returns `expected` (or the timeout ends). Returns the last value read.
async function waitFor(read, expected, { timeout = 12000 } = {}) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    last = await read().catch(() => undefined);
    if (JSON.stringify(last) === JSON.stringify(expected)) return last;
    await new Promise((r) => setTimeout(r, 200));
  }
  return last;
}

// Waits until the workspace card shows the pill of `key` (or no pill when key is ''): one [data-workspace-color]
// element with the ws-chip class, or none at all.
async function expectListColor(ctx, key) {
  const { page } = ctx;
  const expected = key ? [{ key, chip: true }] : [];
  const got = await waitFor(
    () => card(page).locator('[data-workspace-color]').evaluateAll((els) =>
      els.map((el) => ({ key: el.dataset.workspaceColor, chip: el.classList.contains('ws-chip') })),
    ),
    expected,
  );
  ctx.assert(
    JSON.stringify(got) === JSON.stringify(expected),
    `lista: pill esperado ${JSON.stringify(expected)}, obtenido ${JSON.stringify(got)}`,
  );
  const chips = await card(page).locator('.ws-chip').count();
  ctx.assert(chips === (key ? 1 : 0), `lista: .ws-chip esperado ${key ? 1 : 0}, obtenido ${chips}`);
}

// Tint of an element: its computed background must not be transparent (the chip is a color-mix of --ws).
async function tintOf(locator) {
  return locator.evaluate((el) => getComputedStyle(el).backgroundColor);
}

async function openEdit(ctx) {
  const { page } = ctx;
  await ctx.goto('/workspaces');
  await card(page).getByRole('link', { name: /Editar|Edit/ }).first().click();
  await page.locator('form[class*="card"], form.card').getByLabel('Nombre').first().waitFor({ timeout: 10000 });
  await page.waitForTimeout(400);
}

async function saveEdit(ctx) {
  const { page } = ctx;
  await page.locator('form[class*="card"], form.card').getByRole('button', { name: /Guardar cambios|Save changes/ }).first().click();
  await page.waitForTimeout(1200);
}

async function setTheme(ctx, want) {
  // The theme cycle button (system -> light -> dark -> system) is on the board; it stores the choice in localStorage.
  const { page } = ctx;
  for (let i = 0; i < 3; i++) {
    const stored = await page.evaluate(() => localStorage.getItem('pi-theme'));
    if (stored === want) break;
    await page.locator('[data-theme-cycle]:visible').first().click();
    await page.waitForTimeout(200);
  }
  const stored = await page.evaluate(() => localStorage.getItem('pi-theme'));
  ctx.assert(stored === want, `tema persistido esperado ${want}, obtenido ${stored}`);
  const applied = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
  ctx.assert(applied === want, `data-theme del documento esperado ${want}, obtenido ${applied}`);
}

module.exports = {
  name: 'workspace_colors',
  tests: [
    {
      name: 'colores: el selector del registro y de la edición reflejan el color guardado en listas, barra, tablero y tema',
      fn: async (ctx) => {
        const { page } = ctx;
        ctx.assert(REPO, 'falta REPO_WORKSPACE_COLORS (repositorio del módulo workspace_colors)');
        fs.mkdirSync(SHOTS, { recursive: true });
        const shot = (file) => page.screenshot({ path: path.join(SHOTS, file) });

        // ---- 1. Registro con color ----
        await ctx.goto('/workspaces/new');
        await page.getByLabel('Nombre').first().fill(NAME);
        await page.getByLabel(/^Ruta absoluta/).first().fill(REPO);
        await page.getByLabel(/Rama base/).first().fill('main');
        ctx.assert((await checkedColor(page)) === '', 'el registro debe empezar sin color (selección "Sin color")');
        await pickColor(page, 'teal');
        ctx.assert((await checkedColor(page)) === 'teal', 'el radio "teal" no quedó seleccionado en el registro');
        await page.getByRole('button', { name: /Registrar|Register/ }).first().click();
        await card(page).waitFor({ timeout: 12000 });
        await expectListColor(ctx, 'teal');
        const registered = await card(page).locator('[data-workspace-color]').getAttribute('data-workspace-color');
        ctx.assert(registered === 'teal', `data-workspace-color esperado teal, obtenido ${registered}`);

        // Persisted: a fresh load of the list still shows the pill.
        await ctx.goto('/workspaces');
        await expectListColor(ctx, 'teal');

        // ---- 2. Edición: preselección, cambio, y "sin color" ----
        await openEdit(ctx);
        ctx.assert((await checkedColor(page)) === 'teal', `la edición debe preseleccionar teal, obtenido "${await checkedColor(page)}"`);
        await picker(page).screenshot({ path: path.join(SHOTS, 'workspace-colors-picker-edit.png') });

        await pickColor(page, 'blue');
        await saveEdit(ctx);
        await ctx.goto('/workspaces');
        await expectListColor(ctx, 'blue');
        ctx.assert((await card(page).locator('.ws-chip.ws-teal').count()) === 0, 'tras el cambio no debe quedar el color teal');
        await shot('workspace-colors-list-light.png');

        await openEdit(ctx);
        ctx.assert((await checkedColor(page)) === 'blue', `tras guardar, la edición debe preseleccionar blue, obtenido "${await checkedColor(page)}"`);
        await pickColor(page, '');
        await saveEdit(ctx);
        await ctx.goto('/workspaces');
        await expectListColor(ctx, '');
        const plain = await card(page).innerText();
        ctx.assert(plain.includes(NAME), 'sin color, el nombre plano debe seguir en la tarjeta');

        // ---- 3. Color de nuevo: barra lateral y tablero ----
        await openEdit(ctx);
        await pickColor(page, 'violet');
        await saveEdit(ctx);
        await ctx.goto('/workspaces');
        await expectListColor(ctx, 'violet');

        await ctx.goto('/');
        await sidebarChip(page).waitFor({ timeout: 12000 });
        const sideKey = await sidebarChip(page).getAttribute('data-workspace-color');
        ctx.assert(sideKey === 'violet', `barra lateral: data-workspace-color esperado violet, obtenido ${sideKey}`);
        ctx.assert(
          (await sidebarChip(page).evaluate((el) => el.classList.contains('ws-chip') && el.classList.contains('ws-violet'))),
          'barra lateral: el chip no tiene las clases ws-chip ws-violet',
        );
        // #board-sidebar is display: contents (no box), so the sidebar is shot as part of the page.
        await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-sidebar-light.png') });

        // A backlog task of the workspace (not started, so no agent runs), then the dashboard badge of its project.
        await sidebarChip(page).click();
        await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await page.getByLabel(/^Workspace/).selectOption({ label: `${NAME} (main)` });
        const prompt = `colores-tarea-${tag}`;
        await page.getByLabel('Prompt').fill(prompt);
        await page.getByRole('button', { name: 'Crear tarea' }).click();
        await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
        // The dashboard is the home tab ("Tablero" in the navigation): the shell may restore the last board instead of it.
        await ctx.goto('/');
        await ctx.nav(/Tablero/).first().click();
        await page.locator('[data-section="pending"]').waitFor({ state: 'attached', timeout: 12000 });
        // The pending tasks (and their badges) are in the list tab; the graph tab is the default.
        const badge = page.locator('.ws-badge.ws-violet', { hasText: NAME }).first();
        if (await page.getByText(/^Tareas \(\d+\)$/).first().isVisible().catch(() => false)) {
          await page.getByText(/^Tareas \(\d+\)$/).first().click();
        }
        await badge.waitFor({ timeout: 12000 }).catch(async (err) => {
          // Diagnostic: what the dashboard rendered instead of the badge.
          await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-dashboard-debug.png') });
          const text = await page.locator('main, body').first().innerText().catch(() => '');
          throw new Error(`${err.message}\nDashboard text (first 600 chars): ${text.slice(0, 600).replace(/\s+/g, ' ')}`);
        });
        ctx.assert(
          (await badge.evaluate((el) => el.classList.contains('badge'))),
          'tablero: la insignia de la tarea debe llevar la clase base badge',
        );

        // ---- 4. Tema claro y oscuro: la pastilla conserva un tinte visible ----
        await setTheme(ctx, 'light');
        const lightTint = await tintOf(sidebarChip(page));
        ctx.assert(lightTint !== TRANSPARENT, `tema claro: el fondo del chip es transparente (${lightTint})`);
        const lightBadgeTint = await tintOf(badge);
        ctx.assert(lightBadgeTint !== TRANSPARENT, `tema claro: el fondo de la insignia es transparente (${lightBadgeTint})`);
        await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-dashboard-light.png') });

        await setTheme(ctx, 'dark');
        const darkTint = await tintOf(sidebarChip(page));
        ctx.assert(darkTint !== TRANSPARENT, `tema oscuro: el fondo del chip es transparente (${darkTint})`);
        const darkBadgeTint = await tintOf(badge);
        ctx.assert(darkBadgeTint !== TRANSPARENT, `tema oscuro: el fondo de la insignia es transparente (${darkBadgeTint})`);
        await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-sidebar-dark.png') });
        await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-dashboard-dark.png') });

        await ctx.goto('/workspaces');
        await card(page).locator('.ws-chip.ws-violet').waitFor({ timeout: 12000 });
        const listDarkTint = await tintOf(card(page).locator('.ws-chip'));
        ctx.assert(listDarkTint !== TRANSPARENT, `tema oscuro: el fondo del chip de la lista es transparente (${listDarkTint})`);
        await shot('workspace-colors-list-dark.png');

        // ---- 4b. Paleta distinta a la predeterminada ----
        await ctx.goto('/settings/appearance');
        await page.locator('[data-palette-choice="ocean"]').first().click();
        await page.waitForTimeout(200);
        const palette = await page.evaluate(() => document.documentElement.getAttribute('data-palette'));
        ctx.assert(palette === 'ocean', `data-palette esperado ocean, obtenido ${palette}`);
        const storedPalette = await page.evaluate(() => localStorage.getItem('pi-palette'));
        ctx.assert(storedPalette === 'ocean', `pi-palette persistido esperado ocean, obtenido ${storedPalette}`);
        await ctx.goto('/workspaces');
        await card(page).locator('.ws-chip.ws-violet').waitFor({ timeout: 12000 });
        const oceanTint = await tintOf(card(page).locator('.ws-chip'));
        ctx.assert(oceanTint !== TRANSPARENT, `paleta ocean: el fondo del chip es transparente (${oceanTint})`);

        // Problems the browser reported (uncaught errors, HTTP failures) fail the test.
        const problems = ctx.getProblems().filter((p) => p.startsWith('[pageerror]') || p.startsWith('[http]'));
        ctx.assert(problems.length === 0, `problemas de página o HTTP:\n${problems.join('\n')}`);
        const saved = fs.readdirSync(SHOTS).filter((f) => f.startsWith('workspace-colors-')).sort();
        for (const f of saved) console.log(`    screenshot: ${path.join(SHOTS, f)}`);
      },
    },
  ],
};
