// end2end/go/open_with.e2e.cjs
// Go-mode e2e of "Abrir con": the dialog of the Actions tab of a task with a worktree, the applications it stores
// on the server, the launch in the worktree and the file picker of the server. The tasks come from
// tasks_actions.e2e.cjs (a task finished by the fake agent keeps its worktree). The suite runs in Spanish.
// The applications are global and the DB is shared, so every test uses a name of its own.

const fs = require('fs');
const path = require('path');
const { finishedTask, openActions, sleep } = require('./tasks_actions.e2e.cjs').helpers;

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

// Opens the task and its "Abrir con" dialog. Returns the locator of the dialog.
async function openDialog(ctx, key) {
  const task = await finishedTask(ctx, key);
  await openActions(ctx);
  await ctx.page.getByRole('button', { name: 'Abrir con', exact: true }).click();
  const dialog = ctx.page.locator('[data-modal]').filter({ hasText: 'Abrir con' });
  await dialog.locator('[data-open-with-new]').waitFor({ timeout: 10000 });
  // The add form starts collapsed behind "Añadir nuevo +".
  await dialog.locator('[data-open-with-new]').click();
  await dialog.getByRole('button', { name: 'Añadir', exact: true }).waitFor({ timeout: 10000 });
  return { task, dialog };
}

async function addApp(dialog, { name, p, args = '', env = '' }) {
  await dialog.getByLabel('Nombre', { exact: true }).fill(name);
  await dialog.getByLabel(/^Ruta del ejecutable/).fill(p);
  if (args) await dialog.getByLabel(/^Parámetros extra/).fill(args);
  if (env) await dialog.getByLabel(/^Variables de entorno/).fill(env);
  await dialog.getByRole('button', { name: 'Añadir', exact: true }).click();
}

function row(dialog, name) {
  return dialog.locator('[data-launcher]').filter({ hasText: name });
}

// The worktree directory shown in the detail of the task.
async function worktreeOf(ctx) {
  const text = await ctx.page.evaluate(() => document.body.innerText);
  const found = text.match(/\/\S*worktrees\/\S+/);
  ctx.assert(found, 'no se encontró la ruta del worktree en el detalle de la tarea');
  return found[0];
}

async function waitForFile(file, timeout = 10000) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    if (fs.existsSync(file)) return fs.readFileSync(file, 'utf8');
    await sleep(200);
  }
  throw new Error(`el fichero ${file} no apareció`);
}

module.exports = {
  name: 'open_with',
  tests: [
    {
      name: 'dialog: sin aplicaciones invita a añadir una y la guarda con sus datos opcionales',
      fn: async (ctx) => {
        const { dialog } = await openDialog(ctx, 'ow-add');
        const name = `Editor ${tag}`;
        await addApp(dialog, { name, p: '/usr/bin/true', args: '--a\n--b', env: 'K=v' });
        const added = row(dialog, name);
        await added.waitFor({ timeout: 10000 });
        await ctx.page.getByText('/usr/bin/true').first().waitFor();
        ctx.assert((await added.innerText()).includes('2 parámetros'), 'la fila no resume los 2 parámetros');
        ctx.assert((await added.innerText()).includes('1 entorno'), 'la fila no resume la variable de entorno');
        // Survives a reload: it is stored on the server.
        await ctx.page.reload({ waitUntil: 'domcontentloaded' });
        await openActions(ctx);
        await ctx.page.getByRole('button', { name: 'Abrir con', exact: true }).click();
        await ctx.page.locator('[data-launcher]').filter({ hasText: name }).waitFor({ timeout: 10000 });
      },
    },
    {
      name: 'dialog: una aplicación sin nombre o con variable inválida muestra el error y conserva lo escrito',
      fn: async (ctx) => {
        const { dialog } = await openDialog(ctx, 'ow-invalid');
        await addApp(dialog, { name: `Mala ${tag}`, p: '/usr/bin/true', env: 'SINIGUAL' });
        await dialog.getByText(/NAME=VALUE/).waitFor({ timeout: 10000 });
        ctx.assert((await dialog.getByLabel('Nombre', { exact: true }).inputValue()) === `Mala ${tag}`, 'el nombre escrito se perdió');
        ctx.assert((await dialog.locator('[data-launcher]').filter({ hasText: `Mala ${tag}` }).count()) === 0, 'se guardó una aplicación inválida');
      },
    },
    {
      name: 'launch: "Abrir" ejecuta la aplicación en el worktree con parámetros y variables de entorno',
      fn: async (ctx) => {
        const { dialog } = await openDialog(ctx, 'ow-launch');
        const name = `Escritor ${tag}`;
        await addApp(dialog, {
          name,
          p: '/bin/sh',
          args: '-c\nprintf %s "$E2E_OPEN_WITH" > opened-by-e2e',
          env: 'E2E_OPEN_WITH=hola',
        });
        await row(dialog, name).waitFor({ timeout: 10000 });
        const worktree = await worktreeOf(ctx);
        await row(dialog, name).getByRole('button', { name: 'Abrir', exact: true }).click();
        // The dialog closes once the application started.
        await dialog.waitFor({ state: 'detached', timeout: 10000 });
        const written = await waitForFile(path.join(worktree, 'opened-by-e2e'));
        ctx.assert(written === 'hola', `la variable de entorno no llegó al proceso: "${written}"`);
      },
    },
    {
      name: 'launch: un ejecutable inexistente muestra el error dentro del diálogo',
      fn: async (ctx) => {
        const { dialog } = await openDialog(ctx, 'ow-missing');
        const name = `Fantasma ${tag}`;
        await addApp(dialog, { name, p: '/no/existe/app' });
        await row(dialog, name).getByRole('button', { name: 'Abrir', exact: true }).click();
        await dialog.getByText(/could not start the application/).waitFor({ timeout: 10000 });
        ctx.assert((await dialog.getByText(/^Lanzada:/).count()) === 0, 'se anunció un lanzamiento fallido');
      },
    },
    {
      name: 'browse: "Examinar…" navega por el servidor, filtra y rellena la ruta sin perder el formulario',
      fn: async (ctx) => {
        const base = path.join(TMP, `open-with-browse-${tag}`);
        fs.mkdirSync(path.join(base, 'bin'), { recursive: true });
        fs.writeFileSync(path.join(base, 'bin', 'mi-app'), '#!/bin/sh\n', { mode: 0o755 });
        fs.writeFileSync(path.join(base, 'bin', 'otra'), '#!/bin/sh\n', { mode: 0o755 });
        const { dialog } = await openDialog(ctx, 'ow-browse');
        await dialog.getByLabel('Nombre', { exact: true }).fill(`Navegada ${tag}`);
        await dialog.getByLabel(/^Ruta del ejecutable/).fill(base);
        await dialog.getByRole('button', { name: 'Examinar…' }).click();
        const picker = dialog.locator('[data-open-with-picker]');
        await picker.waitFor({ timeout: 10000 });
        await picker.getByRole('button', { name: /bin$/ }).click();
        await picker.getByRole('button', { name: 'mi-app' }).waitFor({ timeout: 10000 });
        // htmx drops an input typed while it still settles the swapped picker; a person never types that fast.
        await ctx.page.waitForFunction(() => !document.querySelector('.htmx-settling, .htmx-swapping, .htmx-request'));
        await sleep(300);
        await picker.getByLabel('Filtrar por nombre').fill('otra');
        await picker.getByRole('button', { name: 'mi-app' }).waitFor({ state: 'detached', timeout: 10000 });
        await picker.getByRole('button', { name: 'otra' }).click();
        await picker.waitFor({ state: 'detached', timeout: 10000 });
        const chosen = await dialog.getByLabel(/^Ruta del ejecutable/).inputValue();
        ctx.assert(chosen === path.join(base, 'bin', 'otra'), `la ruta elegida no se rellenó: "${chosen}"`);
        ctx.assert((await dialog.getByLabel('Nombre', { exact: true }).inputValue()) === `Navegada ${tag}`, 'el nombre escrito se perdió');
      },
    },
    {
      name: 'browse: "Cancelar" cierra el selector y conserva la ruta escrita',
      fn: async (ctx) => {
        const { dialog } = await openDialog(ctx, 'ow-cancel');
        await dialog.getByLabel(/^Ruta del ejecutable/).fill('code');
        await dialog.getByRole('button', { name: 'Examinar…' }).click();
        const picker = dialog.locator('[data-open-with-picker]');
        await picker.waitFor({ timeout: 10000 });
        await picker.getByRole('button', { name: 'Cancelar' }).click();
        await picker.waitFor({ state: 'detached', timeout: 10000 });
        ctx.assert((await dialog.getByLabel(/^Ruta del ejecutable/).inputValue()) === 'code', 'la ruta escrita se perdió');
      },
    },
    {
      name: 'delete: "Eliminar" quita la aplicación de la lista',
      fn: async (ctx) => {
        const { dialog } = await openDialog(ctx, 'ow-delete');
        const name = `Temporal ${tag}`;
        await addApp(dialog, { name, p: '/usr/bin/true' });
        await row(dialog, name).waitFor({ timeout: 10000 });
        await row(dialog, name).getByRole('button', { name: 'Eliminar' }).click();
        await row(dialog, name).waitFor({ state: 'detached', timeout: 10000 });
      },
    },
  ],
};
