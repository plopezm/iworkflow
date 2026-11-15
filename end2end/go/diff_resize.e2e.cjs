// end2end/go/diff_resize.e2e.cjs
// The changed-files tree of the diff tab is resized by dragging the separator between it and the file cards, like
// the side panels: the width survives a layout swap and a reload, the keyboard moves it and double click resets it.
// Screenshots: set E2E_SHOTS_DIR (default /tmp/iwf-diff-resize-shots).
const fs = require('fs');
const path = require('path');
const { finishedTask, openActions } = require('./tasks_actions.e2e.cjs').helpers;

const SHOTS = process.env.E2E_SHOTS_DIR || '/tmp/iwf-diff-resize-shots';
const KEY = 'iwf.diff.tree.width';

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

const sidebar = (ctx) => ctx.page.locator('[data-diff-sidebar]:visible').first();
const handle = (ctx) => ctx.page.locator('[data-diff-resize-handle]:visible').first();
const widthOf = async (ctx) => Math.round((await sidebar(ctx).boundingBox()).width);

async function drag(ctx, dx) {
  const box = await handle(ctx).boundingBox();
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 4;
  await ctx.page.mouse.move(x, y);
  await ctx.page.mouse.down();
  await ctx.page.mouse.move(x + dx / 2, y, { steps: 5 });
  await ctx.page.mouse.move(x + dx, y, { steps: 5 });
  await ctx.page.mouse.up();
}

async function openDiff(ctx) {
  await finishedTask(ctx, 'diffresize');
  await openActions(ctx);
  await ctx.page.locator('a[href$="/diff"]').first().click();
  await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}\/diff/, { timeout: 15000 });
  await sidebar(ctx).waitFor({ timeout: 20000 });
}

module.exports = {
  name: 'diff_resize',
  tests: [
    {
      name: 'diff_resize: the file tree resizes by dragging, with the keyboard, survives a layout swap and a reload, and double click resets it',
      fn: async (ctx) => {
        await openDiff(ctx);
        await ctx.page.evaluate((k) => localStorage.removeItem(k), KEY);
        const initial = await widthOf(ctx);
        await shot(ctx, '1-tree-default');

        await drag(ctx, 100);
        const wider = await widthOf(ctx);
        ctx.assert(Math.abs(wider - (initial + 100)) <= 2, `el árbol debía crecer 100px: ${initial} -> ${wider}`);
        await shot(ctx, '2-tree-wider');

        await drag(ctx, -1000);
        ctx.assert((await widthOf(ctx)) === 200, 'el ancho mínimo del árbol es 200px');
        await shot(ctx, '3-tree-minimum');

        await drag(ctx, 1500);
        const max = await widthOf(ctx);
        ctx.assert(max <= 720, `el ancho máximo se respeta: ${max}`);

        await drag(ctx, -(max - 340));
        const kept = await widthOf(ctx);
        ctx.assert((await ctx.page.evaluate((k) => localStorage.getItem(k), KEY)) !== null, 'el ancho se guarda en localStorage');

        // A layout button swaps the whole diff view: the width must come back.
        await ctx.page.getByRole('button', { name: 'Dividido' }).first().click();
        await ctx.page.waitForTimeout(800);
        await sidebar(ctx).waitFor({ timeout: 10000 });
        ctx.assert(Math.abs((await widthOf(ctx)) - kept) <= 2, 'el ancho debe sobrevivir al cambio Dividido/Unificado');
        await shot(ctx, '4-after-split-swap');

        await ctx.page.reload();
        await sidebar(ctx).waitFor({ timeout: 15000 });
        // The stored width is put back right after the tab settles, so wait for it instead of reading at once.
        await ctx.page.waitForFunction((w) => Math.abs(document.querySelector('[data-diff-sidebar]')?.getBoundingClientRect().width - w) <= 2, kept, { timeout: 5000 })
          .catch(() => {});
        ctx.assert(Math.abs((await widthOf(ctx)) - kept) <= 2, 'el ancho debe sobrevivir a la recarga');
        await shot(ctx, '5-after-reload');

        await handle(ctx).focus();
        await ctx.page.keyboard.press('ArrowRight');
        ctx.assert((await widthOf(ctx)) === kept + 16, 'ArrowRight agranda el árbol 16px');
        await ctx.page.keyboard.press('ArrowLeft');
        await ctx.page.keyboard.press('ArrowLeft');
        ctx.assert((await widthOf(ctx)) === kept - 16, 'ArrowLeft encoge el árbol 16px');
        await shot(ctx, '6-tree-keyboard');

        await handle(ctx).dblclick({ position: { x: 2, y: 100 } });
        ctx.assert(Math.abs((await widthOf(ctx)) - initial) <= 2, 'doble clic restaura el ancho por defecto');
        ctx.assert((await ctx.page.evaluate((k) => localStorage.getItem(k), KEY)) === null, 'doble clic borra el ancho guardado');
        await shot(ctx, '7-tree-reset');
      },
    },
  ],
};
