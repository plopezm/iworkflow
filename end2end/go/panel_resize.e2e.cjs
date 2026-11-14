// end2end/go/panel_resize.e2e.cjs
// The desktop side panels (the sidebar on the left, the task panel on the right) are resized by dragging their
// inner edge; the width survives a reload and double click restores the default one.
// Screenshots: set E2E_SHOTS_DIR (default /tmp/iwf-panel-resize-shots).
const fs = require('fs');
const path = require('path');
const { taskDetail } = require('./tasks_actions.e2e.cjs').helpers;

const tag = Date.now().toString();
const SHOTS = process.env.E2E_SHOTS_DIR || '/tmp/iwf-panel-resize-shots';

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

const asideOf = (ctx, side) => ctx.page.locator(`[data-collapse-root]:has([data-resize-handle][data-side="${side}"]):visible`).last().locator(':scope > aside');
const handleOf = (ctx, side) => ctx.page.locator(`[data-resize-handle][data-side="${side}"]:visible`).last();
const widthOf = async (aside) => Math.round((await aside.boundingBox()).width);

// Drags the handle by dx pixels with the real mouse.
async function drag(ctx, handle, dx) {
  const box = await handle.boundingBox();
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 4; // the collapse button covers the middle of the edge
  await ctx.page.mouse.move(x, y);
  await ctx.page.mouse.down();
  await ctx.page.mouse.move(x + dx / 2, y, { steps: 5 });
  await ctx.page.mouse.move(x + dx, y, { steps: 5 });
  await ctx.page.mouse.up();
}

module.exports = {
  name: 'panel_resize',
  tests: [
    {
      name: 'panel_resize: the sidebar grows and shrinks by dragging, keeps its width after a reload and double click resets it',
      fn: async (ctx) => {
        await ctx.goto('/');
        await ctx.page.evaluate(() => localStorage.removeItem('iwf-panel-width-left'));
        await ctx.goto('/');
        const aside = asideOf(ctx, 'left');
        const initial = await widthOf(aside);
        await shot(ctx, '1-sidebar-default');

        await drag(ctx, handleOf(ctx, 'left'), 100);
        const wider = await widthOf(aside);
        ctx.assert(Math.abs(wider - (initial + 100)) <= 2, `el sidebar debía crecer 100px: ${initial} -> ${wider}`);
        await shot(ctx, '2-sidebar-wider');

        await drag(ctx, handleOf(ctx, 'left'), -1000);
        ctx.assert((await widthOf(aside)) === 200, 'el ancho mínimo del sidebar es 200px');

        await drag(ctx, handleOf(ctx, 'left'), 1500);
        const max = await widthOf(aside);
        ctx.assert(max <= 720 && max <= 1440 / 2, `el ancho máximo se respeta: ${max}`);

        await drag(ctx, handleOf(ctx, 'left'), -(max - 360));
        const kept = await widthOf(aside);
        await ctx.goto('/');
        ctx.assert(Math.abs((await widthOf(asideOf(ctx, 'left'))) - kept) <= 2, 'el ancho del sidebar debe sobrevivir a la recarga');
        ctx.assert((await ctx.page.evaluate(() => localStorage.getItem('iwf-panel-width-left'))) !== null, 'el ancho se guarda en localStorage');

        await handleOf(ctx, 'left').dblclick({ position: { x: 3, y: 200 } });
        ctx.assert(Math.abs((await widthOf(asideOf(ctx, 'left'))) - initial) <= 2, 'doble clic restaura el ancho por defecto');
        ctx.assert((await ctx.page.evaluate(() => localStorage.getItem('iwf-panel-width-left'))) === null, 'doble clic borra el ancho guardado');
      },
    },
    {
      name: 'panel_resize: the task panel on the right resizes toward the left, with the keyboard too',
      fn: async (ctx) => {
        await taskDetail(ctx, 'panelresize', `panel-${tag}`, { start: true, match: `panel-${tag}` });
        await ctx.page.evaluate(() => localStorage.removeItem('iwf-panel-width-right'));
        const aside = asideOf(ctx, 'right');
        await aside.waitFor({ timeout: 15000 });
        const initial = await widthOf(aside);
        await shot(ctx, '3-task-panel-default');

        await drag(ctx, handleOf(ctx, 'right'), -120);
        const wider = await widthOf(aside);
        ctx.assert(Math.abs(wider - (initial + 120)) <= 2, `el panel derecho debía crecer hacia la izquierda: ${initial} -> ${wider}`);
        await shot(ctx, '4-task-panel-wider');

        await handleOf(ctx, 'right').focus();
        await ctx.page.keyboard.press('ArrowRight');
        ctx.assert((await widthOf(aside)) === wider - 16, 'ArrowRight encoge el panel derecho 16px');
        await ctx.page.keyboard.press('ArrowLeft');
        await ctx.page.keyboard.press('ArrowLeft');
        ctx.assert((await widthOf(aside)) === wider + 16, 'ArrowLeft agranda el panel derecho 16px');
        await shot(ctx, '5-task-panel-keyboard');

        // Both panels resized at once.
        await drag(ctx, handleOf(ctx, 'left'), 60);
        await shot(ctx, '6-both-panels-resized');
        await ctx.page.evaluate(() => { localStorage.removeItem('iwf-panel-width-left'); localStorage.removeItem('iwf-panel-width-right'); });
      },
    },
  ],
};
