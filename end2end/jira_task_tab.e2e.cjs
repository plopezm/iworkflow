// end2end/jira_task_tab.e2e.cjs
// E2E (Playwright CJS, harness compartido) de la pestaña "Jira" de la pantalla de una tarea en ejecución (T7, S1-S8).
// Contrato: la pestaña "Jira" solo aparece con conexión configurada (S1); vincular una clave manual (S2); el formulario
// de worklog con tiempo precargado en múltiplos de 5m y descripción (S4, S5, S6); registrar muestra "Registrado: X" y
// envía el worklog real a la API (S4, S7); tiempo inválido no envía nada; el banner de tarea terminada (S8).
//
// Requiere el mock de Jira (end2end/jira_mock.cjs) en $JIRA_MOCK_URL. Lo arranca end2end/go/run_go.sh.
// Usa los helpers de end2end/go/tasks_actions.e2e.cjs (liveTask / finishedTask) para crear tareas con el agente falso.
//
// Notas de aislamiento (BD compartida entre tests):
//   - Vincular un issue a una tarea es exclusivo: ABC-1 solo puede quedar ligado una vez. Por eso cada test que vincula
//     hace freshConnection(): desconectar (borra los issues importados y sus vínculos) y reconectar/sincronizar.
//   - Cada test reinicia el mock (/_reset), así ABC-1 vuelve a "To Do" y sin worklogs.
//   - Al final del test (f) se reconecta, para que los módulos siguientes encuentren la conexión.
const fs = require('fs');
const path = require('path');
const { finishedTask, liveTask, sleep } = require('./go/tasks_actions.e2e.cjs').helpers;

const JIRA_URL = process.env.JIRA_MOCK_URL || 'http://127.0.0.1:9999';
const JIRA_TOKEN = 'e2e-token';
// Capturas opcionales: con JIRA_TAB_SHOTS=<dir> guarda PNG de la pestaña para revisión visual.
async function shot(ctx, name) {
  const dir = process.env.JIRA_TAB_SHOTS;
  if (!dir) return;
  fs.mkdirSync(dir, { recursive: true });
  await ctx.screenshot(path.join(dir, `${name}.png`));
}

const ISSUE = 'ABC-1';

// Lee los worklogs que el mock ha recibido desde el último reset.
async function mockWorklogs() {
  const res = await fetch(`${JIRA_URL}/__worklogs`);
  return res.json();
}

// Desconecta: el primer clic abre la confirmación ("Desconectar" ghost); el botón btn-danger confirma y olvida
// el token y los issues importados.
async function disconnect(ctx) {
  await ctx.page.getByRole('button', { name: 'Desconectar' }).first().click();
  await ctx.page.locator('button.btn-danger', { hasText: 'Desconectar' }).first().click();
  await ctx.page.getByLabel(/^URL del servidor/).first().waitFor({ timeout: 15000 });
}

// Conecta con el mock y sincroniza. Con `fresh`, desconecta antes: se olvidan los issues importados (y sus vínculos).
async function connectAndSync(ctx, { fresh = false } = {}) {
  try { await fetch(`${JIRA_URL}/_reset`); }
  catch (e) { /* el mock puede no estar; no bloquear la conexión */ }
  await ctx.goto('/jira');
  await ctx.page.getByText('Conexión con Jira').first().waitFor({ timeout: 10000 });
  if (fresh && await ctx.page.getByText('Editar conexión').first().count() > 0) {
    await disconnect(ctx);
  }
  if (await ctx.page.getByText('Editar conexión').first().count() === 0) {
    await ctx.page.getByLabel(/^URL del servidor/).first().fill(JIRA_URL);
    await ctx.page.getByLabel(/^Token de usuario/).first().fill(JIRA_TOKEN);
    await ctx.page.getByRole('button', { name: /Conectar|Comprobando/ }).first().click();
    await ctx.page.getByText('Editar conexión').first().waitFor({ timeout: 15000 });
  }
  await ctx.page.getByRole('button', { name: /Sincronizar/ }).first().click();
  await ctx.page.getByText('Mis tareas').first().waitFor({ timeout: 15000 });
  await ctx.page.getByText('ABC-1').first().waitFor({ timeout: 15000 });
  await sleep(1200);
}

// Pestaña "Jira" del detalle de la tarea y su panel (el div de la región con sufijo "-jira").
async function openJiraTab(ctx) {
  await ctx.page.getByRole('tab', { name: 'Jira', exact: true }).first().click();
  await panel(ctx).getByLabel('Clave del issue').waitFor({ timeout: 15000 });
}

function panel(ctx) {
  return ctx.page.locator('div[id$="-jira"]').first();
}

// Vincula ABC-1 desde el formulario de la pestaña y espera al resumen del issue.
async function linkIssue(ctx, key) {
  await openJiraTab(ctx);
  await panel(ctx).getByLabel('Clave del issue').fill(key);
  await panel(ctx).getByRole('button', { name: 'Vincular', exact: true }).click();
  await panel(ctx).getByRole('link', { name: key, exact: true }).first().waitFor({ timeout: 15000 });
}

// Minutos de un tiempo en formato Jira ("15m", "1h", "1h 30m"); NaN si no es ese formato.
function jiraMinutes(text) {
  const m = /^(?:(\d+)h)?(?:\s*(\d+)m)?$/.exec(text.trim());
  if (!m || (m[1] === undefined && m[2] === undefined)) return NaN;
  return Number(m[1] || 0) * 60 + Number(m[2] || 0);
}

module.exports = {
  name: 'jira_task_tab',
  tests: [
    {
      name: 'jira_tab_visible: con conexión, una tarea en ejecución muestra la pestaña "Jira"',
      fn: async (ctx) => {
        await connectAndSync(ctx, { fresh: true });
        await liveTask(ctx, 'tab-a');
        await ctx.page.getByRole('tab', { name: 'Acciones', exact: true }).first().waitFor({ timeout: 15000 });
        await ctx.page.getByRole('tab', { name: 'Jira', exact: true }).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'link_task_issue: una tarea sin vincular enlaza ABC-1 y muestra clave, resumen y estado',
      fn: async (ctx) => {
        await connectAndSync(ctx, { fresh: true });
        await liveTask(ctx, 'tab-b');
        await linkIssue(ctx, ISSUE);
        await panel(ctx).getByText('Falla el login', { exact: true }).first().waitFor({ timeout: 15000 });
        await panel(ctx).getByText('To Do', { exact: true }).first().waitFor({ timeout: 15000 });
        await shot(ctx, '00-linked-issue-details');
      },
    },
    {
      name: 'link_task_issue: una clave inválida muestra el error y la tarea sigue sin vincular',
      fn: async (ctx) => {
        await connectAndSync(ctx, { fresh: true });
        await liveTask(ctx, 'tab-c');
        await openJiraTab(ctx);
        await panel(ctx).getByLabel('Clave del issue').fill('no valida');
        await panel(ctx).getByRole('button', { name: 'Vincular', exact: true }).click();
        await panel(ctx).getByText(/clave de tarea de Jira inválida/).first().waitFor({ timeout: 15000 });
        ctx.assert(await panel(ctx).getByLabel('Clave del issue').count() > 0, 'el formulario de vinculación debe seguir visible');
        ctx.assert(await panel(ctx).getByRole('link', { name: ISSUE, exact: true }).count() === 0, 'no debe mostrarse ningún issue vinculado');
      },
    },
    {
      name: 'log_worklog_prefill: el formulario precarga un tiempo múltiplo de 5m y la descripción de la tarea',
      fn: async (ctx) => {
        await connectAndSync(ctx, { fresh: true });
        await finishedTask(ctx, 'tab-d');
        await linkIssue(ctx, ISSUE);
        await panel(ctx).getByText('Registrar tiempo', { exact: true }).first().waitFor({ timeout: 15000 });
        await shot(ctx, '01-linked-worklog-prefilled');
        const time = panel(ctx).getByLabel('Tiempo', { exact: true });
        const prefilled = await time.inputValue();
        const minutes = jiraMinutes(prefilled);
        ctx.assert(minutes > 0 && minutes % 5 === 0, `tiempo precargado "${prefilled}" no es un múltiplo positivo de 5m`);
        // El <textarea> va dentro de su <label>: el nombre accesible incluye el contenido (la descripción), así que
        // getByLabel exacto no lo encuentra. Se apunta por su atributo name.
        const description = await panel(ctx).locator('textarea[name="description"]').inputValue();
        ctx.assert(description.trim().length > 0, 'la descripción precargada no debe estar vacía');
      },
    },
    {
      name: 'log_worklog_submit: registrar con tiempo editado "15m" muestra "Registrado: 15m" y envía 900 s',
      fn: async (ctx) => {
        await connectAndSync(ctx, { fresh: true });
        await finishedTask(ctx, 'tab-f-submit');
        await linkIssue(ctx, ISSUE);
        // S8: tarea terminada con tiempo pendiente, un aviso discreto (no un modal).
        await panel(ctx).getByRole('status').filter({ hasText: /Task terminada/ }).first().waitFor({ timeout: 15000 });
        await panel(ctx).getByLabel('Tiempo', { exact: true }).fill('15m');
        await panel(ctx).getByRole('button', { name: 'Registrar en Jira', exact: true }).click();
        await panel(ctx).getByText('Registrado: 15m', { exact: true }).first().waitFor({ timeout: 15000 });
        await shot(ctx, '02-worklog-registered');
        const sent = (await mockWorklogs()).filter((w) => w.issueKey === ISSUE);
        ctx.assert(sent.length === 1, `el mock debía recibir 1 worklog y recibió ${sent.length}`);
        ctx.assert(sent[0].timeSpentSeconds === 900, `timeSpentSeconds ${sent[0].timeSpentSeconds} !== 900`);
        ctx.assert(await panel(ctx).getByRole('button', { name: 'Registrar en Jira', exact: true }).isDisabled(),
          'el botón debe quedar deshabilitado tras registrar');
      },
    },
    {
      name: 'jira_tab_scroll: con poca altura el panel Jira hace scroll y el botón "Registrar en Jira" es alcanzable',
      fn: async (ctx) => {
        await connectAndSync(ctx, { fresh: true });
        await finishedTask(ctx, 'tab-scroll');
        await linkIssue(ctx, ISSUE);
        await ctx.page.setViewportSize({ width: 1280, height: 480 });
        await sleep(300);
        const pane = panel(ctx);
        const metrics = await pane.evaluate((el) => ({ scrollHeight: el.scrollHeight, clientHeight: el.clientHeight }));
        ctx.assert(metrics.scrollHeight > metrics.clientHeight, `el panel debía desbordar (scrollHeight ${metrics.scrollHeight} <= clientHeight ${metrics.clientHeight})`);
        await pane.evaluate((el) => { el.scrollTop = el.scrollHeight; });
        await sleep(200);
        const top = await pane.evaluate((el) => el.scrollTop);
        ctx.assert(top > 0, 'el panel no se desplazó (scrollTop 0)');
        const button = pane.getByRole('button', { name: 'Registrar en Jira', exact: true });
        const box = await button.boundingBox();
        const view = ctx.page.viewportSize();
        ctx.assert(box && box.y >= 0 && box.y + box.height <= view.height, 'el botón Registrar en Jira no es visible tras hacer scroll');
        await shot(ctx, '03-scrolled-low-viewport');
        await ctx.page.setViewportSize({ width: 1440, height: 860 });
      },
    },
    {
      name: 'log_worklog_invalid: un tiempo inválido ("abc") muestra error inline y no envía nada',
      fn: async (ctx) => {
        await connectAndSync(ctx, { fresh: true });
        await finishedTask(ctx, 'tab-e');
        await linkIssue(ctx, ISSUE);
        const before = (await mockWorklogs()).length;
        await panel(ctx).getByLabel('Tiempo', { exact: true }).fill('abc');
        await panel(ctx).getByRole('button', { name: 'Registrar en Jira', exact: true }).click();
        await panel(ctx).getByText(/duración inválida/).first().waitFor({ timeout: 15000 });
        ctx.assert(await panel(ctx).getByText(/^Registrado:/).count() === 0, 'no debe mostrarse "Registrado" con un tiempo inválido');
        const after = (await mockWorklogs()).length;
        ctx.assert(after === before, `con tiempo inválido no debe enviarse nada (${before} -> ${after})`);
      },
    },
    {
      name: 'jira_tab_hidden: sin conexión configurada la tarea no muestra la pestaña "Jira"',
      fn: async (ctx) => {
        try {
          await connectAndSync(ctx, { fresh: true });
          await ctx.goto('/jira');
          await disconnect(ctx);
          const t = await liveTask(ctx, 'tab-g');
          await ctx.page.getByRole('tab', { name: 'Acciones', exact: true }).first().waitFor({ timeout: 15000 });
          await ctx.goto(`/tasks/${t.id}`);
          await ctx.page.getByRole('tab', { name: 'Acciones', exact: true }).first().waitFor({ timeout: 15000 });
          ctx.assert(await ctx.page.getByRole('tab', { name: 'Jira', exact: true }).count() === 0,
            'sin conexión no debe mostrarse la pestaña "Jira"');
        } finally {
          // Deja la conexión configurada para los módulos siguientes.
          await connectAndSync(ctx, { fresh: true });
        }
      },
    },
  ],
};
