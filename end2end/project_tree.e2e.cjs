// end2end/project_tree.e2e.cjs
// Cubre los 9 casos de uso de project_tree alcanzables desde la UI:
//   list_project_directory, read_project_file, write_project_file,
//   read_project_file_base, search_project_files (tree/composer),
//   list_project_git_changes, stage_project_paths, unstage_project_paths,
//   commit_project_changes.
//
// Dos superficies de la misma funcionalidad:
//   * La vista de workspace  (/workspaces/<id>  ->  ProjectTreePage, y el visor /workspaces/<id>/files/<path>)
//     opera sobre el repo del workspace.
//   * La vista de tarea     (/tasks/<id>  con pestañas Archivos/Cambios, y el visor /tasks/<id>/files/<path>)
//     opera sobre el worktree de la tarea. El panel "Cambios" (GitChangesPanel) solo existe
//     en la vista de tarea: de ahí sale el stage/unstage/commit.
//
// Cada test crea SU PROPIO workspace (y, para los de tarea, SU PROPIO worktree) para no
// contaminarse entre sí. Los nombres usan un tag global + prefijo por test.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

const REPO_FILES = (dir, prefix) => {
  fs.mkdirSync(path.join(dir, 'src'), { recursive: true });
  fs.writeFileSync(path.join(dir, 'README.md'), `hola ${prefix}\n`);
  fs.writeFileSync(path.join(dir, 'nota.txt'), `nota ${prefix}\n`);
  fs.writeFileSync(path.join(dir, 'src', 'main.rs'), `fn main() {}\n`);
};

// ---- helpers ----

// Repo git fresco y único con README.md, nota.txt y src/main.rs. Devuelve su ruta.
function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `pt-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  REPO_FILES(dir, prefix);
  execSync(`git -C '${dir}' -c user.name=smoke -c user.email=smoke@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=smoke -c user.email=smoke@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Registra un workspace desde /workspaces y devuelve el id (UUID) del workspace
// JUSTO registrado. En la suite compartida hay workspaces de otros módulos, así
// que no se toma el primer enlace: se busca la tarjeta (article.card) que
// contiene el nombre y se extrae el UUID del enlace de edición de ESA tarjeta.
async function registerWs(ctx, wsName, dir) {
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').fill(wsName);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  const id = await ctx.page.evaluate((name) => {
    const cards = [...document.querySelectorAll('article.card')];
    for (const card of cards) {
      if (!card.textContent.includes(name)) continue;
      const link = card.querySelector('a[href*="/workspaces/"]');
      if (!link) continue;
      const m = link.href.match(/\/workspaces\/([0-9a-f-]{36})/);
      if (m) return m[1];
    }
    return null;
  }, wsName);
  ctx.assert(id, `no se pudo extraer el id del workspace "${wsName}"`);
  return id;
}

// Espera a que el valor del input contenga `needle`; devuelve el valor final.
// (Mismo patrón que workspaces.e2e.cjs; evita sleeps fijos como único punto de sincronía.)
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

// Crea una tarea sobre el workspace wsName (start_now => worktree + agente) y devuelve su id.
async function createTask(ctx, wsName, prompt) {
  await ctx.goto('/');
  await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await ctx.page.getByLabel(/^Agente/).selectOption({ index: 1 }); // pi
  await ctx.page.getByRole('button', { name: /Nueva tarea/ }).first().click();
  // El select de workspace es el cuyo valor contenga "wsName (main)".
  const wsIdx = await ctx.page.evaluate((name) => {
    const sels = [...document.querySelectorAll('select')];
    for (let i = 0; i < sels.length; i++) {
      if ([...sels[i].options].some((o) => o.text.includes(name + ' (main)'))) return i;
    }
    return -1;
  }, wsName);
  ctx.assert(wsIdx >= 0, `no hay select de workspace con "${wsName}"`);
  await ctx.page.locator('select').nth(wsIdx).selectOption({ label: wsName + ' (main)' });
  await ctx.page.locator('textarea').first().fill(prompt);
  await ctx.page.getByRole('checkbox', { name: /Iniciar ahora/ }).first().check();
  await ctx.page.getByRole('button', { name: /Crear tarea/ }).first().click({ timeout: 10000 });
  await ctx.page.waitForTimeout(6000); // worktree + agente (falso) terminan casi al instante
  const taskId = await ctx.page.evaluate(() => {
    const links = [...document.querySelectorAll('a[href*="/tasks/"]')];
    const a = links[0];
    return a ? a.href.split('/tasks/')[1] : null;
  });
  ctx.assert(taskId, 'no se pudo extraer el id de la tarea');
  return taskId;
}

// Edita un fichero del WORKTREE de la tarea (visor /tasks/<id>/files/<path>) y guarda.
async function modifyTaskFile(ctx, taskId, relPath, newContent) {
  await ctx.goto(`/tasks/${taskId}/files/${encodeURIComponent(relPath)}`);
  await waitForValue(ctx.page.locator('textarea').first(), 'hola', { timeout: 12000 });
  await ctx.page.locator('textarea').first().fill(newContent);
  await ctx.page.getByRole('button', { name: /Guardar|Save/ }).first().click();
  await ctx.page.waitForTimeout(1500);
}

// Abre el panel "Cambios" (GitChangesPanel) de la tarea.
async function openChanges(ctx, taskId) {
  await ctx.goto(`/tasks/${taskId}`);
  await ctx.page.getByRole('tab', { name: 'Cambios', exact: true }).first().click();
  await ctx.page.getByText(/Cargando cambios|Sin cambios|Con stage|Cambios|Archivos sin seguimiento|README/).first().waitFor({ timeout: 12000 }).catch(() => {});
  await ctx.page.waitForTimeout(1500);
  ctx.assert(
    await ctx.page.locator('[data-testid="git-changes"]').first().count(),
    'falta el panel [data-testid="git-changes"]'
  );
  return ctx.page.locator('[data-testid="git-changes"]').first();
}

// Localiza la fila de un fichero dentro de una sección del panel de cambios.
function rowIn(section, filePath) {
  return `section[data-testid="${section}"] li[data-testid="git-row"][data-path="${filePath}"]`;
}

// Raíces posibles del directorio de worktrees (run_tests.sh las sitúa en $WORK/worktrees).
function worktreeRoots() {
  const tmpRoot = TMP || '';
  const candidates = [
    process.env.IWF_WORKTREES_DIR,
    process.env.SMOKE_WORKTREES,
    path.join(path.dirname(tmpRoot), 'worktrees'),
    path.join(tmpRoot, 'worktrees'),
  ];
  return candidates.filter((c) => c && fs.existsSync(c));
}

// Busca el worktree cuyo README.md contiene el marcador (búsqueda recursiva).
function findWorktree(marker) {
  for (const root of worktreeRoots()) {
    const stack = [root];
    while (stack.length) {
      const d = stack.shift();
      let entries;
      try {
        entries = fs.readdirSync(d);
      } catch {
        continue;
      }
      for (const name of entries) {
        const full = path.join(d, name);
        let st;
        try {
          st = fs.statSync(full);
        } catch {
          continue;
        }
        if (st.isDirectory()) {
          const readme = path.join(full, 'README.md');
          const git = path.join(full, '.git');
          if (fs.existsSync(readme) && fs.existsSync(git)) {
            if (fs.readFileSync(readme, 'utf8').includes(marker)) return full;
          } else {
            stack.push(full);
          }
        }
      }
    }
  }
  return null;
}

module.exports = {
  name: 'project_tree',
  tests: [
    {
      name: 'list_project_directory: el árbol lista los ficheros y carpetas del workspace',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('tree');
        const ws = `tree-${tag}`;
        const id = await registerWs(ctx, ws, dir);
        await ctx.goto(`/workspaces/${id}`);
        // Raíz: README.md, nota.txt (ficheros, enlazados) y src (carpeta, botón).
        const readme = ctx.page.getByRole('link', { name: 'README.md', exact: true });
        await readme.first().waitFor({ timeout: 12000 });
        const nota = ctx.page.getByRole('link', { name: 'nota.txt', exact: true });
        await nota.first().waitFor({ timeout: 12000 });
        const src = ctx.page.getByRole('button', { name: 'src', exact: true });
        await src.first().waitFor({ timeout: 12000 });
        ctx.assert(
          await readme.first().getAttribute('href'),
          'el fichero README.md debe ser un enlace del árbol'
        );
        // Abrir src carga los hijos (otra llamada a list_project_directory) y muestra main.rs.
        await src.first().click();
        const main = ctx.page.getByRole('link', { name: 'main.rs', exact: true });
        await main.first().waitFor({ timeout: 12000 });
      },
    },
    {
      name: 'read_project_file: abrir un fichero muestra su contenido en el visor',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('read');
        const ws = `read-${tag}`;
        const id = await registerWs(ctx, ws, dir);
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        // El editor (textarea.code-input) carga el contenido del fichero.
        const content = await waitForValue(ctx.page.locator('textarea').first(), 'hola', { timeout: 12000 });
        ctx.assert(content.includes('hola read'), `el visor no mostró el contenido esperado (got "${content}")`);
        // El título muestra el nombre del fichero.
        const title = await ctx.page.getByRole('heading', { level: 1 }).first().textContent();
        ctx.assert(title.includes('README.md'), `el título no es README.md (got "${title}")`);
      },
    },
    {
      name: 'write_project_file: editar y guardar un fichero persiste el cambio',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('write');
        const ws = `write-${tag}`;
        const id = await registerWs(ctx, ws, dir);
        const newContent = `hola write\nmódulo ${tag}\n`;
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        await waitForValue(ctx.page.locator('textarea').first(), 'hola', { timeout: 12000 });
        await ctx.page.locator('textarea').first().fill(newContent);
        // Al modificar aparece "Cambios sin guardar" y se habilita Guardar.
        await ctx.page.getByText('Cambios sin guardar').first().waitFor({ timeout: 6000 });
        await ctx.page.getByRole('button', { name: /Guardar|Save/ }).first().click();
        // Verificación en disco: el fichero del workspace se escribió.
        await ctx.page.waitForTimeout(1500);
        const disk = fs.readFileSync(path.join(dir, 'README.md'), 'utf8');
        ctx.assert(disk.includes('módulo ' + tag), `el fichero en disco no guardó el cambio (got "${disk}")`);
        // Y reabiéndolo el visor muestra el contenido guardado.
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        const again = await waitForValue(ctx.page.locator('textarea').first(), 'módulo', { timeout: 12000 });
        ctx.assert(again.includes('módulo ' + tag), `re-lectura no mostró el contenido guardado (got "${again}")`);
      },
    },
    {
      name: 'read_project_file_base: el visor carga la base de git y pinta marcas de diferencia',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('base');
        const ws = `base-${tag}`;
        const id = await registerWs(ctx, ws, dir);
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        // El contenido base (HEAD) es "hola base\n". Cambiamos el borrador para que
        // line_marks(base, draft) produzca al menos una marca en el margen.
        await waitForValue(ctx.page.locator('textarea').first(), 'hola', { timeout: 12000 });
        await ctx.page.locator('textarea').first().fill('hola base\nlínea nueva\n');
        // Espera a que el margen pinte una marca (la base se carga asíncrona).
        let marked = false;
        const start = Date.now();
        while (Date.now() - start < 10000) {
          marked = await ctx.page.locator('pre.code-gutter-edit span.gm-added, pre.code-gutter-edit span.gm-modified, pre.code-gutter-edit span.gm-deleted').first().count();
          if (marked) break;
          await ctx.page.waitForTimeout(300);
        }
        ctx.assert(
          marked > 0,
          'el margen no pintó marcas de diferencia: la base de git no se cargó (markes=0)'
        );
      },
    },
    {
      name: 'search_project_files: la búsqueda @ del chat lista los ficheros del worktree',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('search');
        const ws = `search-${tag}`;
        await registerWs(ctx, ws, dir);
        const taskId = await createTask(ctx, ws, `parche de búsqueda ${tag}`);
        await ctx.goto(`/tasks/${taskId}`);
        await ctx.page.waitForTimeout(3000);
        // El compositor está en el pie de la vista de tarea.
        const composer = ctx.page.getByPlaceholder(/Escribe un mensaje|@ ficheros|comandos/);
        ctx.assert(await composer.count(), 'no hay compositor en la vista de tarea');
        await composer.first().click();
        await composer.first().fill('@READ');
        // La sugerencia "README.md" debe aparecer en el menú.
        const item = ctx.page.locator('li.composer-menu-item');
        // La búsqueda de ficheros es asíncrona (LocalResource -> el servidor
        // lista el worktree); el menú solo pinta los "li.composer-menu-item"
        // cuando los ficheros llegan. Se espera al primer item antes de afirmar.
        await item.first().waitFor({ timeout: 15000 }).catch(() => {});
        const n = await item.count();
        ctx.assert(n > 0, 'la búsqueda @READ no abrió el menú de ficheros');
        const text = await item.first().textContent();
        ctx.assert(text.includes('README.md'), `la sugerencia de fichero no incluye README.md (got "${text}")`);
      },
    },
    {
      name: 'list_project_git_changes: la pestaña Cambios lista los cambios del worktree',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('listchanges');
        const ws = `listch-${tag}`;
        await registerWs(ctx, ws, dir);
        const taskId = await createTask(ctx, ws, `listar cambios ${tag}`);
        const panel = await openChanges(ctx, taskId);
        // El agente (pi) crea CAMBIO.md (sin seguimiento); README.md y nota.txt están limpios.
        const rows = await panel.locator('li[data-testid="git-row"]').count();
        ctx.assert(rows > 0, `no hay filas de cambios en el panel (rows=${rows})`);
        const html = await panel.first().textContent();
        ctx.assert(
          html.includes('CAMBIO.md') || html.includes('README.md'),
          `el panel no muestra un fichero de cambio (got "${html.slice(0, 400)}")`
        );
      },
    },
    {
      name: 'stage_project_paths: hacer stage de un fichero lo mueve a "Con stage"',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('stage');
        const ws = `stage-${tag}`;
        await registerWs(ctx, ws, dir);
        const taskId = await createTask(ctx, ws, `hacer stage ${tag}`);
        const marker = `hola stage\nmodificado ${tag}\n`;
        await modifyTaskFile(ctx, taskId, 'README.md', marker);
        const panel = await openChanges(ctx, taskId);
        // README.md debe estar en "Cambios" (no-stageado).
        await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).first().waitFor({ timeout: 15000 });
        // Hacer stage de la fila lo mueve a "Con stage".
        await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).locator('[data-testid="git-toggle"]').first().click();
        await ctx.page.locator(rowIn('git-section-staged', 'README.md')).first().waitFor({ timeout: 15000 });
        // El fichero ya no está en la sección de cambios.
        ctx.assert(
          await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).count() === 0,
          'README.md sigue en la sección de cambios tras hacer stage'
        );
      },
    },
    {
      name: 'unstage_project_paths: quitar del stage devuelve el fichero a "Cambios"',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('unstage');
        const ws = `unstage-${tag}`;
        await registerWs(ctx, ws, dir);
        const taskId = await createTask(ctx, ws, `quitar stage ${tag}`);
        const marker = `hola unstage\nmodificado ${tag}\n`;
        await modifyTaskFile(ctx, taskId, 'README.md', marker);
        const panel = await openChanges(ctx, taskId);
        // Primero lo ponemos en stage.
        await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).first().waitFor({ timeout: 15000 });
        await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).locator('[data-testid="git-toggle"]').first().click();
        await ctx.page.locator(rowIn('git-section-staged', 'README.md')).first().waitFor({ timeout: 15000 });
        // Y lo sacamos: vuelve a "Cambios".
        await ctx.page.locator(rowIn('git-section-staged', 'README.md')).locator('[data-testid="git-toggle"]').first().click();
        await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).first().waitFor({ timeout: 15000 });
        ctx.assert(
          await ctx.page.locator(rowIn('git-section-staged', 'README.md')).count() === 0,
          'README.md sigue en "Con stage" tras quitarlo del stage'
        );
      },
    },
    {
      name: 'commit_project_changes: commit de lo stageado crea un commit y limpia el panel',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('commit');
        const ws = `commit-${tag}`;
        await registerWs(ctx, ws, dir);
        const taskId = await createTask(ctx, ws, `hacer commit ${tag}`);
        const marker = `hola commit\nmodificado ${tag}\n`;
        await modifyTaskFile(ctx, taskId, 'README.md', marker);
        const panel = await openChanges(ctx, taskId);

        // Stage README.md (modificado) y CAMBIO.md (sin seguimiento, creado por el agente).
        await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).first().waitFor({ timeout: 15000 });
        await ctx.page.locator(rowIn('git-section-unstaged', 'README.md')).locator('[data-testid="git-toggle"]').first().click();
        // Si hay sección de sin seguimiento, stageamos toda (incluye CAMBIO.md).
        const untrackedToggle = ctx.page.locator(rowIn('git-section-untracked', 'CAMBIO.md')).locator('[data-testid="git-toggle"]').first();
        const untrackedSection = ctx.page.locator('section[data-testid="git-section-untracked"]');
        if (await untrackedSection.count()) {
          await untrackedSection.locator('[data-testid="git-section-toggle"]').first().click();
          await ctx.page.locator(rowIn('git-section-staged', 'README.md')).first().waitFor({ timeout: 15000 });
        }
        // El formulario de commit aparece cuando hay algo en stage.
        await ctx.page.locator('[data-testid="git-commit-message"]').first().waitFor({ timeout: 15000 });
        const commitMsg = `commit e2e ${tag}`;
        await ctx.page.getByLabel('Mensaje del commit').first().fill(commitMsg);
        await ctx.page.getByRole('button', { name: /Commit/ }).first().click();
        // Tras el commit, el árbol queda limpio y el panel muestra el estado vacío.
        const empty = ctx.page.getByTestId('git-empty');
        let ok = false;
        const start = Date.now();
        while (Date.now() - start < 15000) {
          ok = await empty.first().count();
          if (ok) break;
          await ctx.page.waitForTimeout(500);
        }
        ctx.assert(ok, `el panel no mostró el estado vacío tras el commit`);
        // Verificación en disco: localizamos el worktree con el marcador y comprobamos el commit.
        const wt = findWorktree('modificado ' + tag);
        if (wt) {
          const out = execSync(`git -C '${wt}' log -1 --format=%s`, { encoding: 'utf8', stdio: 'pipe' }).trim();
          ctx.assert(out === commitMsg, `el mensaje del commit no es el esperado (got "${out}")`);
          const count = Number(
            execSync(`git -C '${wt}' rev-list --count HEAD`, { encoding: 'utf8', stdio: 'pipe' }).trim()
          );
          ctx.assert(count >= 2, `se esperaba al menos 2 commits en el worktree (got ${count})`);
        }
        // Si no se localiza el worktree en disco, la verificación queda en el estado vacío UI
        // (aceptado por el enunciado: "verify e.g. via the Changes tab clearing").
      },
    },
  ],
};
