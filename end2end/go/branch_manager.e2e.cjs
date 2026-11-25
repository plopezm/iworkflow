// end2end/go/branch_manager.e2e.cjs
// The "Ramas" button of a workspace card opens the branch manager (internal/modules/workspace_branches): local and
// remote branches, a server-side filter, per-row and bulk delete with a browser confirmation, and protected rows.
// Every effect is checked against the real repositories (a working repo and a bare "remote"), not only the page.
// The suite runs in Spanish (the language cookie of the harness).
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

const sh = (cmd) => execSync(cmd, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();

// A working repo on main with the local branches old/a, old/b, keep/x, plus a bare origin that holds main and
// old/remote-x (fetched, so the repo has the remote-tracking refs).
function repoWithRemote(prefix) {
  const dir = path.join(TMP, `branch-manager-${prefix}-${tag}`);
  const bare = `${dir}-origin.git`;
  fs.mkdirSync(dir, { recursive: true });
  const git = (args) => sh(`git -C '${dir}' -c user.name=t -c user.email=t@l ${args}`);
  sh(`git init -q --bare -b main '${bare}'`);
  sh(`git -C '${dir}' init -q -b main`);
  fs.writeFileSync(path.join(dir, 'README.md'), `branch manager ${prefix}\n`);
  git('add .');
  git('commit -qm init');
  for (const branch of ['old/a', 'old/b', 'keep/x', 'old/remote-x']) git(`branch '${branch}'`);
  git(`remote add origin '${bare}'`);
  git('push -q origin main old/remote-x');
  git('branch -D old/remote-x');
  git('fetch -q origin');
  return { dir, bare, git };
}

const localBranches = (repo) => repo.git("for-each-ref --format='%(refname:short)' refs/heads").split('\n');
const remoteBranches = (repo) => sh(`git -C '${repo.bare}' for-each-ref --format='%(refname:short)' refs/heads`).split('\n');

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Registers the workspace and opens its branch manager from the "Ramas" button of its card.
async function openManager(ctx, name) {
  const repo = repoWithRemote(name);
  await ctx.goto('/workspaces/new');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(repo.dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  const card = ctx.page.locator('article.card').filter({ hasText: name }).first();
  await card.waitFor({ timeout: 12000 });
  await card.getByText('Ramas', { exact: true }).first().click();
  await ctx.page.locator('#workspace-branches').waitFor({ timeout: 10000 });
  await ctx.page.locator('#branch-list li').first().waitFor({ timeout: 10000 });
  return repo;
}

const rowOf = (page, branch) => page.locator('#branch-list li').filter({ has: page.locator(`input[aria-label="${branch}"]`) });

// Records the text of the confirmations and answers them through the harness (its default listener accepts).
function answerDialogs(ctx, accept) {
  const messages = [];
  const handler = (dialog) => messages.push(dialog.message());
  ctx.page.on('dialog', handler);
  ctx.setDialog(accept ? 'accept' : 'reject');
  return { messages, stop: () => { ctx.page.off('dialog', handler); ctx.setDialog('accept'); } };
}

async function waitUntil(read, timeout = 12000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (await read().catch(() => false)) return true;
    await sleep(150);
  }
  return false;
}

module.exports = {
  name: 'branch_manager',
  tests: [
    {
      name: 'el botón Ramas abre el listado con locales y remotas, y protege la actual y la base',
      fn: async (ctx) => {
        const repo = await openManager(ctx, `bm-list-${tag}`);
        const page = ctx.page;
        for (const name of ['old/a', 'old/b', 'keep/x', 'origin/old/remote-x']) {
          ctx.assert((await rowOf(page, name).count()) === 1, `debe listarse la rama ${name}`);
        }
        const main = rowOf(page, 'main');
        ctx.assert((await main.locator('input[type=checkbox]').isDisabled()), 'la rama actual/base no se puede seleccionar');
        ctx.assert((await main.getByRole('button', { name: /Eliminar|Borrar/ }).isDisabled()), 'la rama actual/base no se puede borrar');
        ctx.assert((await page.locator('#branch-list li').filter({ hasText: 'origin/HEAD' }).count()) <= 1, 'origin/HEAD, si aparece, va protegida');
        await shot(ctx, 'branch-manager-1-list');
        ctx.assert(localBranches(repo).includes('old/a'), 'abrir la pantalla no borra nada');
      },
    },
    {
      name: 'el filtro del servidor reduce el listado',
      fn: async (ctx) => {
        await openManager(ctx, `bm-filter-${tag}`);
        const page = ctx.page;
        await page.locator('#workspace-branches input[type=search]').fill('remote-x');
        await waitUntil(async () => (await page.locator('#branch-list li').count()) === 1);
        const count = await page.locator('#branch-list li').count();
        ctx.assert(count === 1, `filtrar por "remote-x" debe dejar 1 rama y deja ${count}`);
        await shot(ctx, 'branch-manager-2-filtered');
      },
    },
    {
      name: 'cancelar la confirmación no borra la rama',
      fn: async (ctx) => {
        const repo = await openManager(ctx, `bm-cancel-${tag}`);
        const dialogs = answerDialogs(ctx, false);
        await rowOf(ctx.page, 'old/a').getByRole('button', { name: /Eliminar|Borrar/ }).click();
        await sleep(800);
        dialogs.stop();
        ctx.assert(dialogs.messages.length === 1, `debe pedir confirmación una vez y pidió ${dialogs.messages.length}`);
        ctx.assert(localBranches(repo).includes('old/a'), 'la rama sigue existiendo tras cancelar');
        ctx.assert((await rowOf(ctx.page, 'old/a').count()) === 1, 'la fila sigue en la lista');
      },
    },
    {
      name: 'el borrado de una rama local la elimina del repositorio y de la lista',
      fn: async (ctx) => {
        const repo = await openManager(ctx, `bm-local-${tag}`);
        const dialogs = answerDialogs(ctx, true);
        await rowOf(ctx.page, 'old/a').getByRole('button', { name: /Eliminar|Borrar/ }).click();
        const gone = await waitUntil(async () => (await rowOf(ctx.page, 'old/a').count()) === 0);
        dialogs.stop();
        ctx.assert(gone, 'la fila de old/a debe desaparecer de la lista');
        ctx.assert(!localBranches(repo).includes('old/a'), 'old/a debe borrarse del repositorio');
        ctx.assert(localBranches(repo).includes('old/b'), 'otra rama local no se toca (hx-params="none")');
        await shot(ctx, 'branch-manager-3-after-delete');
      },
    },
    {
      name: 'el borrado múltiple elimina locales y remotas, y avisa de que la remota se quita del remoto',
      fn: async (ctx) => {
        const repo = await openManager(ctx, `bm-bulk-${tag}`);
        const page = ctx.page;
        await page.locator('#branch-list input[aria-label="old/b"]').check();
        await page.locator('#branch-list input[aria-label="origin/old/remote-x"]').check();
        await shot(ctx, 'branch-manager-4-selected');
        const dialogs = answerDialogs(ctx, true);
        await page.getByRole('button', { name: /Eliminar seleccionadas|Borrar seleccionadas/ }).click();
        const gone = await waitUntil(async () => (await rowOf(page, 'old/b').count()) === 0
          && (await rowOf(page, 'origin/old/remote-x').count()) === 0);
        dialogs.stop();
        ctx.assert(gone, 'las filas seleccionadas deben desaparecer de la lista');
        ctx.assert(/remoto/i.test(dialogs.messages[0] || ''), `la confirmación debe mencionar el remoto: "${dialogs.messages[0]}"`);
        ctx.assert(!localBranches(repo).includes('old/b'), 'old/b debe borrarse del repositorio local');
        ctx.assert(!remoteBranches(repo).includes('old/remote-x'), 'old/remote-x debe borrarse del remoto bare');
        ctx.assert(remoteBranches(repo).includes('main'), 'main sigue en el remoto');
        ctx.assert(localBranches(repo).includes('keep/x'), 'las ramas no seleccionadas siguen existiendo');
        await shot(ctx, 'branch-manager-5-after-bulk');
      },
    },
    {
      name: 'seleccionar todas marca solo las ramas borrables y el borrado las elimina',
      fn: async (ctx) => {
        const repo = await openManager(ctx, `bm-all-${tag}`);
        const page = ctx.page;
        await page.locator('[data-branch-select-all]').check();
        const checked = await page.locator('#branch-list input[name=branch]:checked').count();
        ctx.assert(checked === 4, `deben marcarse las 4 ramas borrables y hay ${checked}`);
        ctx.assert(!(await rowOf(page, 'main').locator('input[type=checkbox]').isChecked()), 'la rama protegida no se marca');
        await shot(ctx, 'branch-manager-6-select-all');
        await page.locator('#branch-list input[aria-label="old/a"]').uncheck();
        ctx.assert(await page.locator('[data-branch-select-all]').evaluate((el) => el.indeterminate), 'con una desmarcada la casilla queda mixta');
        await page.locator('[data-branch-select-all]').check();
        const dialogs = answerDialogs(ctx, true);
        await page.getByRole('button', { name: /Eliminar seleccionadas/ }).click();
        const gone = await waitUntil(async () => (await page.locator('#branch-list input[name=branch]:not([disabled])').count()) === 0);
        dialogs.stop();
        ctx.assert(gone, 'no deben quedar ramas borrables');
        ctx.assert(localBranches(repo).join(',') === 'main', `solo debe quedar main y queda "${localBranches(repo).join(',')}"`);
        ctx.assert(remoteBranches(repo).join(',') === 'main', 'el remoto conserva solo main');
      },
    },
  ],
};
