// end2end/go/engram.e2e.cjs
// Go-mode e2e of the read-only Engram integration (odd/tasks/engram-integration.md, T5, S7; checks S3, S4, S6):
// the Engram chip of the Workspaces screen and the collapsed Memory panel of a task, against a FAKE Engram API.
//
// The fake is a node http server that answers exactly the two read endpoints the feature uses and records every
// request (method and path). run_go.sh starts the server under test with IWF_ENGRAM_URL pointing at a port it
// reserves, and passes the same URL to this suite as IWF_E2E_ENGRAM_URL; the fake listens on that port. The
// server reads Engram on every page render, so the fake only has to be up when the page is requested.
//
// The memory content is a unique secret: it must reach the server's JSON answer but never a rendered page (S4).
// The suite runs in Spanish. Every test is self-contained: it starts the fake for its own page loads through
// withFake (which stops it again when the test ends) and gets its workspaces and task from readyFixture. So each
// test passes alone and the suite passes in any order; no test reads state that an earlier test left behind.

const http = require('http');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SECRET = `ENGRAM-CONTENT-SECRET-${tag}`;
const PROJECT = 'demo';
const CHIP_MANY = 'Engram · 3 memorias';
const CHIP_NONE = 'Engram · sin memorias';
const CHIP_OFF = 'Engram no disponible';
const PANEL_TITLE = 'Memoria';
const SHOT_DIR = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';

// Saves a screenshot of the current page and prints its path.
async function shot(ctx, name) {
  fs.mkdirSync(SHOT_DIR, { recursive: true });
  const file = path.join(SHOT_DIR, name);
  await ctx.page.screenshot({ path: file });
  console.log(`screenshot: ${file}`);
}

// The workspace id, read from the Memories link of its row on the Workspaces screen.
async function workspaceId(ctx, name) {
  await ctx.goto('/workspaces');
  const href = await rowOf(ctx, name).locator('a[href$="/memories"]').first().getAttribute('href', { timeout: 10000 });
  return href.split('/')[2];
}

// The three memories the fake answers. Only type, title and date may reach the page; content is the secret.
const MEMORIES = [
  {
    id: 101, sync_id: 'sync-101', session_id: 'session-101', type: 'decision', title: 'Usar WAL en SQLite',
    content: SECRET, project: PROJECT, scope: 'project', topic_key: 'db/wal', revision_count: 1,
    duplicate_count: 0, last_seen_at: '2026-11-11 17:25:30',
  },
  {
    id: 102, sync_id: 'sync-102', session_id: 'session-102', type: 'bugfix', title: 'Cerrar el PTY al salir del shell',
    content: SECRET, project: PROJECT, scope: 'project', topic_key: 'terminal/pty', revision_count: 2,
    duplicate_count: 1, last_seen_at: '2026-11-11 16:02:10',
  },
  {
    id: 103, sync_id: 'sync-103', session_id: 'session-103', type: 'discovery', title: 'Tailwind necesita el CSS compilado',
    content: SECRET, project: PROJECT, scope: 'project', topic_key: 'build/tailwind', revision_count: 1,
    duplicate_count: 0, last_seen_at: '2026-11-10 09:14:00',
  },
];

// Fake state: the directories of the one project (set once the workspace repo exists) and every request seen.
const fake = { server: null, directories: [], requests: [], extra: [] };

function sendJSON(res, status, body) {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

// Answers exactly the two GET endpoints of the feature; every other request is refused and still recorded.
function handle(req, res) {
  const url = new URL(req.url, 'http://127.0.0.1');
  fake.requests.push(`${req.method} ${url.pathname}`);
  if (req.method !== 'GET') {
    sendJSON(res, 405, { error: 'read-only fake' });
    return;
  }
  if (url.pathname === '/projects') {
    sendJSON(res, 200, {
      count: 1,
      projects: [{
        name: PROJECT, observation_count: MEMORIES.length, session_count: 1, prompt_count: 0,
        directories: fake.directories,
      }],
    });
    return;
  }
  if (url.pathname === '/observations/recent' && url.searchParams.get('project') === PROJECT) {
    sendJSON(res, 200, [...MEMORIES, ...fake.extra]);
    return;
  }
  const observation = url.pathname.match(/^\/observations\/(\d+)$/);
  if (observation) {
    const found = [...MEMORIES, ...fake.extra].find((memory) => memory.id === Number(observation[1]));
    if (found) {
      sendJSON(res, 200, { ...found, updated_at: found.last_seen_at });
      return;
    }
  }
  sendJSON(res, 404, { error: 'not found' });
}

// Starts the fake on the port of IWF_E2E_ENGRAM_URL. Idempotent while it runs.
async function ensureFake() {
  if (fake.server) return;
  const configured = process.env.IWF_E2E_ENGRAM_URL;
  if (!configured) {
    throw new Error('IWF_E2E_ENGRAM_URL no está definida: ejecuta el suite con end2end/go/run_go.sh');
  }
  const { port } = new URL(configured);
  const server = http.createServer(handle);
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(Number(port), '127.0.0.1', resolve);
  });
  fake.server = server;
}

// Stops the fake and drops its keep-alive sockets, so the server under test sees it unavailable at once.
async function stopFake() {
  if (!fake.server) return;
  const server = fake.server;
  fake.server = null;
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
}

// Runs `body` with the fake listening and stops it when `body` ends, even on failure. Tests that need Engram
// up use this, so the fake never outlives the test that started it (other suites expect Engram unavailable).
async function withFake(body) {
  await ensureFake();
  try {
    return await body();
  } finally {
    await stopFake();
  }
}

// Creates a git repo with one commit and returns its real path (the server stores the path it was given).
let repoCount = 0;
function uniqueGitRepo(prefix) {
  repoCount += 1;
  const dir = path.join(TMP, `engram-repo-${prefix}-${tag}-${repoCount}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `engram ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return fs.realpathSync(dir);
}

// Registers a workspace from the Workspaces screen and waits until its row is listed.
async function registerWs(ctx, name, dir) {
  await ctx.goto('/workspaces');
  await ctx.page.getByRole('button', { name: /Nuevo workspace/ }).first().click();
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(name).first().waitFor({ timeout: 12000 });
}

// Creates a backlog task in the workspace from the board and returns its id (the card carries it as data-task-id)
async function createTask(ctx, ws, title) {
  const page = ctx.page;
  await ctx.goto('/');
  await page.locator('#board-sidebar').getByRole('link', { name: ws }).first().click();
  await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  const form = 'form:has(select[name=workspace_id]):visible';
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.locator(`${form} select[name=workspace_id]`).selectOption({ label: `${ws} (main)` });
  await page.locator(`${form} input[name=title]`).fill(title);
  await page.locator(`${form} textarea[name=prompt]`).fill(`prompt ${title}`);
  await page.locator(`${form} button[type=submit]`, { hasText: 'Crear tarea' }).click();
  await page.locator(form).waitFor({ state: 'detached', timeout: 10000 });
  const card = page.locator('[id^="board-list-"]:visible [data-loose-task]', { hasText: title }).first();
  await card.waitFor({ state: 'attached', timeout: 10000 });
  const id = await card.getAttribute('data-task-id');
  if (!/^[0-9a-f-]{36}$/.test(id || '')) throw new Error(`la tarjeta de "${title}" no trae un id de tarea valido`);
  return id;
}

// Shared fixture, built on demand: a workspace whose repo the fake reports as the project directory, a workspace
// with no matching project, and one task in the matching workspace. The build only sets the data the fake will
// answer; it does not need the fake to be listening. The promise is cached per process, so the first test that
// needs it builds it and the others reuse it; a test that runs alone builds it itself.
let fixturePromise = null;
function readyFixture(ctx) {
  if (!fixturePromise) fixturePromise = buildFixture(ctx);
  return fixturePromise;
}

async function buildFixture(ctx) {
  const matchName = `eng-match-${tag}`;
  const otherName = `eng-other-${tag}`;
  const matchDir = uniqueGitRepo('match');
  const otherDir = uniqueGitRepo('other');
  fake.directories = [matchDir];
  await registerWs(ctx, matchName, matchDir);
  await registerWs(ctx, otherName, otherDir);
  const taskId = await createTask(ctx, matchName, `memoria-${tag}`);
  return { matchName, otherName, taskId };
}

// Creates a task in the workspace with "Iniciar ahora", waits for the fake agent and finishes it, so the task has a
// branch and its Review group (Ver diff, Memorias) is offered. Returns the task id.
async function finishedTaskIn(ctx, ws) {
  const page = ctx.page;
  const prompt = `rama-${tag}`;
  await ctx.goto('/');
  await page.locator('#board-sidebar').getByRole('link', { name: ws }).first().click();
  await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${ws} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  const href = await link.getAttribute('href');
  const id = (href.match(/\/tasks\/([0-9a-f-]{36})/) || [])[1];
  await ctx.goto(href);
  await page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
  await page.getByText(/❓ ¿Seguir\?/).first().waitFor({ timeout: 15000 });
  if (await page.getByText(/❓ ¿Seguir\? \(pendiente\)/).first().isVisible().catch(() => false)) {
    await page.getByRole('button', { name: 'Sí' }).first().click();
  }
  await page.getByRole('button', { name: 'Finalizar' }).click();
  await page.getByText('Pendiente de revisión').first().waitFor({ timeout: 25000 });
  return id;
}

// The workspace row (article.card) of the given workspace name.
const rowOf = (ctx, name) => ctx.page.locator('article.card', { hasText: name }).first();

module.exports = {
  name: 'engram',
  tests: [
    {
      name: 'chip: la fila del workspace con proyecto coincidente muestra "Engram · 3 memorias"',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          await ctx.goto('/workspaces');
          await rowOf(ctx, f.matchName).getByText(CHIP_MANY, { exact: true }).waitFor({ timeout: 10000 });
        });
      },
    },
    {
      name: 'chip: un workspace sin proyecto coincidente muestra "Engram · sin memorias"',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          await ctx.goto('/workspaces');
          await rowOf(ctx, f.otherName).getByText(CHIP_NONE, { exact: true }).waitFor({ timeout: 10000 });
          ctx.assert((await rowOf(ctx, f.otherName).getByText(CHIP_MANY).count()) === 0, 'el workspace sin proyecto muestra memorias');
        });
      },
    },
    {
      name: 'panel: el detalle de la tarea trae "Memoria" colapsado y al abrirlo lista título y tipo de cada memoria',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          await ctx.goto(`/tasks/${f.taskId}`);
          const panel = ctx.page.locator('details[data-task-memory]');
          await panel.waitFor({ state: 'attached', timeout: 10000 });
          ctx.assert((await panel.evaluate((el) => el.open)) === false, 'el panel de memoria nace abierto');
          ctx.assert((await panel.locator('summary').innerText()).trim() === PANEL_TITLE, 'el resumen del panel no es "Memoria"');
          await panel.locator('summary').click();
          const rows = panel.locator('[data-memory-row]');
          await rows.nth(MEMORIES.length - 1).waitFor({ state: 'visible', timeout: 5000 });
          ctx.assert((await rows.count()) === MEMORIES.length, `el panel lista ${await rows.count()} memorias, no ${MEMORIES.length}`);
          for (const memory of MEMORIES) {
            const row = panel.locator('[data-memory-row]', { hasText: memory.title });
            ctx.assert((await row.count()) === 1, `falta la memoria "${memory.title}"`);
            ctx.assert((await row.innerText()).includes(memory.type), `la memoria "${memory.title}" no muestra su tipo ${memory.type}`);
          }
        });
      },
    },
    {
      name: 'privacidad: el contenido de una memoria nunca aparece en la página de la tarea',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          await ctx.goto(`/tasks/${f.taskId}`);
          const panel = ctx.page.locator('details[data-task-memory]');
          await panel.waitFor({ state: 'attached', timeout: 10000 });
          await panel.locator('summary').click();
          await panel.locator('[data-memory-row]').first().waitFor({ state: 'visible', timeout: 5000 });
          const html = await ctx.page.content();
          ctx.assert(!html.includes(SECRET), 'el contenido de la memoria aparece en el HTML de la página');
          const text = await ctx.page.innerText('body');
          ctx.assert(!text.includes(SECRET), 'el contenido de la memoria aparece en el texto de la página');
        });
      },
    },
    {
      name: 'sin Engram: Workspaces responde con "Engram no disponible" y el detalle no trae panel de memoria',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        // Engram must be down for this test: stop it if it is up (a no-op when no fake is listening).
        await stopFake();
        const response = await ctx.page.goto(`${ctx.base}/workspaces`, { waitUntil: 'domcontentloaded' });
        ctx.assert(response && response.status() === 200, `Workspaces respondió ${response && response.status()}`);
        await rowOf(ctx, f.matchName).getByText(CHIP_OFF, { exact: true }).waitFor({ timeout: 10000 });
        await rowOf(ctx, f.otherName).getByText(CHIP_OFF, { exact: true }).waitFor({ timeout: 10000 });

        const taskResponse = await ctx.page.goto(`${ctx.base}/tasks/${f.taskId}`, { waitUntil: 'domcontentloaded' });
        ctx.assert(taskResponse && taskResponse.status() === 200, `el detalle respondió ${taskResponse && taskResponse.status()}`);
        await ctx.page.waitForFunction(() => document.querySelector('[data-app-shell]') !== null, null, { timeout: 10000 });
        ctx.assert((await ctx.page.locator('details[data-task-memory]').count()) === 0, 'el detalle muestra panel de memoria sin Engram');
      },
    },
    {
      name: 'memorias: el boton de la tarjeta abre la lista de memorias del workspace, sin contenido',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          await ctx.goto('/workspaces');
          await shot(ctx, 'memories-1-workspaces-button.png');
          await rowOf(ctx, f.matchName).getByRole('link', { name: 'Memorias' }).click();
          const list = ctx.page.locator('[data-memories-list]:visible');
          await list.waitFor({ timeout: 10000 });
          const rows = list.locator('[data-memory-row]');
          await rows.nth(MEMORIES.length - 1).waitFor({ state: 'visible', timeout: 5000 });
          ctx.assert((await rows.count()) === MEMORIES.length, 'la lista no trae las 3 memorias');
          ctx.assert((await list.innerText()).includes('Solo lectura'), 'falta la marca "Solo lectura"');
          ctx.assert(!(await list.innerText()).includes(SECRET), 'la lista muestra el contenido');
          ctx.assert((await list.locator('form, button[type=submit], [hx-post], [hx-delete]').count()) === 0, 'la lista trae controles de escritura');
          await shot(ctx, 'memories-2-list.png');
        });
      },
    },
    {
      name: 'memorias: el boton del tablero esta a la izquierda de "Nueva tarea" y lleva a la lista',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          const id = await workspaceId(ctx, f.matchName);
          await ctx.goto(`/board/${id}`);
          const toolbar = ctx.page.locator('form[id^="board-controls-"]:visible');
          await toolbar.waitFor({ timeout: 10000 });
          const memories = toolbar.getByRole('button', { name: /Memorias/ });
          const newTask = toolbar.getByRole('button', { name: /Nueva tarea/ });
          const a = await memories.boundingBox();
          const b = await newTask.boundingBox();
          ctx.assert(a && b && a.x + a.width <= b.x, 'Memorias no esta a la izquierda de Nueva tarea');
          await shot(ctx, 'memories-3-board-button.png');
          await memories.click();
          await ctx.page.locator('[data-memories-list]:visible [data-memory-row]').first().waitFor({ timeout: 10000 });
        });
      },
    },
    {
      name: 'memorias: abrir una memoria muestra su contenido completo en solo lectura',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          const id = await workspaceId(ctx, f.matchName);
          await ctx.goto(`/workspaces/${id}/memories`);
          await ctx.page.locator('[data-memories-list]:visible').getByRole('link', { name: MEMORIES[0].title }).click();
          const detail = ctx.page.locator('[data-memory-detail]:visible');
          await detail.waitFor({ timeout: 10000 });
          await detail.locator('[data-memory-content]').getByText(SECRET).waitFor({ timeout: 5000 });
          ctx.assert((await detail.innerText()).includes('Solo lectura'), 'falta la marca "Solo lectura"');
          ctx.assert((await detail.locator('form, textarea, input, [contenteditable]').count()) === 0, 'el detalle trae controles de edicion');
          await shot(ctx, 'memories-4-detail.png');
        });
      },
    },
    {
      name: 'memorias: pulsar en cualquier parte de la tarjeta abre la memoria y su Markdown se ve renderizado, sin ejecutar HTML',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        const title = `Con formato ${tag}`;
        const now = new Date().toISOString().replace('T', ' ').slice(0, 19);
        fake.extra = [{
          id: 301, sync_id: 's-301', session_id: 'x', type: 'decision', title,
          content: `**Negrita-${tag}**\n\n- punto uno\n- punto dos\n\n[mala](javascript:window.__pwn=1) <script>window.__pwn=1</script>`,
          project: PROJECT, scope: 'project', topic_key: 'fmt/x', revision_count: 1, duplicate_count: 0, last_seen_at: now,
        }];
        try {
          await withFake(async () => {
            const id = await workspaceId(ctx, f.matchName);
            await ctx.goto(`/workspaces/${id}/memories`);
            const card = ctx.page.locator('[data-memories-list]:visible [data-memory-row]', { hasText: title });
            await card.waitFor({ timeout: 10000 });
            // Tap the type and date line, not the title: the whole card is the link.
            await card.locator('span.text-xs').click();
            const content = ctx.page.locator('[data-memory-detail]:visible [data-memory-content]');
            await content.waitFor({ timeout: 10000 });
            await content.locator('strong', { hasText: `Negrita-${tag}` }).waitFor({ timeout: 5000 });
            ctx.assert((await content.locator('li').count()) === 2, 'la lista de Markdown no se renderiza');
            ctx.assert((await content.locator('script').count()) === 0, 'se inyecto un <script>');
            ctx.assert((await content.locator('a[href^="javascript:"]').count()) === 0, 'se dejo un enlace javascript:');
            ctx.assert((await ctx.page.evaluate(() => window.__pwn)) === undefined, 'se ejecuto HTML del contenido');
            await shot(ctx, 'memories-9-markdown.png');
          });
        } finally {
          fake.extra = [];
        }
      },
    },
    {
      name: 'memorias: un workspace sin proyecto no puede leer una memoria ajena',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          const id = await workspaceId(ctx, f.otherName);
          await ctx.goto(`/workspaces/${id}/memories/${MEMORIES[0].id}`);
          await ctx.page.getByText('memory not found').first().waitFor({ timeout: 10000 });
          ctx.assert(!(await ctx.page.innerText('body')).includes(SECRET), 'se filtro el contenido de otro proyecto');
          await shot(ctx, 'memories-5-foreign-memory.png');
        });
      },
    },
    {
      name: 'memorias: sin Engram la pagina dice que no esta disponible',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        let id;
        await withFake(async () => { id = await workspaceId(ctx, f.matchName); });
        await stopFake();
        await ctx.goto(`/workspaces/${id}/memories`);
        await ctx.page.getByText(/no está disponible/).first().waitFor({ timeout: 10000 });
        await shot(ctx, 'memories-6-unavailable.png');
      },
    },
    {
      name: 'tarea: "Memorias" está a la derecha de "Ver diff" y separa las de esta rama de las anteriores',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          const id = await finishedTaskIn(ctx, f.matchName);
          // A memory written after the task started: it belongs to the branch. The fixed ones predate the task.
          const now = new Date().toISOString().replace('T', ' ').slice(0, 19);
          fake.extra = [{
            id: 201, sync_id: 's-201', session_id: 'x', type: 'bugfix', title: `Memoria de la rama ${tag}`,
            content: `RAMA-CONTENIDO-${tag}`, project: PROJECT, scope: 'project', topic_key: 'branch/x',
            revision_count: 1, duplicate_count: 0, last_seen_at: now,
          }];
          try {
            await ctx.page.getByText('Acciones', { exact: true }).first().click();
            const diff = ctx.page.getByRole('button', { name: 'Ver diff' });
            const memories = ctx.page.getByRole('button', { name: 'Memorias' });
            await memories.waitFor({ timeout: 10000 });
            const a = await diff.boundingBox();
            const b = await memories.boundingBox();
            ctx.assert(a && b && a.x + a.width <= b.x && Math.abs(a.y - b.y) < 6, 'Memorias no esta a la derecha de Ver diff');
            await shot(ctx, 'memories-7-task-actions.png');
            await memories.click();
            await ctx.page.waitForURL(new RegExp(`/tasks/${id}/memories$`), { timeout: 15000 });
            const page = ctx.page.locator('[data-task-memories]:visible');
            await page.waitFor({ timeout: 10000 });
            const branch = page.locator('[data-memory-group="branch"]');
            const earlier = page.locator('[data-memory-group="earlier"]');
            await branch.getByText(`Memoria de la rama ${tag}`).waitFor({ timeout: 10000 });
            ctx.assert((await branch.locator('[data-memory-row]').count()) === 1, 'el grupo de la rama no trae solo la nueva');
            ctx.assert((await earlier.locator('[data-memory-row]').count()) === MEMORIES.length, 'las anteriores no son las 3 previas');
            ctx.assert(!(await page.innerText()).includes(`RAMA-CONTENIDO-${tag}`), 'la lista muestra el contenido');
            await shot(ctx, 'memories-8-task-memories.png');
            await branch.getByRole('link', { name: `Memoria de la rama ${tag}` }).click();
            await ctx.page.locator('[data-memory-detail]:visible [data-memory-content]').getByText(`RAMA-CONTENIDO-${tag}`).waitFor({ timeout: 10000 });
          } finally {
            fake.extra = [];
          }
        });
      },
    },
    {
      name: 'tarea sin empezar: el detalle de una tarea en backlog no ofrece "Memorias" (no tiene rama)',
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          await ctx.goto(`/tasks/${f.taskId}`);
          await ctx.page.locator('details[data-task-memory]').waitFor({ state: 'attached', timeout: 10000 });
          ctx.assert((await ctx.page.locator(`a[href="/tasks/${f.taskId}/memories"]`).count()) === 0, 'una tarea en backlog ofrece Memorias');
        });
      },
    },
    {
      name: 'solo lectura: el servidor de Engram solo recibió peticiones GET',
      // The test makes its own page loads with the fake up and checks the requests those loads produced. Requests
      // from the fixture build or from other tests are not evidence, so the log is cleared first.
      fn: async (ctx) => {
        const f = await readyFixture(ctx);
        await withFake(async () => {
          fake.requests.length = 0;
          await ctx.goto('/workspaces');
          await rowOf(ctx, f.matchName).getByText(CHIP_MANY, { exact: true }).waitFor({ timeout: 10000 });
          await ctx.goto(`/tasks/${f.taskId}`);
          await ctx.page.locator('details[data-task-memory]').waitFor({ state: 'attached', timeout: 10000 });
          if (fake.requests.length === 0) throw new Error('el servidor no consultó Engram al cargar las páginas');
          const others = fake.requests.filter((request) => !request.startsWith('GET '));
          if (others.length > 0) throw new Error(`peticiones que no son GET: ${others.join(', ')}`);
          const paths = new Set(fake.requests.map((request) => request.slice(4).split('?')[0]));
          if (!paths.has('/projects') || !paths.has('/observations/recent')) {
            throw new Error(`faltan lecturas de Engram: ${[...paths].join(', ')}`);
          }
        });
      },
    },
  ],
};
