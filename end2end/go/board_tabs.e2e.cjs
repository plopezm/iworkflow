// end2end/go/board_tabs.e2e.cjs
// Two board tabs open at once (the home board and a workspace board) must each work on their own elements. The
// dock keeps both in the document, so an id the boards share used to resolve to the hidden one: the create dialog
// of the visible board opened in the hidden tab and the new task never showed where the user was looking.
// The steps select by role, label and visibility (not by element id), so they check the behaviour only.
// The suite runs in Spanish (the language cookie of the harness), so the texts are the Spanish ones.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `board-tabs-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `board tabs ${prefix}\n`);
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

// ---- tests ----
module.exports = {
  name: 'board_tabs',
  tests: [
    {
      name: 'two_tabs: "Nueva tarea" abre el diálogo en el tablero visible y la tarea aparece en su lista',
      fn: async (ctx) => {
        const page = ctx.page;
        const ws1 = await registerWs(ctx, `bt-one-${tag}`);
        const ws2 = await registerWs(ctx, `bt-two-${tag}`);
        await ctx.goto('/');
        await page.locator('#board-sidebar').getByRole('link', { name: ws1.name }).first().click();
        await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
        // The second workspace board opens as a second tab; the first stays in the document, hidden.
        await page.locator('#board-sidebar').getByRole('link', { name: ws2.name }).first().click();
        await page.locator('input[name=workspace]').nth(1).waitFor({ state: 'attached', timeout: 10000 });
        ctx.assert((await page.locator('input[name=workspace]').count()) === 2, 'los dos tableros deben estar abiertos a la vez');

        // Role queries match only what is visible, so this is the button of the visible board.
        await page.getByRole('button', { name: 'Nueva tarea' }).first().click();

        const dialogWorkspace = page.getByLabel(/^Workspace/);
        await dialogWorkspace.waitFor({ state: 'attached', timeout: 10000 });
        ctx.assert(await dialogWorkspace.isVisible(), 'el diálogo debe abrirse en el tablero visible, no en la pestaña oculta');
        await dialogWorkspace.selectOption({ label: `${ws2.name} (main)` });

        const prompt = `two-tabs-${tag}`;
        await page.locator('textarea[name=prompt]:visible').fill(prompt);
        await page.locator('form:visible button[type=submit]', { hasText: 'Crear tarea' }).click();

        await page.locator('[data-stage]:visible').getByText(prompt).first().waitFor({ timeout: 10000 });
      },
    },
  ],
};
