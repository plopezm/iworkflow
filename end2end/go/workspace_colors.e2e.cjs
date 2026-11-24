// end2end/go/workspace_colors.e2e.cjs
// Color of a workspace, end to end in the browser: the radio picker of the registration and edit forms,
// the name led by a colored dot (`.ws-name.ws-<key> > .ws-dot`, no pill) in the workspaces list and the board
// sidebar, the rail on the dashboard task card (`.ws-rail.ws-<key>`), and the color of dot and rail in light, dark and
// a non-default palette. Every check reads an observable effect: a DOM class or
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
  page.locator(`[data-testid="workspace-color-picker"] input[name="color"][value="${key}"]:not([data-ws-custom-radio])`).check();
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

// Waits until the workspace card shows the dotted name of `key` (or none when key is ''): one [data-workspace-color]
// element with the ws-name class, or none at all.
async function expectListColor(ctx, key) {
  const { page } = ctx;
  const expected = key ? [{ key, chip: true }] : [];
  const got = await waitFor(
    () => card(page).locator('[data-workspace-color]').evaluateAll((els) =>
      els.map((el) => ({ key: el.dataset.workspaceColor, chip: el.classList.contains('ws-name') })),
    ),
    expected,
  );
  ctx.assert(
    JSON.stringify(got) === JSON.stringify(expected),
    `lista: nombre con punto esperado ${JSON.stringify(expected)}, obtenido ${JSON.stringify(got)}`,
  );
  const names = await card(page).locator('.ws-name').count();
  ctx.assert(names === (key ? 1 : 0), `lista: .ws-name esperado ${key ? 1 : 0}, obtenido ${names}`);
  const dots = await card(page).locator('.ws-name .ws-dot').count();
  ctx.assert(dots === (key ? 1 : 0), `lista: .ws-dot esperado ${key ? 1 : 0}, obtenido ${dots}`);
  if (key) await expectNoPill(ctx, card(page).locator('.ws-name'), 'lista');
}

// The name is not a pill: no background and no border around it (the color lives only in the dot).
async function expectNoPill(ctx, locator, where) {
  const style = await locator.evaluate((el) => {
    const cs = getComputedStyle(el);
    return { bg: cs.backgroundColor, border: cs.borderTopWidth };
  });
  ctx.assert(style.bg === TRANSPARENT, `${where}: el nombre no debe tener fondo (${style.bg})`);
  ctx.assert(style.border === '0px', `${where}: el nombre no debe tener borde (${style.border})`);
}

// The rail of a dashboard card: the ::before of the card, 2px wide, in a visible (non-transparent) color.
async function railOf(locator) {
  return locator.evaluate((el) => {
    const cs = getComputedStyle(el, '::before');
    return { bg: cs.backgroundColor, width: cs.width, left: cs.left };
  });
}

// Computed background of an element: the dot takes it from --ws, so it must not be transparent.
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
        ctx.assert((await card(page).locator('.ws-name.ws-teal').count()) === 0, 'tras el cambio no debe quedar el color teal');
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

        // ---- 3b. Color personalizado: el "+" abre el selector nativo y se guarda como #rrggbb ----
        await openEdit(ctx);
        const custom = page.locator('[data-testid="workspace-color-picker"] [data-ws-custom-input]');
        await custom.fill('#1a2b3c');
        ctx.assert((await checkedColor(page)) === '#1a2b3c', `el color personalizado debe quedar seleccionado, obtenido "${await checkedColor(page)}"`);
        await saveEdit(ctx);
        await ctx.goto('/workspaces');
        await card(page).locator('.ws-name.ws-custom').waitFor({ timeout: 12000 });
        const customDot = await card(page).locator('.ws-name .ws-dot').evaluate((el) => getComputedStyle(el).backgroundColor);
        ctx.assert(customDot === 'rgb(26, 43, 60)', `el punto personalizado esperado rgb(26, 43, 60), obtenido ${customDot}`);
        await openEdit(ctx);
        ctx.assert((await checkedColor(page)) === '#1a2b3c', `la edición debe preseleccionar el personalizado, obtenido "${await checkedColor(page)}"`);
        await picker(page).screenshot({ path: path.join(SHOTS, 'workspace-colors-picker-custom.png') });
        await pickColor(page, 'violet');
        await saveEdit(ctx);
        await ctx.goto('/workspaces');
        await expectListColor(ctx, 'violet');

        await ctx.goto('/');
        await sidebarChip(page).waitFor({ timeout: 12000 });
        const sideKey = await sidebarChip(page).getAttribute('data-workspace-color');
        ctx.assert(sideKey === 'violet', `barra lateral: data-workspace-color esperado violet, obtenido ${sideKey}`);
        ctx.assert(
          (await sidebarChip(page).evaluate((el) => el.classList.contains('ws-name') && el.classList.contains('ws-violet'))),
          'barra lateral: el nombre no tiene las clases ws-name ws-violet',
        );
        // #board-sidebar is display: contents (no box), so the sidebar is shot as part of the page.
        await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-sidebar-light.png') });

        // A backlog task of the workspace (not started, so no agent runs), then the dashboard badge of its project.
        await expectNoPill(ctx, sidebarChip(page), 'barra lateral');
        await sidebarChip(page).click();

        // The board tab of the project: its tab carries the color (a dot and the active underline) and the page a
        // faint wash with a thin line on top.
        const boardTab = page.locator('[data-dock-tab-button].dock-tab-ws.ws-violet', { hasText: NAME }).first();
        await boardTab.waitFor({ timeout: 12000 });
        ctx.assert((await boardTab.locator('.ws-dot').count()) === 1, 'pestaña del tablero: debe llevar el punto de color');
        const boardPage = page.locator('.ws-board.ws-violet').first();
        await boardPage.waitFor({ timeout: 12000 });
        const wash = await boardPage.evaluate((el) => getComputedStyle(el).backgroundImage);
        ctx.assert(wash.includes('linear-gradient'), `tablero del proyecto: falta el degradado suave (${wash})`);
        const line = await railOf(boardPage);
        ctx.assert(line.bg !== TRANSPARENT && line.width !== 'auto', `tablero del proyecto: falta la línea superior (${line.bg})`);
        const underline = await boardTab.evaluate((el) => getComputedStyle(el, '::after').backgroundColor);
        ctx.assert(underline === 'rgb(142, 78, 198)', `pestaña del tablero: subrayado esperado violeta, obtenido ${underline}`);
        await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-board-tab.png') });
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
        // The pending tasks (and their cards) are in the list tab; the graph tab is the default.
        const taskCard = page.locator('[data-dashboard-task].ws-rail.ws-violet', { hasText: NAME }).first();
        if (await page.getByText(/^Tareas \(\d+\)$/).first().isVisible().catch(() => false)) {
          await page.getByText(/^Tareas \(\d+\)$/).first().click();
        }
        await taskCard.waitFor({ timeout: 12000 }).catch(async (err) => {
          // Diagnostic: what the dashboard rendered instead of the card.
          await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-dashboard-debug.png') });
          const text = await page.locator('main, body').first().innerText().catch(() => '');
          throw new Error(`${err.message}\nDashboard text (first 600 chars): ${text.slice(0, 600).replace(/\s+/g, ' ')}`);
        });
        const badge = taskCard.locator('.badge', { hasText: NAME }).first();
        ctx.assert(
          (await badge.locator('.ws-dot').count()) === 1,
          'tablero: la insignia de la tarea debe llevar el punto de color',
        );
        const badgeBg = await badge.evaluate((el) => getComputedStyle(el).backgroundColor);
        const plainBadge = await page.locator('[data-dashboard-task]:not(.ws-rail) .badge-muted').first().evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => badgeBg);
        ctx.assert(badgeBg === plainBadge, `tablero: la insignia no debe teñirse (${badgeBg} vs ${plainBadge})`);

        // ---- 4. Tema claro y oscuro: el punto y la raya conservan un color visible ----
        const dotOf = () => sidebarChip(page).locator('.ws-dot');
        for (const theme of ['light', 'dark']) {
          await setTheme(ctx, theme);
          const dot = await tintOf(dotOf());
          ctx.assert(dot !== TRANSPARENT, `tema ${theme}: el punto de la barra es transparente (${dot})`);
          const rail = await railOf(taskCard);
          ctx.assert(rail.bg !== TRANSPARENT, `tema ${theme}: la raya de la tarjeta es transparente (${rail.bg})`);
          ctx.assert(rail.width === '2px', `tema ${theme}: ancho de la raya esperado 2px, obtenido ${rail.width}`);
          await page.screenshot({ path: path.join(SHOTS, `workspace-colors-dashboard-${theme}.png`) });
        }
        await page.screenshot({ path: path.join(SHOTS, 'workspace-colors-sidebar-dark.png') });

        await ctx.goto('/workspaces');
        await card(page).locator('.ws-name.ws-violet').waitFor({ timeout: 12000 });
        const listDot = await tintOf(card(page).locator('.ws-name .ws-dot'));
        ctx.assert(listDot !== TRANSPARENT, `tema oscuro: el punto de la lista es transparente (${listDot})`);
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
        await card(page).locator('.ws-name.ws-violet').waitFor({ timeout: 12000 });
        const oceanDot = await tintOf(card(page).locator('.ws-name .ws-dot'));
        ctx.assert(oceanDot !== TRANSPARENT, `paleta ocean: el punto es transparente (${oceanDot})`);

        // Problems the browser reported (uncaught errors, HTTP failures) fail the test.
        const problems = ctx.getProblems().filter((p) => p.startsWith('[pageerror]') || p.startsWith('[http]'));
        ctx.assert(problems.length === 0, `problemas de página o HTTP:\n${problems.join('\n')}`);
        const saved = fs.readdirSync(SHOTS).filter((f) => f.startsWith('workspace-colors-')).sort();
        for (const f of saved) console.log(`    screenshot: ${path.join(SHOTS, f)}`);
      },
    },
  ],
};
