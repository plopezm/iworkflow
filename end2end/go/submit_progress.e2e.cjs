// end2end/go/submit_progress.e2e.cjs
// The progress panel that covers the create and edit forms while their request waits for the server (SubmitProgress,
// web/static/js/submit_progress.js). With fake agents the request answers at once, so each test holds the POST back with
// page.route to keep the panel on screen. The suite runs in Spanish. Screenshots go to IWF_E2E_SCREENSHOT_DIR
// (default /tmp/iwf-e2e-screens/submit-progress); the run prints their paths.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.IWF_E2E_SCREENSHOT_DIR || '/tmp/iwf-e2e-screens/submit-progress';
const CREATE = 'form:has(select[name=workspace_id]):visible';
const PANEL = '[data-submit-progress]';

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `progress-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `progress ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

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

async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
}

// Holds back the POST whose path matches until the returned release() is called or delayMs passed, so the panel stays up.
async function holdPost(page, matches, delayMs) {
  await page.route('**/*', async (route) => {
    const request = route.request();
    if (request.method() === 'POST' && matches(new URL(request.url()).pathname)) {
      await sleep(delayMs);
    }
    await route.continue();
  });
}

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  const file = path.join(SHOTS, `${name}.png`);
  await ctx.page.screenshot({ path: file });
  console.log(`    screenshot: ${file}`);
}

async function openCreate(ctx, ws, prompt, { startNow }) {
  await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  const form = ctx.page.locator(CREATE);
  await form.locator('select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
  await form.locator('textarea[name=prompt]').fill(prompt);
  if (startNow) await form.locator('input[name=start_now]').check(); else await form.locator('input[name=start_now]').uncheck();
  return form;
}

const states = (form) => form.locator(`${PANEL} [data-progress-step]:not([hidden])`).evaluateAll((items) => items.map((li) => li.dataset.state));

module.exports = {
  name: 'submit_progress',
  tests: [
    {
      name: 'create_task: el panel cubre el formulario, avanza por los pasos y avisa cuando tarda',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `sp-create-${tag}`);
        await openBoard(ctx, ws.name);
        await holdPost(ctx.page, (p) => p === '/board/tasks', 9500);
        const form = await openCreate(ctx, ws, `progreso-${tag}`, { startNow: true });
        ctx.assert(!(await form.locator(PANEL).isVisible()), 'el panel no debe verse antes de enviar');
        await shot(ctx, '01-create-form');
        await form.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
        await form.locator(PANEL).waitFor({ state: 'visible', timeout: 3000 });
        ctx.assert((await form.locator('.submit-progress-title').innerText()) === 'Creando tu tarea', 'título del panel de creación');
        ctx.assert((await form.locator(`${PANEL} [data-progress-step]:not([hidden])`).count()) === 4, 'con "Iniciar ahora" hay 4 pasos');
        ctx.assert((await states(form))[0] === 'active', 'el primer paso empieza activo');
        await shot(ctx, '02-create-saving');
        // The stage moves on by itself while the answer is pending.
        await form.locator(`${PANEL} [data-progress-step][data-state=active]`, { hasText: 'Preparando el worktree' }).waitFor({ timeout: 6000 });
        await shot(ctx, '03-create-worktree');
        await form.locator(`${PANEL} [data-progress-step][data-state=active]`, { hasText: 'Arrancando el agente' }).waitFor({ timeout: 6000 });
        ctx.assert(JSON.stringify(await states(form)) === JSON.stringify(['done', 'done', 'done', 'active']), 'los pasos previos quedan hechos');
        // After 8 s the panel explains that the wait is longer than usual.
        await form.locator(`${PANEL} [data-progress-slow]`).waitFor({ state: 'visible', timeout: 6000 });
        ctx.assert(/\d+ s/.test(await form.locator(`${PANEL} [data-progress-elapsed]`).innerText()), 'el contador de segundos');
        await shot(ctx, '04-create-slow-hint');
        // The answer closes the create tab and the task is on the board.
        await form.waitFor({ state: 'detached', timeout: 15000 });
        await ctx.page.locator('[id^="board-list-"]:visible', { hasText: `progreso-${tag}` }).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'create_task: sin "Iniciar ahora" el panel solo muestra el paso de guardar',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `sp-backlog-${tag}`);
        await openBoard(ctx, ws.name);
        await holdPost(ctx.page, (p) => p === '/board/tasks', 2500);
        const form = await openCreate(ctx, ws, `sin-inicio-${tag}`, { startNow: false });
        await form.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
        await form.locator(PANEL).waitFor({ state: 'visible', timeout: 3000 });
        ctx.assert((await form.locator(`${PANEL} [data-progress-step]:not([hidden])`).count()) === 1, 'solo el paso de guardar');
        ctx.assert((await form.locator(`${PANEL} [data-progress-step][hidden]`).count()) === 3, 'los pasos de inicio quedan ocultos');
        await shot(ctx, '05-create-backlog-only');
        await form.waitFor({ state: 'detached', timeout: 10000 });
      },
    },
    {
      name: 'update_task: guardar los cambios muestra el panel de edición y vuelve al tablero',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `sp-edit-${tag}`);
        await openBoard(ctx, ws.name);
        const prompt = `editar-${tag}`;
        const create = await openCreate(ctx, ws, prompt, { startNow: false });
        await create.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
        await create.waitFor({ state: 'detached', timeout: 10000 });
        const card = ctx.page.locator('[id^="board-list-"]:visible [data-stage="backlog"]', { hasText: prompt }).first();
        await card.waitFor({ timeout: 10000 });
        await card.getByRole('button', { name: 'Editar' }).click();
        await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ timeout: 15000 });
        await holdPost(ctx.page, (p) => p.endsWith('/update'), 3500);
        await ctx.page.getByLabel(/^Título/).fill(`titulo-${tag}`);
        await ctx.page.getByLabel('Prompt').fill(`${prompt}-v2`);
        const form = ctx.page.locator('form:has(h1.page-title):visible', { hasText: 'Editar tarea' }).first();
        await form.getByRole('button', { name: 'Guardar cambios' }).click();
        await form.locator(PANEL).waitFor({ state: 'visible', timeout: 3000 });
        ctx.assert((await form.locator('.submit-progress-title').innerText()) === 'Guardando los cambios', 'título del panel de edición');
        ctx.assert((await form.locator(`${PANEL} [data-progress-step]:not([hidden])`).count()) === 3, 'la edición tiene 3 pasos');
        await shot(ctx, '06-edit-saving');
        await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ state: 'detached', timeout: 15000 });
        await ctx.page.locator('[id^="board-list-"]:visible').getByText(`titulo-${tag}`).first().waitFor({ timeout: 15000 });
      },
    },
  ],
};
