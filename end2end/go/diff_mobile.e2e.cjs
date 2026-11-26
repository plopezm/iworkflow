// end2end/go/diff_mobile.e2e.cjs
// The unified diff on a phone: the line number columns and the sign are narrow, so the code keeps the width, and
// the page does not scroll sideways. On a desktop the columns keep their normal width.
// Screenshots: set E2E_SHOTS_DIR (default /tmp/iwf-diff-mobile-shots).
const fs = require('fs');
const path = require('path');
const os = require('os');
const { execFileSync } = require('child_process');
const { finishedTask, openActions } = require('./tasks_actions.e2e.cjs').helpers;

const SHOTS = process.env.E2E_SHOTS_DIR || '/tmp/iwf-diff-mobile-shots';

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  const abs = path.join(SHOTS, `${name}.png`);
  await ctx.screenshot(abs);
  console.log(`  screenshot: ${abs}`);
}

// Commits more changes in the worktree of the task, so the diff has two files: CAMBIO.md (the agent's) and a
// modified README.md with a deletion, context-free additions and a second file with three-digit line numbers.
function addChanges(ws) {
  const tmp = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || os.tmpdir();
  const repoName = fs.readdirSync(tmp).find((n) => n.startsWith(`actions-repo-${ws}-`) && !n.endsWith('.git'));
  const repo = path.join(tmp, repoName);
  const out = execFileSync('git', ['-C', repo, 'worktree', 'list', '--porcelain'], { encoding: 'utf8' });
  const trees = out.split('\n').filter((l) => l.startsWith('worktree ')).map((l) => l.slice(9));
  const wt = trees.find((t) => t !== repo);
  const lines = Array.from({ length: 14 }, (_, i) => `README linea nueva ${i + 1}`);
  fs.writeFileSync(path.join(wt, 'README.md'), lines.join('\n') + '\n');
  const long = Array.from({ length: 120 }, (_, i) => `const value${i + 1} = ${i + 1};`);
  fs.writeFileSync(path.join(wt, 'LARGO.js'), long.join('\n') + '\n');
  const git = (...a) => execFileSync('git', ['-C', wt, '-c', 'user.name=t', '-c', 'user.email=t@l', ...a], { stdio: 'ignore' });
  git('add', '.');
  git('commit', '-qm', 'more changes');
}

// Width of the first line number cell, the sign cell and the code cell of the unified table.
const widths = (ctx) => ctx.page.evaluate(() => {
  const w = (sel) => document.querySelector(sel)?.getBoundingClientRect().width ?? 0;
  return { num: w('.diff-table td.diff-num'), sign: w('.diff-table td.diff-sign'), code: w('.diff-table td.diff-cell-add, .diff-table td.diff-cell') };
});

module.exports = {
  name: 'diff_mobile',
  tests: [
    {
      name: 'diff_mobile: the unified diff gives the code more room on a phone and does not overflow the page',
      fn: async (ctx) => {
        const task = await finishedTask(ctx, 'diffmobile');
        addChanges(task.ws);
        await openActions(ctx);
        await ctx.page.locator('a[href$="/diff"]').first().click();
        await ctx.page.waitForURL(/\/tasks\/[0-9a-f-]{36}\/diff/, { timeout: 15000 });
        await ctx.page.getByRole('button', { name: 'Unificado' }).first().click();
        await ctx.page.locator('.diff-card-header', { hasText: 'LARGO.js' }).first().waitFor({ timeout: 20000 });
        await ctx.page.locator('.diff-table td.diff-num').first().waitFor({ timeout: 20000 });

        const desktop = await widths(ctx);
        ctx.assert(desktop.num >= 36, `en escritorio el número de línea mantiene su ancho: ${desktop.num}px`);
        await shot(ctx, '1-unified-desktop');

        await ctx.page.setViewportSize({ width: 390, height: 844 });
        await ctx.page.waitForTimeout(400);
        const phone = await widths(ctx);
        ctx.assert(phone.num <= 28, `en móvil el número de línea debe medir 28px o menos: ${phone.num}px`);
        ctx.assert(phone.num < desktop.num, `el número de línea debe ser más estrecho en móvil: ${phone.num} vs ${desktop.num}`);
        ctx.assert(phone.sign <= 14, `en móvil el signo debe medir 14px o menos: ${phone.sign}px`);
        const overflow = await ctx.page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
        ctx.assert(overflow <= 0, `la página no debe desbordar en horizontal: ${overflow}px`);
        // The modified README.md has a deleted line and added ones: the phone shows one number column for both.
        const readme = ctx.page.locator('.diff-card', { hasText: 'README.md' }).last();
        const showReadme = async () => {
          await readme.scrollIntoViewIfNeeded();
          await readme.evaluate((el) => el.scrollIntoView({ block: 'start' }));
          await ctx.page.waitForTimeout(200);
        };
        const OLD_LOOK = '.diff-num{width:2.5rem!important;min-width:2.5rem!important;padding:0 .5rem!important;font-size:11px!important}'
          + '.diff-table-unified .diff-num-old{display:table-cell!important}.diff-table-unified .diff-num-fallback{display:none!important}'
          + '.diff-sign{width:1rem!important}.diff-cell{padding:0 .5rem!important}';
        const style = await ctx.page.addStyleTag({ content: OLD_LOOK });
        await showReadme();
        await shot(ctx, '2a-phone-before-two-number-columns');
        await style.evaluate((el) => el.remove());
        await showReadme();
        await shot(ctx, '2b-phone-after-one-number-column');

        const cols = await readme.evaluate((el) => {
          const row = [...el.querySelectorAll('.diff-table-unified tr')].find((r) => r.querySelector('.diff-cell-del'));
          const vis = (sel) => getComputedStyle(row.querySelector(sel)).display;
          return { old: vis('.diff-num-old'), fallback: vis('.diff-num-fallback'), fallbackText: row.querySelector('.diff-num-fallback').textContent };
        });
        ctx.assert(cols.old === 'none', `la columna del número antiguo se oculta en móvil: ${cols.old}`);
        ctx.assert(cols.fallback !== 'none' && cols.fallbackText === '1', `una línea borrada muestra su número antiguo: ${JSON.stringify(cols)}`);
        await ctx.page.locator('.diff-card-header', { hasText: 'LARGO.js' }).first().scrollIntoViewIfNeeded();
        ctx.assert((await widths(ctx)).num <= 28, 'con números de tres cifras sigue estrecho');

        // The vertical layout is intact: the cards stack and the page still scrolls down.
        const scrolls = await ctx.page.evaluate(() => document.documentElement.scrollHeight >= window.innerHeight);
        ctx.assert(scrolls, 'la vista vertical debe seguir intacta');
      },
    },
  ],
};
