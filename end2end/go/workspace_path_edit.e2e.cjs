// end2end/go/workspace_path_edit.e2e.cjs
// Editing the path of a registered workspace from its edit tab, end to end in the browser
// (odd/tasks/workspace-path-edit.md, T4, S1-S7).
// Five flows, one test each (every test gets its own page, and registers its own workspaces with a unique name because
// the server is shared):
//  1. Locked: with an unfinished task (started with the default fake agent) the path input is disabled and shows the
//     exact reason (S2, S6).
//  2. Moved folder: the folder is renamed on disk; a save without the same-project checkbox is rejected with the exact
//     reason and the checkbox appears; the stored path is still the old one; with the checkbox the save goes through,
//     the dock goes to the list and the list and the edit tab show the new path (S4b, S5).
//  3. Not a git repository: the exact reason is shown and nothing is saved (S3, S7).
//  4. Another project: an independent git repository is rejected with the exact reason and nothing is saved (S4, S7).
//  5. A plain edit (name and color, path untouched) keeps working and keeps the path (previous behavior).
// Every rejection is checked by reloading the edit tab and reading the stored path from the input value.
// Every check reads an observable effect: the input state, the text shown, the dock pane visible or the state the server
// keeps (read again after a reload). Screenshots go to SMOKE_SCREENSHOT_DIR (set by run_go.sh to <run dir>/screenshots).
// The texts are the Spanish ones (the harness language). The task is started with the default fake agent, so no real
// model runs.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// The exact texts of the edit tab (presentation/pages/messages.go, Spanish).
const TEXT_LOCKED = 'La ruta no puede cambiar mientras el workspace tenga tareas sin terminar.';
const TEXT_SAME_PROJECT_UNCONFIRMED = 'La carpeta anterior ya no existe, así que la nueva carpeta no se puede comparar; confirma que es el mismo proyecto';
const TEXT_CONFIRM_LABEL = 'Confirmo que es el mismo proyecto';
const TEXT_NOT_GIT = (p) => `La carpeta no es un repositorio git: ${p}`;
const TEXT_NOT_SAME_PROJECT = (p) => `La nueva ruta no es el mismo proyecto que el workspace: ${p}`;
const SAVE_BUTTON = 'Guardar cambios';

// A git repo with one commit on main, under the run's scratch dir.
function gitRepo(prefix) {
  const dir = fs.mkdtempSync(path.join(TMP, `workspace-path-edit-${prefix}-`));
  fs.writeFileSync(path.join(dir, 'README.md'), `${prefix}\n`);
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Polls read() until done(value) holds; returns the last value read.
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

// Problems the browser reported (uncaught errors, HTTP failures) fail the test.
function assertNoProblems(ctx) {
  const problems = ctx.getProblems().filter((p) => p.startsWith('[pageerror]') || p.startsWith('[http]'));
  ctx.assert(problems.length === 0, `problemas de página o HTTP:\n${problems.join('\n')}`);
}

// Registers a workspace on dir from the Workspaces screen and returns its name, folder and id (read from the sidebar).
async function registerWorkspace(ctx, key, dir) {
  const { page } = ctx;
  const name = `${key}-ruta-${tag}`;
  await ctx.goto('/workspaces/new');
  await page.getByLabel('Nombre').first().fill(name);
  await page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await page.getByLabel(/Rama base/).first().fill('main');
  await page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await page.getByText(dir).first().waitFor({ timeout: 12000 });
  await ctx.goto('/');
  const link = page.locator('#board-sidebar').getByRole('link', { name }).first();
  await link.waitFor({ state: 'visible', timeout: 12000 });
  const href = await link.getAttribute('href');
  return { key, name, dir, id: href.replace('/board/', '') };
}

// Starts a task in the workspace with the default agent (the fake one): it goes idle and waits for the user.
async function startTask(ctx, ws, prompt) {
  const { page } = ctx;
  await ctx.goto('/');
  await page.locator('#board-sidebar').locator(`a[href="/board/${ws.id}"]`).first().click();
  await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  const form = page.locator('form:has(select[name=workspace_id]):visible');
  await form.locator('select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
  await form.locator('textarea[name=prompt]').fill(prompt);
  await page.getByRole('checkbox', { name: /Iniciar ahora/ }).first().check();
  await form.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
  await form.waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});
}

// Opens the edit tab of a workspace and waits for its path input.
async function openEdit(ctx, id) {
  await ctx.goto(`/workspaces/${id}/edit`);
  await ctx.page.locator('input[name="path"]').waitFor({ state: 'visible', timeout: 12000 });
}

// The stored path, read from the edit tab after a reload.
async function storedPath(ctx, id) {
  await openEdit(ctx, id);
  return ctx.page.locator('input[name="path"]').inputValue();
}

// Sends the edit form as it is (Guardar cambios) and waits for the answer slot to settle.
async function save(ctx) {
  await ctx.page.getByRole('button', { name: SAVE_BUTTON }).click();
}

const resultSlot = (page) => page.locator('#workspace-edit-result');

module.exports = {
  name: 'workspace_path_edit',
  tests: [
    {
      name: 'ruta: con una tarea sin terminar el campo de ruta queda bloqueado con el motivo exacto',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const dir = gitRepo('bloqueado');
        const ws = await registerWorkspace(ctx, 'bloqueado', dir);
        await startTask(ctx, ws, `bloqueo-ruta-${tag}`);

        await openEdit(ctx, ws.id);
        const input = page.locator('input[name="path"]');
        const disabled = await until(() => input.isDisabled(), (v) => v === true);
        ctx.assert(disabled === true, 'con una tarea sin terminar el campo de ruta debe estar deshabilitado');
        const reason = page.locator('[data-path-locked]');
        await reason.waitFor({ state: 'visible', timeout: 8000 });
        ctx.assert((await reason.textContent()).trim() === TEXT_LOCKED, `motivo del bloqueo: ${await reason.textContent()}`);
        ctx.assert(await input.inputValue() === dir, 'el campo muestra la ruta guardada');
        await page.screenshot({ path: path.join(SHOTS, 'path-edit-locked.png') });
        assertNoProblems(ctx);
      },
    },
    {
      name: 'ruta: tras mover la carpeta, la ruta nueva se guarda solo con la confirmación del mismo proyecto',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const dirA = gitRepo('movida-a');
        const ws = await registerWorkspace(ctx, 'movida', dirA);
        const dirB = path.join(TMP, `workspace-path-edit-movida-b-${tag}`);
        fs.renameSync(dirA, dirB); // the repo keeps its history, only the folder moves

        // 1) Without the confirmation the save is rejected with the reason and the checkbox appears.
        await openEdit(ctx, ws.id);
        await page.locator('input[name="path"]').fill(dirB);
        await save(ctx);
        await resultSlot(page).getByText(TEXT_SAME_PROJECT_UNCONFIRMED, { exact: true }).waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert(await resultSlot(page).getByLabel(TEXT_CONFIRM_LABEL, { exact: true }).isVisible(), 'aparece la casilla "Confirmo que es el mismo proyecto"');
        await page.screenshot({ path: path.join(SHOTS, 'path-edit-needs-confirmation.png') });

        // The stored path is still the old one.
        ctx.assert(await storedPath(ctx, ws.id) === dirA, 'tras el rechazo la ruta guardada sigue siendo la anterior');

        // 2) With the checkbox the same save goes through and the dock goes to the list.
        await openEdit(ctx, ws.id);
        await page.locator('input[name="path"]').fill(dirB);
        await save(ctx);
        await resultSlot(page).getByLabel(TEXT_CONFIRM_LABEL, { exact: true }).waitFor({ state: 'visible', timeout: 10000 });
        await resultSlot(page).getByLabel(TEXT_CONFIRM_LABEL, { exact: true }).check();
        await save(ctx);
        const paths = await until(() => visiblePaths(page), (p) => p.includes('/workspaces'));
        ctx.assert(paths.length === 1 && paths[0] === '/workspaces', `tras guardar, el dock va al listado: ${JSON.stringify(paths)}`);

        // The list and the edit tab show the new path.
        await ctx.goto('/workspaces');
        await page.locator('#workspace-table').getByText(dirB, { exact: false }).first().waitFor({ state: 'visible', timeout: 12000 });
        await page.screenshot({ path: path.join(SHOTS, 'path-edit-saved.png') });
        ctx.assert(await storedPath(ctx, ws.id) === dirB, 'la edición muestra la ruta nueva tras guardar');
        assertNoProblems(ctx);
      },
    },
    {
      name: 'ruta: una carpeta que no es un repositorio git se rechaza con el motivo exacto y no se guarda nada',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const dir = gitRepo('nogit-origen');
        const ws = await registerWorkspace(ctx, 'nogit', dir);
        const empty = fs.mkdtempSync(path.join(TMP, 'workspace-path-edit-empty-'));

        await openEdit(ctx, ws.id);
        await page.locator('input[name="path"]').fill(empty);
        await save(ctx);
        await resultSlot(page).getByText(TEXT_NOT_GIT(empty), { exact: true }).waitFor({ state: 'visible', timeout: 10000 });
        await page.screenshot({ path: path.join(SHOTS, 'path-edit-not-git.png') });

        ctx.assert(await storedPath(ctx, ws.id) === dir, 'la ruta guardada no cambia tras el rechazo');
        assertNoProblems(ctx);
      },
    },
    {
      name: 'ruta: un repositorio de otro proyecto se rechaza con el motivo exacto y no se guarda nada',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const dirA = gitRepo('otro-a');
        const ws = await registerWorkspace(ctx, 'otro', dirA);
        const dirC = gitRepo('otro-proyecto-c'); // independent repository: its root commit is not the one of A

        await openEdit(ctx, ws.id);
        await page.locator('input[name="path"]').fill(dirC);
        await save(ctx);
        await resultSlot(page).getByText(TEXT_NOT_SAME_PROJECT(dirC), { exact: true }).waitFor({ state: 'visible', timeout: 10000 });
        await page.screenshot({ path: path.join(SHOTS, 'path-edit-other-project.png') });

        ctx.assert(await storedPath(ctx, ws.id) === dirA, 'la ruta guardada no cambia tras el rechazo');
        assertNoProblems(ctx);
      },
    },
    {
      name: 'ruta: editar solo el nombre y el color deja la ruta como estaba',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const dir = gitRepo('plano');
        const ws = await registerWorkspace(ctx, 'plano', dir);
        const newName = `plano-renombrado-${tag}`;

        await openEdit(ctx, ws.id);
        await page.getByLabel('Nombre').first().fill(newName);
        await page.locator('input[name="color"][value="teal"]').check({ force: true });
        await save(ctx);
        const paths = await until(() => visiblePaths(page), (p) => p.includes('/workspaces'));
        ctx.assert(paths.length === 1 && paths[0] === '/workspaces', `tras guardar, el dock va al listado: ${JSON.stringify(paths)}`);

        await openEdit(ctx, ws.id);
        ctx.assert((await page.getByLabel('Nombre').first().inputValue()) === newName, 'el nombre nuevo quedó guardado');
        ctx.assert(await page.locator('input[name="color"][value="teal"]').isChecked(), 'el color nuevo quedó guardado');
        ctx.assert(await page.locator('input[name="path"]').inputValue() === dir, 'la ruta no cambia cuando no se edita');
        assertNoProblems(ctx);
      },
    },
  ],
};
