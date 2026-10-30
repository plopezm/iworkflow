// end2end/go/task_composer.e2e.cjs
// The message box of the detail page of a live task (web/static/js/composer.js, POST /tasks/{id}/messages).
// The fake agent answers "eco: <text>" to any message and reports no mode, so /plan is an ordinary message here.
// The suite runs in Spanish (the language cookie of the harness).
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const PNG_BASE64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `composer-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `composer ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Creates a task that starts at once in a fresh repository and opens its detail page once the agent has answered.
// Returns the id of the task.
async function liveTask(ctx, name) {
  const dir = uniqueGitRepo(name);
  const prompt = `composer-${name}-${tag}`;
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await ctx.page.getByLabel(/^Workspace/).selectOption({ label: `${name} (main)` });
  await ctx.page.getByLabel('Prompt').fill(prompt);
  await ctx.page.getByLabel('Iniciar ahora').check();
  await ctx.page.getByRole('button', { name: 'Crear tarea' }).click();
  // Navigating before the create -> start chain ends cancels the start (as tasks.e2e does): wait for the modal to close.
  await ctx.page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = ctx.page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  const id = ((await link.getAttribute('href')) || '').match(/\/tasks\/([0-9a-f-]{36})/)?.[1];
  ctx.assert(id, 'no se obtuvo el id de la tarea creada');
  await ctx.goto(`/tasks/${id}`);
  await ctx.page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
  // A message sent while the agent's turn is still open is a steer, which the fake agent refuses: wait for the turn to settle.
  await ctx.page.getByText('Esperando', { exact: true }).first().waitFor({ timeout: 20000 });
  return id;
}

const box = (ctx) => ctx.page.locator('[data-composer-input]').first();
const sendButton = (ctx) => ctx.page.getByRole('button', { name: 'Enviar' }).first();

// Dispatches a paste of one PNG on the message box; returns whether the script took it.
function pasteImage(ctx, name) {
  return ctx.page.evaluate(({ name, base64 }) => {
    const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
    const data = new DataTransfer();
    data.items.add(new File([bytes], name, { type: 'image/png' }));
    const event = new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true });
    document.querySelector('[data-composer-input]').dispatchEvent(event);
    return event.defaultPrevented;
  }, { name, base64: PNG_BASE64 });
}

module.exports = {
  name: 'task_composer',
  tests: [
    {
      name: 'Ctrl+Enter envía el mensaje, el agente responde y el campo se vacía',
      fn: async (ctx) => {
        const id = await liveTask(ctx, 'ctrl-enter');
        const text = `hola-ctrl-${tag}`;
        await box(ctx).fill(text);
        await box(ctx).press('Control+Enter');
        await ctx.page.getByText(`eco: ${text}`).first().waitFor({ timeout: 20000 });
        ctx.assert((await box(ctx).inputValue()) === '', 'el campo debe vaciarse tras enviar');
        ctx.assert(id, 'la tarea debe existir');
      },
    },
    {
      name: 'el botón Enviar envía el mensaje y está deshabilitado con el campo vacío',
      fn: async (ctx) => {
        await liveTask(ctx, 'send-button');
        // The button is disabled by the script once the page is up: wait for that state instead of reading it early.
        await ctx.page.waitForFunction(() => document.querySelector('[data-composer-send]')?.disabled === true);
        ctx.assert(await sendButton(ctx).isDisabled(), 'Enviar debe estar deshabilitado con el campo vacío');
        const text = `hola-boton-${tag}`;
        await box(ctx).fill(text);
        await sendButton(ctx).click();
        await ctx.page.getByText(`eco: ${text}`).first().waitFor({ timeout: 20000 });
      },
    },
    {
      name: '/plan sin modo reportado por el agente es un mensaje normal',
      fn: async (ctx) => {
        await liveTask(ctx, 'plan-plain');
        // The mode is known once the agent's first turn has settled, which liveTask waits for.
        await box(ctx).waitFor();
        ctx.assert(
          (await ctx.page.locator('.composer-mode-option').count()) === 0,
          'el agente falso no reporta modo: no debe aparecer el selector de modo',
        );
        await box(ctx).fill('/plan');
        await box(ctx).press('Control+Enter');
        await ctx.page.getByText('eco: /plan').first().waitFor({ timeout: 20000 });
      },
    },
    {
      name: 'escribir / abre el menú de comandos y Enter inserta el comando',
      fn: async (ctx) => {
        await liveTask(ctx, 'slash-menu');
        await box(ctx).fill('/co');
        const items = ctx.page.locator('li.composer-menu-item');
        await items.first().waitFor({ timeout: 5000 });
        const first = await items.first().textContent();
        ctx.assert(first.includes('/compact'), `el primer comando debe ser /compact y es "${first}"`);
        await box(ctx).press('Enter');
        ctx.assert((await box(ctx).inputValue()) === '/compact ', 'Enter debe insertar el comando con un espacio');
      },
    },
    {
      name: 'pegar una imagen en el mensaje la previsualiza en el selector del compositor',
      fn: async (ctx) => {
        await liveTask(ctx, 'paste-image');
        const name = `pegada-composer-${tag}.png`;
        const taken = await pasteImage(ctx, name);
        ctx.assert(taken, 'el pegado de una imagen debe interceptarse');
        await ctx.page.locator(`[data-image-previews] img[alt="${name}"]`).first().waitFor({ timeout: 10000 });
        // An image alone is a message: the send button follows the picked images, with no text in the box.
        await ctx.page.waitForFunction(() => document.querySelector('[data-composer-send]')?.disabled === false, null, { timeout: 5000 });
      },
    },
  ],
};
