// end2end/go/branch_search.e2e.cjs
// The base branch select of the create dialog is a searchable combobox (web/static/js/searchable_select.js, S1).
// Typing filters the branches, the keyboard and the mouse pick one, and the real select keeps the pick that the form
// submits. The suite runs in Spanish (the language cookie of the harness).
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

// A git repo with main and a few branches, so the base branch select has a list to filter.
function repoWithBranches(prefix) {
  const dir = path.join(TMP, `branch-search-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  const git = (args) => execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l ${args}`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `branch search ${prefix}\n`);
  git('add .');
  git('commit -qm init');
  for (const branch of ['develop', 'feature/login-form', 'release/1.0']) git(`branch '${branch}'`);
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Registers a workspace with those branches, opens the board and the create dialog; returns the dialog form locator.
async function openDialog(ctx, name) {
  const dir = repoWithBranches(name);
  await ctx.goto('/workspaces/new');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
  await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  const form = ctx.page.locator('form:has(select[name=workspace_id]):visible');
  await form.locator('select[name=base_branch]').waitFor({ state: 'attached', timeout: 10000 });
  return form;
}

// The combobox the script put in front of the base branch select of the dialog.
function comboOf(form) {
  return form.locator('input[role=combobox]').first();
}

async function selectedBranch(form) {
  return form.locator('select[name=base_branch]').first().evaluate((select) => select.value);
}

module.exports = {
  name: 'branch_search',
  tests: [
    {
      name: 'la rama base del diálogo se filtra escribiendo y el select real guarda la elección',
      fn: async (ctx) => {
        const form = await openDialog(ctx, `bs-filter-${tag}`);
        const select = form.locator('select[name=base_branch]').first();
        ctx.assert((await select.getAttribute('data-searchable')) !== null, 'el select de ramas debe marcarse data-searchable');
        const hidden = await select.evaluate((el) => el.classList.contains('sr-only'));
        ctx.assert(hidden, 'el select real debe quedar oculto (sr-only) tras el buscador');

        const combo = comboOf(form);
        await combo.click();
        await shot(ctx, 'branch-search-1-open');
        await combo.fill('login');
        const options = form.locator('[role=option]');
        await options.first().waitFor({ timeout: 5000 });
        const count = await options.count();
        ctx.assert(count === 1, `filtrar por "login" debe dejar 1 rama y deja ${count}`);
        await shot(ctx, 'branch-search-2-filtered');
        await options.first().click();
        await shot(ctx, 'branch-search-3-picked');

        const value = await selectedBranch(form);
        ctx.assert(value === 'feature/login-form', `el select debe guardar la rama elegida y guarda "${value}"`);
        const shown = await combo.inputValue();
        ctx.assert(shown.includes('feature/login-form'), `el buscador debe mostrar la rama elegida y muestra "${shown}"`);
      },
    },
    {
      name: 'las flechas y Enter eligen la rama sin enviar el formulario',
      fn: async (ctx) => {
        const form = await openDialog(ctx, `bs-keys-${tag}`);
        const combo = comboOf(form);
        await combo.click();
        await combo.fill('release');
        await form.locator('[role=option]').first().waitFor({ timeout: 5000 });
        await combo.press('ArrowDown');
        await combo.press('Enter');

        const value = await selectedBranch(form);
        ctx.assert(value === 'release/1.0', `Enter debe elegir la rama resaltada y el select tiene "${value}"`);
        const open = await combo.getAttribute('aria-expanded');
        ctx.assert(open === 'false', `tras elegir la lista debe cerrarse y aria-expanded es "${open}"`);
        ctx.assert(await form.isVisible(), 'Enter con la lista abierta no debe enviar el formulario');
      },
    },
    {
      name: 'sin coincidencias aparece la línea vacía y Escape cierra la lista restaurando la rama elegida',
      fn: async (ctx) => {
        const form = await openDialog(ctx, `bs-empty-${tag}`);
        const combo = comboOf(form);
        const before = await selectedBranch(form);
        await combo.click();
        await combo.fill('zzz-no-such-branch');
        const empty = form.locator('[data-combobox-empty]').first();
        await empty.waitFor({ state: 'visible', timeout: 5000 });
        await shot(ctx, 'branch-search-4-empty');
        const count = await form.locator('[role=option]').count();
        ctx.assert(count === 0, `no debe haber opciones y hay ${count}`);

        await combo.press('Escape');
        const open = await combo.getAttribute('aria-expanded');
        ctx.assert(open === 'false', `Escape debe cerrar la lista y aria-expanded es "${open}"`);
        ctx.assert((await selectedBranch(form)) === before, 'Escape no debe cambiar la rama elegida');
      },
    },
  ],
};
