// end2end/go/task_header_workspace.e2e.cjs
// Go-mode e2e of the workspace line in the header of a task: a task shows the name of its workspace under its title, started
// (running) or still in the backlog; its branch shows in the Actions panel of a started one. The suite runs in Spanish and leaves a screenshot of the header.

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `hdr-ws-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `hdr ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Registers a git repo as a workspace, creates a task in it (started or left in the backlog) and opens its detail.
async function openTask(ctx, kind, start) {
  const page = ctx.page;
  const wsName = `hdr-ws-${kind}-${tag}`;
  const dir = uniqueGitRepo(kind);
  await ctx.goto('/workspaces/new');
  await page.getByLabel('Nombre').first().fill(wsName);
  await page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await page.getByLabel(/Rama base/).first().fill('main');
  await page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await page.getByText(dir).first().waitFor({ timeout: 12000 });

  await ctx.goto('/');
  await page.locator('#board-sidebar').getByRole('link', { name: wsName }).first().click();
  await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
  const prompt = `header-ws-${kind}-${tag}`;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${wsName} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  if (start) await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  // Navigating before the create -> start chain ends cancels the start: wait for the modal to close first.
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = page.locator('[id^="board-list-"]:visible [data-stage]', { hasText: prompt }).locator('a[href*="/tasks/"]').first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  await sleep(1500);
  // A backlog card links to the edit page; its detail is the same address without "/edit".
  await ctx.goto((await link.getAttribute('href')).replace(/\/edit$/, ''));
  await page.waitForURL(/\/tasks\/[0-9a-f-]{36}/, { timeout: 15000, waitUntil: 'commit' });
  return wsName;
}

module.exports = {
  name: 'task_header_workspace',
  tests: [
    {
      name: 'cabecera: una tarea en ejecución muestra su workspace bajo el título y su rama en Acciones',
      fn: async (ctx) => {
        const page = ctx.page;
        const wsName = await openTask(ctx, 'run', true);
        const line = page.locator('[data-task-workspace]:visible').first();
        await line.waitFor({ timeout: 10000 });
        ctx.assert((await line.innerText()).includes(wsName), 'la cabecera debe mostrar el nombre del workspace');
        await page.getByRole('tab', { name: 'Acciones' }).first().click();
        const branch = page.locator('[data-task-branch]:visible').first();
        await branch.waitFor({ timeout: 10000 });
        ctx.assert((await branch.innerText()).startsWith('pi/'), 'el panel de acciones debe mostrar la rama de la tarea');
        ctx.assert(await line.locator('xpath=..').getByText(/desde main/).count() === 0, 'la cabecera ya no debe mostrar la línea de la rama');
        await ctx.screenshot(path.join(process.env.SMOKE_SCREENSHOT_DIR || TMP, 'task_header_workspace.png'));
      },
    },
    {
      name: 'cabecera: una tarea en backlog también muestra su workspace',
      fn: async (ctx) => {
        const wsName = await openTask(ctx, 'backlog', false);
        const line = ctx.page.locator('[data-task-workspace]:visible').first();
        await line.waitFor({ timeout: 10000 });
        ctx.assert((await line.innerText()).includes(wsName), 'la cabecera de una tarea en backlog debe mostrar el workspace');
      },
    },
  ],
};
