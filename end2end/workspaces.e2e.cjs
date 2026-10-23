// end2end/workspaces.e2e.cjs
// Prueba de la suite: cubre el pipeline completo (harness -> servidor -> navegador -> assert)
// y los casos de uso de workspaces alcanzables desde la UI:
//   list_workspaces, register_workspace, update_workspace, delete_workspace,
//   get_workspace, (list_agents_file_options se ejecuta al registrar),
//   + el flujo de confirmación de inicialización (NotAGitRepository / NoCommits).
//
// Cada test crea SU PROPIO repo git único (el servidor rechaza registrar dos veces
// la misma ruta). Los nombres de workspace son únicos para no chocar en la BD.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

// ---- helpers ----

// Crea un repo git fresco y único (con un commit). Devuelve su ruta.
function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `ws-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `content ${prefix} ${tag}`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Crea una carpeta que NO es un repo git (para probar la confirmación de inicialización).
function uniqueNonGitDir(prefix) {
  const dir = path.join(TMP, `ws-nongit-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}

// Botón de confirmación inline (ConfirmPanel) al registrar una carpeta no-git.
const INIT_BTN = /Inicializar|Crear e inicializar|Crear commit/i;

// Registra un workspace desde el formulario y espera a que aparezca en la tabla.
// Si `expectConfirm` es true, confirma la inicialización inline de una carpeta no-git.
async function registerWs(ctx, { name, gitRepo, nonGitDir, baseBranch = 'main', expectConfirm = false } = {}) {
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').fill(name);
  const dir = gitRepo || nonGitDir;
  ctx.assert(dir, 'falta gitRepo o nonGitDir en registerWs');
  await ctx.page.getByLabel(/^Ruta absoluta/).fill(dir);
  if (baseBranch) {
    await ctx.page.getByLabel(/Rama base/).fill(baseBranch);
  }
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  if (expectConfirm) {
    await ctx.page.getByRole('button', { name: INIT_BTN }).first().waitFor({ timeout: 10000 });
    await ctx.page.getByRole('button', { name: INIT_BTN }).first().click();
    await ctx.page.waitForTimeout(1500);
  }
  // Asegura que el workspace se registró correctamente.
  await ctx.page.getByText(name).first().waitFor({ timeout: 12000 });
}

// Índice del artículo de la tabla que contiene a `name` (o -1).
async function cardIndex(ctx, name) {
  return ctx.page.evaluate((n) => {
    return [...document.querySelectorAll('article.card')].findIndex((a) => a.textContent.includes(n));
  }, name);
}

// Clicka el enlace "Editar" del workspace `name`.
async function clickEditFor(ctx, name) {
  const idx = await cardIndex(ctx, name);
  ctx.assert(idx >= 0, `workspace "${name}" no está en la tabla`);
  await ctx.page.locator('article.card').nth(idx).getByRole('link', { name: /Editar|Edit/ }).first().click();
}

// Clicka el botón "Quitar" del workspace `name` (confirm nativo aceptado por el harness).
async function clickRemoveFor(ctx, name) {
  const idx = await cardIndex(ctx, name);
  ctx.assert(idx >= 0, `workspace "${name}" no está en la tabla`);
  await ctx.page.locator('article.card').nth(idx).getByRole('button', { name: /Quitar|Remove/ }).first().click();
}

// Formulario de edición (distinguido del de registro por su clase "card").
function editForm(ctx) {
  return ctx.page.locator('form[class*="card"], form.card');
}

// Input del campo Nombre dentro del formulario de edición.
function editNameInput(ctx) {
  return editForm(ctx).getByLabel('Nombre').first();
}

// Selector de carpetas (modal "Elegir carpeta", panel #modal-panel).
function modal(ctx) {
  return ctx.page.locator('#modal-panel');
}

// Abre el selector partiendo de `dirPath` en el formulario de registro.
async function openPicker(ctx, dirPath) {
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dirPath);
  await ctx.page.getByRole('button', { name: /Elegir/ }).first().click();
  await modal(ctx).waitFor({ timeout: 10000 });
}

// Espera a que el valor de un input contenga `needle`; devuelve el valor final.
// (Evita sleeps fijos que pierden si el round-trip al servidor tarda más.)
async function waitForValue(locator, needle, { timeout = 10000 } = {}) {
  const start = Date.now();
  let last = '';
  while (Date.now() - start < timeout) {
    try {
      last = await locator.first().inputValue();
    } catch {
      last = '';
    }
    if (last.includes(needle)) return last;
    await new Promise((r) => setTimeout(r, 250));
  }
  return last;
}

// Directorio de trabajo bajo el HOME real: la raíz permitida del explorador del
// servidor es home_dir (HOME del proceso); crear carpetas fuera de ahí fallaría con
// PathOutsideAllowedRoot. Se crea y se borra por test para no dejar residuos.
function wsHomeDir(name) {
  const realHome = process.env.HOME || process.env.USERPROFILE;
  const dir = path.join(realHome, `iwf-e2e-ws-${name}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}

// ---- tests ----
module.exports = {
  name: 'workspaces',
  tests: [
    {
      name: 'list_workspaces: estado vacío muestra placeholder',
      fn: async (ctx) => {
        // workspaces es el primer módulo del runner (run.e2e.cjs): la BD es
        // fresca y vacía, así que el placeholder de estado vacío se pinta
        // sin más. (Si alguien lo lanza fuera del orden, este test fallaría;
        // el orden del runner es la garantía.)
        await ctx.goto('/workspaces');
        const placeholder = ctx.page
          .locator('text=Aún no hay workspaces')
          .first()
          .or(ctx.page.locator('text=No workspaces yet').first());
        await placeholder.waitFor({ timeout: 10000 });
      },
    },
    {
      name: 'register_workspace: registrar un workspace git aparece en la lista',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('register');
        const name = `git-ws-${tag}`;
        await registerWs(ctx, { name, gitRepo: repo, baseBranch: 'main' });
        // El camino se muestra en la tarjeta.
        await ctx.page.getByText(repo).first().waitFor({ timeout: 12000 });
      },
    },
    {
      name: 'register_workspace: carpeta no-git muestra confirmación de inicialización',
      fn: async (ctx) => {
        const dir = uniqueNonGitDir('nongit');
        const name = `nongit-ws-${tag}`;
        await registerWs(ctx, { name, nonGitDir: dir, baseBranch: 'main', expectConfirm: true });
        await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
      },
    },
    {
      name: 'get_workspace + update_workspace: editar y guardar cambia el nombre',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('edit');
        const name = `edit-ws-${tag}`;
        const newName = `edit-ws-upd-${tag}`;
        await registerWs(ctx, { name, gitRepo: repo, baseBranch: 'main' });
        await clickEditFor(ctx, name);
        await ctx.page.waitForTimeout(2000);
        const nameInput = editNameInput(ctx);
        const current = await nameInput.inputValue();
        ctx.assert(current === name, `campo nombre no prefijado (esperado "${name}", got "${current}")`);
        await nameInput.fill(newName);
        await editForm(ctx).getByRole('button', { name: /Guardar cambios|Save changes/ }).first().click();
        await ctx.page.waitForTimeout(1500);
        await ctx.goto('/workspaces');
        await ctx.page.getByText(newName).first().waitFor({ timeout: 12000 });
      },
    },
    {
      name: 'delete_workspace: quitar un workspace lo elimina de la lista',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('del');
        const name = `del-ws-${tag}`;
        await registerWs(ctx, { name, gitRepo: repo, baseBranch: 'main' });
        await clickRemoveFor(ctx, name);
        await ctx.page.waitForTimeout(1500);
        const count = await ctx.page.getByText(name).count();
        ctx.assert(count === 0, `el workspace "${name}" no fue eliminado (${count} ocurrencias restantes)`);
      },
    },
    {
      name: 'get_workspace: abrir edición muestra el workspace precargado',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('get');
        const name = `get-ws-${tag}`;
        await registerWs(ctx, { name, gitRepo: repo, baseBranch: 'main' });
        await clickEditFor(ctx, name);
        await ctx.page.waitForTimeout(2000);
        const current = await editNameInput(ctx).inputValue();
        ctx.assert(current === name, `edición no precargó el nombre (got "${current}")`);
      },
    },
    {
      name: 'browse_directories: el selector lista y navega carpetas reales',
      fn: async (ctx) => {
        const browseDir = wsHomeDir('browse');
        const subName = 'subfolder';
        fs.mkdirSync(path.join(browseDir, subName), { recursive: true });
        try {
          await openPicker(ctx, browseDir);
          // la lista muestra la subcarpeta creada
          await modal(ctx)
            .getByRole('button', { name: subName })
            .first()
            .waitFor({ timeout: 10000 });
          // navegar a la subcarpeta (vacía)
          await modal(ctx).getByRole('button', { name: subName }).first().click();
          await ctx.page.getByText('Sin subcarpetas').first().waitFor({ timeout: 10000 });
          // el input Ruta del selector debe contener la subcarpeta (el round-trip de listing)
          const mPath = await waitForValue(modal(ctx).getByLabel('Ruta'), subName);
          ctx.assert(
            mPath.includes(subName),
            `el selector no navegó a la subcarpeta (got "${mPath}")`
          );
          // cerrar eligiendo la carpeta actual: select_directory aplica la ruta al input
          // "Ruta absoluta" del formulario y cierra el selector.
          await modal(ctx).getByRole('button', { name: /Usar esta carpeta/ }).first().click();
          const formPath = await waitForValue(ctx.page.getByLabel(/^Ruta absoluta/), subName);
          ctx.assert(
            formPath.includes(subName),
            `el formulario no recibió la ruta elegida (got "${formPath}")`
          );
        } finally {
          fs.rmSync(browseDir, { recursive: true, force: true });
        }
      },
    },
    {
      name: 'create_directory: crear una carpeta y navegar a ella',
      fn: async (ctx) => {
        const createParent = wsHomeDir('create');
        const newName = `newfolder-${tag}`;
        try {
          await openPicker(ctx, createParent);
          await modal(ctx).getByLabel('Nombre de la nueva carpeta').first().fill(newName);
          await modal(ctx).getByRole('button', { name: /Crear/ }).first().click();
          // el selector navega a la nueva carpeta (vacía): esperar el round-trip de listing
          const mPath = await waitForValue(modal(ctx).getByLabel('Ruta'), newName);
          ctx.assert(
            mPath.includes(newName),
            `no navegó a la carpeta creada (got "${mPath}")`
          );
          await ctx.page.getByText('Sin subcarpetas').first().waitFor({ timeout: 5000 });
          // seleccionar y verificar el formulario
          await modal(ctx).getByRole('button', { name: /Usar esta carpeta/ }).first().click();
          const formPath = await waitForValue(ctx.page.getByLabel(/^Ruta absoluta/), newName);
          ctx.assert(
            formPath.includes(newName),
            `el formulario no recibió la nueva ruta (got "${formPath}")`
          );
          ctx.assert(
            fs.existsSync(path.join(createParent, newName)),
            'el directorio no fue creado en disco'
          );
        } finally {
          fs.rmSync(createParent, { recursive: true, force: true });
        }
      },
    },
  ],
};
