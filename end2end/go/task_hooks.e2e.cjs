// end2end/go/task_hooks.e2e.cjs
// Go-mode e2e of the Hooks screen (T3, T4): the hooks of the library are created, edited, assigned to a project by
// event and moment, reordered inside their group, enabled and deleted, and a script hook is tried with "Probar" (its
// output is shown). The history tab opens with the failed-only filter on. The workspace is registered through the UI
// first, so the test runs against a real directory. The library is global, so each test names its hooks with the tag.
// The suite runs in Spanish.

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

// Registers a workspace from the list, so it can be chosen as the project of an assignment.
async function registerWorkspace(ctx, name) {
  const repo = uniqueGitRepo(name.replace(/\W+/g, '-'));
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).fill(repo);
  await ctx.page.getByLabel(/Rama base/).fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.locator('article.card').filter({ hasText: name }).first().waitFor({ timeout: 12000 });
  return name;
}

// Opens the Hooks screen. It is a top-level section, so the page is loaded at its address.
async function openScreen(ctx) {
  await ctx.goto('/hooks');
  await ctx.page.locator('[data-hooks-screen]').waitFor({ timeout: 10000 });
}

function rowOf(ctx, text) {
  return ctx.page.locator('[data-hook]').filter({ hasText: text }).first();
}

// Opens the editor of a new hook, fills its name and command, saves it and waits for its row in the library.
async function createScriptHook(ctx, name, command) {
  await ctx.page.getByRole('button', { name: 'Nuevo hook +' }).click();
  const editor = ctx.page.locator('[data-hook-editor]');
  await editor.waitFor({ timeout: 10000 });
  await editor.getByLabel('Nombre', { exact: true }).fill(name);
  await editor.getByLabel('Comando', { exact: true }).fill(command);
  await editor.getByRole('button', { name: 'Guardar', exact: true }).click();
  await rowOf(ctx, name).waitFor({ timeout: 10000 });
}

// Opens the assignments of a hook in the library.
async function openAssignments(ctx, name) {
  await rowOf(ctx, name).getByRole('button', { name: 'Asignaciones', exact: true }).click();
  await ctx.page.locator('[data-hook-assignments]').waitFor({ timeout: 10000 });
}

// Assigns the open hook to a project, event and moment through its assignment form.
async function assignTo(ctx, project, event, phase) {
  const form = ctx.page.locator('[data-assignment-form]');
  await form.getByLabel('Proyecto', { exact: true }).selectOption({ label: project });
  await form.getByLabel('Evento', { exact: true }).selectOption(event);
  await form.getByLabel('Momento', { exact: true }).selectOption(phase);
  await form.getByRole('button', { name: 'Asignar +', exact: true }).click();
  await ctx.page.getByText('Asignación añadida.').waitFor({ timeout: 10000 });
}

// The run order panel of a project, event and moment.
function orderPanel(ctx) {
  return ctx.page.locator('[data-hooks-group]');
}

module.exports = {
  name: 'task_hooks',
  tests: [
    {
      name: 'biblioteca: crea un hook de script y lo asigna a un proyecto antes de empezar una tarea',
      fn: async (ctx) => {
        const project = await registerWorkspace(ctx, `Hooks asignar ${tag}`);
        await openScreen(ctx);
        const name = `lint-${tag}`;
        await createScriptHook(ctx, name, 'echo');
        ctx.assert(await ctx.page.getByText('Hook creado.').count() > 0, 'el aviso de hook creado no aparece');

        await openAssignments(ctx, name);
        await assignTo(ctx, project, 'task_start', 'pre');

        const assignment = ctx.page.locator('[data-assignment]').filter({ hasText: project }).first();
        await assignment.waitFor({ timeout: 10000 });
        ctx.assert((await assignment.innerText()).includes('Antes de'), 'la asignación no muestra su momento');
        ctx.assert((await rowOf(ctx, name).innerText()).includes('1 asignaciones'), 'la fila no cuenta la asignación');
      },
    },
    {
      name: 'biblioteca: un agente se crea con su instrucción y muestra su tipo',
      fn: async (ctx) => {
        await openScreen(ctx);
        await ctx.page.getByRole('button', { name: 'Nuevo hook +' }).click();
        const editor = ctx.page.locator('[data-hook-editor]');
        await editor.waitFor({ timeout: 10000 });
        const name = `review-${tag}`;
        await editor.getByLabel('Nombre', { exact: true }).fill(name);
        await editor.getByRole('button', { name: 'Agente', exact: true }).click();
        await editor.getByLabel(/^Instrucción para el agente/).waitFor({ timeout: 10000 });
        await editor.getByLabel(/^Instrucción para el agente/).fill('Revisa que los tests pasen');
        await editor.getByRole('button', { name: 'Guardar', exact: true }).click();

        const row = rowOf(ctx, name);
        await row.waitFor({ timeout: 10000 });
        ctx.assert((await row.innerText()).includes('Agente'), 'el hook de agente no muestra su tipo');
      },
    },
    {
      name: 'biblioteca: un formulario inválido se muestra con el error y conserva lo escrito',
      fn: async (ctx) => {
        await openScreen(ctx);
        await ctx.page.getByRole('button', { name: 'Nuevo hook +' }).click();
        const editor = ctx.page.locator('[data-hook-editor]');
        await editor.waitFor({ timeout: 10000 });
        await editor.getByLabel('Nombre', { exact: true }).fill(`invalido-${tag}`);
        await editor.getByLabel('Comando', { exact: true }).fill('make');
        await editor.getByLabel(/^Variables de entorno/).fill('SIN_IGUAL');
        await editor.getByRole('button', { name: 'Guardar', exact: true }).click();

        await ctx.page.getByText(/NAME=VALUE/).waitFor({ timeout: 10000 });
        ctx.assert((await ctx.page.locator('[data-hook-editor]').getByLabel('Comando', { exact: true }).inputValue()) === 'make', 'el comando escrito se perdió');
        ctx.assert((await rowOf(ctx, `invalido-${tag}`).count()) === 0, 'se guardó un hook inválido');
      },
    },
    {
      name: 'biblioteca: editar cambia el comando guardado y mantiene el hook en la lista',
      fn: async (ctx) => {
        await openScreen(ctx);
        const name = `editar-${tag}`;
        await createScriptHook(ctx, name, 'echo');

        await rowOf(ctx, name).getByRole('button', { name: 'Editar', exact: true }).click();
        const editor = ctx.page.locator('[data-hook-editor]');
        await editor.waitFor({ timeout: 10000 });
        ctx.assert((await editor.getByLabel('Comando', { exact: true }).inputValue()) === 'echo', 'el editor no carga el comando guardado');
        await editor.getByLabel('Comando', { exact: true }).fill('echo changed');
        await editor.getByRole('button', { name: 'Guardar', exact: true }).click();

        await ctx.page.getByText('Hook actualizado.').waitFor({ timeout: 10000 });
        ctx.assert((await rowOf(ctx, name).count()) === 1, 'el hook desapareció tras editarlo');
      },
    },
    {
      name: 'orden: subir y bajar reordena las asignaciones del grupo y guarda el orden completo',
      fn: async (ctx) => {
        const project = await registerWorkspace(ctx, `Hooks orden ${tag}`);
        await openScreen(ctx);
        const first = `first-${tag}`;
        const second = `second-${tag}`;
        for (const name of [first, second]) {
          await createScriptHook(ctx, name, name);
          await openAssignments(ctx, name);
          await assignTo(ctx, project, 'task_start', 'pre');
        }

        const panel = orderPanel(ctx);
        await panel.getByLabel('Proyecto', { exact: true }).selectOption({ label: project });
        await panel.getByRole('button', { name: 'Mostrar', exact: true }).click();
        const rows = () => orderPanel(ctx).locator('[data-group-row]').allInnerTexts();
        ctx.assert((await rows()).length === 2, 'el grupo debe mostrar las dos asignaciones');
        ctx.assert((await rows())[0].includes(first), `orden inicial: ${await rows()}`);

        await orderPanel(ctx).locator('[data-group-row]').filter({ hasText: first }).getByRole('button', { name: 'Bajar' }).click();
        await ctx.page.getByText('Orden guardado.').waitFor({ timeout: 10000 });
        ctx.assert((await orderPanel(ctx).locator('[data-group-row]').first().innerText()).includes(second), 'tras bajar el orden no cambió');

        // The first row of its group cannot go up any further: its button is disabled.
        const top = orderPanel(ctx).locator('[data-group-row]').first().getByRole('button', { name: 'Subir' });
        ctx.assert(await top.isDisabled(), 'la primera fila del grupo debería tener "Subir" deshabilitado');
      },
    },
    {
      name: 'asignación: desactivar y activar una asignación cambia su estado sin tocar el resto',
      fn: async (ctx) => {
        const project = await registerWorkspace(ctx, `Hooks activo ${tag}`);
        await openScreen(ctx);
        const name = `toggle-${tag}`;
        await createScriptHook(ctx, name, 'lint-check');
        await openAssignments(ctx, name);
        await assignTo(ctx, project, 'task_start', 'pre');
        const assignment = ctx.page.locator('[data-assignment]').filter({ hasText: project }).first();

        await assignment.getByRole('button', { name: 'Desactivar', exact: true }).click();
        await assignment.getByText('Inactivo', { exact: true }).waitFor({ timeout: 10000 });
        await assignment.getByRole('button', { name: 'Activar', exact: true }).click();
        await assignment.getByText('Activo', { exact: true }).waitFor({ timeout: 10000 });
      },
    },
    {
      name: 'probar: "Probar" ejecuta el script en el proyecto elegido y muestra su salida',
      fn: async (ctx) => {
        const project = await registerWorkspace(ctx, `Hooks probar ${tag}`);
        await openScreen(ctx);
        const name = `probar-${tag}`;
        await createScriptHook(ctx, name, 'echo');
        await openAssignments(ctx, name);

        const form = ctx.page.locator('[data-hook-test-form]');
        await form.getByLabel('Proyecto', { exact: true }).selectOption({ label: project });
        await form.getByRole('button', { name: 'Probar', exact: true }).click();

        const result = ctx.page.locator('[data-hook-test]');
        await result.waitFor({ timeout: 15000 });
        ctx.assert((await result.getAttribute('data-hook-test')) === 'ok', 'la prueba no terminó bien');
        ctx.assert((await result.innerText()).includes('Correcto'), 'el estado de la prueba no se muestra');
      },
    },
    {
      name: 'historial: la pestaña de errores abre con el filtro de solo fallos activado',
      fn: async (ctx) => {
        await openScreen(ctx);
        await ctx.page.getByRole('button', { name: 'Errores y ejecuciones', exact: true }).click();
        const filter = ctx.page.locator('[data-history-filter]');
        await filter.waitFor({ timeout: 10000 });
        ctx.assert(await filter.getByLabel('Solo fallos', { exact: true }).isChecked(), 'el filtro de solo fallos debe empezar activado');
      },
    },
    {
      name: 'borrar: "Eliminar" pide confirmación; cancelarla conserva el hook y aceptarla lo quita',
      fn: async (ctx) => {
        await openScreen(ctx);
        const name = `borrar-${tag}`;
        await createScriptHook(ctx, name, 'cleanup');

        // The confirmation is the browser dialog of hx-confirm. The harness answers it with the mode set here; this
        // listener only records the question, so the test sees what was asked.
        const dialogs = [];
        ctx.page.on('dialog', (dialog) => dialogs.push(dialog.message()));
        ctx.setDialog('reject');

        await rowOf(ctx, name).getByRole('button', { name: 'Eliminar', exact: true }).click();
        for (let attempt = 0; attempt < 50 && dialogs.length === 0; attempt++) await ctx.page.waitForTimeout(100);
        ctx.assert(dialogs.length === 1, 'eliminar debe pedir confirmación');
        ctx.assert(dialogs[0] === '¿Eliminar este hook y todas sus asignaciones?', `la pregunta de confirmación es "${dialogs[0]}"`);
        await ctx.page.waitForTimeout(500);
        ctx.assert((await rowOf(ctx, name).count()) === 1, 'cancelar la confirmación no debe quitar el hook');
        ctx.assert((await ctx.page.getByText('Hook eliminado.').count()) === 0, 'cancelar no debe mostrar que el hook se eliminó');

        ctx.setDialog('accept');
        await rowOf(ctx, name).getByRole('button', { name: 'Eliminar', exact: true }).click();
        await ctx.page.getByText('Hook eliminado.').waitFor({ timeout: 10000 });
        ctx.assert((await rowOf(ctx, name).count()) === 0, 'el hook sigue en la lista');
      },
    },
  ],
};
