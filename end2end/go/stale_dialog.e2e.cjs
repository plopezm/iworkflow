// end2end/go/stale_dialog.e2e.cjs
// Go port of end2end/stale_dialog_smoke.cjs, and the answer path of the dialogs of the transcript.
// The fake cursor agent (tests/fixtures/fake_cursor.py) asks a permission, which the transcript draws as a dialog:
//  - "abandoned" asks the permission and ends its turn without waiting for the answer. The dialog must close with the
//    turn: no pending controls and no "no hay un diálogo pendiente" error.
//  - "permission" waits for the answer. Answering "Sí" posts it to the task and the agent goes on with it.
// The agent runtime is a setting of the whole server, so each test chooses cursor and restores the runtime it found.
// Waits are on the page (locators and responses), never on fixed sleeps.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const RUNTIME = 'cursor';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `stale-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `stale dialog ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Chooses the agent runtime on /settings and waits until the server has saved it. Returns the runtime it found, so
// the caller can restore it.
async function setRuntime(ctx, runtime) {
  await ctx.goto('/settings');
  await ctx.page.getByText('Se aplica a las tareas nuevas').first().waitFor({ timeout: 15000 });
  const select = ctx.page.locator('.select-pill').filter({ hasText: 'Agente' }).first().locator('select').first();
  await select.waitFor({ timeout: 15000 });
  const previous = await select.inputValue();
  if (previous !== runtime) {
    const saved = ctx.page.waitForResponse(
      (r) => r.url().includes('/agent-settings/runtime') && r.request().method() === 'POST',
      { timeout: 15000 },
    );
    await select.selectOption(runtime);
    await saved;
  }
  return previous;
}

// Runs the body with the runtime chosen, then restores the runtime found. A failure of the body is the failure reported:
// a restore that fails after it is logged, not thrown over it.
async function withRuntime(ctx, runtime, body) {
  const previous = await setRuntime(ctx, runtime);
  let failure = null;
  try {
    await body();
  } catch (err) {
    failure = err;
  }
  try {
    await setRuntime(ctx, previous);
  } catch (restoreErr) {
    if (failure === null) {
      failure = restoreErr;
    } else {
      console.error(`no se pudo restaurar el runtime ${previous}: ${restoreErr.message}`);
    }
  }
  if (failure !== null) {
    throw failure;
  }
}

// The hrefs of the task links on the page, in the order they are shown.
function taskHrefs(page) {
  return page.locator('a[href*="/tasks/"]').evaluateAll((links) => links.map((link) => link.getAttribute('href')));
}

// Creates a task in a fresh repository that starts at once with the given prompt, and opens its detail page.
// Returns the id of the task. The prompt is the exact text the fake agent matches, so it is not made unique: the task
// is found by the link that the create adds, not by its prompt.
async function startedTask(ctx, name, prompt) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  await ctx.goto('/');
  await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  // Choosing the workspace refreshes the list of the board: the new-task button is used once that answer has arrived.
  const listed = ctx.page.waitForResponse((r) => r.url().includes('/board/frame'), { timeout: 10000 });
  await ctx.page.locator('[id^="board-controls-"]:visible select[name=workspace]').selectOption({ label: `${name} (main)` });
  await listed;
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  const knownHrefs = await taskHrefs(ctx.page);
  await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await ctx.page.getByLabel(/^Workspace/).selectOption({ label: `${name} (main)` });
  await ctx.page.getByLabel('Prompt').fill(prompt);
  await ctx.page.getByLabel('Iniciar ahora').check();
  await ctx.page.getByRole('button', { name: 'Crear tarea' }).click();
  // Navigating before the create -> start chain ends cancels the start (as tasks.e2e does): wait for the modal to close.
  await ctx.page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const created = await ctx.page.waitForFunction(
    (known) => [...document.querySelectorAll('a[href*="/tasks/"]')].map((a) => a.getAttribute('href')).find((href) => !known.includes(href)) || null,
    knownHrefs,
    { timeout: 15000 },
  );
  const href = await created.jsonValue();
  const id = (href || '').match(/\/tasks\/([0-9a-f-]{36})/)?.[1];
  ctx.assert(id, 'no se obtuvo el id de la tarea creada');
  await ctx.goto(`/tasks/${id}`);
  return id;
}

// The labels of a dialog the transcript shows once it is no longer open: closed by its turn, or answered by the user.
const CLOSED_DIALOG = /❓ Run \(cerrado\)/;
const ANSWERED_DIALOG = /❓ Run \(respondido\)/;

// Checks the whole page for a dialog left open: a pending dialog is a section with its controls, and its error is drawn
// in it. Called once the closed or answered line is shown, so a dialog still open at that point fails here.
async function assertNoOpenDialog(ctx) {
  ctx.assert(
    (await ctx.page.locator('section[data-dialog]').count()) === 0,
    'no debe quedar ningún diálogo abierto',
  );
  ctx.assert(
    (await ctx.page.getByRole('button', { name: 'Cancelar diálogo' }).count()) === 0,
    'no debe quedar ningún control de diálogo abierto',
  );
  ctx.assert(
    (await ctx.page.getByText(/no hay un diálogo pendiente/).count()) === 0,
    'no debe aparecer el error de diálogo pendiente',
  );
}

module.exports = {
  name: 'stale_dialog',
  tests: [
    {
      name: 'un diálogo que el agente abandona al cerrar su turno se cierra sin error',
      fn: async (ctx) => {
        await withRuntime(ctx, RUNTIME, async () => {
          await startedTask(ctx, `stale-${tag}`, 'abandoned');
          // The agent ends its turn without waiting for the answer: the dialog closes with it, as a line of the transcript.
          await ctx.page.getByText('turno cerrado').first().waitFor({ timeout: 25000 });
          const closedDialog = ctx.page.locator('[data-dialog]', { hasText: CLOSED_DIALOG });
          await closedDialog.first().waitFor({ timeout: 15000 });
          await assertNoOpenDialog(ctx);
        });
      },
    },
    {
      name: 'responder «Sí» a un diálogo llega al agente, que continúa su turno',
      fn: async (ctx) => {
        await withRuntime(ctx, RUNTIME, async () => {
          await startedTask(ctx, `answer-${tag}`, 'permission');
          // The pending dialog offers its answer as a control of the transcript.
          const yes = ctx.page.getByRole('button', { name: 'Sí', exact: true }).first();
          await yes.waitFor({ timeout: 25000 });
          await yes.click();
          // The answer goes to the agent, which reports the option it took and ends its turn.
          await ctx.page.getByText('chose=allow-once').first().waitFor({ timeout: 20000 });
          const answeredDialog = ctx.page.locator('[data-dialog]', { hasText: ANSWERED_DIALOG });
          await answeredDialog.first().waitFor({ timeout: 15000 });
          await assertNoOpenDialog(ctx);
        });
      },
    },
  ],
};
