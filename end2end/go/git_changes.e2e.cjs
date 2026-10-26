// end2end/go/git_changes.e2e.cjs
// Go-mode copy of end2end/git_changes_smoke.cjs: the Cambios tab of a live task (stage, unstage, commit).
// Same flow and assertions as the Rust smoke. The effects are checked on the real index of the task
// worktree (`git diff --cached`) and on the files on disk, not on the UI state.
// Only two things are Rust-specific and are replaced here: the runtime change is saved by
// POST /agent-settings/runtime (not the update_agent_settings server function), and the task is
// opened from the board flow used by task_composer.e2e.cjs. The fake pi agent (tests/fixtures/smoke_pi.sh)
// writes CAMBIO.md in its worktree; the suite runs in Spanish (the language cookie of the harness).
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const tag = Date.now().toString();
const WORKTREES = process.env.SMOKE_WORKTREES;
// Repository registered as the workspace of this module (run_go.sh creates it for this script).
const REPO = process.env.REPO_GIT_CHANGES;
const WORKSPACE = 'git-changes-demo';
// A little longer than the polling period of the panel (GIT_POLL_SECONDS = 5 s).
const POLL_WAIT_MS = 6500;

const sameList = (ctx, actual, expected, what) => ctx.assert(
  JSON.stringify(actual) === JSON.stringify(expected),
  `${what}: esperado ${JSON.stringify(expected)}, obtenido ${JSON.stringify(actual)}`,
);
const git = (wt, ...args) => execFileSync('git', ['-C', wt, ...args], { encoding: 'utf8' }).trim();
const staged = (wt) => git(wt, 'diff', '--cached', '--name-only').split('\n').filter(Boolean).sort();

// Worktree of this module's repository that the fake agent wrote CAMBIO.md into. The repository is
// private to this module, so its worktrees list (git worktree list) names the task worktree exactly.
async function findWorktree(ctx) {
  const deadline = Date.now() + 15000;
  const root = fs.realpathSync(WORKTREES);
  while (Date.now() < deadline) {
    const listed = git(REPO, 'worktree', 'list', '--porcelain')
      .split('\n')
      .filter((line) => line.startsWith('worktree '))
      .map((line) => line.slice('worktree '.length))
      .filter((wt) => wt !== fs.realpathSync(REPO) && wt.startsWith(root + path.sep));
    const found = listed.find((wt) => fs.existsSync(path.join(wt, 'CAMBIO.md')));
    if (found) return found;
    await ctx.page.waitForTimeout(200);
  }
  return null;
}

// Registers the workspace, opens the board, picks the pi runtime, creates a task that starts at once and
// waits for the agent to answer. Returns the id of the task.
async function liveTask(ctx) {
  const { page } = ctx;
  const prompt = `cambios-git-${tag}`;
  await ctx.goto('/workspaces');
  await page.getByLabel('Nombre').first().fill(WORKSPACE);
  await page.getByLabel(/^Ruta absoluta/).first().fill(REPO);
  await page.getByLabel(/Rama base/).first().fill('main');
  await page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await page.getByText(REPO).first().waitFor({ timeout: 12000 });

  await ctx.goto('/');
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  // The runtime is saved by the agent bar (POST /agent-settings/runtime) before the task is created.
  const saved = page.waitForResponse(
    (r) => r.request().method() === 'POST' && new URL(r.url()).pathname === '/agent-settings/runtime',
    { timeout: 10000 },
  );
  await page.getByLabel(/^Agente/).first().selectOption('pi');
  await saved;
  // Choosing the workspace refreshes the list of the board. The board may already show it (it is the only
  // workspace of a fresh database): a select on the same option fires no change, so no refresh is awaited then.
  const workspaceChoice = page.locator('[id^="board-controls-"]:visible select[name=workspace]');
  const label = `${WORKSPACE} (main)`;
  if ((await workspaceChoice.evaluate((s) => s.selectedOptions[0]?.textContent)) !== label) {
    const listed = page.waitForResponse((r) => r.url().includes('/board/frame'), { timeout: 10000 });
    await workspaceChoice.selectOption({ label });
    await listed;
  }
  await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${WORKSPACE} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  // Navigating before the create -> start chain ends cancels the start (as task_composer does): wait for the modal to close.
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  const link = page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  const id = ((await link.getAttribute('href')) || '').match(/\/tasks\/([0-9a-f-]{36})/)?.[1];
  ctx.assert(id, 'no se obtuvo el id de la tarea creada');
  await ctx.goto(`/tasks/${id}`);
  await page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
  // The agent's turn must be settled before the worktree is edited under it.
  await page.getByText('Esperando', { exact: true }).first().waitFor({ timeout: 20000 });
  return id;
}

module.exports = {
  name: 'git_changes',
  tests: [
    {
      name: 'Cambios: stage, unstage y commit reflejan el índice real del worktree de la tarea',
      fn: async (ctx) => {
        const { page } = ctx;
        ctx.assert(WORKTREES, 'falta SMOKE_WORKTREES (directorio de worktrees del servidor)');
        ctx.assert(REPO, 'falta REPO_GIT_CHANGES (repositorio del módulo git_changes)');
        await liveTask(ctx);
        const worktree = await findWorktree(ctx);
        ctx.assert(worktree, `no hay worktree con CAMBIO.md bajo ${WORKTREES} para ${REPO}`);

        const panel = () => page.locator('[data-testid="git-changes"]').locator('visible=true').first();
        const section = (name) => panel().locator(`[data-testid="git-section-${name}"]`);
        const row = (name, file) => section(name).locator(`li[data-testid="git-row"][data-path="${file}"]`);
        const paths = (name) => section(name).locator('li[data-testid="git-row"]').evaluateAll((els) => els.map((e) => e.dataset.path).sort());
        // The UI refreshes after each action and every 5 s: wait until it shows the expected state.
        const until = async (name, expected, what) => {
          const deadline = Date.now() + 15000;
          let last;
          while (Date.now() < deadline) {
            last = await paths(name).catch(() => null);
            if (last && JSON.stringify(last) === JSON.stringify(expected)) return;
            await page.waitForTimeout(150);
          }
          throw new Error(`${what}: esperado ${JSON.stringify(expected)} en «${name}», obtenido ${JSON.stringify(last)}`);
        };

        // Real initial state: README.md modified (tracked), CAMBIO.md and nuevo.txt untracked.
        fs.writeFileSync(path.join(worktree, 'README.md'), 'hola\ncambiado\n');
        fs.writeFileSync(path.join(worktree, 'nuevo.txt'), 'nuevo\n');
        await page.getByRole('tab', { name: 'Cambios', exact: true }).click();
        await until('unstaged', ['README.md'], 'modificado sin stage');
        await until('untracked', ['CAMBIO.md', 'nuevo.txt'], 'archivos sin seguimiento');
        ctx.assert((await section('staged').count()) === 0, 'sin stage no debe haber sección «Con stage»');
        ctx.assert((await row('unstaged', 'README.md').innerText()).includes('M'), 'la fila muestra la letra de estado M');
        ctx.assert((await row('untracked', 'nuevo.txt').innerText()).includes('U'), 'la fila muestra la letra de estado U');
        sameList(ctx, staged(worktree), [], 'índice inicial');
        if (process.env.SMOKE_SCREENSHOT_CHANGES) await ctx.screenshot(process.env.SMOKE_SCREENSHOT_CHANGES);

        // One file: moves to «Con stage» and only that file enters the index.
        await row('unstaged', 'README.md').locator('[data-testid="git-toggle"]').click();
        await until('staged', ['README.md'], 'README.md con stage');
        await until('unstaged', [], 'README.md sale de «Cambios»');
        sameList(ctx, staged(worktree), ['README.md'], 'índice tras stage de un fichero');

        // Whole untracked section.
        await section('untracked').locator('[data-testid="git-section-toggle"]').click();
        await until('staged', ['CAMBIO.md', 'README.md', 'nuevo.txt'], 'todo con stage');
        ctx.assert((await section('untracked').count()) === 0, 'la sección sin seguimiento desaparece al quedar vacía');
        sameList(ctx, staged(worktree), ['CAMBIO.md', 'README.md', 'nuevo.txt'], 'índice tras stage de la sección');

        // Unstage one file: back to untracked, out of the index, and NOT deleted from disk.
        await row('staged', 'nuevo.txt').locator('[data-testid="git-toggle"]').click();
        await until('untracked', ['nuevo.txt'], 'nuevo.txt vuelve a sin seguimiento');
        sameList(ctx, staged(worktree), ['CAMBIO.md', 'README.md'], 'índice tras quitar nuevo.txt');
        ctx.assert(fs.readFileSync(path.join(worktree, 'nuevo.txt'), 'utf8') === 'nuevo\n', 'quitar del stage no debe tocar el contenido');

        // Switching tabs keeps the panel.
        await page.getByRole('tab', { name: 'Acciones', exact: true }).click();
        await page.getByRole('tab', { name: 'Cambios', exact: true }).click();
        await until('staged', ['CAMBIO.md', 'README.md'], 'el panel conserva su contenido al cambiar de pestaña');

        // Edit after staging: the file shows in both sections (partial stage).
        fs.writeFileSync(path.join(worktree, 'README.md'), 'hola\ncambiado\notra vez\n');
        await until('unstaged', ['README.md'], 'README.md editado tras el stage aparece en «Cambios»');
        await until('staged', ['CAMBIO.md', 'README.md'], 'y sigue en «Con stage»');

        // Unstage everything: the index is empty and the content intact.
        await section('staged').locator('[data-testid="git-section-toggle"]').click();
        await until('staged', [], 'nada con stage');
        sameList(ctx, staged(worktree), [], 'índice tras quitar todo');
        ctx.assert(fs.readFileSync(path.join(worktree, 'README.md'), 'utf8') === 'hola\ncambiado\notra vez\n', 'quitar todo no toca el contenido');
        await until('unstaged', ['README.md'], 'README.md modificado sin stage');
        await until('untracked', ['CAMBIO.md', 'nuevo.txt'], 'sin seguimiento tras quitar todo');

        // Clicking a row opens the file (editor) in its own tab.
        await row('unstaged', 'README.md').locator('a').click();
        await page.locator('.dock-tab', { hasText: 'README.md' }).first().waitFor({ timeout: 15000 });
        await page.getByText('cambiado', { exact: false }).first().waitFor({ timeout: 15000 });
        await page.getByRole('tab', { name: 'Cambios', exact: true }).click();
        await until('unstaged', ['README.md'], 'el panel sigue mostrando README.md tras volver');

        // Commit: only with something staged and a message; it records only what is staged.
        const form = () => panel().locator('[data-testid="git-commit-form"]');
        // The commit form stays in the DOM and is hidden by CSS while nothing is staged (git_panel.templ).
        ctx.assert(!(await form().isVisible()), 'sin nada en stage no hay formulario de commit');
        await row('unstaged', 'README.md').locator('[data-testid="git-toggle"]').click();
        await until('staged', ['README.md'], 'README.md con stage para el commit');
        const message = () => form().locator('[data-testid="git-commit-message"]');
        const commitButton = () => form().locator('[data-testid="git-commit"]');
        await message().waitFor();
        // The Go button is never disabled: the message is `required`, so the browser refuses an empty commit.
        ctx.assert(await message().evaluate((el) => el.validity.valueMissing), 'sin mensaje el campo es obligatorio');
        await message().fill('docs: actualizar README');
        await message().focus();
        // A poll must not rebuild the form: text and focus are kept.
        await page.waitForTimeout(POLL_WAIT_MS);
        ctx.assert((await message().inputValue()) === 'docs: actualizar README', 'el sondeo no borra el mensaje');
        ctx.assert(await message().evaluate((el) => el === document.activeElement), 'el sondeo no quita el foco al mensaje');
        ctx.assert(!(await commitButton().isDisabled()), 'con mensaje el botón Commit está activo');
        const before = git(worktree, 'rev-list', '--count', 'HEAD');
        await commitButton().click();
        await until('staged', [], 'el commit vacía «Con stage»');
        await until('unstaged', [], 'README.md ya no está modificado');
        ctx.assert(git(worktree, 'log', '-1', '--format=%s') === 'docs: actualizar README', 'el commit lleva el mensaje escrito');
        ctx.assert(Number(git(worktree, 'rev-list', '--count', 'HEAD')) === Number(before) + 1, 'se crea exactamente un commit');
        sameList(ctx, git(worktree, 'show', '--name-only', '--format=', 'HEAD').split('\n').filter(Boolean), ['README.md'], 'ficheros del commit');
        await until('untracked', ['CAMBIO.md', 'nuevo.txt'], 'lo que no estaba en stage queda sin tocar');
        await form().waitFor({ state: 'hidden', timeout: 15000 });

        // A clean worktree shows the empty state.
        git(worktree, 'checkout', '--', 'README.md');
        fs.rmSync(path.join(worktree, 'nuevo.txt'));
        fs.rmSync(path.join(worktree, 'CAMBIO.md'));
        await panel().locator('[data-testid="git-empty"]').waitFor({ timeout: 15000 });

        if (process.env.SMOKE_SCREENSHOT) await ctx.screenshot(process.env.SMOKE_SCREENSHOT);
        // htmx logs every aborted request as console.error (`htmx:afterRequest` + `htmx:sendAbort`, target a div):
        // the poll of the changes panel is cut off when a tab switch removes it mid-request. Those are not errors of the
        // page and carry no HTTP status, so only they are ignored; any other console, page or HTTP problem fails the test.
        const problems = ctx.getProblems().filter((p) => !/htmx:(afterRequest|sendAbort), \[object HTMLDivElement\]/.test(p));
        ctx.assert(problems.length === 0, `problemas de consola, página o HTTP:\n${problems.join('\n')}`);
      },
    },
  ],
};
