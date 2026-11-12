// end2end/go/code_hosting.e2e.cjs
// Go port of end2end/code_hosting.e2e.cjs (T8, phase 2).
//
// Scope: the code hosting screens against the GitLab mock (end2end/gitlab_mock.cjs, API v4): the
// GitLab settings card of /settings (save, test connection) and the merge request panel of a task
// (get, create, merge and push).
//
// Selectors and assertions are the same as the Rust script: the Go screens render the same texts
// (translated from the same Rust constants) and the same labels. The only difference is the path of
// the shared task helpers, which are one folder up.
//
// push_task_branch: the real push goes to a local bare repository (see `pushInsteadOf` in
// ../tasks.e2e.cjs), because the mock does not speak the git protocol over HTTP.
//
// The "PR / MR" tab is the task tab that the tasks screen embeds with the merge request panel. The
// tests need the tasks screen to exist (finishedTask and openActions come from it).
const tasks = require('./tasks_actions.e2e.cjs');
const { finishedTask, liveTask, openActions } = tasks.helpers;

const GITLAB_URL = process.env.GITLAB_MOCK_URL || 'http://127.0.0.1:9998';
const GOOD_TOKEN = 'gl-good-token';
const tag = Date.now().toString();

async function gotoGitlabSettings(ctx) {
  await ctx.goto('/settings');
  await ctx.page.getByRole('tab', { name: 'GitLab' }).first().click();
  await ctx.page.getByLabel(/^URL de GitLab/).first().waitFor({ timeout: 15000 });
}

async function saveSettings(ctx, token) {
  await gotoGitlabSettings(ctx);
  await ctx.page.getByLabel(/^URL de GitLab/).first().fill(GITLAB_URL);
  await ctx.page.getByLabel(/^Token de acceso personal/).first().fill(token);
  await ctx.page.getByRole('button', { name: 'Guardar' }).first().click();
  await ctx.page.waitForTimeout(1200);
}

async function openMrTab(ctx) {
  await ctx.page.getByRole('tab', { name: 'PR / MR' }).first().click();
}

const SHOTS = process.env.PR_TAB_SHOTS || '/tmp/pr-tab-shots';
async function shot(ctx, name) {
  require('fs').mkdirSync(SHOTS, { recursive: true });
  await ctx.page.screenshot({ path: `${SHOTS}/${name}.png` });
}

async function createOnly(ctx) {
  await ctx.page.getByRole('button', { name: 'Crear merge/pull request' }).first().click();
}

// Tarea finalizada en un workspace cuyo `origin` apunta al GitLab mock.
async function mrTask(ctx, key) {
  await saveSettings(ctx, GOOD_TOKEN);
  const t = await finishedTask(ctx, key, { origin: `${GITLAB_URL}/e2e/${key}-${tag}.git` });
  await openMrTab(ctx);
  return t;
}

module.exports = {
  name: 'code_hosting',
  tests: [
    {
      name: 'save_code_host_settings + get_code_host_settings: guardar URL y token los persiste',
      fn: async (ctx) => {
        await saveSettings(ctx, GOOD_TOKEN);
        await gotoGitlabSettings(ctx);
        const url = await ctx.page.getByLabel(/^URL de GitLab/).first().inputValue();
        ctx.assert(url.replace(/\/$/, '') === GITLAB_URL, `URL no persistida: "${url}"`);
        const token = await ctx.page.getByLabel(/^Token de acceso personal/).first().inputValue();
        ctx.assert(token === '', 'el token guardado no debe volver a mostrarse');
        ctx.assert(await ctx.page.getByPlaceholder(/déjalo vacío para conservarlo/).count() > 0,
          'falta el placeholder que indica token ya guardado');
      },
    },
    {
      name: 'test_code_host_connection: con token válido muestra "Conectado como"',
      fn: async (ctx) => {
        await saveSettings(ctx, GOOD_TOKEN);
        await ctx.page.getByRole('button', { name: 'Probar conexión' }).first().click();
        await ctx.page.getByText(/Conectado como e2e-user/).first().waitFor({ timeout: 15000 });
      },
    },
    {
      name: 'test_code_host_connection: con token inválido muestra el error y no "Conectado"',
      fn: async (ctx) => {
        await saveSettings(ctx, 'gl-bad-token');
        await ctx.page.getByRole('button', { name: 'Probar conexión' }).first().click();
        await ctx.page.getByText(/401|Unauthorized|no autorizado|rechaz/i).first().waitFor({ timeout: 15000 });
        ctx.assert(await ctx.page.getByText(/Conectado como/).count() === 0, 'mostró "Conectado" con token inválido');
        await saveSettings(ctx, GOOD_TOKEN); // restaura el estado para el resto de tests
      },
    },
    {
      name: 'get_task_merge_request: una rama sin MR ofrece crearlo',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-none');
        await ctx.page.getByText('Esta rama aún no tiene merge/pull request.').first().waitFor({ timeout: 20000 });
        ctx.assert(await ctx.page.getByTestId('mr-steps').count() > 0, 'falta el indicador de pasos');
        await ctx.page.getByRole('button', { name: 'Crear merge/pull request' }).first().waitFor({ timeout: 5000 });
        await ctx.page.getByRole('button', { name: 'Guardar en remoto' }).first().waitFor({ timeout: 5000 });
        ctx.assert(await ctx.page.getByRole('button', { name: /Publicar y mezclar/ }).count() === 0, 'no debe haber un botón que publique y mezcle de una vez');
        await shot(ctx, '01-sin-pr');
      },
    },
    {
      name: 'tarea en ejecución: la pestaña PR / MR ofrece guardar en remoto sin haber subido la rama',
      fn: async (ctx) => {
        await saveSettings(ctx, GOOD_TOKEN);
        await liveTask(ctx, 'mr-live', { origin: `${GITLAB_URL}/e2e/mr-live-${tag}.git` });
        await openMrTab(ctx);
        await ctx.page.getByRole('button', { name: 'Guardar en remoto' }).first().waitFor({ timeout: 20000 });
        await shot(ctx, '00-tarea-en-ejecucion');
      },
    },
    {
      name: 'push_task_branch: "Guardar en remoto" sube la rama sin abrir PR/MR ni mezclar',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-save');
        await ctx.page.getByRole('button', { name: 'Guardar en remoto' }).first().click();
        await ctx.page.getByText('Rama subida a origin.').first().waitFor({ timeout: 25000 });
        await ctx.page.getByRole('button', { name: 'Crear merge/pull request' }).first().waitFor({ timeout: 5000 });
        ctx.assert(await ctx.page.getByText('abierto', { exact: true }).count() === 0, 'guardar no debe abrir una PR/MR');
        await shot(ctx, '02-guardado-en-remoto-sin-pr');
      },
    },
    {
      name: 'create_task_merge_request: crear el MR lo muestra como abierto',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-create');
        await createOnly(ctx);
        await ctx.page.getByText('abierto', { exact: true }).first().waitFor({ timeout: 25000 });
        ctx.assert(await ctx.page.getByRole('button', { name: /^Mezclar/ }).count() > 0, 'falta el botón Mezclar');
        await shot(ctx, '03-pr-abierta');
      },
    },
    {
      name: 'merge_task_merge_request: Mezclar deja el MR como mezclado',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-merge');
        await createOnly(ctx);
        await ctx.page.getByText('abierto', { exact: true }).first().waitFor({ timeout: 25000 });
        await ctx.page.getByRole('button', { name: /^Mezclar/ }).first().click();
        // ConfirmPanel inline: el botón de confirmación también se llama "Mezclar".
        const confirm = ctx.page.locator('button.btn-primary', { hasText: /^Mezclar$/ }).first();
        await confirm.waitFor({ timeout: 8000 });
        await shot(ctx, '04-confirmar-mezcla');
        await confirm.click();
        await ctx.page.getByText('mezclado', { exact: true }).first().waitFor({ timeout: 25000 });
        await ctx.page.getByTestId('mr-merged').waitFor({ timeout: 5000 });
        ctx.assert(await ctx.page.getByRole('button', { name: 'Guardar en remoto' }).count() === 0, 'tras mezclar no se ofrece guardar');
        await shot(ctx, '05-mezclado');
      },
    },
    {
      name: 'push_task_branch: con PR abierta, "Guardar en remoto" publica la rama en origin',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-push');
        await createOnly(ctx);
        await ctx.page.getByText('abierto', { exact: true }).first().waitFor({ timeout: 25000 });
        await ctx.page.getByRole('button', { name: 'Guardar en remoto' }).first().click();
        await ctx.page.getByText('Rama subida a origin.').first().waitFor({ timeout: 25000 });
      },
    },
  ],
};
