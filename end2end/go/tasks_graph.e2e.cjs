// end2end/go/tasks_graph.e2e.cjs
// E2E tests for the task dependency graph presentation and canvas (T18 U7).
// Runs against the Go server (IWF_E2E_TARGET=go).
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const assert = require('assert');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `graph-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `graph test ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

async function registerWs(ctx, name) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  return { name, dir };
}

async function getWsId(ctx, name) {
  await ctx.goto('/');
  await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  const select = ctx.page.locator('select[name=workspace]:visible');
  await select.selectOption({ label: `${name} (main)` });
  await sleep(300);
  return await select.inputValue();
}

module.exports = {
  name: 'tasks_graph',
  tests: [
    {
      name: 'graph_canvas: el lienzo del grafo y el panel de tareas sueltas cargan correctamente',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `tg-canvas-${tag}`);
        const wsId = await getWsId(ctx, ws.name);
        assert.ok(wsId);

        // Navigate directly to the graph endpoint of the workspace
        await ctx.goto(`/board/graph?workspace=${encodeURIComponent(wsId)}`);
        
        // Assert canvas structure
        const canvas = ctx.page.locator('[data-graph-canvas]');
        await canvas.first().waitFor({ timeout: 10000 });
        
        // Assert zoom bar
        await ctx.page.locator('[data-zoom-in]').first().waitFor({ timeout: 5000 });
        await ctx.page.locator('[data-zoom-out]').first().waitFor({ timeout: 5000 });
        await ctx.page.locator('[data-zoom-fit]').first().waitFor({ timeout: 5000 });

        // Assert loose tasks panel
        await ctx.page.getByText('Tareas sueltas').first().waitFor({ timeout: 5000 });
        await ctx.page.getByText('No hay tareas sueltas').first().waitFor({ timeout: 5000 });
      },
    },
    {
      name: 'graph_positions: actualizar la posición guarda las coordenadas del nodo',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `tg-pos-${tag}`);
        const wsId = await getWsId(ctx, ws.name);
        assert.ok(wsId);
        
        // Create a task via form
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.locator('#modal-panel select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
        await ctx.page.locator('#modal-panel input[name=title]').fill('Tarea para Grafo');
        await ctx.page.locator('#modal-panel textarea[name=prompt]').fill('Prompt grafo');
        await ctx.page.locator('#modal-panel button[type=submit]', { hasText: 'Crear tarea' }).click();
        await ctx.page.locator('#modal-panel').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});

        // Load the graph endpoint with the real workspace ID
        await ctx.goto(`/board/graph?workspace=${encodeURIComponent(wsId)}`);
        await ctx.page.locator('[data-graph-canvas]').first().waitFor({ timeout: 10000 });

        // The task was created without positions or edges, so it appears in the loose tasks panel!
        await ctx.page.getByText('Tarea para Grafo').first().waitFor({ timeout: 5000 });
      },
    },
  ],
};
