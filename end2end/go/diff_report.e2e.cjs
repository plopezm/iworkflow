// end2end/go/diff_report.e2e.cjs
// El informe imprimible del diff de una tarea (el botón Exportar PDF del diff abre el informe; cerrarlo
// responde el cierre del informe). El agente falso escribe CAMBIO.md, así que la tarea tiene un diff que imprimir.
// GET /tasks/{id}/diff/report?action=plain|explained|close.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `diff-report-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `diff report ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`);
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

async function registerWs(ctx, name) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
}

async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await ctx.page.locator('aside').getByRole('link', { name: new RegExp(name) }).first().click();
  await sleep(500);
}

async function taskDetail(ctx, key, prompt, opts = {}) {
  const name = `drep-${key}-${tag}`;
  await registerWs(ctx, name);
  await openBoard(ctx, name);
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${name} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  if (opts.start) await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  const href = await link.getAttribute('href');
  const id = (href.match(/\/tasks\/([0-9a-f-]{36})/) || [])[1];
  await ctx.goto(href);
  await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}/, { timeout: 15000, waitUntil: 'commit' });
  ctx.assert(id, `no se obtuvo el id de la tarea "${prompt}"`);
  return { ws: name, id, prompt };
}

async function liveTask(ctx, key) {
  const prompt = `${key}-${tag}`;
  const t = await taskDetail(ctx, key, prompt, { start: true });
  await ctx.page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
  await ctx.page.getByRole('heading', { name: '❓ ¿Seguir?' }).waitFor({ timeout: 15000 });
  return t;
}

async function openDiff(ctx) {
  const page = ctx.page;
  await openActions(ctx);
  await page.getByRole('button', { name: 'Ver diff' }).click();
  await page.waitForURL(/\/tasks\/[0-9a-f-]{36}\/diff$/, { timeout: 15000 });
}

async function openActions(ctx) {
  const page = ctx.page;
  await page.getByText('Acciones', { exact: true }).first().click();
  await sleep(300);
}

module.exports = {
  name: 'diff_report',
  tests: [
    {
      name: 'el informe del diff se abre desde Exportar PDF y se cierra',
      fn: async (ctx) => {
        const t = await liveTask(ctx, 'informe');
        await openDiff(ctx);
        // El diff del agente falso (CAMBIO.md) se lee en la pestaña.
        await ctx.page.getByText('cambio del agente').first().waitFor({ timeout: 20000 });
        // El informe se abre con el botón del diff y no se cierra solo.
        await ctx.page.getByRole('button', { name: 'Exportar PDF' }).first().click();
        const report = ctx.page.locator('.print-report');
        await report.waitFor({ state: 'visible', timeout: 15000 });
        await ctx.page.locator('.print-report-page').waitFor({ state: 'visible', timeout: 15000 });
        ctx.assert(
          (await ctx.page.locator('.print-report').count()) === 1,
          'el informe debe responder con una sola copia del informe',
        );
        // El cierre del informe responde el cierre del report del diff.
        await ctx.page.getByRole('button', { name: 'Cerrar' }).first().click();
        await ctx.page.locator('.print-report').waitFor({ state: 'detached', timeout: 15000 });
        ctx.assert(!ctx.page.url().includes('/diff'), 'el cierre del informe no debe navegar');
      },
    },
  ],
};
