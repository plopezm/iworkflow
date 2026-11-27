// end2end/go/workspace_create_flow.e2e.cjs
// Two flows of the creation screens:
//  1. The registration screen offers the groups and puts the new workspace in the chosen one.
//  2. A task created from a screen returns to that screen (the dashboard or the board), not always to the board of its
//     workspace.
// Every check reads an observable effect: the sidebar the server renders after a reload and the dock pane that is
// visible. Screenshots go to SMOKE_SCREENSHOT_DIR (set by run_go.sh to <run dir>/screenshots). The texts are the Spanish
// ones (the harness language).
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function gitRepo(prefix) {
  const dir = fs.mkdtempSync(path.join(TMP, `create-flow-${prefix}-`));
  fs.writeFileSync(path.join(dir, 'README.md'), `${prefix}\n`);
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function until(read, done, { timeout = 12000 } = {}) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    last = await read().catch(() => undefined);
    if (done(last)) return last;
    await sleep(200);
  }
  return last;
}

// The paths of the dock panes that are showing.
const visiblePaths = (page) =>
  page.evaluate(() =>
    [...document.querySelectorAll('[data-dock-pane]')].filter((p) => !p.classList.contains('hidden')).map((p) => p.dataset.path));

// The names of the workspaces the sidebar shows inside each group, by group name.
const sidebarGroups = (page) =>
  page.evaluate(() =>
    [...document.querySelectorAll('#board-sidebar [data-group-id]')].map((group) => ({
      id: group.dataset.groupId,
      text: group.querySelector('button[aria-expanded]:not([data-menu-trigger])')?.textContent.replace(/^\s*[▾▸]\s*/, '').trim() ?? '',
      rows: [...group.querySelectorAll('[data-workspace-row]')].map((row) => row.textContent.trim()),
    })));

async function createGroup(ctx, name) {
  const res = await ctx.page.request.post(`${ctx.base}/workspace-groups`, { form: { name } });
  ctx.assert(res.ok(), `crear el grupo ${name}: HTTP ${res.status()}`);
}

async function register(ctx, name, dir, groupLabel) {
  await ctx.goto('/workspaces/new');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  if (groupLabel) await ctx.page.locator('[data-register-group] select[name=group_id]').selectOption({ label: groupLabel });
}

async function submitRegister(ctx, dir) {
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
}

// Opens a screen the way the user does, from the sidebar links (a reload restores the saved tabs, not the address).
async function openFromSidebar(ctx, dockPath) {
  await ctx.goto('/');
  await ctx.page.locator(`a[data-dock-path="${dockPath}"]:visible`).first().click();
}

// Creates a task from the screen already open: "Nueva tarea" opens the tab, the form is filled and sent.
async function createTaskFrom(ctx, button, wsName, prompt) {
  const { page } = ctx;
  await button.click();
  const form = page.locator('form:has(select[name=workspace_id]):visible');
  await form.waitFor({ state: 'visible', timeout: 10000 });
  await form.locator('select[name=workspace_id]').selectOption({ label: `${wsName} (main)` });
  await form.locator('textarea[name=prompt]').fill(prompt);
  return form;
}

module.exports = {
  name: 'workspace_create_flow',
  tests: [
    {
      name: 'crear workspace: se añade al grupo elegido desde la misma pantalla de creación',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const groupName = `Grupo alta ${tag}`;
        await createGroup(ctx, groupName);

        const name = `alta-grupo-${tag}`;
        const dir = gitRepo('alta');
        await register(ctx, name, dir, groupName);
        const select = page.locator('[data-register-group] select[name=group_id]');
        ctx.assert((await select.locator('option', { hasText: groupName }).count()) === 1, 'el formulario ofrece el grupo creado');
        await page.screenshot({ path: path.join(SHOTS, 'workspace-create-group-form.png'), fullPage: true });

        await submitRegister(ctx, dir);
        await page.screenshot({ path: path.join(SHOTS, 'workspace-create-group-saved.png'), fullPage: true });

        await ctx.goto('/');
        const groups = await until(() => sidebarGroups(page), (g) => g.some((x) => x.text.includes(groupName) && x.rows.some((r) => r.includes(name))));
        const group = groups.find((x) => x.text.includes(groupName));
        ctx.assert(group && group.rows.some((r) => r.includes(name)), `el workspace debe quedar en el grupo: ${JSON.stringify(groups)}`);

        // Sin elegir grupo el workspace queda suelto (comportamiento previo).
        const loose = `alta-suelto-${tag}`;
        const looseDir = gitRepo('suelto');
        await register(ctx, loose, looseDir, null);
        await submitRegister(ctx, looseDir);
        await ctx.goto('/');
        const after = await until(() => sidebarGroups(page), (g) => g.length > 0);
        ctx.assert(!after.some((x) => x.rows.some((r) => r.includes(loose))), 'sin elegir grupo, el workspace no entra en ninguno');
        ctx.assert((await page.locator('#board-sidebar').getByRole('link', { name: loose }).count()) >= 1, 'el workspace sin grupo aparece en la barra lateral');
      },
    },
    {
      name: 'crear workspace: los grupos cargan en segundo plano y mientras tanto sale "Cargando grupos…"',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        await createGroup(ctx, `Grupo carga ${tag}`);

        // The groups request is held until the test releases it, so the loading state can be observed.
        let release;
        const gate = new Promise((resolve) => { release = resolve; });
        await page.route('**/workspaces/register/groups*', async (route) => {
          await gate;
          await route.continue();
        });
        await ctx.goto('/workspaces/new');

        // The form is already usable while the groups are still loading.
        await page.getByLabel('Nombre').first().waitFor({ state: 'visible', timeout: 12000 });
        const slot = page.locator('#workspace-register-groups');
        await slot.getByText('Cargando grupos…').waitFor({ state: 'visible', timeout: 5000 });
        ctx.assert((await slot.getAttribute('aria-busy')) === 'true', 'el hueco de grupos está marcado como ocupado mientras carga');
        ctx.assert((await page.locator('[data-register-group] select').count()) === 0, 'no hay selector de grupo hasta que llegan los grupos');
        await page.screenshot({ path: path.join(SHOTS, 'workspace-create-groups-loading.png'), fullPage: true });

        release();
        const select = page.locator('[data-register-group] select[name=group_id]');
        await select.waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await page.getByText('Cargando grupos…').count()) === 0, 'el texto de carga desaparece al llegar los grupos');
        ctx.assert((await select.locator('option', { hasText: `Grupo carga ${tag}` }).count()) === 1, 'el selector ofrece el grupo creado');
        await page.screenshot({ path: path.join(SHOTS, 'workspace-create-groups-loaded.png'), fullPage: true });
      },
    },
    {
      name: 'editar workspace: los grupos cargan en segundo plano y el selector guarda el grupo elegido',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const groupName = `Grupo edicion ${tag}`;
        await createGroup(ctx, groupName);
        const name = `edicion-${tag}`;
        const dir = gitRepo('edicion');
        await register(ctx, name, dir, null);
        await submitRegister(ctx, dir);
        await ctx.goto('/');
        const href = await page.locator('#board-sidebar').getByRole('link', { name }).first().getAttribute('href');
        const wsId = href.replace('/board/', '');

        // The groups request is held until the test releases it, so the loading state can be observed.
        let release;
        const gate = new Promise((resolve) => { release = resolve; });
        await page.route(`**/workspaces/${wsId}/edit/groups`, async (route) => {
          await gate;
          await route.continue();
        });
        await ctx.goto(`/workspaces/${wsId}/edit`);

        // The form is already usable while the groups are still loading.
        await page.getByLabel('Nombre').first().waitFor({ state: 'visible', timeout: 12000 });
        const slot = page.locator('#workspace-edit-groups');
        await slot.getByText('Cargando grupos…').waitFor({ state: 'visible', timeout: 5000 });
        ctx.assert((await slot.getAttribute('aria-busy')) === 'true', 'el hueco de grupos está marcado como ocupado mientras carga');
        ctx.assert((await page.locator('[data-workspace-group-select] select').count()) === 0, 'no hay selector de grupo hasta que llegan los grupos');
        await page.screenshot({ path: path.join(SHOTS, 'workspace-edit-groups-loading.png'), fullPage: true });

        release();
        const select = page.locator('[data-workspace-group-select] select');
        await select.waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await page.getByText('Cargando grupos…').count()) === 0, 'el texto de carga desaparece al llegar los grupos');
        ctx.assert((await select.locator('option', { hasText: groupName }).count()) === 1, 'el selector ofrece el grupo creado');

        // Choosing a group saves on its own, and the next visit already shows it selected.
        const saved = page.waitForResponse((r) => r.url().includes(`/workspace-groups/workspaces/${wsId}/group`));
        const groupId = await select.locator('option', { hasText: groupName }).getAttribute('value');
        await select.selectOption(groupId);
        ctx.assert((await saved).ok(), 'elegir el grupo lo guarda sin pulsar Guardar');
        await page.screenshot({ path: path.join(SHOTS, 'workspace-edit-groups-loaded.png'), fullPage: true });
        await page.unroute(`**/workspaces/${wsId}/edit/groups`);
        await ctx.goto(`/workspaces/${wsId}/edit`);
        const again = page.locator('[data-workspace-group-select] select');
        await again.waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await again.inputValue()) === groupId, 'al volver a abrir la edición el grupo sigue seleccionado');
      },
    },
    {
      name: 'crear workspace: si fallan los grupos sale un error con Reintentar que los vuelve a pedir (HTTP 500)',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const groupName = `Grupo error ${tag}`;
        await createGroup(ctx, groupName);

        // The first groups request fails with a 500; the next one (the Retry) reaches the server.
        let requests = 0;
        await page.route('**/workspaces/register/groups*', (route) => {
          requests += 1;
          return requests === 1 ? route.fulfill({ status: 500, contentType: 'text/plain', body: 'boom' }) : route.continue();
        });
        await ctx.goto('/workspaces/new');

        // The form is usable while the groups are in error, and the slot is no longer busy.
        await page.getByLabel('Nombre').first().waitFor({ state: 'visible', timeout: 12000 });
        const slot = page.locator('#workspace-register-groups');
        await slot.getByText('No se pudieron cargar los grupos').waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await slot.getAttribute('aria-busy')) === null, 'el hueco deja de estar ocupado tras el error');
        ctx.assert(!(await slot.getByText('Cargando grupos…').isVisible()), 'el texto de carga se oculta tras el error');
        ctx.assert((await slot.locator('input[name=group_id]').count()) === 1, 'el campo group_id sigue en el hueco tras el error');
        ctx.assert(await page.getByRole('button', { name: /Registrar|Register/ }).first().isEnabled(), 'el formulario sigue usable tras el error');
        await page.screenshot({ path: path.join(SHOTS, 'workspace-groups-error.png'), fullPage: true });

        // Retry asks for the same URL into the same slot, which then offers the selector.
        await slot.getByRole('button', { name: 'Reintentar' }).click();
        const select = page.locator('[data-register-group] select[name=group_id]');
        await select.waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await select.locator('option', { hasText: groupName }).count()) === 1, 'Reintentar carga el selector con el grupo creado');
        ctx.assert((await page.getByText('No se pudieron cargar los grupos').count()) === 0, 'el texto de error desaparece al cargar');
        await page.unroute('**/workspaces/register/groups*');
      },
    },
    {
      name: 'editar workspace: si falla la red al cargar los grupos sale un error con Reintentar',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const groupName = `Grupo red ${tag}`;
        await createGroup(ctx, groupName);
        const name = `red-${tag}`;
        const dir = gitRepo('red');
        await register(ctx, name, dir, null);
        await submitRegister(ctx, dir);
        await ctx.goto('/');
        const href = await page.locator('#board-sidebar').getByRole('link', { name }).first().getAttribute('href');
        const wsId = href.replace('/board/', '');

        // The first groups request never reaches the server (network error); the Retry gets through.
        let requests = 0;
        await page.route(`**/workspaces/${wsId}/edit/groups`, (route) => {
          requests += 1;
          return requests === 1 ? route.abort() : route.continue();
        });
        await ctx.goto(`/workspaces/${wsId}/edit`);

        const slot = page.locator('#workspace-edit-groups');
        await slot.getByText('No se pudieron cargar los grupos').waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await slot.getAttribute('aria-busy')) === null, 'el hueco deja de estar ocupado tras el error de red');
        await slot.getByRole('button', { name: 'Reintentar' }).click();
        const select = page.locator('[data-workspace-group-select] select');
        await select.waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await select.locator('option', { hasText: groupName }).count()) === 1, 'Reintentar carga el selector de grupo de la edición');
        await page.unroute(`**/workspaces/${wsId}/edit/groups`);
      },
    },
    {
      name: 'crear tarea: vuelve a la pantalla desde la que se inició (inicio o tablero)',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const name = `retorno-${tag}`;
        const dir = gitRepo('retorno');
        await register(ctx, name, dir, null);
        await submitRegister(ctx, dir);
        const wsId = await (async () => {
          await ctx.goto('/');
          const href = await page.locator('#board-sidebar').getByRole('link', { name }).first().getAttribute('href');
          return href.replace('/board/', '');
        })();

        // 1) Desde el inicio (dashboard): al crear vuelve al inicio, no al tablero del workspace.
        await openFromSidebar(ctx, '/');
        await page.locator('[data-dock-pane]:not(.hidden) a[role=button][data-dock-title="Nueva tarea"]:visible').first().waitFor({ state: 'visible', timeout: 12000 });
        let form = await createTaskFrom(ctx, page.locator('[data-dock-pane]:not(.hidden) a[role=button][data-dock-title="Nueva tarea"]:visible').first(), name, `Tarea desde inicio ${tag}`);
        await page.screenshot({ path: path.join(SHOTS, 'task-create-from-dashboard.png'), fullPage: true });
        await form.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
        await form.waitFor({ state: 'detached', timeout: 15000 });
        let paths = await until(() => visiblePaths(page), (p) => p.includes('/'));
        ctx.assert(paths.length === 1 && paths[0] === '/', `tras crear desde el inicio se vuelve al inicio: ${JSON.stringify(paths)}`);
        await page.screenshot({ path: path.join(SHOTS, 'task-created-back-on-dashboard.png'), fullPage: true });

        // 2) Desde el tablero: vuelve al tablero del workspace.
        await openFromSidebar(ctx, `/board/${wsId}`);
        await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 12000 });
        form = await createTaskFrom(ctx, page.locator('[data-dock-pane]:not(.hidden) a[role=button][data-dock-title="Nueva tarea"]:visible').first(), name, `Tarea desde tablero ${tag}`);
        await form.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
        await form.waitFor({ state: 'detached', timeout: 15000 });
        paths = await until(() => visiblePaths(page), (p) => p.includes(`/board/${wsId}`));
        ctx.assert(paths.length === 1 && paths[0] === `/board/${wsId}`, `tras crear desde el tablero se vuelve al tablero: ${JSON.stringify(paths)}`);
        await page.screenshot({ path: path.join(SHOTS, 'task-created-back-on-board.png'), fullPage: true });

        // 3) Cancelar también vuelve a la pantalla de origen (se parte de la pestaña del inicio, ya abierta).
        await page.locator('[data-dock-tab-button]', { hasText: /Tablero/ }).first().click();
        await until(() => visiblePaths(page), (p) => p.includes('/'));
        await page.locator('[data-dock-pane]:not(.hidden) a[role=button][data-dock-title="Nueva tarea"]:visible').first().waitFor({ state: 'visible', timeout: 12000 });
        await page.locator('[data-dock-pane]:not(.hidden) a[role=button][data-dock-title="Nueva tarea"]:visible').first().click();
        await page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'visible', timeout: 10000 });
        await page.locator('a[data-dock-link]:visible', { hasText: 'Cancelar' }).first().click();
        paths = await until(() => visiblePaths(page), (p) => p.includes('/'));
        ctx.assert(paths.includes('/'), `cancelar desde el inicio vuelve al inicio: ${JSON.stringify(paths)}`);
      },
    },
  ],
};
