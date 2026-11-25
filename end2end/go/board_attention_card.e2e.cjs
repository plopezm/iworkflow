// end2end/go/board_attention_card.e2e.cjs
// The dashboard card of a task that needs attention keeps the colour of its project visible: no red border, no red
// background, no red dot in the "Necesita atención" line or badge. The text stays red. A fake agent goes idle after the
// first turn, which is what puts the task in the attention stage. Screenshots go to SMOKE_SCREENSHOT_DIR.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.SMOKE_SCREENSHOT_DIR || '/tmp/iwf-e2e-screenshots';
const NAME = `atencion-${tag}`;

function gitRepo() {
  const dir = path.join(TMP, `attention-card-repo-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), 'attention card\n');
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

module.exports = {
  name: 'board_attention_card',
  tests: [
    {
      name: 'atencion: la tarjeta que necesita atención no lleva borde, fondo ni punto rojos y conserva el color del proyecto',
      fn: async (ctx) => {
        const { page } = ctx;
        fs.mkdirSync(SHOTS, { recursive: true });
        const dir = gitRepo();

        await ctx.goto('/workspaces/new');
        await page.getByLabel('Nombre').first().fill(NAME);
        await page.getByLabel(/^Ruta absoluta/).first().fill(dir);
        await page.getByLabel(/Rama base/).first().fill('main');
        await page.locator('[data-testid="workspace-color-picker"] input[name="color"][value="teal"]:not([data-ws-custom-radio])').check();
        await page.getByRole('button', { name: /Registrar|Register/ }).first().click();
        await page.getByText(dir).first().waitFor({ timeout: 12000 });

        await ctx.goto('/');
        await page.locator('#board-sidebar').getByRole('link', { name: NAME }).first().click();
        await page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
        await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
        const form = 'form:has(select[name=workspace_id]):visible';
        await page.locator(`${form} select[name=workspace_id]`).selectOption({ label: `${NAME} (main)` });
        await page.locator(`${form} textarea[name=prompt]`).fill(`attention-card-${tag}`);
        await page.getByRole('checkbox', { name: /Iniciar ahora/ }).first().check();
        await page.locator(`${form} button[type=submit]`, { hasText: 'Crear tarea' }).click();
        await page.locator(form).waitFor({ state: 'detached', timeout: 10000 }).catch(() => {});

        await ctx.goto('/');
        await page.locator('a.sidebar-nav[data-dock-path="/"]').first().click();
        await page.locator('[data-graph-canvas]').first().waitFor({ timeout: 10000 });
        const card = page.locator('article[data-dashboard-task]', { hasText: NAME }).first();
        await card.getByText('Necesita atención').first().waitFor({ timeout: 30000 }).catch(async (error) => {
          await page.screenshot({ path: path.join(SHOTS, 'board-attention-debug.png') });
          const cards = await page.locator('article[data-dashboard-task]').evaluateAll((els) => els.map((el) => `${el.dataset.stage}: ${el.innerText.slice(0, 120)}`));
          throw new Error(`${error.message.split('\n')[0]} | tarjetas: ${JSON.stringify(cards)} | url ${page.url()}`);
        });

        const look = await card.evaluate((el) => {
          const cs = getComputedStyle(el);
          return {
            cls: el.className,
            border: cs.borderTopColor,
            bg: cs.backgroundColor,
            rail: getComputedStyle(el, '::before').backgroundColor,
            redDots: el.querySelectorAll('.live-dot.bg-error').length,
            badgeDot: (() => {
              const badge = el.querySelector('.badge-attention');
              return badge ? getComputedStyle(badge, '::before').display : 'none';
            })(),
          };
        });
        ctx.assert(!/border-error|bg-error/.test(look.cls.replace(/\bgroup\b/, '')), `la tarjeta no debe llevar clases de error rojo: ${look.cls}`);
        ctx.assert(look.redDots === 0, `no debe haber puntos rojos en la tarjeta (hay ${look.redDots})`);
        ctx.assert(look.badgeDot === 'none', `la etiqueta no debe llevar punto (display ${look.badgeDot})`);
        ctx.assert(look.rail !== 'rgba(0, 0, 0, 0)', `la barra con el color del proyecto debe verse (${look.rail})`);

        await page.screenshot({ path: path.join(SHOTS, 'board-attention-card.png') });
        await card.screenshot({ path: path.join(SHOTS, 'board-attention-card-only.png') });
      },
    },
  ],
};
