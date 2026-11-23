// end2end/go/markdown_view.e2e.cjs
// The Code | Rendered switch of the editors of Markdown files, on the two surfaces that open one:
//   * the file tab of the explorer (/workspaces/<id>/files/<path>, the same editor as /tasks/<id>/files/<path>)
//   * the inline editor of a file card in the diff of a task.
// Screenshots: set E2E_SHOTS_DIR (default /tmp/iwf-markdown-view-shots).
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const { taskDetail } = require('./tasks_actions.e2e.cjs').helpers;

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.E2E_SHOTS_DIR || '/tmp/iwf-markdown-view-shots';

const README = [
  `# Titulo ${tag}`,
  '',
  'Un parrafo con **negrita** y un [enlace](https://example.com).',
  '',
  '- uno',
  '- dos',
  '',
  '```go',
  'fmt.Println("hola")',
  '```',
  '',
  '<script>window.__pwned = true</script>',
  '',
].join('\n');

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `md-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), README);
  fs.writeFileSync(path.join(dir, 'main.go'), 'package main\n');
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function registerWs(ctx, name, dir) {
  await ctx.goto('/workspaces/new');
  await ctx.page.getByLabel('Nombre').fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  const id = await ctx.page.evaluate((n) => {
    for (const card of document.querySelectorAll('article.card')) {
      if (!card.textContent.includes(n)) continue;
      const m = (card.querySelector('a[href*="/workspaces/"]') || {}).href?.match(/\/workspaces\/([0-9a-f-]{36})/);
      if (m) return m[1];
    }
    return null;
  }, name);
  ctx.assert(id, `no se pudo extraer el id del workspace "${name}"`);
  return id;
}

// The remembered mode is shared by every test: start each one in code mode.
async function resetMode(ctx) {
  await ctx.page.evaluate(() => localStorage.removeItem('iwf-md-mode'));
}

const switchButton = (ctx, name) => ctx.page.locator(`[data-md-mode-set="${name}"]:visible`).first();

module.exports = {
  name: 'markdown_view',
  tests: [
    {
      name: 'el explorador de ficheros conmuta un .md entre código y renderizado',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('tree');
        const id = await registerWs(ctx, `md-${tag}`, dir);
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        await resetMode(ctx);
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        const editor = ctx.page.locator('textarea[data-code-editor]').first();
        await editor.waitFor({ state: 'visible', timeout: 12000 });
        await shot(ctx, '1-explorer-md-code');

        await switchButton(ctx, 'rendered').click();
        const rendered = ctx.page.locator('[data-md-preview]:visible').first();
        await rendered.locator('h1').waitFor({ timeout: 10000 });
        ctx.assert((await rendered.locator('h1').textContent()).includes(`Titulo ${tag}`), 'el h1 renderizado no tiene el titulo');
        ctx.assert((await rendered.locator('li').count()) === 2, 'la lista debe tener 2 elementos');
        ctx.assert((await rendered.locator('strong').count()) === 1, 'falta la negrita');
        ctx.assert(!(await editor.isVisible()), 'el editor de código debe ocultarse en la vista renderizada');
        ctx.assert((await ctx.page.evaluate(() => window.__pwned)) === undefined, 'el HTML crudo del markdown no debe ejecutarse');
        ctx.assert((await rendered.locator('script').count()) === 0, 'no debe haber <script> en la vista renderizada');
        await shot(ctx, '2-explorer-md-rendered');

        await switchButton(ctx, 'code').click();
        await editor.waitFor({ state: 'visible', timeout: 5000 });
        ctx.assert(!(await rendered.isVisible().catch(() => false)), 'la vista renderizada debe ocultarse en modo código');
        ctx.assert((await editor.inputValue()).includes('# Titulo'), 'el editor debe conservar el texto');
      },
    },
    {
      name: 'la vista renderizada muestra el borrador sin guardar y la elección se recuerda',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('draft');
        const id = await registerWs(ctx, `mdd-${tag}`, dir);
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        await resetMode(ctx);
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        const editor = ctx.page.locator('textarea[data-code-editor]').first();
        await editor.waitFor({ state: 'visible', timeout: 12000 });
        await editor.fill('## Borrador sin guardar\n');
        await switchButton(ctx, 'rendered').click();
        await ctx.page.locator('[data-md-preview]:visible h2', { hasText: 'Borrador sin guardar' }).waitFor({ timeout: 10000 });

        // Reopening the file keeps the chosen mode.
        await ctx.goto(`/workspaces/${id}/files/README.md`);
        await ctx.page.locator('[data-md-preview]:visible h1').waitFor({ timeout: 10000 });
        await resetMode(ctx);
      },
    },
    {
      name: 'un fichero que no es markdown no ofrece el conmutador',
      fn: async (ctx) => {
        const dir = uniqueGitRepo('nomd');
        const id = await registerWs(ctx, `mdn-${tag}`, dir);
        await ctx.goto(`/workspaces/${id}/files/main.go`);
        await ctx.page.locator('textarea[data-code-editor]').first().waitFor({ state: 'visible', timeout: 12000 });
        ctx.assert((await ctx.page.locator('[data-md-mode-set]').count()) === 0, 'main.go no debe tener conmutador');
      },
    },
    {
      name: 'el editor de una tarjeta del diff conmuta un .md entre código y renderizado',
      fn: async (ctx) => {
        const prompt = `mddiff-${tag}`;
        await taskDetail(ctx, 'mddiff', prompt, { start: true });
        await ctx.page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
        await ctx.page.getByText('Acciones', { exact: true }).first().click();
        await ctx.page.getByRole('button', { name: 'Ver diff' }).click();
        await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}\/diff$/, { timeout: 15000 });
        await resetMode(ctx);
        // The fake agent writes CAMBIO.md, so the diff has a Markdown card.
        const card = ctx.page.locator('.diff-card', { hasText: 'CAMBIO.md' }).first();
        await card.waitFor({ timeout: 20000 });
        await card.getByRole('button', { name: 'Editar' }).click();
        const textarea = card.locator('textarea[name="content"]');
        await textarea.waitFor({ state: 'visible', timeout: 10000 });
        await textarea.fill('# Cambio del agente\n\n- punto uno\n- punto dos\n');
        await shot(ctx, '3-diff-md-code');

        await card.locator('[data-md-mode-set="rendered"]').click();
        const rendered = card.locator('[data-md-preview]');
        await rendered.locator('h1').waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert((await rendered.locator('li').count()) === 2, 'la lista renderizada debe tener 2 elementos');
        ctx.assert(!(await textarea.isVisible()), 'el textarea debe ocultarse en la vista renderizada');
        await shot(ctx, '4-diff-md-rendered');

        await card.locator('[data-md-mode-set="code"]').click();
        await textarea.waitFor({ state: 'visible', timeout: 5000 });
        await resetMode(ctx);
      },
    },
  ],
};
