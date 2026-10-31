// end2end/go/related_projects.e2e.cjs
// Related projects of a task (odd/tasks/related-projects.md, S2 and S5, and S1/S3 where the UI shows them): the
// "Proyectos relacionados" list of the create dialog and the edit page (RelatedProjectsField, web/static/js/task_form.js),
// and the @ completion that offers the files of the checked projects (web/static/js/composer.js, the file routes of
// the composer). The suite runs in Spanish (the language cookie of the harness). Each test registers its own git repos
// as workspaces, so the tests do not see each other.
// Workspaces are shared by every module of a run. The single-workspace test checks the rule against the workspaces the
// database holds; the empty list is only exercised when this module runs alone on a fresh database, and it is the first
// test so that it sees a database with no other workspace.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix, { guide }) {
  const dir = fs.mkdtempSync(path.join(TMP, `related-repo-${prefix}-`));
  fs.writeFileSync(path.join(dir, 'README.md'), `related ${prefix}\n`);
  if (guide) {
    fs.mkdirSync(path.join(dir, 'docs'), { recursive: true });
    fs.writeFileSync(path.join(dir, 'docs', 'guide.md'), `guide ${prefix}\n`);
  }
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Each registration gets its own name and directory: the workspaces of a shared database must not collide.
let registrations = 0;

// Registers a workspace from the Workspaces screen and waits until it is listed.
async function registerWs(ctx, prefix, { guide }) {
  registrations += 1;
  const name = `${prefix}-${tag}-${registrations}`;
  const dir = uniqueGitRepo(name, { guide });
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

// Opens the create tab of the board and chooses the workspace in its select.
async function openCreate(ctx, ws) {
  await openBoard(ctx, ws.name);
  await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
}

// The id of a workspace, read from the option of the create dialog select (its value is the workspace id).
async function workspaceId(ctx, name) {
  const id = await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id] option').evaluateAll(
    (options, label) => options.find((option) => option.textContent === label)?.value ?? '',
    `${name} (main)`,
  );
  return id;
}

// The create tab: the visible form that holds the workspace select.
const CREATE = 'form:has(select[name=workspace_id]):visible';

// Opens the related projects dialog of the visible form (the button "Elegir proyectos") and closes it ("Hecho").
const openRelated = (ctx) => ctx.page.locator('form:has([data-related-projects]):visible [data-related-open]').click();
const closeRelated = (ctx) => ctx.page.locator('form:has([data-related-projects]):visible [data-modal]:visible').getByRole('button', { name: 'Hecho' }).click();

// The checkbox of a related project in the create dialog.
const dialogBox = (ctx, id) => ctx.page.locator(`${CREATE} input[name="related_project_id"][value="${id}"]`);

// The checkbox of a related project in the edit form (the form that holds the related list; only the visible one).
const editBox = (ctx, id) => ctx.page.locator(`form:has([data-related-projects]):visible input[name="related_project_id"][value="${id}"]`);

// The @ menu item that inserts a file. Only visible items count: hidden composers keep their menus in the DOM.
const menuItem = (ctx, insert) => ctx.page.locator(`[data-composer-item][data-insert="${insert}"]:visible`);

// Submits the create tab and waits until it closes.
async function submitDialog(ctx) {
  await ctx.page.locator(`${CREATE} button[type=submit]`, { hasText: 'Crear tarea' }).click();
  await ctx.page.locator(CREATE).waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});
}

// Opens the edit tab of the backlog card that shows the prompt.
async function openEditFromCard(ctx, prompt) {
  const card = ctx.page.locator('[id^="board-list-"]:visible [data-stage="backlog"]', { hasText: prompt }).first();
  await card.waitFor({ timeout: 10000 });
  await card.getByRole('button', { name: 'Editar' }).click();
  await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ timeout: 15000 });
}

// Saves the edit page and waits until it returns to the board.
async function saveEdit(ctx) {
  await ctx.page.getByRole('button', { name: 'Guardar cambios' }).click();
  await ctx.page.getByRole('heading', { name: 'Editar tarea' }).waitFor({ state: 'detached', timeout: 15000 });
}

// The prompt of the edit form: the textarea of the form that holds the related list.
const editPrompt = (ctx) => ctx.page.locator('form:has([data-related-projects]):visible textarea[name=prompt]');

// ---- tests ----
module.exports = {
  name: 'related_projects',
  tests: [
    {
      name: 'related_projects: sin otro workspace no se muestra la lista de proyectos relacionados',
      fn: async (ctx) => {
        const solo = await registerWs(ctx, 'rp-solo', { guide: false });
        await openCreate(ctx, solo);
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').waitFor({ timeout: 10000 });
        const ownId = await workspaceId(ctx, solo.name);
        ctx.assert(ownId !== '', 'el workspace no aparece en el select del modal');
        const ids = await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id] option').evaluateAll(
          (options) => options.map((option) => option.value).filter((value) => value !== ''),
        );
        const others = ids.filter((id) => id !== ownId);
        const lists = await ctx.page.locator('form:has(select[name=workspace_id]):visible [data-related-projects]').count();
        // With no other workspace the list is not rendered at all. With others (a shared database) it is rendered.
        ctx.assert(
          lists === (others.length > 0 ? 1 : 0),
          `la lista de proyectos relacionados debería ${others.length > 0 ? 'mostrarse' : 'no mostrarse'} con ${others.length} otros workspaces`,
        );
      },
    },
    {
      name: 'related_projects: el modal de nueva tarea lista el otro workspace y oculta el propio al elegirlo',
      fn: async (ctx) => {
        const a = await registerWs(ctx, 'rp-a', { guide: false });
        const b = await registerWs(ctx, 'rp-b', { guide: true });
        // Open the dialog from the board of A: the list is rendered for A, so A is not listed and B is.
        await openBoard(ctx, a.name);
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').waitFor({ state: 'visible', timeout: 10000 });
        const idA = await workspaceId(ctx, a.name);
        const idB = await workspaceId(ctx, b.name);
        ctx.assert(idA && idB, 'los workspaces no aparecen en el select del modal');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible [data-related-projects]').waitFor({ state: 'visible', timeout: 10000 });
        await openRelated(ctx);
        await dialogBox(ctx, idB).waitFor({ state: 'visible', timeout: 5000 });
        ctx.assert((await dialogBox(ctx, idA).count()) === 0, 'el proyecto propio aparece en la lista de relacionados');
        await closeRelated(ctx);
        // Choosing B in the workspace select hides it, so the task cannot relate to the project it runs in.
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${b.name} (main)` });
        await openRelated(ctx);
        await dialogBox(ctx, idB).waitFor({ state: 'hidden', timeout: 5000 });
        ctx.assert(!(await dialogBox(ctx, idB).isChecked()), 'B sigue marcado tras elegirlo como proyecto propio');
      },
    },
    {
      name: 'related_projects: @ ofrece los ficheros del proyecto marcado con su etiqueta y mantiene los del propio',
      fn: async (ctx) => {
        const a = await registerWs(ctx, 'rp-a', { guide: false });
        const b = await registerWs(ctx, 'rp-b', { guide: true });
        await openCreate(ctx, a);
        const idB = await workspaceId(ctx, b.name);
        await openRelated(ctx);
        await dialogBox(ctx, idB).check();
        await closeRelated(ctx);
        const prompt = ctx.page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]');
        // The current project's files still come first: README.md belongs to A.
        await prompt.fill('@READ');
        await menuItem(ctx, '@README.md').waitFor({ timeout: 10000 });
        // The checked project's file is offered with its label, and it is inserted as @<label>:<path>.
        const insert = `@${b.name}:docs/guide.md`;
        await prompt.fill('@guide');
        await menuItem(ctx, insert).waitFor({ timeout: 10000 });
        await menuItem(ctx, insert).click();
        ctx.assert((await prompt.inputValue()).includes(insert), `el texto no contiene ${insert}`);
      },
    },
    {
      name: 'related_projects: la edición precarga la selección, la guarda y limita @ a los proyectos marcados',
      fn: async (ctx) => {
        const a = await registerWs(ctx, 'rp-a', { guide: false });
        const b = await registerWs(ctx, 'rp-b', { guide: true });
        await openCreate(ctx, a);
        const idB = await workspaceId(ctx, b.name);
        await openRelated(ctx);
        await dialogBox(ctx, idB).check();
        await closeRelated(ctx);
        const prompt = `rp-edit-${tag}`;
        await ctx.page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').fill(prompt);
        // Create the task without starting it.
        await ctx.page.locator('form:has(select[name=workspace_id]):visible input[name=start_now]').uncheck();
        await submitDialog(ctx);
        await openEditFromCard(ctx, prompt);

        // B was checked when the task was created, so the edit page pre-checks it.
        await openRelated(ctx);
        const box = editBox(ctx, idB);
        await box.waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert(await box.isChecked(), 'B no aparece marcado al editar la tarea');

        // Untick and save: the reopened edit page shows it unchecked.
        await box.uncheck();
        await closeRelated(ctx);
        await saveEdit(ctx);
        await openEditFromCard(ctx, prompt);
        await openRelated(ctx);
        await editBox(ctx, idB).waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert(!(await editBox(ctx, idB).isChecked()), 'B sigue marcado tras desmarcarlo y guardar');

        // Tick again and save: the reopened edit page shows it checked.
        await editBox(ctx, idB).check();
        await closeRelated(ctx);
        await saveEdit(ctx);
        await openEditFromCard(ctx, prompt);
        await openRelated(ctx);
        await editBox(ctx, idB).waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert(await editBox(ctx, idB).isChecked(), 'B no sigue marcado tras volver a marcarlo y guardar');
        await closeRelated(ctx);

        // @ on the edit page offers B's file while B is ticked...
        const insert = `@${b.name}:docs/guide.md`;
        await editPrompt(ctx).fill('@guide');
        await menuItem(ctx, insert).waitFor({ timeout: 10000 });

        // ...and stops offering it once B is unticked (the form decides, before any save).
        await openRelated(ctx);
        await editBox(ctx, idB).uncheck();
        await closeRelated(ctx);
        await editPrompt(ctx).fill('@READ');
        // The answer has arrived when the own project's README.md is listed; B's file must not be among them.
        await menuItem(ctx, '@README.md').waitFor({ timeout: 10000 });
        ctx.assert((await menuItem(ctx, insert).count()) === 0, 'B sigue ofreciendo su fichero con B desmarcado');
      },
    },
    {
      name: 'related_projects: el diálogo filtra los proyectos por nombre o ruta y resume los elegidos',
      fn: async (ctx) => {
        const a = await registerWs(ctx, 'rp-a', { guide: false });
        const b = await registerWs(ctx, 'rp-b', { guide: true });
        await openCreate(ctx, a);
        const idB = await workspaceId(ctx, b.name);
        await openRelated(ctx);
        const filter = ctx.page.locator(`${CREATE} [data-related-filter]`);
        await filter.fill(b.name);
        await dialogBox(ctx, idB).waitFor({ state: 'visible', timeout: 5000 });
        await filter.fill(`sin-coincidencias-${tag}`);
        await dialogBox(ctx, idB).waitFor({ state: 'hidden', timeout: 5000 });
        await ctx.page.locator(`${CREATE} [data-related-empty]`).waitFor({ state: 'visible', timeout: 5000 });
        await filter.fill('');
        await dialogBox(ctx, idB).check();
        await closeRelated(ctx);
        const chips = await ctx.page.locator(`${CREATE} [data-related-summary] li`).allTextContents();
        ctx.assert(chips.includes(b.name), `el resumen no muestra ${b.name}: ${chips.join(',')}`);
      },
    },
  ],
};
