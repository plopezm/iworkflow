// end2end/related_projects.e2e.cjs
// Real-agent run of the related projects feature (odd/tasks/related-projects.md): a project related from the actions
// tab of a task whose claude agent is live. The agent restarts with --add-dir and is told about the project; the @
// completion of the chat offers the project's files; the agent reads the referenced file. It needs the real claude
// binary and the login of the host, so it is NOT part of the fake-agent suite. Run it only through
// end2end/go/related_projects_real.sh, which starts the server with IWF_CLAUDE_BIN pointing at claude, an isolated
// HOME and a Haiku model. Prompts are tiny and the tasks run one at a time to keep the model usage low.
//
// The page shows a turn only when it settles: while the agent works, the transcript keeps showing the state of the last
// settled turn. So the waits below look for what the transcript shows once a turn is over, never for a running label.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SERVER_LOG = process.env.IWF_E2E_SERVER_LOG || '';
const RUN_DIR = process.env.IWF_E2E_RUN_DIR || '';
// A real model turn takes seconds, not milliseconds: every wait for an agent uses this budget.
const TURN = 120000;
const NOTICE = 'The user has just related more projects to this task';
// The warnings the restarter logs when the agent could not be restarted or told (see restart_for_related.go).
const RESTART_FAILURE = /could not restart the agent|cannot resume with the related projects|could not tell the agent|could not reload the task|could not mark the task failed|the task has no live agent/;

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// A git repo with the given files, committed on main. The directory lives under the run's scratch dir.
function gitRepo(prefix, files) {
  const dir = fs.mkdtempSync(path.join(TMP, `related-real-${prefix}-`));
  for (const [name, content] of Object.entries(files)) fs.writeFileSync(path.join(dir, name), content);
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Each registration gets its own name and directory: the workspaces of the shared database must not collide.
let registrations = 0;
async function registerWs(ctx, prefix, files) {
  registrations += 1;
  const name = `${prefix}-${tag}-${registrations}`;
  const dir = gitRepo(name, files);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  return { name, dir };
}

async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
}

// The visible create form (the one that holds the workspace select).
const CREATE = 'form:has(select[name=workspace_id]):visible';

// Creates a task in the workspace with the claude runtime, starts it, and opens its detail page.
async function startTask(ctx, ws, prompt, { startNow = true } = {}) {
  const t0 = Date.now();
  const step = (label) => console.log(`    [startTask] ${label}: ${Date.now() - t0} ms`);
  await openBoard(ctx, ws.name);
  await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  const form = ctx.page.locator(CREATE);
  await form.locator('select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
  // The agent selector lives in the advanced options, which are collapsed by default.
  await form.locator('input[name=show_advanced]').check();
  await form.locator('select[name=agent_runtime]').selectOption('claude');
  await form.locator('textarea[name=prompt]').fill(prompt);
  if (startNow) await form.locator('input[name=start_now]').check(); else await form.locator('input[name=start_now]').uncheck();
  await form.locator('button[type=submit]', { hasText: 'Crear tarea' }).click();
  await form.waitFor({ state: 'detached', timeout: 15000 }).catch(() => {});
  step('form closed');
  // The card can sit in the board or in the "Requieren atención" list of the sidebar: either link leads to the task.
  const link = ctx.page.locator('a[href*="/tasks/"]', { hasText: prompt }).first();
  await link.waitFor({ state: 'attached', timeout: 30000 });
  step('task link attached');
  await ctx.goto(await link.getAttribute('href'));
  step('detail opened');
  await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}/, { timeout: 15000, waitUntil: 'commit' });
}

// The text of the conversation of the task (its transcript panel).
const transcriptText = (ctx) => ctx.page.locator('[hx-get*="/transcript"]').first().innerText();

// Waits until the transcript shows `text` (for example the prompt that a start has just sent).
async function waitTranscriptHas(ctx, text) {
  const started = Date.now();
  while (Date.now() - started < TURN) {
    if ((await transcriptText(ctx).catch(() => '')).includes(text)) return;
    await sleep(200);
  }
  throw new Error(`la conversación no mostró "${text}"`);
}

// Waits until the transcript shows the agent idle after `text`: the turn that answers `text` has settled, because its
// status line (Esperando) comes after the text. Before that the last status line is an older one, so this never passes
// while the agent still works on the answer.
async function waitSettledAfter(ctx, text) {
  const started = Date.now();
  let body = '';
  while (Date.now() - started < TURN) {
    body = await transcriptText(ctx).catch(() => '');
    const at = body.lastIndexOf(text);
    if (at !== -1 && body.lastIndexOf('Esperando') > at) return;
    await sleep(300);
  }
  throw new Error(`la conversación no se asentó tras "${text.slice(0, 40)}" en ${TURN} ms`);
}

// The failures the transcript shows: an interrupted or failed run (labels of task_status_messages.go).
async function failedStatusCount(ctx) {
  const body = await transcriptText(ctx).catch(() => '');
  return (body.match(/Interrumpida|Fallida/g) || []).length;
}

// Only visible matches count: the page keeps hidden copies of the same texts.
const visibleText = (ctx, text) => ctx.page.getByText(text, { exact: false }).locator('visible=true');
const noticeCount = async (ctx) => (await transcriptText(ctx).catch(() => '')).split(NOTICE).length - 1;

// The actions tab and its related projects form (rendered only when another workspace exists).
const relatedForm = (ctx) => ctx.page.locator('form[data-task-related-projects]');
const relatedRow = (ctx, ws) => relatedForm(ctx).locator(`[data-related-row][data-related-name="${ws.name}"] input[name="related_project_id"]`);

async function openActions(ctx) {
  await ctx.page.getByText('Acciones', { exact: true }).first().click();
  await sleep(300);
}

async function ensureActions(ctx) {
  if (!(await relatedForm(ctx).first().isVisible().catch(() => false))) await openActions(ctx);
  await relatedForm(ctx).first().waitFor({ state: 'visible', timeout: 10000 });
}

// Ticks (on) or unticks (off) a project in the actions chooser and saves it. The save answers after the restart
// decision, so the response is the point where the server has acted.
async function setRelated(ctx, ws, on) {
  const t0 = Date.now();
  const step = (label) => console.log(`    [setRelated ${on ? 'on' : 'off'} ${ws.name}] ${label}: ${Date.now() - t0} ms`);
  await ensureActions(ctx);
  step('actions tab open');
  await relatedForm(ctx).getByRole('button', { name: 'Elegir proyectos' }).click();
  const box = relatedRow(ctx, ws);
  await box.waitFor({ state: 'visible', timeout: 5000 });
  if (on) await box.check(); else await box.uncheck();
  step('box set');
  const saved = ctx.page.waitForResponse((r) => r.request().method() === 'POST' && /\/related-projects/.test(r.url()), { timeout: TURN });
  await relatedForm(ctx).getByRole('button', { name: 'Hecho' }).click();
  const response = await saved;
  step(`POST answered ${response.status()}`);
  ctx.assert(response.status() === 200, `related-projects respondió ${response.status()}`);
  await sleep(600);
}

// The composer of the task (the message box of the chat) and its send button.
const composer = (ctx) => ctx.page.locator('textarea[name=message]:visible');
const sendButton = (ctx) => ctx.page.getByRole('button', { name: 'Enviar' });
const menuItem = (ctx, insert) => ctx.page.locator(`[data-composer-item][data-insert="${insert}"]:visible`);

// The server log lines written since the offset. Empty when the log is not known to the runner.
function serverLogSince(offset) {
  if (!SERVER_LOG) return '';
  return fs.readFileSync(SERVER_LOG, 'utf8').slice(offset);
}
const serverLogSize = () => (SERVER_LOG ? fs.statSync(SERVER_LOG).size : 0);

// Fails when the server logged a restart warning since the offset (the restart did not complete).
function assertNoRestartFailure(ctx, offset, when) {
  const failures = serverLogSince(offset).split('\n').filter((line) => RESTART_FAILURE.test(line));
  ctx.assert(failures.length === 0, `${when}: el servidor registró fallos de reinicio:\n${failures.join('\n')}`);
}

// On failure, keeps the text and a screenshot of the page in the run dir, so the state can be read afterwards.
async function keepEvidence(ctx, label) {
  if (!RUN_DIR) return;
  const text = await ctx.page.locator('body').innerText().catch((e) => `innerText failed: ${e.message}`);
  fs.writeFileSync(path.join(RUN_DIR, `${label}.txt`), text);
  await ctx.page.screenshot({ path: path.join(RUN_DIR, `${label}.png`), fullPage: true }).catch(() => {});
}

const withEvidence = (label, scenario) => async (ctx) => {
  try {
    await scenario(ctx);
  } catch (error) {
    await keepEvidence(ctx, `${label}-failure`);
    throw error;
  }
};

// (a) to (f): one task in A, one project B related while it is idle, the @ reference and the agent reading the file.
async function scenarioIdleRelate(ctx) {
  const marker = `REL-${tag}-CONTENT`;
  const a = await registerWs(ctx, 'rr-a', { 'README.md': 'own project\n' });
  const b = await registerWs(ctx, 'rr-b', { 'MARKER.txt': `${marker}\n` });

  // (b) A task in A with claude; the answer is one word that the prompt does not contain.
  await startTask(ctx, a, `Reply with the opposite of up, in capitals, and nothing else. (${tag})`);
  await ctx.page.getByText(/^DOWN\.?$/).first().waitFor({ timeout: TURN });
  await waitSettledAfter(ctx, 'Reply with the opposite of up');
  ctx.assert((await failedStatusCount(ctx)) === 0, 'la tarea falló antes de relacionar B');

  // (c) + (d) Relate B from the actions tab. The restart happens inside the save: the task must not end interrupted.
  const offset = serverLogSize();
  const noticesBefore = await noticeCount(ctx);
  await setRelated(ctx, b, true);
  // The restart tells the agent about B as a new turn: the notice shows, and the agent answers it.
  await visibleText(ctx, NOTICE).first().waitFor({ timeout: TURN });
  await waitSettledAfter(ctx, NOTICE);
  ctx.assert((await failedStatusCount(ctx)) === 0, 'la tarea quedó interrumpida o fallida tras el reinicio');
  ctx.assert((await noticeCount(ctx)) > noticesBefore, 'el agente no recibió el aviso de los proyectos nuevos');
  assertNoRestartFailure(ctx, offset, 'tras relacionar B');
  await ensureActions(ctx);
  ctx.assert(await relatedRow(ctx, b).isChecked(), 'B no aparece marcado tras guardar y recargar');
  const chips = await relatedForm(ctx).locator('[data-related-summary] li').allTextContents();
  ctx.assert(chips.some((text) => text.includes(b.name)), `el resumen no muestra ${b.name}: ${chips.join(',')}`);

  // (e) The @ completion of the chat offers B's file under B's label and inserts it.
  const insert = `@${b.name}:MARKER.txt`;
  await composer(ctx).fill('@MARKER');
  await menuItem(ctx, insert).waitFor({ timeout: 15000 });
  await menuItem(ctx, insert).click();
  const typed = await composer(ctx).inputValue();
  ctx.assert(typed.includes(insert), `el texto no contiene ${insert}: "${typed}"`);

  // (f) Ask for the file's content. The marker is not in the prompt, so only reading the file produces it.
  await composer(ctx).fill(`${typed.trim()} Read the referenced file and reply with its exact content only.`);
  await sendButton(ctx).click();
  const deadline = Date.now() + TURN;
  let answered = false;
  while (Date.now() < deadline) {
    if ((await visibleText(ctx, marker).count()) > 0) {
      answered = true;
      break;
    }
    // A permission prompt for reading B would be a finding: the folder is granted by --add-dir.
    if (await ctx.page.getByRole('button', { name: 'Sí' }).first().isVisible().catch(() => false)) {
      throw new Error('el agente pidió permiso para leer el fichero de B (un diálogo "Sí"): --add-dir no basta');
    }
    await sleep(1000);
  }
  ctx.assert(await ctx.page.getByText('no permite redirigir', { exact: false }).count() === 0, 'el mensaje con @ fue rechazado: el agente seguía trabajando al enviarlo');
  ctx.assert(answered, `el agente no respondió con el contenido de ${b.name}/MARKER.txt`);
  await waitSettledAfter(ctx, marker);
  ctx.assert((await failedStatusCount(ctx)) === 0, 'la tarea quedó interrumpida o fallida al leer el fichero');
  assertNoRestartFailure(ctx, offset, 'tras preguntar por el fichero');
}

// (g) B is related while the agent works. The run must finish uncut, and the notice must come after it: the restart
// waits for the idle state. Then B is removed: no notice and no restart.
async function scenarioBusyRelate(ctx) {
  const a = await registerWs(ctx, 'rr-ga', { 'README.md': 'own project\n' });
  const b = await registerWs(ctx, 'rr-gb', { 'MARKER.txt': `busy-${tag}\n` });

  // A long answer keeps the agent working for ~20 s. The task is started from its own page, because the card of a
  // running task shows up in the lists only after the run ends. "2999" is not in the prompt.
  const prompt = `Write the numbers from 1 to 3000, one per line, and nothing else. (${tag})`;
  await startTask(ctx, a, prompt, { startNow: false });
  await ctx.page.getByRole('button', { name: 'Iniciar', exact: true }).first().click();
  await waitTranscriptHas(ctx, 'Write the numbers from 1 to 3000');
  const offset = serverLogSize();
  await setRelated(ctx, b, true);
  assertNoRestartFailure(ctx, offset, 'al guardar con el agente en marcha');

  // The run completes: its last numbers are in the transcript and the agent is idle again.
  await waitSettledAfter(ctx, '2999');
  ctx.assert((await failedStatusCount(ctx)) === 0, 'la tarea quedó interrumpida o fallida durante la ejecución');
  // The notice comes after the numbers: it was sent when the agent became idle, not in the middle of the run.
  await visibleText(ctx, NOTICE).first().waitFor({ timeout: TURN });
  await waitSettledAfter(ctx, NOTICE);
  const body = await transcriptText(ctx);
  ctx.assert(body.indexOf('2999') !== -1 && body.indexOf(NOTICE) > body.indexOf('2999'), 'el aviso no llegó después de terminar la ejecución');
  ctx.assert((await failedStatusCount(ctx)) === 0, 'la tarea quedó interrumpida o fallida tras el reinicio pendiente');
  assertNoRestartFailure(ctx, offset, 'tras el reinicio pendiente');

  // Removing B: the save sends no notice and does not restart the agent.
  const noticesAfterAdd = await noticeCount(ctx);
  await setRelated(ctx, b, false);
  await sleep(4000);
  ctx.assert((await noticeCount(ctx)) === noticesAfterAdd, 'quitar B envió un aviso al agente');
  ctx.assert((await failedStatusCount(ctx)) === 0, 'quitar B interrumpió o hizo fallar la tarea');
  ctx.assert(!(await relatedRow(ctx, b).isChecked()), 'B sigue marcado tras quitarlo');
  assertNoRestartFailure(ctx, offset, 'tras quitar B');
}

module.exports = {
  name: 'related_projects_real',
  tests: [
    {
      name: 'related_projects real agent: relacionar B desde Acciones reinicia claude sin interrumpirlo, B se usa con @ y el agente lee su fichero',
      fn: withEvidence('idle-relate', scenarioIdleRelate),
    },
    {
      name: 'related_projects real agent: relacionar B con el agente en marcha espera a que esté inactivo; quitar B no envía aviso',
      fn: withEvidence('busy-relate', scenarioBusyRelate),
    },
  ],
};
