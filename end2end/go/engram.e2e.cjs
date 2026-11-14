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
// The suite runs in Spanish. Tests run in declaration order: the last one stops the fake.

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
const fake = { server: null, directories: [], requests: [] };

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
    sendJSON(res, 200, MEMORIES);
    return;
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
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
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

// Shared fixture: a workspace whose repo the fake reports as the project directory, a workspace with no
// matching project, and one task in the matching workspace. Built once by the first test that needs it; later
// tests reuse the same promise, so a failed build is reported by its first test and not retried into bad repos.
let fixturePromise = null;
function ensureFixture(ctx) {
  if (!fixturePromise) fixturePromise = buildFixture(ctx);
  return fixturePromise;
}

async function buildFixture(ctx) {
  await ensureFake();
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

// The workspace row (article.card) of the given workspace name.
const rowOf = (ctx, name) => ctx.page.locator('article.card', { hasText: name }).first();

module.exports = {
  name: 'engram',
  tests: [
    {
      name: 'chip: la fila del workspace con proyecto coincidente muestra "Engram · 3 memorias"',
      fn: async (ctx) => {
        const f = await ensureFixture(ctx);
        await ctx.goto('/workspaces');
        await rowOf(ctx, f.matchName).getByText(CHIP_MANY, { exact: true }).waitFor({ timeout: 10000 });
      },
    },
    {
      name: 'chip: un workspace sin proyecto coincidente muestra "Engram · sin memorias"',
      fn: async (ctx) => {
        const f = await ensureFixture(ctx);
        await ctx.goto('/workspaces');
        await rowOf(ctx, f.otherName).getByText(CHIP_NONE, { exact: true }).waitFor({ timeout: 10000 });
        ctx.assert((await rowOf(ctx, f.otherName).getByText(CHIP_MANY).count()) === 0, 'el workspace sin proyecto muestra memorias');
      },
    },
    {
      name: 'panel: el detalle de la tarea trae "Memoria" colapsado y al abrirlo lista título y tipo de cada memoria',
      fn: async (ctx) => {
        const f = await ensureFixture(ctx);
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
      },
    },
    {
      name: 'privacidad: el contenido de una memoria nunca aparece en la página de la tarea',
      fn: async (ctx) => {
        const f = await ensureFixture(ctx);
        await ctx.goto(`/tasks/${f.taskId}`);
        const panel = ctx.page.locator('details[data-task-memory]');
        await panel.waitFor({ state: 'attached', timeout: 10000 });
        await panel.locator('summary').click();
        await panel.locator('[data-memory-row]').first().waitFor({ state: 'visible', timeout: 5000 });
        const html = await ctx.page.content();
        ctx.assert(!html.includes(SECRET), 'el contenido de la memoria aparece en el HTML de la página');
        const text = await ctx.page.innerText('body');
        ctx.assert(!text.includes(SECRET), 'el contenido de la memoria aparece en el texto de la página');
      },
    },
    {
      name: 'sin Engram: Workspaces responde con "Engram no disponible" y el detalle no trae panel de memoria',
      fn: async (ctx) => {
        const f = await ensureFixture(ctx);
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
      name: 'solo lectura: el servidor de Engram solo recibió peticiones GET',
      // No browser state is needed: the requests the fake recorded are the evidence.
      fn: async () => {
        if (fake.requests.length === 0) throw new Error('el servidor no consultó Engram en ninguna prueba');
        const others = fake.requests.filter((request) => !request.startsWith('GET '));
        if (others.length > 0) throw new Error(`peticiones que no son GET: ${others.join(', ')}`);
        const paths = new Set(fake.requests.map((request) => request.slice(4).split('?')[0]));
        if (!paths.has('/projects') || !paths.has('/observations/recent')) {
          throw new Error(`faltan lecturas de Engram: ${[...paths].join(', ')}`);
        }
      },
    },
  ],
};
