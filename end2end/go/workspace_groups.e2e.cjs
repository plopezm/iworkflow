// end2end/go/workspace_groups.e2e.cjs
// Workspace groups of the Home sidebar, end to end in the browser (odd/tasks/workspace-groups.md, T6, S1-S9).
// One ordered flow over three workspaces: create a group from the "+" prompt, move workspaces in and out of it from
// their "…" menus, collapse it and reload, see the count and the attention marker, pin a workspace, filter by name
// (focus and text survive a sidebar refresh, matches inside a collapsed group appear, no match shows the empty line),
// recolor, rename and delete the group, and check that the open menus survive a refresh.
// Every check reads an observable effect: a DOM attribute of the sidebar, the text shown, or the state the server
// keeps (read again after a reload). The sidebar is read through readSidebar(), never through its CSS classes.
// Screenshots go to SMOKE_SCREENSHOT_DIR (set by run_go.sh to <run dir>/screenshots). The texts are the Spanish ones
// (the harness language). The task that needs attention is started with the default fake agent, like
// board_live_sidebar does, so no real model runs.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// A git repo with one commit on main, under the run's scratch dir (one repo per workspace: the server refuses two
// workspaces on the same root).
function gitRepo(prefix) {
  const dir = fs.mkdtempSync(path.join(TMP, `workspace-groups-${prefix}-`));
  fs.writeFileSync(path.join(dir, 'README.md'), `${prefix}\n`);
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Polls read() until done(value) holds; returns the last value read.
async function until(read, done, { timeout = 12000 } = {}) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    last = await read().catch(() => undefined);
    if (done(last)) return last;
    await sleep(200);
  }
  return last;
}

// The sidebar as the page renders it right now: the pinned block, each group with its stored state and rows, the
// ungrouped rows, the attention list, the count of the section and the value of the filter.
function readSidebar(page) {
  return page.evaluate(() => {
    const root = document.querySelector('#board-sidebar');
    const rowOf = (row) => ({
      id: row.dataset.workspaceRow,
      name: row.querySelector('a[href^="/board/"] .truncate')?.textContent.trim() ?? '',
    });
    const rowsIn = (scope) => [...scope.querySelectorAll('[data-workspace-row]')].map(rowOf);
    const groups = [...root.querySelectorAll('[data-group-id]')].map((group) => {
      const header = group.querySelector('button[aria-expanded]:not([data-menu-trigger])');
      return {
        id: group.dataset.groupId,
        name: (header?.textContent ?? '').replace(/^\s*[▾▸]\s*/, '').trim(),
        collapsed: group.dataset.groupCollapsed === 'true',
        count: group.dataset.groupCount,
        headerCount: group.querySelector(':scope > div > .sidebar-count')?.textContent.trim() ?? '',
        color: header?.querySelector('[data-workspace-color]')?.dataset.workspaceColor ?? '',
        attention: group.querySelector('[data-group-attention]') !== null,
        locked: header?.disabled === true,
        rows: rowsIn(group),
      };
    });
    const pinnedSection = root.querySelector('[data-pinned-section]');
    const firstGroup = root.querySelector('[data-group-id]');
    const rows = [...root.querySelectorAll('[data-workspace-row]')];
    return {
      pinned: pinnedSection ? rowsIn(pinnedSection) : null,
      pinnedBeforeGroups: pinnedSection && firstGroup
        ? Boolean(pinnedSection.compareDocumentPosition(firstGroup) & Node.DOCUMENT_POSITION_FOLLOWING)
        : null,
      groups,
      ungrouped: rows.filter((row) => !row.closest('[data-group-id]') && !row.closest('[data-pinned-section]')).map(rowOf),
      attention: [...root.querySelectorAll('[data-attention-task]')].map((el) => el.dataset.attentionTitle),
      sectionCount: root.querySelector('.sidebar-count')?.textContent.trim() ?? '',
      filter: root.querySelector('#board-sidebar-filter')?.value ?? null,
      text: root.textContent,
    };
  });
}

// Every workspace row the sidebar renders, pinned, grouped or not.
const allRows = (s) => [...(s.pinned ?? []), ...s.groups.flatMap((g) => g.rows), ...s.ungrouped];
const rowCount = (s, id) => allRows(s).filter((r) => r.id === id).length;
const groupById = (s, id) => s.groups.find((g) => g.id === id);
const groupByName = (s, name) => s.groups.find((g) => g.name === name);

// A "…" menu: the trigger and the panel of the first menu inside scope (a workspace row or a group).
function menuOf(scope) {
  const menu = scope.locator('[data-menu]').first();
  return { trigger: menu.locator('[data-menu-trigger]'), panel: menu.locator('[data-menu-panel]') };
}

async function openMenu(scope) {
  const { trigger, panel } = menuOf(scope);
  await trigger.click();
  await panel.waitFor({ state: 'visible', timeout: 5000 });
  return panel;
}

const workspaceRow = (page, id) => page.locator(`#board-sidebar [data-workspace-row="${id}"]`).first();
const groupEl = (page, id) => page.locator(`#board-sidebar [data-group-id="${id}"]`);
const groupToggle = (page, id) => groupEl(page, id).locator('button[aria-expanded]:not([data-menu-trigger])').first();
const filterInput = (page) => page.locator('#board-sidebar #board-sidebar-filter');

// Re-renders the sidebar the way the periodic refreshes do (SSE and board:refresh both land on this event).
const refreshSidebar = (page) =>
  page.evaluate(() => document.body.dispatchEvent(new CustomEvent('board:refresh', { bubbles: true })));

// Answers the next htmx prompt (window.prompt) with `answer` while action runs. The harness accepts every dialog
// without an answer, so its listener is swapped for the time of the action and put back afterwards.
async function answerPrompt(ctx, answer, action) {
  const { page } = ctx;
  const seen = [];
  const handler = (dialog) => {
    seen.push(dialog.type());
    dialog.accept(answer);
  };
  page.removeAllListeners('dialog');
  page.on('dialog', handler);
  try {
    await action();
    await until(async () => seen.length, (n) => n > 0, { timeout: 8000 });
  } finally {
    page.off('dialog', handler);
    page.on('dialog', (dialog) => dialog.accept());
  }
  ctx.assert(seen.length === 1 && seen[0] === 'prompt', `se esperaba un prompt de htmx y llegaron ${JSON.stringify(seen)}`);
}

// Registers a workspace from the Workspaces screen and returns its name and id (read from the sidebar).
async function registerWorkspace(ctx, key) {
  const name = `${key}-grupos-${tag}`;
  const dir = gitRepo(key);
  await ctx.goto('/workspaces/new');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  return { key, name, dir };
}

// Starts a task in the workspace with the default agent (the fake one): it goes idle and waits for the user.
async function startTask(ctx, ws, prompt) {
  const { page } = ctx;
  await ctx.goto('/');
  await page.locator('#board-sidebar').locator(`a[href="/board/${ws.id}"]`).first().click();
  await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  const form = page.locator('form:has(select[name=workspace_id]):visible');
  await form.locator('select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
  await form.locator('textarea[name=prompt]').fill(prompt);
  await page.getByRole('checkbox', { name: /Iniciar ahora/ }).first().check();
  await form.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
  await form.waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});
}

module.exports = {
  name: 'workspace_groups',
  tests: [
    {
      name: 'grupos: la barra lateral crea, mueve, pliega, fija, filtra, colorea, renombra y elimina grupos',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const shot = (file) => page.screenshot({ path: path.join(SHOTS, file) });

        // ---- 0. Tres workspaces registrados ----
        const alpha = await registerWorkspace(ctx, 'alpha');
        const beta = await registerWorkspace(ctx, 'beta');
        const gamma = await registerWorkspace(ctx, 'gamma');
        await ctx.goto('/');
        let snap = await until(() => readSidebar(page), (s) => [alpha, beta, gamma].every((w) => allRows(s).some((r) => r.name === w.name)));
        for (const ws of [alpha, beta, gamma]) {
          ws.id = allRows(snap).find((r) => r.name === ws.name)?.id;
          ctx.assert(ws.id, `la barra lateral debe listar ${ws.name}`);
          ctx.assert(snap.ungrouped.some((r) => r.id === ws.id), `${ws.name} empieza sin grupo`);
        }

        // ---- 1. S1: crear un grupo con el "+" (prompt de htmx) ----
        const groupName = `Grupo ${tag}`;
        await answerPrompt(ctx, groupName, () => page.locator('#board-sidebar').getByRole('button', { name: /Nuevo grupo|New group/ }).click());
        snap = await until(() => readSidebar(page), (s) => Boolean(groupByName(s, groupName)));
        const group = groupByName(snap, groupName);
        ctx.assert(group, `el grupo "${groupName}" debe aparecer en la barra lateral`);
        ctx.assert(group.count === '0' && group.rows.length === 0, `grupo nuevo vacío: ${JSON.stringify(group)}`);
        ctx.assert(group.collapsed === false, 'un grupo nuevo empieza desplegado');
        ctx.assert(group.color === '', 'un grupo nuevo empieza sin color');

        // ---- 2. S2: mover workspaces desde su "…" y sacarlos de nuevo ----
        const moveTo = async (ws, target) => {
          const panel = await openMenu(workspaceRow(page, ws.id));
          await panel.getByRole('button', { name: `Mover a ${target.name}`, exact: true }).click();
          return until(() => readSidebar(page), (s) => groupById(s, target.id)?.rows.some((r) => r.id === ws.id));
        };
        snap = await moveTo(alpha, group);
        ctx.assert(groupById(snap, group.id).rows.map((r) => r.id).includes(alpha.id), 'alpha debe quedar dentro del grupo');
        ctx.assert(groupById(snap, group.id).count === '1', `contador tras mover alpha: ${groupById(snap, group.id).count}`);
        ctx.assert(!snap.ungrouped.some((r) => r.id === alpha.id), 'alpha ya no debe aparecer suelta');
        ctx.assert(rowCount(snap, alpha.id) === 1, 'alpha debe aparecer una sola vez');

        snap = await moveTo(beta, group);
        ctx.assert(groupById(snap, group.id).count === '2', `contador tras mover beta: ${groupById(snap, group.id).count}`);

        // Back to no group: "Quitar del grupo" puts beta in the ungrouped rows, then it is moved again.
        {
          const panel = await openMenu(workspaceRow(page, beta.id));
          await panel.getByRole('button', { name: 'Quitar del grupo', exact: true }).click();
          snap = await until(() => readSidebar(page), (s) => s.ungrouped.some((r) => r.id === beta.id));
          ctx.assert(groupById(snap, group.id).count === '1', `contador tras quitar beta: ${groupById(snap, group.id).count}`);
          ctx.assert(!groupById(snap, group.id).rows.some((r) => r.id === beta.id), 'beta ya no debe estar en el grupo');
          snap = await moveTo(beta, group);
          ctx.assert(groupById(snap, group.id).count === '2', 'beta vuelve a entrar en el grupo');
        }

        // ---- 3. S5: una tarea que necesita atención en uno de sus workspaces ----
        const attentionPrompt = `atencion-grupos-${tag}`;
        await startTask(ctx, alpha, attentionPrompt);
        snap = await until(() => readSidebar(page), (s) => s.attention.some((t) => t.includes(attentionPrompt)), { timeout: 40000 });
        ctx.assert(snap.attention.some((t) => t.includes(attentionPrompt)), 'la tarea debe aparecer en "Requieren atención"');
        ctx.assert(groupById(snap, group.id).attention === false, 'un grupo desplegado no muestra la marca de atención');

        // ---- 4. S4 + S5: plegar, contador y marca; recargar y comprobar que persiste ----
        await groupToggle(page, group.id).click();
        snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.collapsed === true);
        const folded = groupById(snap, group.id);
        ctx.assert(folded.collapsed === true, 'el grupo debe quedar plegado');
        ctx.assert(folded.rows.length === 0, 'un grupo plegado no muestra sus filas');
        ctx.assert(folded.count === '2' && folded.headerCount === '2', `contador plegado: ${folded.count}/${folded.headerCount}`);
        ctx.assert(folded.attention === true, 'un grupo plegado con una tarea que necesita atención muestra la marca');
        await shot('workspace-groups-collapsed-attention.png');

        await ctx.goto('/');
        snap = await until(() => readSidebar(page), (s) => Boolean(groupById(s, group.id)));
        const reloaded = groupById(snap, group.id);
        ctx.assert(reloaded.collapsed === true, 'tras recargar el grupo sigue plegado (estado en el servidor)');
        ctx.assert(reloaded.rows.length === 0 && reloaded.count === '2' && reloaded.attention === true, `tras recargar: ${JSON.stringify(reloaded)}`);

        // ---- 5. S8: fijar un workspace; el bloque "Fijados" va encima de los grupos ----
        {
          const panel = await openMenu(workspaceRow(page, gamma.id));
          await panel.getByRole('button', { name: 'Fijar', exact: true }).click();
          snap = await until(() => readSidebar(page), (s) => s.pinned?.some((r) => r.id === gamma.id));
        }
        ctx.assert(snap.pinned.map((r) => r.id).join() === gamma.id, `bloque fijado: ${JSON.stringify(snap.pinned)}`);
        ctx.assert(!snap.ungrouped.some((r) => r.id === gamma.id), 'un workspace fijado no aparece suelto');
        ctx.assert(rowCount(snap, gamma.id) === 1, 'un workspace fijado aparece una sola vez');
        ctx.assert(snap.pinnedBeforeGroups === true, 'el bloque fijado va encima de los grupos');
        {
          const panel = await openMenu(workspaceRow(page, gamma.id));
          ctx.assert((await panel.getByRole('button', { name: 'Desfijar', exact: true }).count()) === 1, 'el menú de un fijado ofrece "Desfijar"');
          await filterInput(page).click(); // closes the menu
        }
        await ctx.goto('/');
        snap = await until(() => readSidebar(page), (s) => s.pinned?.some((r) => r.id === gamma.id));
        ctx.assert(snap.pinned?.some((r) => r.id === gamma.id), 'tras recargar el workspace sigue fijado');

        // ---- 6. S7: filtro por nombre ----
        await filterInput(page).fill(alpha.name);
        snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.collapsed === false);
        const filtered = groupById(snap, group.id);
        ctx.assert(filtered.collapsed === false, 'con filtro, el grupo con coincidencias se muestra desplegado');
        ctx.assert(filtered.rows.map((r) => r.id).join() === alpha.id, `coincidencia dentro del grupo plegado: ${JSON.stringify(filtered.rows)}`);
        ctx.assert(filtered.locked === true, 'con filtro, el botón de plegar del grupo está deshabilitado');
        ctx.assert(snap.pinned === null, 'con filtro, el bloque fijado sin coincidencias desaparece');
        ctx.assert(snap.ungrouped.length === 0, 'con filtro, las filas sueltas sin coincidencias desaparecen');
        ctx.assert(snap.sectionCount === '1', `el contador de la sección cuenta las coincidencias: ${snap.sectionCount}`);
        ctx.assert(snap.filter === alpha.name, `el texto del filtro: ${snap.filter}`);
        await shot('workspace-groups-filter.png');

        // Focus and typed text survive a periodic refresh, and the refresh keeps the filter.
        await filterInput(page).fill(alpha.name.slice(0, 6));
        await until(() => readSidebar(page), (s) => s.groups.some((g) => g.id === group.id && g.rows.length === 1));
        await refreshSidebar(page);
        await sleep(700);
        const afterRefresh = await readSidebar(page);
        const focus = await page.evaluate(() => ({ id: document.activeElement?.id, value: document.querySelector('#board-sidebar-filter')?.value }));
        ctx.assert(focus.id === 'board-sidebar-filter', `tras el refresco el foco sigue en el filtro: ${focus.id}`);
        ctx.assert(focus.value === alpha.name.slice(0, 6), `tras el refresco el texto escrito se conserva: ${focus.value}`);
        ctx.assert(afterRefresh.pinned === null && afterRefresh.ungrouped.length === 0, 'el refresco conserva el filtro activo');
        await page.keyboard.type(alpha.name.slice(6));
        snap = await until(() => readSidebar(page), (s) => s.filter === alpha.name && groupById(s, group.id)?.rows.length === 1);
        ctx.assert(snap.filter === alpha.name, `el texto escrito tras el refresco sigue en el campo: ${snap.filter}`);
        ctx.assert(groupById(snap, group.id).rows.map((r) => r.id).join() === alpha.id, 'tras seguir escribiendo, la coincidencia sigue visible');

        // No match: the empty line shows and nothing else.
        const noMatch = `zzz-${tag}`;
        await filterInput(page).fill(noMatch);
        snap = await until(() => readSidebar(page), (s) => s.groups.length === 0 && s.text.includes('Ningún workspace coincide.'));
        ctx.assert(snap.groups.length === 0 && snap.pinned === null && snap.ungrouped.length === 0, 'sin coincidencias no queda ninguna fila');
        ctx.assert(snap.text.includes('Ningún workspace coincide.'), 'sin coincidencias aparece la línea de vacío');
        ctx.assert(!snap.text.includes('Aún no hay workspaces.'), 'sin coincidencias no aparece el mensaje de no hay workspaces');

        // Clearing the filter restores the stored state: the group is still folded, its toggle works again.
        await filterInput(page).fill('');
        snap = await until(() => readSidebar(page), (s) => s.pinned?.some((r) => r.id === gamma.id) && groupById(s, group.id)?.collapsed === true);
        const cleared = groupById(snap, group.id);
        ctx.assert(cleared.collapsed === true, 'quitar el filtro no cambia el estado guardado: el grupo sigue plegado');
        ctx.assert(cleared.locked === false, 'sin filtro, el botón de plegar vuelve a estar habilitado');
        ctx.assert(cleared.rows.length === 0 && cleared.count === '2', 'sin filtro, el grupo plegado muestra su contador y no sus filas');

        // ---- 7. S6: color del grupo ----
        {
          const panel = await openMenu(groupEl(page, group.id));
          await panel.getByRole('button', { name: 'teal', exact: true }).click();
          snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.color === 'teal');
        }
        const colored = groupById(snap, group.id);
        ctx.assert(colored.color === 'teal', `color del grupo tras elegirlo: ${colored.color}`);
        const chipClasses = await groupEl(page, group.id).locator('[data-workspace-color="teal"]').first().getAttribute('class');
        ctx.assert(/\bws-chip\b/.test(chipClasses) && /\bws-teal\b/.test(chipClasses), `el chip del grupo lleva ws-chip ws-teal: ${chipClasses}`);
        await ctx.goto('/');
        snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.color === 'teal');
        ctx.assert(groupById(snap, group.id)?.color === 'teal', 'tras recargar el grupo conserva el color');

        // ---- 8. S1: renombrar el grupo (prompt de htmx), el id no cambia ----
        const renamed = `Equipo ${tag}`;
        {
          const panel = await openMenu(groupEl(page, group.id));
          await answerPrompt(ctx, renamed, () => panel.getByRole('button', { name: 'Renombrar grupo', exact: true }).click());
        }
        snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.name === renamed);
        ctx.assert(groupById(snap, group.id)?.name === renamed, `nombre tras renombrar: ${groupById(snap, group.id)?.name}`);
        ctx.assert(snap.groups.length === 1, 'renombrar no crea ni duplica grupos');
        await ctx.goto('/');
        snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.name === renamed);
        ctx.assert(groupById(snap, group.id)?.name === renamed, 'tras recargar el grupo conserva el nombre nuevo');

        // ---- 9. S9: los menús abiertos sobreviven a un refresco ----
        {
          const panel = await openMenu(workspaceRow(page, gamma.id));
          await refreshSidebar(page);
          await sleep(700);
          ctx.assert(await panel.isVisible(), 'el menú abierto de un fijado sigue abierto tras el refresco');
          const expanded = await page.locator(`#board-sidebar [data-workspace-row="${gamma.id}"] [data-menu-trigger]`).first().getAttribute('aria-expanded');
          ctx.assert(expanded === 'true', `aria-expanded del disparador tras el refresco: ${expanded}`);
          await filterInput(page).click();
          ctx.assert(!(await panel.isVisible()), 'un clic fuera cierra el menú');
        }
        // Expanding the group works and persists (S4 in the other direction).
        await groupToggle(page, group.id).click();
        snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.collapsed === false && groupById(s, group.id)?.rows.length === 2);
        ctx.assert(groupById(snap, group.id).rows.map((r) => r.id).sort().join() === [alpha.id, beta.id].sort().join(), 'al desplegar aparecen los dos workspaces del grupo');
        await ctx.goto('/');
        snap = await until(() => readSidebar(page), (s) => groupById(s, group.id)?.collapsed === false);
        ctx.assert(groupById(snap, group.id)?.collapsed === false, 'tras recargar el grupo sigue desplegado');
        await shot('workspace-groups-expanded.png');

        // ---- 10. S1: eliminar el grupo; sus workspaces vuelven a "sin grupo" ----
        {
          const panel = await openMenu(groupEl(page, group.id));
          await panel.getByRole('button', { name: 'Eliminar grupo', exact: true }).click();
          snap = await until(() => readSidebar(page), (s) => !groupById(s, group.id));
        }
        ctx.assert(!groupById(snap, group.id), 'el grupo eliminado desaparece');
        ctx.assert(snap.groups.length === 0, 'no queda ningún grupo');
        ctx.assert(snap.ungrouped.some((r) => r.id === alpha.id) && snap.ungrouped.some((r) => r.id === beta.id), 'alpha y beta vuelven a estar sin grupo');
        ctx.assert(rowCount(snap, alpha.id) === 1 && rowCount(snap, beta.id) === 1, 'cada workspace aparece una sola vez tras eliminar');
        ctx.assert(snap.pinned?.some((r) => r.id === gamma.id), 'eliminar un grupo no toca los fijados');
        await ctx.goto('/');
        snap = await until(() => readSidebar(page), (s) => s.ungrouped.some((r) => r.id === alpha.id));
        ctx.assert(snap.groups.length === 0, 'tras recargar no hay grupos');
        ctx.assert(snap.ungrouped.some((r) => r.id === beta.id), 'tras recargar beta sigue sin grupo');

        // Problems the browser reported (uncaught errors, HTTP failures) fail the test.
        const problems = ctx.getProblems().filter((p) => p.startsWith('[pageerror]') || p.startsWith('[http]'));
        ctx.assert(problems.length === 0, `problemas de página o HTTP:\n${problems.join('\n')}`);
        const saved = fs.readdirSync(SHOTS).filter((f) => f.startsWith('workspace-groups-')).sort();
        for (const f of saved) console.log(`    screenshot: ${path.join(SHOTS, f)}`);
      },
    },
  ],
};
