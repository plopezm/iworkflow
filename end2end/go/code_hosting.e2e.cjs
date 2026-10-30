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
const { finishedTask, openActions } = tasks.helpers;

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
        await ctx.page.getByRole('button', { name: 'Crear merge/pull request' }).first().waitFor({ timeout: 5000 });
      },
    },
    {
      name: 'create_task_merge_request: crear el MR lo muestra como abierto',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-create');
        await ctx.page.getByRole('button', { name: 'Crear merge/pull request' }).first().click();
        await ctx.page.getByText('abierto', { exact: true }).first().waitFor({ timeout: 25000 });
        ctx.assert(await ctx.page.getByRole('button', { name: /^Mezclar/ }).count() > 0, 'falta el botón Mezclar');
      },
    },
    {
      name: 'merge_task_merge_request: Mezclar deja el MR como mezclado',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-merge');
        await ctx.page.getByRole('button', { name: 'Crear merge/pull request' }).first().click();
        await ctx.page.getByText('abierto', { exact: true }).first().waitFor({ timeout: 25000 });
        await ctx.page.getByRole('button', { name: /^Mezclar/ }).first().click();
        // ConfirmPanel inline: el botón de confirmación también se llama "Mezclar".
        const confirm = ctx.page.locator('button.btn-primary', { hasText: /^Mezclar$/ }).first();
        await confirm.waitFor({ timeout: 8000 });
        await confirm.click();
        await ctx.page.getByText('mezclado', { exact: true }).first().waitFor({ timeout: 25000 });
      },
    },
    {
      name: 'push_task_branch: "Subir rama (push)" publica la rama en origin',
      fn: async (ctx) => {
        await mrTask(ctx, 'mr-push');
        await ctx.page.getByRole('button', { name: 'Crear merge/pull request' }).first().click();
        await ctx.page.getByText('abierto', { exact: true }).first().waitFor({ timeout: 25000 });
        await ctx.page.getByRole('button', { name: 'Subir rama (push)' }).first().click();
        await ctx.page.getByText('Rama subida a origin.').first().waitFor({ timeout: 25000 });
      },
    },
  ],
};
