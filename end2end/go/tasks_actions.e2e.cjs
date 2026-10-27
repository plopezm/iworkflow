// end2end/go/tasks_actions.e2e.cjs
// Go-mode e2e of the task actions (U6c): finish, merge, discard and reset from the Actions tab, plus the
// subagents panel opened from that tab (U6b landed at HEAD 59f9370). The steps are ported from end2end/tasks.e2e.cjs
// (finish_task, merge_task + list_done_tasks, discard_task, reset_task) to the Go server, whose templates keep the
// Spanish labels of the Rust version. The subagents step proves the panel of a task without subagents: the fake
// agents of the harness do not emit subagent activity (no fake speaks the "gentle-agents" widget), so the panel
// shows its empty state. The suite runs in Spanish (the language cookie of the harness).
// Scope: the actions of a task in review and the panel they open. The transcript, composer, messaging (U4), diff
// (U5) and terminal (U5/U6) are not covered here. Each test registers its own git repo as a workspace, so the tests
// do not see each other.

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `actions-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `actions ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Registers a workspace from the Workspaces screen and waits until it is listed.
async function registerWs(ctx, name) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  return { name, dir };
}

// Opens the board from the navigation bar and selects the workspace in its switcher.
async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await ctx.page.locator('select[name=workspace]:visible').selectOption({ label: `${name} (main)` });
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
}

// Registers workspace + opens board + creates task and opens its detail. Returns { ws, id }. The task starts
// when `opts.start` is set; the create -> start chain is awaited before navigating, so the start is not cancelled.
async function taskDetail(ctx, key, prompt, opts = {}) {
  const name = `tk-${key}-${tag}`;
  const ws = await registerWs(ctx, name);
  await openBoard(ctx, ws.name);
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${ws.name} (main)` });
  if (opts.title) await page.getByLabel(/^Título/).fill(opts.title);
  await page.getByLabel('Prompt').fill(prompt);
  if (opts.start) await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  // Navigating before the create -> start chain ends cancels the start (as tasks.e2e does): wait for the modal to close.
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  const href = await link.getAttribute('href');
  const id = (href.match(/\/tasks\/([0-9a-f-]{36})/) || [])[1];
  ctx.assert(id, `no se obtuvo el id de la tarea "${prompt}"`);
  await ctx.goto(href);
  await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}/, { timeout: 15000, waitUntil: 'commit' });
  return { ws: ws.name, id };
}

// Tarea iniciada con el agente falso; espera a que el diálogo "¿Seguir?" aparezca en el transcript.
// El agente falso termina su turno justo después de pedir el diálogo, así que el turno cierra el diálogo
// (estado "cerrado"): el estado pendiente se observa igualmente antes de que el turno se cierre.
async function liveTask(ctx, key, opts = {}) {
  const prompt = `${key}-${tag}`;
  const t = await taskDetail(ctx, key, prompt, { ...opts, start: true });
  const page = ctx.page;
  await page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
  await page.getByText(/❓ ¿Seguir\? \(/).first().waitFor({ timeout: 15000 });
  return { ...t, prompt };
}

// Abre la pestaña de acciones del detalle de la tarea.
async function openActions(ctx) {
  await ctx.page.getByRole('tab', { name: 'Acciones' }).first().click();
  await sleep(300);
}

// Finaliza la tarea en vivo: responde el diálogo si sigue pendiente y pulsa "Finalizar y revisar".
async function finishedTask(ctx, key, opts = {}) {
  const t = await liveTask(ctx, key, opts);
  const page = ctx.page;
  const pendingDialog = page.getByText(/❓ ¿Seguir\? \(pendiente\)/);
  if (await pendingDialog.first().isVisible().catch(() => false)) {
    await page.getByRole('button', { name: 'Sí' }).first().click();
  }
  await page.getByRole('button', { name: 'Finalizar y revisar' }).click();
  await page.getByText('Pendiente de revisión').first().waitFor({ timeout: 25000 });
  return t;
}

// ---- tests ----
module.exports = {
  name: 'tasks_actions',
  tests: [
    {
      name: 'finish_task: "Finalizar y revisar" deja la tarea pendiente de revisión',
      fn: async (ctx) => {
        await finishedTask(ctx, 'finish');
      },
    },
    {
      name: 'merge_task + list_done_tasks: Merge integra la rama y la tarea sale en terminadas',
      fn: async (ctx) => {
        const t = await finishedTask(ctx, 'merge');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Merge' }).click();
        await ctx.page.getByText('Integrada en la rama base.').first().waitFor({ timeout: 30000 });
        // A task that is not awaiting review is not mergeable: after integrating the task is merged, so the
        // action is not offered again and the state is left unchanged.
        await ctx.page.getByRole('button', { name: 'Merge' }).waitFor({ state: 'detached', timeout: 15000 });
        await openBoard(ctx, t.ws);
        // The Go toolbar renders the completed toggle as a checkbox label, not as a button.
        await ctx.page.getByText('Mostrar terminadas').first().click();
        await ctx.page.getByText('Hecho', { exact: true }).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'discard_task: Descartar borra el worktree y deja la tarea descartada',
      fn: async (ctx) => {
        await finishedTask(ctx, 'discard');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Descartar' }).click();
        await ctx.page.getByText('Descartada', { exact: true }).first().waitFor({ timeout: 20000 });
        await ctx.page.getByRole('button', { name: 'Merge' }).waitFor({ state: 'detached', timeout: 15000 });
      },
    },
    {
      name: 'reset_task: Restablecer devuelve la tarea a Backlog',
      fn: async (ctx) => {
        await finishedTask(ctx, 'reset');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Restablecer' }).click();
        await ctx.page.getByRole('button', { name: 'Iniciar' }).first().waitFor({ timeout: 20000 });
      },
    },
    {
      name: 'subagents: el panel "Subagentes" de la pestaña Acciones lista las subagentes del agente',
      fn: async (ctx) => {
        await finishedTask(ctx, 'subagents');
        await openActions(ctx);
        // The fake agents do not emit subagent activity, so a task without subagents shows the empty state.
        await ctx.page.getByRole('button', { name: /Subagentes/ }).first().click();
        await ctx.page.getByText(/Este agente no ha lanzado subagentes/).first().waitFor({ timeout: 15000 });
      },
    },
  ],
};
module.exports.helpers = { finishedTask, openActions, sleep };
