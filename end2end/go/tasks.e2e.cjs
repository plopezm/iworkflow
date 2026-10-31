// end2end/go/tasks.e2e.cjs
// Go port of end2end/tasks.e2e.cjs.
// End-to-end task lifecycle tests reachable from the UI with fake agents.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `tasks-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `tasks ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

async function registerWs(ctx, name, { origin } = {}) {
  const dir = uniqueGitRepo(name);
  if (origin) {
    execSync(`git -C '${dir}' remote add origin '${origin}'`, { stdio: 'ignore' });
    const bare = `${dir}-remote.git`;
    execSync(`git init -q --bare '${bare}'`, { stdio: 'ignore' });
    execSync(`git -C '${dir}' config 'url.${bare}.pushInsteadOf' '${origin}'`, { stdio: 'ignore' });
  }
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  return { name, dir };
}

// Abre el tablero del workspace `name` desde la barra lateral.
async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
}

// Crea una tarea desde el modal "Nueva tarea". Devuelve el id (UUID) si se abre su detalle.
async function createTask(ctx, ws, prompt, { start = false, title } = {}) {
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${ws} (main)` });
  if (title) await page.getByLabel(/^Título/).fill(title);
  await page.getByLabel('Prompt').fill(prompt);
  if (start) await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  await page.locator('a[href*="/tasks/"]', { hasText: prompt }).first().waitFor({ state: 'attached', timeout: 15000 });
  if (start) await sleep(2000);
  await openBoard(ctx, ws);
}

// Registra workspace + abre tablero + crea tarea y abre su detalle. Devuelve { ws, id }.
async function taskDetail(ctx, key, prompt, opts = {}) {
  const name = `tk-${key}-${tag}`;
  await registerWs(ctx, name, opts);
  await openBoard(ctx, name);
  await createTask(ctx, name, prompt, opts);
  const link = ctx.page.locator('a[href*="/tasks/"]', { hasText: prompt }).locator('visible=true').first();
  await link.waitFor({ timeout: 15000 });
  const href = await link.getAttribute('href');
  const id = (href.match(/\/tasks\/([0-9a-f-]{36})/) || [])[1];
  await ctx.goto(href);
  await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}/, { timeout: 15000, waitUntil: 'commit' });
  ctx.assert(id, `no se obtuvo el id de la tarea "${prompt}"`);
  return { ws: name, id };
}

// Tarea iniciada con el agente falso; espera a que el diálogo "¿Seguir?" esté listo.
async function liveTask(ctx, key, opts = {}) {
  const prompt = `${key}-${tag}`;
  const t = await taskDetail(ctx, key, prompt, { ...opts, start: true });
  await ctx.page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
  await ctx.page.getByText(/❓ ¿Seguir\? \(/).first().waitFor({ timeout: 15000 });
  return { ...t, prompt };
}

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

module.exports = {
  name: 'tasks',
  tests: [
    {
      name: 'create_task + list_tasks + get_task: crear en Backlog aparece en el tablero y abre su detalle',
      fn: async (ctx) => {
        const prompt = `crear-${tag}`;
        const { id } = await taskDetail(ctx, 'create', prompt);
        await ctx.page.getByText(prompt).first().waitFor({ timeout: 10000 });
        ctx.assert(/\/tasks\//.test(ctx.page.url()), 'no se abrió el detalle');
        ctx.assert(id.length === 36, 'id de tarea inválido');
      },
    },
    {
      name: 'update_task: editar título y prompt en su pantalla y ver los cambios',
      fn: async (ctx) => {
        const { id } = await taskDetail(ctx, 'update', `orig-${tag}`);
        await ctx.goto(`/tasks/${id}/edit`);
        await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ timeout: 15000 });
        ctx.assert((await ctx.page.getByLabel('Prompt').inputValue()) === `orig-${tag}`, 'el prompt no se precargó');
        await ctx.page.getByLabel(/^Título/).fill(`titulo-editado-${tag}`);
        await ctx.page.getByLabel('Prompt').fill(`prompt-editado-${tag}`);
        await ctx.page.getByRole('button', { name: 'Guardar cambios' }).click();
        await ctx.page.waitForURL((u) => !u.pathname.endsWith('/edit'), { timeout: 15000 });
        await ctx.page.getByText(`titulo-editado-${tag}`).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'delete_task: borrar una tarea de Backlog la elimina',
      fn: async (ctx) => {
        await taskDetail(ctx, 'delete', `borrar-${tag}`);
        await ctx.page.getByRole('button', { name: 'Borrar' }).first().click();
        const confirmBtn = ctx.page.getByRole('button', { name: /Eliminar|Confirmar/ });
        if (await confirmBtn.count() > 0) {
          await confirmBtn.first().click();
        }
        await ctx.page.waitForURL((u) => u.pathname === '/' || u.pathname.startsWith('/board'), { timeout: 15000 });
      },
    },
    {
      name: 'start_task + get_task_events: iniciar muestra el transcript en vivo del agente',
      fn: async (ctx) => {
        const t = await liveTask(ctx, 'start');
        await ctx.page.getByText('en vivo').first().waitFor({ timeout: 10000 });
        ctx.assert(t.prompt.length > 0, 'sin prompt');
      },
    },
    {
      name: 'answer_dialog: responder "Sí" llega al agente',
      fn: async (ctx) => {
        await liveTask(ctx, 'dialog');
        await ctx.page.getByRole('button', { name: 'Sí' }).click();
        await ctx.page.getByText('respuesta {"confirmed": true').first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'send_message: un mensaje de seguimiento obtiene respuesta del agente',
      fn: async (ctx) => {
        await liveTask(ctx, 'msg');
        await ctx.page.locator('[data-composer-input]').first().fill('sigamos');
        await ctx.page.getByRole('button', { name: 'Enviar' }).click();
        await ctx.page.getByText('eco: sigamos').first().waitFor({ timeout: 25000 });
      },
    },
    {
      name: 'finish_task: "Finalizar y revisar" deja la tarea pendiente de revisión',
      fn: async (ctx) => {
        await finishedTask(ctx, 'finish');
        await openActions(ctx);
        ctx.assert(await ctx.page.getByRole('button', { name: 'Merge' }).count() > 0, 'no ofreció merge');
      },
    },
    {
      name: 'get_task_diff: "Ver diff" muestra el cambio del agente en su pestaña',
      fn: async (ctx) => {
        await finishedTask(ctx, 'diff');
        await openActions(ctx);
        await ctx.page.locator('a[href$="/diff"]').first().click();
        await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}\/diff/, { timeout: 15000 });
        await ctx.page.locator('text=CAMBIO.md').first().waitFor({ state: 'attached', timeout: 20000 });
      },
    },
    {
      name: 'merge_task + list_done_tasks: Merge integra la rama y la tarea sale en terminadas',
      fn: async (ctx) => {
        const { ws } = await finishedTask(ctx, 'merge');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Merge' }).click();
        await ctx.page.getByText('Integrada en la rama base.').first().waitFor({ timeout: 30000 });
        await ctx.page.getByRole('button', { name: 'Merge' }).waitFor({ state: 'detached', timeout: 15000 });
        await openBoard(ctx, ws);
        await ctx.page.locator('label[role="button"]:has(input[name="completed"])').first().click();
        await ctx.page.getByText('Hecho', { exact: true }).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'discard_task: Descartar borra el worktree y deja la tarea descartada',
      fn: async (ctx) => {
        await finishedTask(ctx, 'discard');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Descartar' }).click();
        await ctx.page.getByRole('button', { name: 'Merge' }).waitFor({ state: 'detached', timeout: 15000 });
        await ctx.page.getByText(/Descartada/).first().waitFor({ timeout: 15000 });
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
      name: 'abort_task: Abortar ejecución detiene al agente',
      fn: async (ctx) => {
        await liveTask(ctx, 'abort');
        await openActions(ctx);
        const abortResponse = ctx.page.waitForResponse((r) => /abort/.test(r.url()), { timeout: 15000 });
        await ctx.page.getByRole('button', { name: 'Abortar ejecución' }).click();
        const res = await abortResponse;
        ctx.assert(res.status() === 200, `abort respondió ${res.status()}`);
        await ctx.page.waitForTimeout(800);
        ctx.assert(await ctx.page.locator('.error-text, [role="alert"]').count() === 0, 'abortar mostró un error');
        ctx.assert(await ctx.page.getByRole('button', { name: 'Enviar' }).count() > 0, 'la tarea dejó de ser operativa');
      },
    },
    {
      name: 'resume_task: Reanudar agente relanza una tarea abortada/en revisión',
      fn: async (ctx) => {
        await finishedTask(ctx, 'resume');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Reanudar agente' }).click();
        await ctx.page.getByText('en vivo').first().waitFor({ timeout: 25000 });
      },
    },
    {
      name: 'open_task_terminal: "Abrir terminal" abre la terminal del worktree',
      fn: async (ctx) => {
        await finishedTask(ctx, 'tterm');
        await openActions(ctx);
        await ctx.page.getByRole('link', { name: 'Abrir terminal' }).click();
        await ctx.page.waitForURL(/\/terminals\/[0-9a-f-]{36}$/, { timeout: 20000 });
        await ctx.page.locator('.xterm-helper-textarea').waitFor({ state: 'attached', timeout: 15000 });
      },
    },
    {
      name: 'list_workspace_branches: el modal de nueva tarea ofrece las ramas del workspace',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `tk-br-${tag}`);
        execSync(`git -C '${ws.dir}' branch rama-extra-${tag}`, { stdio: 'ignore' });
        await openBoard(ctx, ws.name);
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
        const select = ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=base_branch]');
        await select.waitFor({ timeout: 15000 });
        const started = Date.now();
        let listed = false;
        while (Date.now() - started < 15000 && !listed) {
          listed = await select.evaluate((el, b) => [...el.options].some((o) => o.textContent.includes(b)), `rama-extra-${tag}`);
          if (!listed) await sleep(400);
        }
        ctx.assert(listed, 'la rama creada no aparece entre las ramas base');
      },
    },
    {
      name: 'list_task_attachments: una imagen adjuntada al crear aparece en el detalle',
      fn: async (ctx) => {
        const name = `tk-attach-${tag}`;
        await registerWs(ctx, name);
        await openBoard(ctx, name);
        const page = ctx.page;
        await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await page.getByLabel(/^Workspace/).selectOption({ label: `${name} (main)` });
        await page.getByLabel('Prompt').fill(`adjunto-${tag}`);
        const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64');
        await page.locator('input[type=file]').first().setInputFiles({ name: `captura-${tag}.png`, mimeType: 'image/png', buffer: PNG });
        await page.getByAltText(`captura-${tag}.png`).first().waitFor({ timeout: 10000 });
        await page.getByRole('button', { name: 'Crear tarea' }).click();
        await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
        await openBoard(ctx, name);
        const link = page.locator('a[href*="/tasks/"]', { hasText: `adjunto-${tag}` }).locator('visible=true').first();
        await link.waitFor({ timeout: 15000 });
        await ctx.goto((await link.getAttribute('href')) + '/edit');
        await page.locator(`img[alt="captura-${tag}.png"]`).first().waitFor({ state: 'attached', timeout: 15000 });
      },
    },
  ],
};
module.exports.helpers = { finishedTask, openActions, sleep };
