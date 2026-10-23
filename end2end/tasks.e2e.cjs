// end2end/tasks.e2e.cjs
// Casos de uso del módulo tasks alcanzables desde la UI, con agentes falsos
// (tests/fixtures/fake_agent.py: responde "eco: <prompt>", pide un diálogo "¿Seguir?" y
// escribe CAMBIO.md en el worktree). Cada test registra SU workspace (repo git único)
// para no contaminarse con el resto de la suite.
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
    // El push del servidor va a la URL del host mock; se redirige a un bare local con
    // `url.<bare>.pushInsteadOf`, así la publicación de la rama funciona sin servidor git HTTP.
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
  await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await ctx.page.locator('aside').getByRole('link', { name: new RegExp(name) }).first().click();
  await sleep(500);
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
  // Navegar antes de que termine la cadena crear -> iniciar cancelaría el arranque:
  // se espera a que el modal se cierre y a que la tarea figure en el tablero.
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  await page.locator('a[href*="/tasks/"]', { hasText: prompt }).first().waitFor({ state: 'attached', timeout: 15000 });
  if (start) await sleep(2000); // deja que la cadena crear -> iniciar termine antes de navegar
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
  // La tarjeta suelta se arrastra (no navega al hacer clic): se abre el detalle por su enlace.
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
  await ctx.page.getByRole('heading', { name: '❓ ¿Seguir?' }).waitFor({ timeout: 15000 });
  return { ...t, prompt };
}

async function openActions(ctx) {
  await ctx.page.getByText('Acciones', { exact: true }).first().click();
  await sleep(300);
}

// Finaliza la tarea en vivo: responde el diálogo y pulsa "Finalizar y revisar".
async function finishedTask(ctx, key, opts = {}) {
  const t = await liveTask(ctx, key, opts);
  await ctx.page.getByRole('button', { name: 'Sí' }).click();
  await ctx.page.getByRole('button', { name: 'Finalizar y revisar' }).click();
  await ctx.page.getByText('Pendiente de revisión').first().waitFor({ timeout: 25000 });
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
        const { ws, id } = await taskDetail(ctx, 'update', `orig-${tag}`);
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
        await ctx.page.getByText(/Tarea eliminada|Esta tarea ya no existe/).first().waitFor({ timeout: 15000 });
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
        await ctx.page.getByPlaceholder(/Escribe un mensaje/).fill(`otra-cosa-${tag}`);
        await ctx.page.getByRole('button', { name: 'Enviar' }).click();
        await ctx.page.getByText(`eco: otra-cosa-${tag}`).first().waitFor({ timeout: 20000 });
      },
    },
    {
      name: 'finish_task: "Finalizar y revisar" deja la tarea pendiente de revisión',
      fn: async (ctx) => {
        await finishedTask(ctx, 'finish');
      },
    },
    {
      name: 'get_task_diff: "Ver diff" muestra el cambio del agente en su pestaña',
      fn: async (ctx) => {
        await finishedTask(ctx, 'diff');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Ver diff' }).click();
        await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}\/diff$/, { timeout: 15000 });
        await ctx.page.getByText('cambio del agente').first().waitFor({ timeout: 20000 });
      },
    },
    {
      name: 'merge_task + list_done_tasks: Merge integra la rama y la tarea sale en terminadas',
      fn: async (ctx) => {
        const t = await finishedTask(ctx, 'merge');
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Merge' }).click();
        await ctx.page.getByText('Integrada en la rama base.').first().waitFor({ timeout: 30000 });
        await openBoard(ctx, t.ws);
        await ctx.page.getByRole('button', { name: /Mostrar terminadas/ }).click();
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
      name: 'abort_task: Abortar ejecución detiene al agente',
      fn: async (ctx) => {
        await liveTask(ctx, 'abort');
        await openActions(ctx);
        // El agente falso no implementa `abort` (solo prompt/steer/diálogos), así que el estado
        // visible no cambia: se comprueba que la acción se acepta sin error y la tarea sigue operativa.
        const abortResponse = ctx.page.waitForResponse((r) => /abort_task/.test(r.url()), { timeout: 15000 });
        await ctx.page.getByRole('button', { name: 'Abortar ejecución' }).click();
        const res = await abortResponse;
        ctx.assert(res.status() === 200, `abort_task respondió ${res.status()}`);
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
        const name = `tk-branches-${tag}`;
        const { dir } = await registerWs(ctx, name);
        execSync(`git -C '${dir}' branch rama-extra-${tag}`, { stdio: 'ignore' });
        await openBoard(ctx, name);
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.getByLabel(/^Workspace/).selectOption({ label: `${name} (main)` });
        const select = ctx.page.locator('select.input').filter({ hasText: /Rama del workspace/ }).first();
        await select.waitFor({ timeout: 15000 });
        const start = Date.now();
        let has = false;
        while (Date.now() - start < 15000 && !has) {
          has = await select.evaluate((el, b) => [...el.options].some((o) => o.textContent.includes(b)), `rama-extra-${tag}`);
          if (!has) await sleep(400);
        }
        ctx.assert(has, 'la rama creada no aparece entre las ramas base');
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
        // En Backlog no hay panel de acciones: los adjuntos se ven en la pantalla de edición.
        await ctx.goto((await link.getAttribute('href')) + '/edit');
        await page.locator(`img[alt="captura-${tag}.png"]`).first().waitFor({ state: 'attached', timeout: 15000 });
      },
    },
    {
      name: 'get_task_graph + set_task_position + detach_task_from_graph: añadir al grafo y quitarla',
      fn: async (ctx) => {
        const name = `tk-graph-${tag}`;
        await registerWs(ctx, name);
        await openBoard(ctx, name);
        await createTask(ctx, name, `grafo-${tag}`);
        const page = ctx.page;
        await page.getByRole('button', { name: /Añadir al grafo/ }).first().click();
        // La tarea sale de "Sueltas" y pasa a ser un nodo del grafo, con acción de quitarla.
        const remove = page.getByRole('button', { name: /^Quitar del grafo/ }).first();
        await remove.waitFor({ timeout: 15000 });
        await page.reload({ waitUntil: 'domcontentloaded' });
        await page.getByRole('button', { name: /^Quitar del grafo/ }).first().waitFor({ timeout: 15000 });
        await page.getByRole('button', { name: /^Quitar del grafo/ }).first().click();
        await page.getByRole('button', { name: /Añadir al grafo/ }).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'add_task_dependency: "Añadir tarea siguiente" crea una sucesora que espera a su predecesora',
      fn: async (ctx) => {
        const name = `tk-dep-${tag}`;
        await registerWs(ctx, name);
        await openBoard(ctx, name);
        await createTask(ctx, name, `pred-${tag}`);
        const page = ctx.page;
        await page.getByRole('button', { name: /Añadir al grafo/ }).first().click();
        await page.getByRole('button', { name: /Añadir tarea siguiente/ }).first().click();
        await page.getByLabel('Prompt').fill(`succ-${tag}`);
        await page.getByRole('button', { name: 'Crear tarea' }).click();
        await page.getByText(/Esperando 1 predecesora/).first().waitFor({ timeout: 20000 });
      },
    },
  ],
};
module.exports.helpers = { finishedTask, openActions, sleep };
