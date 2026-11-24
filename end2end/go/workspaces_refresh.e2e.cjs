// end2end/go/workspaces_refresh.e2e.cjs
// Go-mode regression for the self-refresh of the workspaces list (`#workspace-table`): the element stays in the page and
// morphs its content (`hx-swap="morph:innerHTML"`) from GET /workspaces/table whenever the task stream reports a change,
// the board asks for a refresh, or a workspace form is posted. Every check reads an observable effect: a card in the list
// without navigating, the requests the table makes, and the identity of the nodes across a refresh.
// Each test registers its own git repository (the server rejects the same root twice) with a unique name.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const TABLE = '#workspace-table';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `ws-refresh-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `content ${prefix} ${tag}`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Opens the list and waits until it is there and the stream has settled.
async function openList(ctx) {
  await ctx.goto('/workspaces');
  await ctx.page.locator(TABLE).waitFor({ timeout: 10000 });
}

// Records, inside the page, every request the table makes (htmx:beforeRequest on the listening element).
async function recordTableRequests(page) {
  await page.evaluate((selector) => {
    window.__tableRequests = [];
    document.querySelector(selector).addEventListener('htmx:beforeRequest', (event) => {
      if (event.target.id === 'workspace-table') window.__tableRequests.push(event.detail.pathInfo.requestPath);
    });
  }, TABLE);
}

async function tableRequests(page) {
  return page.evaluate(() => window.__tableRequests.slice());
}

async function waitUntil(read, timeout = 12000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (await read().catch(() => false)) return true;
    await sleep(150);
  }
  return false;
}

// Registers a workspace with a plain request from the page, the way a form does, without touching the list.
async function postRegister(page, name, repo) {
  await page.evaluate(async ({ name, repo }) => {
    const body = new URLSearchParams({ name, path: repo, base_branch: 'main' });
    const response = await fetch('/workspaces/register', {
      method: 'POST',
      headers: { 'HX-Request': 'true', 'Content-Type': 'application/x-www-form-urlencoded' },
      body,
    });
    if (!response.ok) throw new Error(`register ${response.status}`);
  }, { name, repo });
}

const hasCard = (page, name) => page.locator('article.card', { hasText: name }).count().then((n) => n > 0);

module.exports = {
  name: 'workspaces_refresh',
  tests: [
    {
      name: 'stream_notice: un aviso del stream recarga la tabla y muestra el workspace nuevo sin navegar',
      fn: async (ctx) => {
        const name = `refresh-stream-${tag}`;
        await openList(ctx);
        await recordTableRequests(ctx.page);
        await postRegister(ctx.page, name, uniqueGitRepo('stream'));
        ctx.assert(!(await hasCard(ctx.page, name)), 'la tabla no debe mostrar el workspace antes del aviso');

        await ctx.page.evaluate((selector) => {
          document.querySelector(selector).dispatchEvent(new CustomEvent('sse:changed', { detail: { data: 'task' } }));
        }, TABLE);

        ctx.assert(await waitUntil(() => hasCard(ctx.page, name)), `el workspace "${name}" debe aparecer sin recargar la página`);
        const requests = await tableRequests(ctx.page);
        ctx.assert(requests.includes('/workspaces/table'), `la tabla debe pedir /workspaces/table (pidió ${JSON.stringify(requests)})`);
      },
    },
    {
      name: 'morph: la recarga conserva el elemento de la tabla y los nodos de las tarjetas',
      fn: async (ctx) => {
        const name = `refresh-morph-${tag}`;
        await openList(ctx);
        await postRegister(ctx.page, name, uniqueGitRepo('morph'));
        await ctx.page.evaluate((selector) => {
          document.querySelector(selector).dispatchEvent(new CustomEvent('sse:changed', { detail: { data: 'task' } }));
        }, TABLE);
        ctx.assert(await waitUntil(() => hasCard(ctx.page, name)), 'la tarjeta debe aparecer tras el primer aviso');

        await ctx.page.evaluate(({ selector, name }) => {
          window.__tableNode = document.querySelector(selector);
          window.__cardNode = [...document.querySelectorAll('article.card')].find((a) => a.textContent.includes(name));
        }, { selector: TABLE, name });
        await recordTableRequests(ctx.page);
        await ctx.page.evaluate((selector) => {
          document.querySelector(selector).dispatchEvent(new CustomEvent('sse:changed', { detail: { data: 'task' } }));
        }, TABLE);
        ctx.assert(await waitUntil(async () => (await tableRequests(ctx.page)).length >= 1), 'el aviso debe provocar una recarga');
        await sleep(600);

        const kept = await ctx.page.evaluate(({ selector, name }) => ({
          table: window.__tableNode === document.querySelector(selector),
          card: window.__cardNode === [...document.querySelectorAll('article.card')].find((a) => a.textContent.includes(name)),
        }), { selector: TABLE, name });
        ctx.assert(kept.table, 'el elemento que escucha debe seguir siendo el mismo');
        ctx.assert(kept.card, 'la tarjeta debe ser el mismo nodo: la recarga hace morph, no reemplaza');
      },
    },
    {
      name: 'delete: tras quitar un workspace la tabla sigue recargándose con los avisos',
      fn: async (ctx) => {
        const name = `refresh-delete-${tag}`;
        const other = `refresh-other-${tag}`;
        await openList(ctx);
        await postRegister(ctx.page, name, uniqueGitRepo('delete'));
        await ctx.page.evaluate((selector) => {
          document.querySelector(selector).dispatchEvent(new CustomEvent('sse:changed', { detail: { data: 'task' } }));
        }, TABLE);
        ctx.assert(await waitUntil(() => hasCard(ctx.page, name)), 'la tarjeta a quitar debe aparecer');

        await ctx.page.locator('article.card', { hasText: name }).getByRole('button', { name: /Quitar|Remove/ }).first().click();
        ctx.assert(await waitUntil(async () => !(await hasCard(ctx.page, name))), 'la tarjeta quitada debe desaparecer');

        // The delete swaps the whole table (outerHTML): the new element must carry its own triggers.
        await postRegister(ctx.page, other, uniqueGitRepo('other'));
        await ctx.page.evaluate((selector) => {
          document.querySelector(selector).dispatchEvent(new CustomEvent('sse:changed', { detail: { data: 'task' } }));
        }, TABLE);
        ctx.assert(await waitUntil(() => hasCard(ctx.page, other)), 'la tabla debe seguir recargándose después de quitar un workspace');
      },
    },
  ],
};
