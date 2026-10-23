// end2end/agents_files.e2e.cjs
// Pruebas end-to-end (Playwright CJS, harness compartido) de la ruta
// /agents-files -> AgentsFilesPage.
//
// Selectores y strings observados en la UI actual (locale es, iw-lang=es):
//   - La página /agents-files se monta como pestaña del dock (SSR renderiza
//     solo el shell); el contenido aparece tras la hidratación y DockBootstrap.
//   - Form de subida: TextField "Descripción" (name=description),
//     <label>Fichero AGENTS.md</label> envolviendo input[type=file] (name=file),
//     BusyButton "Subir" / "Subiendo…" (type=submit).
//   - Error de subida (ActionError -> <p>): el <ServerFnError> de Leptos se
//     muestra con prefijo fijo:
//     "error running server function: la descripción es obligatoria (máximo 200 caracteres)"
//     (AgentsFileError::InvalidDescription; MAX_DESCRIPTION_CHARS = 200).
//   - Listado: un <article class="card"> por fichero (los formularios son
//     <section class="card">, así que `article.card` es exclusivo del list).
//   - Estado vacío: <p class="muted">Aún no hay ficheros AGENTS.md.</p>.
//     Nota: `getByRole('paragraph', { name })` NO hace match en estos <p>
//     (verificado empíricamente); se afirma con `getByText` exacto.
//   - Borrar: botón "Eliminar" -> confirm() nativo (el harness lo auto-acepta).
//   - Selector de proyecto: <select> con etiqueta "Proyecto", opción
//     placeholder "Elige un proyecto". Las opciones no seleccionadas son
//     "hidden" para Playwright; se afirma con count() de locator('option').
//   - Aplicar: botón "Añadir a proyecto" (deshabilitado sin proyecto o
//     durante la acción); feedback por carta:
//       * éxito: <p>Añadido y commiteado</p> + botón "Cerrar" (btn-ghost)
//       * overwrite: panel ConfirmPanel "Ya existe AGENTS.md en ese proyecto.
//         ¿Sobrescribir?" + confirmación "Sobrescribir" + "Cancelar"
//
// Contrato de la suite:
//   - getByLabel / getByRole / getByText (nunca locator('text=...')).
//   - Polling de valores dinámicos (cardIndex / assertInList / expectCount);
//     sin sleeps fijos para el resultado.
//   - Nombres únicos por test (Date.now()) para no chocar en la BD compartida.
//   - Cada test sube su propio fichero (descripción única) y solo afirma sobre
//     su tarjeta del list.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

// ---- helpers ----

// Repo git fresco y único (con un commit) para registrar como workspace/proyecto.
function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `af-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `content ${prefix} ${tag}`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Fichero .md temporal para la subida (campo "Fichero AGENTS.md").
function uniqueAgentsMdFile(prefix, content) {
  const file = path.join(TMP, `af-file-${prefix}-${tag}.md`);
  fs.writeFileSync(file, content);
  return file;
}

// Registra un workspace git desde /workspaces (mis selectors que workspaces.e2e.cjs).
async function registerWorkspace(ctx, { name, gitRepo, baseBranch = 'main' }) {
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).fill(gitRepo);
  await ctx.page.getByLabel(/Rama base/).fill(baseBranch);
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(name).first().waitFor({ timeout: 12000 });
}

// Índice de la tarjeta del list que contiene el fichero `name` (-1 si no existe).
// Solo mira `article.card`: las tarjetas de formulario del list son
// <section class="card">, así que este selector es exclusivo del list.
function cardIndex(ctx, name) {
  return ctx.page.evaluate(
    (n) => [...document.querySelectorAll('article.card')].findIndex((a) => a.textContent.includes(n)),
    name
  );
}

// Polling hasta que el fichero `name` esté (visible) o no (absente) en el list.
// Se mira el DOM del list, nunca el campo "Descripción" del formulario (que
// puede contener el mismo texto durante la subida o tras un error).
async function assertInList(ctx, name, { visible = true, timeoutMs = 15000 } = {}) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const idx = await cardIndex(ctx, name);
    if ((visible && idx >= 0) || (!visible && idx === -1)) return;
    await new Promise((r) => setTimeout(r, 250));
  }
  ctx.assert(
    false,
    visible ? `el fichero "${name}" no aparece en la lista` : `el fichero "${name}" sigue en la lista`
  );
}

// La tarjeta (article.card) del fichero `name`.
async function cardOf(ctx, name) {
  const idx = await cardIndex(ctx, name);
  ctx.assert(idx >= 0, `el fichero "${name}" no está en la lista`);
  return ctx.page.locator('article.card').nth(idx);
}

// Polling hasta que un locator tenga al menos un elemento (las opciones de
// <select> no seleccionadas no son "visible" para getByRole; count() sí las
// ve). Mismo espíritu que el waitForValue de workspaces.e2e.cjs.
async function expectCount(ctx, loc, { timeoutMs = 10000, msg } = {}) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (await loc.count() > 0) return;
    await new Promise((r) => setTimeout(r, 250));
  }
  ctx.assert(false, msg || 'el elemento no apareció en el tiempo esperado');
}

// Polling del estado de un botón (enabled/disabled): Leptos re-renderiza entre
// el click y la lectura de isEnabled(), y un solo sample puede caer a medias.
async function waitForBtnState(ctx, btn, wantEnabled, { timeoutMs = 5000 } = {}) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const disabled = await btn.isDisabled();
    if (disabled === !wantEnabled) return;
    await new Promise((r) => setTimeout(r, 150));
  }
  ctx.assert(false, `el botón no alcanzó ${wantEnabled ? 'habilitado' : 'deshabilitado'}`);
}

// Sube un AGENTS.md desde el formulario y espera a que aparezca en el list.
async function uploadAgentsFile(ctx, { description, filePath }) {
  await ctx.goto('/agents-files');
  await ctx.page.getByLabel('Descripción').first().fill(description);
  await ctx.page.getByLabel('Fichero AGENTS.md').first().setInputFiles([filePath]);
  await ctx.page.getByRole('button', { name: /Subir|Subiendo/ }).first().click();
  await assertInList(ctx, description, { visible: true });
}

// Selecciona el proyecto `project` en la tarjeta de `name` y pulsa "Añadir a proyecto".
async function applyToProject(ctx, name, project) {
  const card = await cardOf(ctx, name);
  await card.getByLabel('Proyecto').first().selectOption({ label: project });
  const btn = card.getByRole('button', { name: 'Añadir a proyecto', exact: true });
  await btn.first().waitFor({ timeout: 10000 });
  await waitForBtnState(ctx, btn, true, { timeoutMs: 5000 });
  await btn.first().click();
}

// ---- tests ----
module.exports = {
  name: 'agents_files',
  tests: [
    {
      name: 'list_agents_files: estado vacío muestra el placeholder',
      fn: async (ctx) => {
        await ctx.goto('/agents-files');
        await ctx.page
          .getByText('Aún no hay ficheros AGENTS.md.', { exact: true })
          .first()
          .waitFor({ timeout: 12000 });
      },
    },
    {
      name: 'upload_agents_file: subir un fichero lo añade a la lista',
      fn: async (ctx) => {
        const description = `Reglas demo ${tag}`;
        const filePath = uniqueAgentsMdFile('upload', `# ${description}\n`);
        await uploadAgentsFile(ctx, { description, filePath });
        // La tarjeta del list muestra la descripción y un camino .md.
        const card = await cardOf(ctx, description);
        await card.getByText(description, { exact: true }).first().waitFor({ timeout: 10000 });
        const code = card.locator('code').first();
        await code.waitFor({ timeout: 10000 });
        const codeText = await code.innerText();
        ctx.assert(/\.md$/.test(codeText), `el camino del fichero no termina en .md (got "${codeText}")`);
      },
    },
    {
      name: 'upload_agents_file: descripción >200 caracteres muestra el error exacto del servidor',
      fn: async (ctx) => {
        const longDescription = 'a'.repeat(201);
        const filePath = uniqueAgentsMdFile('long-desc', `# Long ${tag}\n`);

        await ctx.goto('/agents-files');
        await ctx.page.getByLabel('Descripción').first().fill(longDescription);
        await ctx.page.getByLabel('Fichero AGENTS.md').first().setInputFiles([filePath]);
        await ctx.page.getByRole('button', { name: /Subir|Subiendo/ }).first().click();

        // Mensaje exacto que pinta la UI: ServerFnError::ServerError de Leptos
        // (prefijo "error running server function: ") +
        // AgentsFileError::InvalidDescription ("la descripción es obligatoria
        // (máximo {MAX_DESCRIPTION_CHARS} caracteres)", MAX_DESCRIPTION_CHARS = 200).
        const expected =
          'error running server function: la descripción es obligatoria (máximo 200 caracteres)';
        await ctx.page
          .getByText(expected, { exact: true })
          .first()
          .waitFor({ timeout: 10000 });

        // El fichero no se guardó: su descripción no aparece en la lista.
        await assertInList(ctx, longDescription, { visible: false });
      },
    },
    {
      name: 'delete_agents_file: eliminar un fichero lo quita de la lista',
      fn: async (ctx) => {
        const description = `Del ${tag}`;
        const filePath = uniqueAgentsMdFile('delete', `# Del ${tag}\n`);
        await uploadAgentsFile(ctx, { description, filePath });

        // La confirmación nativa confirm() la auto-acepta el harness; el fichero
        // desaparece del list.
        const card = await cardOf(ctx, description);
        await card.getByRole('button', { name: 'Eliminar', exact: true }).first().click();
        await assertInList(ctx, description, { visible: false });
      },
    },
    {
      name: 'list_projects: los workspaces registrados aparecen como opciones en "Proyecto"',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('projects');
        const wsName = `af-projects-${tag}`;
        await registerWorkspace(ctx, { name: wsName, gitRepo: repo, baseBranch: 'main' });

        const description = `Proyectos ${tag}`;
        const filePath = uniqueAgentsMdFile('projects', `# ${tag}\n`);
        await uploadAgentsFile(ctx, { description, filePath });

        const card = await cardOf(ctx, description);
        const select = card.getByLabel('Proyecto');
        await select.first().waitFor({ timeout: 10000 });

        // El workspace registrado aparece como opción (no seleccionada: "hidden"
        // para Playwright; count() la ve igual).
        const wsOption = select.locator('option', { exact: true, hasText: wsName });
        await expectCount(ctx, wsOption, { msg: `el workspace "${wsName}" no es una opción de "Proyecto"` });

        // El placeholder vacío también aparece.
        const placeholderOption = select
          .locator('option', { exact: true, hasText: 'Elige un proyecto' });
        await expectCount(ctx, placeholderOption, {
          msg: 'falta la opción placeholder "Elige un proyecto" de "Proyecto"',
        });
      },
    },
    {
      name: 'apply_agents_file_to_project: añadir a proyecto muestra "Añadido y commiteado" y se limpia con "Cerrar"',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('apply');
        const wsName = `af-apply-${tag}`;
        const description = `Aplicar ${tag}`;
        const filePath = uniqueAgentsMdFile('apply', `# Aplicar ${tag}\n`);

        await registerWorkspace(ctx, { name: wsName, gitRepo: repo, baseBranch: 'main' });
        await uploadAgentsFile(ctx, { description, filePath });

        // apply_agents_file -> install_agents_file: el contenido se escribe en el
        // workspace y el commit se crea -> feedback Committed.
        await applyToProject(ctx, description, wsName);
        const card = await cardOf(ctx, description);
        await card
          .getByText('Añadido y commiteado', { exact: true })
          .first()
          .waitFor({ timeout: 15000 });

        // El feedback se descarta con el botón inline "Cerrar".
        await card.getByRole('button', { name: 'Cerrar', exact: true }).first().click();
        await card
          .getByText('Añadido y commiteado', { exact: true })
          .first()
          .waitFor({ state: 'detached', timeout: 10000 });
      },
    },
    {
      name: 'apply_agents_file_to_project: objetivo existente abre el panel de sobrescribir y confirmar re-sube',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('ovw');
        const wsName = `af-ovw-${tag}`;
        const description = `Sobrescribir ${tag}`;
        const filePath = uniqueAgentsMdFile('ovw', `# Ov ${tag}\n`);

        await registerWorkspace(ctx, { name: wsName, gitRepo: repo, baseBranch: 'main' });
        await uploadAgentsFile(ctx, { description, filePath });

        // Primera aplicación: AGENTS.md no existe -> se crea y se commitea.
        await applyToProject(ctx, description, wsName);
        await (await cardOf(ctx, description))
          .getByText('Añadido y commiteado', { exact: true })
          .first()
          .waitFor({ timeout: 15000 });

        // Segunda aplicación: AGENTS.md ya existe -> AgentsFileError::TargetExists
        // -> panel inline ConfirmPanel (no un dialog nativo).
        await applyToProject(ctx, description, wsName);
        await (await cardOf(ctx, description))
          .getByText('Ya existe AGENTS.md en ese proyecto. ¿Sobrescribir?', { exact: true })
          .first()
          .waitFor({ timeout: 15000 });

        // El botón inline "Sobrescribir" reenvía el último apply con
        // overwrite=true: se re-escribe el mismo contenido, no hay cambios que
        // commitear (git diff --cached limpio) y el instalador devuelve
        // committed=false -> la UI pinta el feedback NotCommitted.
        await (await cardOf(ctx, description))
          .getByRole('button', { name: 'Sobrescribir', exact: true })
          .first()
          .click();
        await (await cardOf(ctx, description))
          .getByText('Añadido (no se pudo crear el commit; queda sin commitear)', { exact: true })
          .first()
          .waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'apply_agents_file_to_project: "Añadir a proyecto" está deshabilitado hasta elegir un proyecto y habilitado después',
      fn: async (ctx) => {
        const repo = uniqueGitRepo('disabled');
        const wsName = `af-disabled-${tag}`;
        const description = `Habilitar ${tag}`;
        const filePath = uniqueAgentsMdFile('disabled', `# Dis ${tag}\n`);

        await registerWorkspace(ctx, { name: wsName, gitRepo: repo, baseBranch: 'main' });
        await uploadAgentsFile(ctx, { description, filePath });

        const card = await cardOf(ctx, description);
        const btn = card.getByRole('button', { name: 'Añadir a proyecto', exact: true });
        await btn.first().waitFor({ timeout: 10000 });
        await waitForBtnState(ctx, btn, false, { timeoutMs: 5000 });

        // Elegir el proyecto habilita el botón.
        await card.getByLabel('Proyecto').first().selectOption({ label: wsName });
        await waitForBtnState(ctx, btn, true, { timeoutMs: 5000 });
      },
    },
  ],
};
