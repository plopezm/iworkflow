// end2end/go/task_hooks.e2e.cjs
// Go-mode e2e of the Hooks tab of a workspace: the hooks of the workspace are created, edited, reordered, enabled and
// deleted through the editor, and a script hook is tried with "Probar" (its output is shown). The workspace is
// registered through the UI first, so the test runs against a real directory. The suite runs in Spanish.

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

// Creates a git repository with one commit: a workspace needs a real one to be registered.
function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `hooks-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `content ${prefix} ${tag}`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Registers a workspace from the list and returns its id, read from the address of its "Editar" link.
async function registerWorkspace(ctx, name) {
  const repo = uniqueGitRepo(name.replace(/\W+/g, '-'));
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).fill(repo);
  await ctx.page.getByLabel(/Rama base/).fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  const card = ctx.page.locator('article.card').filter({ hasText: name }).first();
  await card.waitFor({ timeout: 12000 });
  const href = await card.getByRole('link', { name: /Editar|Edit/ }).first().getAttribute('href');
  const match = href && href.match(/\/workspaces\/([^/]+)\/edit/);
  ctx.assert(match, `no se encontró el id del workspace en ${href}`);
  return match[1];
}

// Opens the Hooks tab of the workspace. The tab is a dock tab, so the page is loaded at its address.
async function openHooks(ctx, workspaceID) {
  await ctx.goto(`/workspaces/${workspaceID}/hooks`);
  await ctx.page.locator('[data-workspace-hooks]').waitFor({ timeout: 10000 });
}

// The group of an event and phase: "task_start" + "pre" is the "Antes de" group of the start.
function group(ctx, event, phase) {
  return ctx.page.locator(`[data-hooks-event="${event}"] [data-hooks-group="${phase}"]`);
}

function rowOf(ctx, hookText) {
  return ctx.page.locator('[data-hook]').filter({ hasText: hookText }).first();
}

// Opens the editor of a new hook in the group and waits for its form.
async function addHook(ctx, event, phase) {
  await group(ctx, event, phase).getByRole('button', { name: 'Añadir hook +' }).click();
  const editor = ctx.page.locator('[data-hook-editor]');
  await editor.waitFor({ timeout: 10000 });
  return editor;
}

// Saves the open editor and waits for the tab to answer.
async function save(ctx, editor) {
  await editor.getByRole('button', { name: 'Guardar', exact: true }).click();
  await ctx.page.locator('[data-hook-editor]').waitFor({ state: 'detached', timeout: 10000 });
}

module.exports = {
  name: 'task_hooks',
  tests: [
    {
      name: 'editor: crea un hook de script y uno de agente en su grupo y los muestra con sus datos',
      fn: async (ctx) => {
        const workspaceID = await registerWorkspace(ctx, `Hooks crear ${tag}`);
        await openHooks(ctx, workspaceID);

        const script = await addHook(ctx, 'task_start', 'pre');
        await script.getByLabel('Comando', { exact: true }).fill('echo');
        await script.getByLabel(/^Argumentos/).fill('hello');
        await save(ctx, script);
        await ctx.page.getByText('Hook guardado.').waitFor({ timeout: 10000 });
        ctx.assert(await rowOf(ctx, 'echo').count() === 1, 'el hook de script no aparece en su grupo');

        const agent = await addHook(ctx, 'task_finish', 'post');
        await agent.getByRole('button', { name: 'Agente', exact: true }).click();
        await agent.getByLabel(/^Instrucción para el agente/).waitFor({ timeout: 10000 });
        await agent.getByLabel(/^Instrucción para el agente/).fill('Revisa que los tests pasen');
        await save(ctx, agent);
        const agentRow = rowOf(ctx, 'Revisa que los tests pasen');
        await agentRow.waitFor({ timeout: 10000 });
        ctx.assert((await agentRow.innerText()).includes('Agente'), 'el hook de agente no muestra su tipo');
        ctx.assert((await group(ctx, 'task_finish', 'post').innerText()).includes('Revisa que los tests pasen'), 'el agente no está en Después de');
      },
    },
    {
      name: 'editor: un formulario inválido se muestra con el error y conserva lo escrito',
      fn: async (ctx) => {
        const workspaceID = await registerWorkspace(ctx, `Hooks invalido ${tag}`);
        await openHooks(ctx, workspaceID);

        const editor = await addHook(ctx, 'task_start', 'pre');
        await editor.getByLabel('Argumentos', { exact: false }).fill('--flag');
        await editor.getByLabel(/^Variables de entorno/).fill('SIN_IGUAL');
        await editor.getByLabel('Comando', { exact: true }).fill('make');
        // A rejected form keeps the editor open: the save helper that waits for it to close is not used here.
        await editor.getByRole('button', { name: 'Guardar', exact: true }).click();

        await ctx.page.getByText(/NAME=VALUE/).waitFor({ timeout: 10000 });
        ctx.assert((await ctx.page.locator('[data-hook-editor]').getByLabel('Comando', { exact: true }).inputValue()) === 'make', 'el comando escrito se perdió');
        ctx.assert((await ctx.page.locator('[data-hook]').count()) === 0, 'se guardó un hook inválido');
      },
    },
    {
      name: 'edit: editar cambia el comando guardado y mantiene el hook en su sitio',
      fn: async (ctx) => {
        const workspaceID = await registerWorkspace(ctx, `Hooks editar ${tag}`);
        await openHooks(ctx, workspaceID);
        const created = await addHook(ctx, 'task_start', 'pre');
        await created.getByLabel('Comando', { exact: true }).fill('echo');
        await created.getByLabel(/^Argumentos/).fill('before');
        await save(ctx, created);

        await rowOf(ctx, 'echo').getByRole('button', { name: 'Editar', exact: true }).click();
        const editor = ctx.page.locator('[data-hook-editor]');
        await editor.waitFor({ timeout: 10000 });
        ctx.assert((await editor.getByLabel('Comando', { exact: true }).inputValue()) === 'echo', 'el editor no carga el comando guardado');
        await editor.getByLabel(/^Argumentos/).fill('after');
        await save(ctx, editor);

        await ctx.page.getByText('Hook actualizado.').waitFor({ timeout: 10000 });
        ctx.assert((await rowOf(ctx, 'echo after').count()) === 1, 'el cambio no se guardó');
      },
    },
    {
      name: 'order: subir y bajar reordena los hooks del grupo y guarda el orden completo',
      fn: async (ctx) => {
        const workspaceID = await registerWorkspace(ctx, `Hooks orden ${tag}`);
        await openHooks(ctx, workspaceID);
        for (const command of ['first-step', 'second-step']) {
          const editor = await addHook(ctx, 'task_start', 'pre');
          await editor.getByLabel('Comando', { exact: true }).fill(command);
          await save(ctx, editor);
        }
        const names = () => group(ctx, 'task_start', 'pre').locator('[data-hook] code').allInnerTexts();
        ctx.assert(JSON.stringify(await names()) === JSON.stringify(['first-step', 'second-step']), `orden inicial: ${await names()}`);

        await rowOf(ctx, 'first-step').getByRole('button', { name: 'Bajar' }).click();
        await ctx.page.getByText('Orden guardado.').waitFor({ timeout: 10000 });
        ctx.assert(JSON.stringify(await names()) === JSON.stringify(['second-step', 'first-step']), `tras bajar: ${await names()}`);

        // The first hook of its group cannot go up any further: its button is disabled.
        await rowOf(ctx, 'second-step').getByRole('button', { name: 'Subir' }).isDisabled().then((disabled) => {
          ctx.assert(disabled, 'el primer hook del grupo debería tener "Subir" deshabilitado');
        });
      },
    },
    {
      name: 'toggle: desactivar y activar un hook cambia su estado sin tocar el resto',
      fn: async (ctx) => {
        const workspaceID = await registerWorkspace(ctx, `Hooks activo ${tag}`);
        await openHooks(ctx, workspaceID);
        const editor = await addHook(ctx, 'task_start', 'pre');
        await editor.getByLabel('Comando', { exact: true }).fill('lint-check');
        await save(ctx, editor);

        await rowOf(ctx, 'lint-check').getByRole('button', { name: 'Desactivar' }).click();
        await rowOf(ctx, 'lint-check').getByText('Inactivo').waitFor({ timeout: 10000 });
        await rowOf(ctx, 'lint-check').getByRole('button', { name: 'Activar' }).click();
        await rowOf(ctx, 'lint-check').getByText('Activo', { exact: true }).waitFor({ timeout: 10000 });
      },
    },
    {
      name: 'test: "Probar" ejecuta el script en el workspace y muestra su salida',
      fn: async (ctx) => {
        const workspaceID = await registerWorkspace(ctx, `Hooks probar ${tag}`);
        await openHooks(ctx, workspaceID);
        const editor = await addHook(ctx, 'task_start', 'pre');
        await editor.getByLabel('Comando', { exact: true }).fill('echo');
        await editor.getByLabel(/^Argumentos/).fill('hello');
        await save(ctx, editor);

        await rowOf(ctx, 'echo hello').getByRole('button', { name: 'Probar', exact: true }).click();
        const result = rowOf(ctx, 'echo hello').locator('[data-hook-test]');
        await result.waitFor({ timeout: 15000 });
        ctx.assert((await result.getAttribute('data-hook-test')) === 'ok', 'la prueba no terminó bien');
        ctx.assert((await result.innerText()).includes('hello'), 'la salida de la prueba no se muestra');
        ctx.assert((await result.innerText()).includes('Correcto'), 'el estado de la prueba no se muestra');
      },
    },
    {
      name: 'delete: "Eliminar" pide confirmación; cancelarla conserva el hook y aceptarla lo quita',
      fn: async (ctx) => {
        const workspaceID = await registerWorkspace(ctx, `Hooks borrar ${tag}`);
        await openHooks(ctx, workspaceID);
        const editor = await addHook(ctx, 'task_finish', 'pre');
        await editor.getByLabel('Comando', { exact: true }).fill('cleanup');
        await save(ctx, editor);
        await rowOf(ctx, 'cleanup').waitFor({ timeout: 10000 });

        // The confirmation is the browser dialog of hx-confirm. The harness answers it with the mode set here; this
        // listener only records the question, so the test sees what was asked.
        const dialogs = [];
        ctx.page.on('dialog', (dialog) => dialogs.push(dialog.message()));
        ctx.setDialog('reject');

        await rowOf(ctx, 'cleanup').getByRole('button', { name: 'Eliminar', exact: true }).click();
        for (let attempt = 0; attempt < 50 && dialogs.length === 0; attempt++) await ctx.page.waitForTimeout(100);
        ctx.assert(dialogs.length === 1, 'eliminar debe pedir confirmación');
        ctx.assert(dialogs[0] === '¿Eliminar este hook?', `la pregunta de confirmación es "${dialogs[0]}"`);
        await ctx.page.waitForTimeout(500);
        ctx.assert((await ctx.page.locator('[data-hook]').filter({ hasText: 'cleanup' }).count()) === 1, 'cancelar la confirmación no debe quitar el hook');
        ctx.assert((await ctx.page.getByText('Hook eliminado.').count()) === 0, 'cancelar no debe mostrar que el hook se eliminó');

        ctx.setDialog('accept');
        await rowOf(ctx, 'cleanup').getByRole('button', { name: 'Eliminar', exact: true }).click();
        await ctx.page.getByText('Hook eliminado.').waitFor({ timeout: 10000 });
        ctx.assert((await ctx.page.locator('[data-hook]').filter({ hasText: 'cleanup' }).count()) === 0, 'el hook sigue en la lista');
      },
    },
  ],
};
