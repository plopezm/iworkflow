// end2end/go/board_live_sidebar.e2e.cjs
// Go-mode regression for the live sidebar (slice H, T18): a task created from the board of one workspace, for
// another workspace, must reach the "Requieren atención" section of the sidebar without a reload. The task stream
// reports the change when the agent goes idle, and the sidebar listens to it. Its listening element must stay in the
// page, or the last event is lost while the element is replaced.
// The suite runs in Spanish (the language cookie of the harness), so the texts are the Spanish ones.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `live-sidebar-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `live sidebar ${prefix}\n`);
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

// Opens the board from the sidebar by clicking on the workspace item.
async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
}

// ---- tests ----
module.exports = {
  name: 'board_live_sidebar',
  tests: [
    {
      name: 'sidebar_attention: una tarea creada para otro workspace aparece en "Requieren atención" sin recargar',
      fn: async (ctx) => {
        const page = ctx.page;
        const origin = await registerWs(ctx, `ls-origin-${tag}`);
        const target = await registerWs(ctx, `ls-target-${tag}`);
        await openBoard(ctx, origin.name);
        // Same page for the whole test: a reload would hide a lost event, so the marker must survive.
        await page.evaluate(() => { window.__noReloadMarker = 'kept'; });

        const prompt = `live-sidebar-${tag}`;
        await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${target.name} (main)` });
        await page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').fill(prompt);
        await page.getByRole('checkbox', { name: /Iniciar ahora/ }).first().check();
        await page.locator('form:has(select[name=workspace_id]):visible button[type=submit]', { hasText: 'Crear tarea' }).click();
        await page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});

        // The id is read from the list of the target workspace, which the sidebar links to.
        const boardHref = await page.locator(`#board-sidebar a[href^="/board/"]`, { hasText: target.name }).first().getAttribute('href');
        const taskId = await page.evaluate(async (boardHref) => {
          const deadline = Date.now() + 10000;
          while (Date.now() < deadline) {
            const workspaceId = boardHref.replace('/board/', '');
            const html = await (await fetch(`/board/list?workspace=${encodeURIComponent(workspaceId)}`)).text();
            const match = html.match(/\/tasks\/([0-9a-f-]{36})/);
            if (match) return match[1];
            await new Promise((r) => setTimeout(r, 200));
          }
          return '';
        }, boardHref);
        ctx.assert(/^[0-9a-f-]{36}$/.test(taskId), 'la tarea creada debe existir en el workspace de destino');

        await page.locator(`#board-sidebar a[href="/tasks/${taskId}"]`).first().waitFor({ timeout: 10000 });
        ctx.assert(await page.evaluate(() => window.__noReloadMarker === 'kept'), 'la prueba no debe recargar la página');
      },
    },
  ],
};
