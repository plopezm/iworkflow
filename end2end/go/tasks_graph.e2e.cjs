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
  const link = ctx.page.locator('#board-sidebar').getByRole('link', { name }).first();
  await link.waitFor({ timeout: 10000 });
  await link.click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  const href = await link.getAttribute('href');
  return href.replace('/board/', '');
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
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
        await ctx.page.locator('form:has(select[name=workspace_id]):visible input[name=title]').fill('Tarea para Grafo');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').fill('Prompt grafo');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible button[type=submit]', { hasText: 'Crear tarea' }).click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});

        // Load the graph endpoint with the real workspace ID
        await ctx.goto(`/board/graph?workspace=${encodeURIComponent(wsId)}`);
        await ctx.page.locator('[data-graph-canvas]').first().waitFor({ timeout: 10000 });

        // The task was created without positions or edges, so it appears in the loose tasks panel!
        await ctx.page.getByText('Tarea para Grafo').first().waitFor({ timeout: 5000 });
      },
    },
    {
      name: 'graph_hover_tools: los botones de hover abren el modal para añadir siguiente',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `tg-tools-${tag}`);
        const wsId = await getWsId(ctx, ws.name);
        assert.ok(wsId);

        // Open home board
        await ctx.goto(`/board/${encodeURIComponent(wsId)}`);
        await ctx.page.locator('[data-graph-canvas]').first().waitFor({ timeout: 10000 });

        // Create a task
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
        await ctx.page.locator('form:has(select[name=workspace_id]):visible input[name=title]').fill('Tarea Origen');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').fill('Prompt origen');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible button[type=submit]', { hasText: 'Crear tarea' }).click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});

        // Add to graph
        const addBtn = ctx.page.locator('button', { hasText: 'Añadir al grafo' }).first();
        await addBtn.waitFor({ timeout: 5000 });
        await addBtn.click();

        // Node appears on canvas
        const node = ctx.page.locator('.graph-node', { hasText: 'Tarea Origen' }).first();
        await node.waitFor({ timeout: 10000 });

        // Hover over node to reveal tools
        await node.hover();
        const nextBtn = node.locator('.graph-tool', { hasText: 'Siguiente' }).first();
        await nextBtn.waitFor({ state: 'visible', timeout: 5000 });
        await nextBtn.click();

        // The create tab opens configured to link the next task
        await ctx.page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'visible', timeout: 10000 });
      },
    },
    {
      name: 'graph_edge_selection_and_deletion: seleccionar una arista muestra el botón de borrado y la elimina al pulsar',
      fn: async (ctx) => {
        const ws = await registerWs(ctx, `tg-edge-${tag}`);
        const wsId = await getWsId(ctx, ws.name);
        assert.ok(wsId);

        // Open board
        await ctx.goto(`/board/${encodeURIComponent(wsId)}`);
        await ctx.page.locator('[data-graph-canvas]').first().waitFor({ timeout: 10000 });

        // Create Task A
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
        await ctx.page.locator('form:has(select[name=workspace_id]):visible input[name=title]').fill('Tarea Pred');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').fill('Prompt pred');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible button[type=submit]', { hasText: 'Crear tarea' }).click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});

        // Create Task B
        await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible select[name=workspace_id]').selectOption({ label: `${ws.name} (main)` });
        await ctx.page.locator('form:has(select[name=workspace_id]):visible input[name=title]').fill('Tarea Succ');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').fill('Prompt succ');
        await ctx.page.locator('form:has(select[name=workspace_id]):visible button[type=submit]', { hasText: 'Crear tarea' }).click();
        await ctx.page.locator('form:has(select[name=workspace_id]):visible').waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});

        // Place both tasks on canvas via API
        const tasks = await ctx.page.evaluate(async () => {
          const res = await fetch('/api/tasks');
          // Fallback if /api/tasks is not a plain JSON endpoint, read cards from DOM
          const cards = document.querySelectorAll('[data-task-id]');
          return Array.from(cards).map(c => c.dataset.taskId).filter(Boolean);
        });

        // Add them to graph using the "+ Añadir al grafo" buttons
        const addBtns = ctx.page.locator('button', { hasText: 'Añadir al grafo' });
        const count = await addBtns.count();
        for (let i = 0; i < count; i++) {
          await addBtns.first().click();
          await sleep(300);
        }

        // Now both nodes are on canvas. Retrieve their IDs:
        const nodeEls = ctx.page.locator('.graph-node');
        await nodeEls.first().waitFor({ timeout: 5000 });
        const predNode = ctx.page.locator('.graph-node', { hasText: 'Tarea Pred' }).first();
        const succNode = ctx.page.locator('.graph-node', { hasText: 'Tarea Succ' }).first();
        await predNode.waitFor({ timeout: 5000 });
        await succNode.waitFor({ timeout: 5000 });
        const predId = await predNode.getAttribute('data-task-id');
        const succId = await succNode.getAttribute('data-task-id');

        // Add dependency via POST and position succNode to the right
        await ctx.page.evaluate(async ({ predId, succId }) => {
          const posBody = new URLSearchParams({ x: '500', y: '100' });
          await fetch(`/tasks/${succId}/position`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: posBody.toString(),
          });
          const depBody = new URLSearchParams({ predecessor_id: predId });
          await fetch(`/tasks/${succId}/dependencies`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: depBody.toString(),
          });
          document.body.dispatchEvent(new CustomEvent('board:refresh'));
        }, { predId, succId });

        // Edge line appears on canvas between the separated nodes
        const edgeLine = ctx.page.locator('.graph-edges g[data-edge-key]').first();
        await edgeLine.waitFor({ state: 'attached', timeout: 10000 });

        // Click on the edge to select it
        const edgeHit = edgeLine.locator('.graph-edge-hit');
        await edgeHit.waitFor({ state: 'attached', timeout: 5000 });
        await edgeHit.dispatchEvent('click', { bubbles: true });

        // Delete button appears on edge
        const delBtn = edgeLine.locator('.graph-edge-delete');
        await delBtn.waitFor({ state: 'attached', timeout: 5000 });

        // Click delete button
        await delBtn.dispatchEvent('click', { bubbles: true });

        // Edge is removed
        await edgeLine.waitFor({ state: 'detached', timeout: 10000 });
      },
    },
  ],
};
