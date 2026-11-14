// end2end/go/task_hooks_strip.e2e.cjs
// Go-mode e2e of the hook status strip of a task (T5b): the assignments of the workspace show in the Actions tab of a
// started task with their status and their folded log, and the per-task toggle skips and includes each one again. The
// hook is created in the Hooks screen and assigned to the workspace before the start (T5a), so this suite depends on it.
// A backlog task has no Actions tab, so the task is started: its hook is a `true` command that exits 0 in the worktree,
// so the strip shows it as run. The suite runs in Spanish. Each test registers its own git repo as a workspace, so the
// tests do not see each other.

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `hooks-strip-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `hooks ${prefix}\n`);
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
  return name;
}

// Opens the Hooks screen, a top-level section, at its address.
async function openHooksScreen(ctx) {
  await ctx.goto('/hooks');
  await ctx.page.locator('[data-hooks-screen]').waitFor({ timeout: 10000 });
}

// Creates a script hook in the Hooks screen and assigns it to the workspace before the start of a task: the default
// event and moment of a new assignment. It waits for the assignment to be listed.
async function addBeforeStartHook(ctx, wsName, command) {
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nuevo hook +' }).click();
  const editor = page.locator('[data-hook-editor]');
  await editor.waitFor({ timeout: 10000 });
  await editor.getByLabel('Nombre', { exact: true }).fill(command);
  await editor.getByLabel('Comando', { exact: true }).fill(command);
  await editor.getByRole('button', { name: 'Guardar', exact: true }).click();
  const row = page.locator('[data-hook]').filter({ hasText: command }).first();
  await row.waitFor({ timeout: 10000 });
  await row.getByRole('button', { name: 'Asignaciones', exact: true }).click();
  // "Asignaciones" only lists them; the form opens from the add button.
  await page.getByRole('button', { name: 'Asignar a un proyecto +', exact: true }).click();
  const form = page.locator('[data-assignment-form]');
  await form.waitFor({ timeout: 10000 });
  await form.locator('select[name="project"]').selectOption({ label: wsName });
  await form.getByRole('button', { name: 'Asignar +', exact: true }).click();
  await page.locator('[data-assignment]').filter({ hasText: wsName }).first().waitFor({ timeout: 10000 });
}

// Creates a task in the workspace, starts it, and opens its detail on the Actions tab.
async function createStartedTask(ctx, wsName, prompt) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name: wsName }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${wsName} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  // Navigating before the create -> start chain ends cancels the start: wait for the modal to close first.
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  await sleep(1500);
  await ctx.goto(await link.getAttribute('href'));
  await page.waitForURL(/\/tasks\/[0-9a-f-]{36}/, { timeout: 15000, waitUntil: 'commit' });
  await page.getByRole('tab', { name: 'Acciones' }).first().click();
  await page.locator('[data-task-hooks]').waitFor({ timeout: 10000 });
}

// Creates a task with "Iniciar ahora" and leaves at once, before the create -> start request ends. By default it loads
// the board again: the browser drops the request, which cancels its context on the server. A different leave runs
// instead of that when given (for example closing the tab of the form).
async function createStartedTaskAndLeave(ctx, wsName, prompt, leave) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name: wsName }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
  const page = ctx.page;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${wsName} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  // Long enough for the request to reach the server and its pre hook to start, shorter than the hook.
  await sleep(1000);
  if (leave) await leave(page); else await ctx.goto('/');
}

// Waits for the marker a slow hook leaves, then opens the task on its Actions tab and expects the hook to have run
// without error.
async function expectSlowHookDone(ctx, wsName, prompt, marker) {
  let done = false;
  for (let attempt = 0; attempt < 30 && !done; attempt++) {
    done = fs.existsSync(marker);
    if (!done) await sleep(1000);
  }
  ctx.assert(done, 'el hook previo no terminó: se canceló con la petición de inicio');

  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name: wsName }).first().click();
  const link = ctx.page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  await ctx.goto(await link.getAttribute('href'));
  await ctx.page.getByRole('tab', { name: 'Acciones' }).first().click();
  await ctx.page.locator('[data-task-hooks]').waitFor({ timeout: 10000 });
  await waitForStatus(ctx, 'Correcto');
  const strip = ctx.page.locator('[data-task-hooks]');
  ctx.assert(await strip.getByText('Error', { exact: true }).count() === 0, 'el hook no debe quedar con error');
  ctx.assert((await strip.innerText()).indexOf('context canceled') === -1, 'el registro no debe mencionar la cancelación');
}

// The strip is rendered with the page, so a run that ends after the page opened shows on the next render: reload until
// the status says so, for a few seconds at most.
async function waitForStatus(ctx, status) {
  const strip = ctx.page.locator('[data-task-hooks]');
  for (let attempt = 0; attempt < 15; attempt++) {
    if (await strip.getByText(status, { exact: true }).count() > 0) return;
    await sleep(1000);
    await ctx.page.reload();
    await ctx.page.getByRole('tab', { name: 'Acciones' }).first().click();
    await ctx.page.locator('[data-task-hooks]').waitFor({ timeout: 10000 });
  }
  ctx.assert(false, `el estado "${status}" no apareció en la franja de hooks`);
}

module.exports = {
  name: 'task_hooks_strip',
  tests: [
    {
      name: 'strip: el hook corrido muestra su estado y registro, y el toggle lo omite e incluye',
      fn: async (ctx) => {
        const wsName = await registerWs(ctx, `hs-toggle-${tag}`);
        await openHooksScreen(ctx);
        await addBeforeStartHook(ctx, wsName, 'true');
        await createStartedTask(ctx, wsName, `hookstrip-${tag}`);
        const strip = ctx.page.locator('[data-task-hooks]');

        await strip.getByText('Antes de').first().waitFor({ timeout: 10000 });
        await waitForStatus(ctx, 'Correcto');
        ctx.assert(await strip.getByText('Registro', { exact: true }).count() > 0, 'el hook corrido debe tener su registro');

        await strip.getByRole('button', { name: 'Omitir' }).click();
        await strip.getByRole('button', { name: 'Incluir' }).waitFor({ timeout: 10000 });
        ctx.assert(await strip.getByText('Omitido', { exact: true }).count() > 0, 'tras omitirlo el estado debe ser omitido');

        await strip.getByRole('button', { name: 'Incluir' }).click();
        await strip.getByRole('button', { name: 'Omitir' }).waitFor({ timeout: 10000 });
        ctx.assert(await strip.getByText('Correcto', { exact: true }).count() > 0, 'tras incluirlo vuelve a su último estado');
      },
    },
    {
      name: 'strip: un hook previo lento termina aunque el navegador abandone la petición de inicio',
      fn: async (ctx) => {
        const wsName = await registerWs(ctx, `hs-leave-${tag}`);
        const marker = path.join(TMP, `hooks-strip-marker-${tag}`);
        await openHooksScreen(ctx);
        await addBeforeStartHook(ctx, wsName, `sleep 4 && touch ${marker}`);
        const prompt = `hookleave-${tag}`;

        await createStartedTaskAndLeave(ctx, wsName, prompt);

        // The hook outlives the request: it ends by itself and leaves its marker.
        await expectSlowHookDone(ctx, wsName, prompt, marker);
      },
    },
    {
      name: 'strip: un hook previo lento termina aunque se cierre la pestaña del formulario durante la espera',
      fn: async (ctx) => {
        const wsName = await registerWs(ctx, `hs-close-${tag}`);
        const marker = path.join(TMP, `hooks-strip-marker-close-${tag}`);
        await openHooksScreen(ctx);
        await addBeforeStartHook(ctx, wsName, `sleep 4 && touch ${marker}`);
        const prompt = `hookclose-${tag}`;

        // The form is in the active tab: its × closes it while the request waits.
        await createStartedTaskAndLeave(ctx, wsName, prompt, async (page) => {
          await page.locator('[data-dock-tab-button][aria-selected=true] .dock-tab-close').first().click();
          // The form left the document while its request was in flight.
          await page.locator('form:has(select[name=workspace_id])').waitFor({ state: 'detached', timeout: 5000 });
        });

        await expectSlowHookDone(ctx, wsName, prompt, marker);
      },
    },
    {
      name: 'pantalla Hooks: se abre en su dirección y el menú del workspace ya no enlaza a hooks',
      fn: async (ctx) => {
        const wsName = await registerWs(ctx, `hs-menu-${tag}`);
        await openHooksScreen(ctx);
        ctx.assert(await ctx.page.getByRole('button', { name: 'Nuevo hook +' }).count() > 0, 'la pantalla de hooks no abrió');
        await ctx.goto('/');
        const row = ctx.page.locator('#board-sidebar [data-workspace-row]', { has: ctx.page.getByRole('link', { name: wsName }) });
        ctx.assert(await row.locator('a[href$="/hooks"]').count() === 0, 'el menú del workspace todavía enlaza a hooks');
      },
    },
  ],
};
