// end2end/terminal.e2e.cjs
// Casos de uso de terminal alcanzables desde la UI:
//   open_home_terminal, list_terminals, open_terminal (workspace), attach_terminal,
//   write_terminal, resize_terminal, close_terminal.
// (open_task_terminal vive en tasks.e2e.cjs: necesita una tarea con worktree.)
// La PTY es real (shell del usuario); los tests escriben comandos `echo` y leen xterm.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `term-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `term ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function registerWs(ctx, name) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
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

// Abre la terminal de un workspace y devuelve el id de sesión (/terminals/<id>).
async function openWorkspaceTerminal(ctx, name) {
  const wsId = await registerWs(ctx, name);
  await ctx.goto(`/workspaces/${wsId}/terminal`);
  await ctx.page.waitForURL(/\/terminals\/[0-9a-f-]{36}$/, { timeout: 20000 });
  await ctx.page.locator('.xterm-helper-textarea').waitFor({ state: 'attached', timeout: 15000 });
  return ctx.page.url().split('/terminals/')[1];
}

// Escribe `text` + Enter en el terminal activo.
async function run(page, text) {
  await page.locator('.xterm').first().click();
  await page.keyboard.type(text);
  await page.keyboard.press('Enter');
}

// Espera a que la pantalla de xterm contenga `needle` al menos `min` veces.
async function screenHas(page, needle, min = 1, timeout = 15000) {
  const start = Date.now();
  let text = '';
  while (Date.now() - start < timeout) {
    text = await page.locator('.xterm-rows').first().innerText();
    if (text.split(needle).length - 1 >= min) return text;
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`xterm no mostró "${needle}" (${min}x); pantalla: ${text.slice(-300)}`);
}

module.exports = {
  name: 'terminal',
  tests: [
    {
      name: 'open_home_terminal: "Nueva terminal en $HOME" abre una sesión',
      fn: async (ctx) => {
        await ctx.goto('/terminals');
        await ctx.page.getByRole('button', { name: /Nueva terminal en \$HOME/ }).first().click();
        await ctx.page.waitForURL(/\/terminals\/[0-9a-f-]{36}$/, { timeout: 20000 });
        await ctx.page.locator('.xterm-helper-textarea').waitFor({ state: 'attached', timeout: 15000 });
      },
    },
    {
      name: 'open_terminal: la terminal de un workspace redirige a su sesión',
      fn: async (ctx) => {
        const sessionId = await openWorkspaceTerminal(ctx, `term-open-${tag}`);
        ctx.assert(/^[0-9a-f-]{36}$/.test(sessionId), `id de sesión inválido: ${sessionId}`);
      },
    },
    {
      name: 'attach_terminal + write_terminal: lo escrito se ejecuta y se ve la salida',
      fn: async (ctx) => {
        await openWorkspaceTerminal(ctx, `term-write-${tag}`);
        await run(ctx.page, "echo marca-4''2");
        await screenHas(ctx.page, 'marca-42');
      },
    },
    {
      name: 'resize_terminal: cambiar el tamaño de la ventana mantiene la sesión operativa',
      fn: async (ctx) => {
        await openWorkspaceTerminal(ctx, `term-resize-${tag}`);
        await ctx.page.setViewportSize({ width: 900, height: 600 });
        await ctx.page.waitForTimeout(800);
        await run(ctx.page, "echo fin-re''size");
        const text = await screenHas(ctx.page, 'fin-resize');
        ctx.assert(!text.includes('[session closed]'), 'la sesión se cerró al redimensionar');
      },
    },
    {
      name: 'list_terminals: la sesión abierta aparece en /terminals como Running',
      fn: async (ctx) => {
        const name = `term-list-${tag}`;
        const sessionId = await openWorkspaceTerminal(ctx, name);
        await ctx.goto('/terminals');
        const row = ctx.page.locator('tr', { hasText: sessionId.slice(0, 8) }).first();
        await row.waitFor({ timeout: 15000 });
        ctx.assert((await row.innerText()).includes(name), 'la fila no muestra el workspace');
        ctx.assert(/Running|En ejecución|Activa/.test(await row.innerText()), 'la sesión no figura como activa');
      },
    },
    {
      name: 'close_terminal: Cerrar quita la sesión del listado o la marca finalizada',
      fn: async (ctx) => {
        const sessionId = await openWorkspaceTerminal(ctx, `term-close-${tag}`);
        await ctx.goto('/terminals');
        const row = ctx.page.locator('tr', { hasText: sessionId.slice(0, 8) }).first();
        await row.waitFor({ timeout: 15000 });
        await row.getByRole('button', { name: /Cerrar/ }).click();
        const start = Date.now();
        let ok = false;
        while (Date.now() - start < 15000) {
          const n = await ctx.page.locator('tr', { hasText: sessionId.slice(0, 8) }).count();
          if (n === 0) { ok = true; break; }
          const t = await ctx.page.locator('tr', { hasText: sessionId.slice(0, 8) }).first().innerText();
          if (/Exited|Finalizada|Terminada/.test(t)) { ok = true; break; }
          await ctx.page.waitForTimeout(400);
        }
        ctx.assert(ok, 'la sesión no se cerró');
      },
    },
  ],
};
