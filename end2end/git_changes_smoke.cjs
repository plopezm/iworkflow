// Smoke de la pestaña "Cambios" (stage/unstage) del panel de la tarea con Playwright contra el
// servidor lanzado por end2end/run_smoke.sh. Comprueba efectos observables: el índice real de
// git del worktree (`git diff --cached`) y los ficheros en disco, no el estado interno de la UI.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
const WORKTREES = process.env.SMOKE_WORKTREES;
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const sameList = (actual, expected, what) => assert(
  JSON.stringify(actual) === JSON.stringify(expected),
  `${what}: esperado ${JSON.stringify(expected)}, obtenido ${JSON.stringify(actual)}`,
);
// Un poco más que el periodo de sondeo del panel (GIT_POLL_SECONDS = 5 s).
const POLL_WAIT_MS = 6500;
let page;

// Directorio (hasta 4 niveles) bajo `root` que contiene `marker`: el worktree de la tarea.
function findWorktree(root, marker, depth = 4) {
  if (depth < 0 || !fs.existsSync(root)) return null;
  if (fs.existsSync(path.join(root, marker))) return root;
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    if (!entry.isDirectory() || entry.name === '.git') continue;
    const found = findWorktree(path.join(root, entry.name), marker, depth - 1);
    if (found) return found;
  }
  return null;
}

(async () => {
  assert(WORKTREES, 'falta SMOKE_WORKTREES (directorio de worktrees del servidor del smoke)');
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  // Sin cookie de idioma la app pide elegirlo; los textos de este smoke están en español.
  await context.addCookies([{ name: 'iw-lang', value: 'es', url: BASE }]);
  page = await context.newPage();
  const problems = [];
  page.on('console', m => { if ((m.type() === 'error' || m.type() === 'warning') && !/reading a resource in `hydrate`/.test(m.text())) problems.push(`${m.type()}: ${m.text()}`); });
  page.on('pageerror', e => problems.push(`pageerror: ${e.message}`));
  page.on('dialog', d => d.accept());
  page.on('response', r => { if (r.status() >= 400) problems.push(`HTTP ${r.status()} ${r.url()}`); });
  const step = (s) => console.log('✔', s);

  const panel = () => page.locator('[data-testid="git-changes"]').locator('visible=true').first();
  const section = (name) => panel().locator(`[data-testid="git-section-${name}"]`);
  const row = (name, file) => section(name).locator(`li[data-testid="git-row"][data-path="${file}"]`);
  const paths = (name) => section(name).locator('li[data-testid="git-row"]').evaluateAll(els => els.map(e => e.dataset.path).sort());
  // Espera a que la UI (que refresca tras cada acción y cada 5 s) refleje el estado esperado.
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
  const git = (wt, ...args) => execFileSync('git', ['-C', wt, ...args], { encoding: 'utf8' }).trim();
  const staged = (wt) => git(wt, 'diff', '--cached', '--name-only').split('\n').filter(Boolean).sort();

  await page.goto(BASE + '/workspaces');
  await page.waitForLoadState('networkidle');
  await page.getByLabel('Nombre').fill('git-changes-demo');
  await page.getByLabel(/^Ruta absoluta/).fill(process.env.SMOKE_REPO);
  await page.getByRole('button', { name: 'Registrar' }).click();
  await page.getByText(process.env.SMOKE_REPO).waitFor();
  step('workspace registrado desde la UI');

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  const runtime = page.getByLabel(/^Agente/);
  const saved = page.waitForResponse(r => r.url().includes('update_agent_settings'));
  await runtime.selectOption('pi');
  await saved;
  await page.getByRole('button', { name: 'Nueva tarea' }).click();
  await page.getByLabel(/^Workspace/).selectOption({ label: 'git-changes-demo (main)' });
  await page.getByLabel('Prompt').fill('Cambios git');
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.locator('aside').getByRole('link', { name: /git-changes-demo/ }).first().click();
  await page.getByRole('link', { name: /Cambios git/ }).first().click();
  await page.waitForURL(/\/tasks\//);
  await page.getByText('eco: Cambios git').waitFor();
  const worktree = findWorktree(WORKTREES, 'CAMBIO.md');
  assert(worktree, `no hay worktree con CAMBIO.md bajo ${WORKTREES}`);
  step('tarea en vivo; el agente dejó CAMBIO.md en su worktree');

  // Estado inicial real: README.md modificado (tracked), CAMBIO.md y nuevo.txt sin seguimiento.
  fs.writeFileSync(path.join(worktree, 'README.md'), 'hola\ncambiado\n');
  fs.writeFileSync(path.join(worktree, 'nuevo.txt'), 'nuevo\n');
  await page.getByRole('tab', { name: 'Cambios', exact: true }).click();
  await until('unstaged', ['README.md'], 'modificado sin stage');
  await until('untracked', ['CAMBIO.md', 'nuevo.txt'], 'archivos sin seguimiento');
  assert((await section('staged').count()) === 0, 'sin stage no debe haber sección «Con stage»');
  assert((await row('unstaged', 'README.md').innerText()).includes('M'), 'la fila muestra la letra de estado M');
  assert((await row('untracked', 'nuevo.txt').innerText()).includes('U'), 'la fila muestra la letra de estado U');
  sameList(staged(worktree), [], 'índice inicial');
  step('pestaña Cambios: modificados y sin seguimiento en su sección, nada con stage');
  if (process.env.SMOKE_SCREENSHOT_CHANGES) await page.screenshot({ path: process.env.SMOKE_SCREENSHOT_CHANGES, fullPage: true });

  // Un fichero: sube a «Con stage» y solo ese entra en el índice.
  await row('unstaged', 'README.md').locator('[data-testid="git-toggle"]').click();
  await until('staged', ['README.md'], 'README.md con stage');
  await until('unstaged', [], 'README.md sale de «Cambios»');
  sameList(staged(worktree), ['README.md'], 'índice tras stage de un fichero');
  step('stage de un fichero: solo README.md entra en el índice');

  // Sección entera de sin seguimiento.
  await section('untracked').locator('[data-testid="git-section-toggle"]').click();
  await until('staged', ['CAMBIO.md', 'README.md', 'nuevo.txt'], 'todo con stage');
  assert((await section('untracked').count()) === 0, 'la sección sin seguimiento desaparece al quedar vacía');
  sameList(staged(worktree), ['CAMBIO.md', 'README.md', 'nuevo.txt'], 'índice tras stage de la sección');
  step('stage de toda la sección sin seguimiento');

  // Quitar un fichero: vuelve a sin seguimiento, sale del índice y NO se borra del disco.
  await row('staged', 'nuevo.txt').locator('[data-testid="git-toggle"]').click();
  await until('untracked', ['nuevo.txt'], 'nuevo.txt vuelve a sin seguimiento');
  sameList(staged(worktree), ['CAMBIO.md', 'README.md'], 'índice tras quitar nuevo.txt');
  assert(fs.readFileSync(path.join(worktree, 'nuevo.txt'), 'utf8') === 'nuevo\n', 'quitar del stage no debe tocar el contenido');
  step('quitar del stage devuelve el fichero a sin seguimiento sin borrarlo');

  // Cambiar de pestaña conserva el panel.
  await page.getByRole('tab', { name: 'Acciones', exact: true }).click();
  await page.getByRole('tab', { name: 'Cambios', exact: true }).click();
  await until('staged', ['CAMBIO.md', 'README.md'], 'el panel conserva su contenido al cambiar de pestaña');
  step('cambiar de pestaña conserva el panel');

  // Editar tras el stage: el fichero queda en ambas secciones (stage parcial).
  fs.writeFileSync(path.join(worktree, 'README.md'), 'hola\ncambiado\notra vez\n');
  await until('unstaged', ['README.md'], 'README.md editado tras el stage aparece en «Cambios»');
  await until('staged', ['CAMBIO.md', 'README.md'], 'y sigue en «Con stage»');
  step('fichero con stage parcial en ambas secciones');

  // Quitar todo del stage: el índice queda vacío y el contenido intacto.
  await section('staged').locator('[data-testid="git-section-toggle"]').click();
  await until('staged', [], 'nada con stage');
  sameList(staged(worktree), [], 'índice tras quitar todo');
  assert(fs.readFileSync(path.join(worktree, 'README.md'), 'utf8') === 'hola\ncambiado\notra vez\n', 'quitar todo no toca el contenido');
  await until('unstaged', ['README.md'], 'README.md modificado sin stage');
  await until('untracked', ['CAMBIO.md', 'nuevo.txt'], 'sin seguimiento tras quitar todo');
  step('quitar todo del stage deja el índice vacío y el contenido intacto');

  // Clic en una fila: abre el fichero (editor) en su propia pestaña.
  await row('unstaged', 'README.md').locator('a').click();
  await page.locator('.dock-tab', { hasText: 'README.md' }).first().waitFor({ timeout: 15000 });
  await page.getByText('cambiado', { exact: false }).first().waitFor({ timeout: 15000 });
  step('clic en una fila abre el fichero en una pestaña');
  await page.getByRole('tab', { name: 'Cambios', exact: true }).click();
  await until('unstaged', ['README.md'], 'el panel sigue mostrando README.md tras volver');

  // Commit: solo con algo en stage y con mensaje; registra únicamente lo que está en stage.
  const form = () => panel().locator('[data-testid="git-commit-form"]');
  assert((await form().count()) === 0, 'sin nada en stage no hay formulario de commit');
  await row('unstaged', 'README.md').locator('[data-testid="git-toggle"]').click();
  await until('staged', ['README.md'], 'README.md con stage para el commit');
  const message = () => form().locator('[data-testid="git-commit-message"]');
  const commitButton = () => form().locator('[data-testid="git-commit"]');
  await message().waitFor();
  assert(await commitButton().isDisabled(), 'sin mensaje el botón Commit está desactivado');
  await message().fill('docs: actualizar README');
  await message().focus();
  // Un sondeo no debe reconstruir el formulario: se conservan texto y foco.
  await page.waitForTimeout(POLL_WAIT_MS);
  assert((await message().inputValue()) === 'docs: actualizar README', 'el sondeo no borra el mensaje');
  assert(await message().evaluate(el => el === document.activeElement), 'el sondeo no quita el foco al mensaje');
  assert(!(await commitButton().isDisabled()), 'con mensaje el botón Commit está activo');
  const before = git(worktree, 'rev-list', '--count', 'HEAD');
  await commitButton().click();
  await until('staged', [], 'el commit vacía «Con stage»');
  await until('unstaged', [], 'README.md ya no está modificado');
  assert(git(worktree, 'log', '-1', '--format=%s') === 'docs: actualizar README', 'el commit lleva el mensaje escrito');
  assert(Number(git(worktree, 'rev-list', '--count', 'HEAD')) === Number(before) + 1, 'se crea exactamente un commit');
  sameList(git(worktree, 'show', '--name-only', '--format=', 'HEAD').split('\n').filter(Boolean), ['README.md'], 'ficheros del commit');
  await until('untracked', ['CAMBIO.md', 'nuevo.txt'], 'lo que no estaba en stage queda sin tocar');
  assert((await form().count()) === 0, 'tras el commit desaparece el formulario');
  step('commit: solo lo que estaba en stage, con su mensaje, y el formulario conserva foco entre sondeos');

  // Un worktree limpio muestra el estado vacío.
  git(worktree, 'checkout', '--', 'README.md');
  fs.rmSync(path.join(worktree, 'nuevo.txt'));
  fs.rmSync(path.join(worktree, 'CAMBIO.md'));
  await panel().locator('[data-testid="git-empty"]').waitFor({ timeout: 15000 });
  step('worktree limpio: estado vacío');

  if (process.env.SMOKE_SCREENSHOT) await page.screenshot({ path: process.env.SMOKE_SCREENSHOT, fullPage: true });
  if (problems.length) { console.error('PROBLEMAS:\n' + problems.join('\n')); process.exit(1); }
  console.log('sin errores de consola');
  await browser.close();
})().catch(async e => {
  console.error('FALLO:', e.message);
  if (page) {
    console.error('URL:', page.url());
    console.error((await page.locator('body').innerText().catch(() => '')).slice(0, 1500));
  }
  process.exit(1);
});
