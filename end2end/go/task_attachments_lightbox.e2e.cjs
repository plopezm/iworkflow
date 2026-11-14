// end2end/go/task_attachments_lightbox.e2e.cjs
// The images of a running task (Acciones tab, "Adjuntos") open enlarged on click.
// Screenshots: set E2E_SHOTS_DIR (default /tmp/iwf-attachments-lightbox-shots).
const fs = require('fs');
const path = require('path');
const zlib = require('zlib');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHOTS = process.env.E2E_SHOTS_DIR || '/tmp/iwf-attachments-lightbox-shots';

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

function crc32(buf) {
  let c;
  let crc = 0xffffffff;
  for (const b of buf) {
    c = (crc ^ b) & 0xff;
    for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    crc = (crc >>> 8) ^ c;
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function chunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(body));
  return Buffer.concat([len, body, crc]);
}

// A WxH gradient PNG, big enough for the enlarged view to differ visibly from the 80px thumbnail.
function gradientPng(w, h) {
  const header = Buffer.alloc(13);
  header.writeUInt32BE(w, 0);
  header.writeUInt32BE(h, 4);
  header[8] = 8; // bit depth
  header[9] = 2; // RGB
  const raw = Buffer.alloc((w * 3 + 1) * h);
  for (let y = 0; y < h; y += 1) {
    const row = y * (w * 3 + 1);
    for (let x = 0; x < w; x += 1) {
      raw[row + 1 + x * 3] = Math.floor((x / w) * 255);
      raw[row + 2 + x * 3] = Math.floor((y / h) * 255);
      raw[row + 3 + x * 3] = 160;
    }
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', header),
    chunk('IDAT', zlib.deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `lightbox-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `lightbox ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

// A running task (fake agent) created with one image; leaves the page on its Acciones tab.
async function runningTaskWithImage(ctx, key, imageName) {
  const page = ctx.page;
  const ws = `lb-${key}-${tag}`;
  const dir = uniqueGitRepo(key);
  await ctx.goto('/workspaces');
  await page.getByLabel('Nombre').first().fill(ws);
  await page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await page.getByLabel(/Rama base/).first().fill('main');
  await page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await page.getByText(dir).first().waitFor({ timeout: 12000 });

  await ctx.goto('/');
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await page.locator('aside').getByRole('link', { name: new RegExp(ws) }).first().click();
  await sleep(500);

  const prompt = `lightbox-${key}-${tag}`;
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.getByLabel(/^Workspace/).selectOption({ label: `${ws} (main)` });
  await page.getByLabel('Prompt').fill(prompt);
  await page.locator('input[type=file]').first().setInputFiles({ name: imageName, mimeType: 'image/png', buffer: gradientPng(640, 400) });
  await page.getByAltText(imageName).first().waitFor({ timeout: 10000 });
  await page.getByLabel('Iniciar ahora').check();
  await page.getByRole('button', { name: 'Crear tarea' }).click();
  await page.getByRole('button', { name: 'Crear tarea' }).waitFor({ state: 'detached', timeout: 15000 });
  await sleep(2000);

  await ctx.goto('/');
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).click();
  await page.locator('aside').getByRole('link', { name: new RegExp(ws) }).first().click();
  const link = page.locator('[data-task-id]:visible', { hasText: prompt }).locator('a[href*="/tasks/"]').first();
  await link.waitFor({ timeout: 15000 });
  const id = ((await link.getAttribute('href')).match(/\/tasks\/([0-9a-f-]{36})/) || [])[1];
  ctx.assert(id, 'no task id');
  await ctx.goto(`/tasks/${id}`);
  await page.getByText(`eco: ${prompt}`).first().waitFor({ timeout: 25000 });
  await page.getByRole('tab', { name: 'Acciones' }).first().click();
  await sleep(300);
  return { id, prompt };
}

module.exports = {
  name: 'task_attachments_lightbox',
  tests: [
    {
      name: 'una miniatura de Adjuntos se amplía al hacer clic y se cierra con Escape, con la X y con el fondo',
      fn: async (ctx) => {
        const page = ctx.page;
        const imageName = `captura-${tag}.png`;
        await runningTaskWithImage(ctx, 'open', imageName);

        const thumb = page.locator('[data-image-lightbox]').first();
        await thumb.waitFor({ timeout: 15000 });
        const dialog = page.locator('[data-lightbox]').first();
        ctx.assert(!(await dialog.isVisible()), 'el visor debe empezar cerrado');
        await shot(ctx, '1-running-task-attachments-thumbnail');

        const before = page.url();
        await thumb.click();
        await dialog.waitFor({ state: 'visible', timeout: 5000 });
        const big = dialog.locator('[data-lightbox-img]');
        ctx.assert((await big.getAttribute('alt')) === imageName, 'el visor muestra la imagen pulsada');
        await page.waitForFunction(() => {
          const img = document.querySelector('[data-lightbox] [data-lightbox-img]');
          return img && img.complete && img.naturalWidth === 640;
        }, null, { timeout: 5000 });
        const box = await big.boundingBox();
        ctx.assert(box && box.width > 200, `la imagen ampliada debe ser mayor que la miniatura (${box && box.width}px)`);
        ctx.assert(page.url() === before, 'el clic no navega ni abre otra pestaña');
        await sleep(600); // let the dialog's entry animation finish before the screenshot
        await shot(ctx, '2-lightbox-open');

        await page.keyboard.press('Escape');
        await dialog.waitFor({ state: 'hidden', timeout: 5000 });

        await thumb.click();
        await dialog.waitFor({ state: 'visible', timeout: 5000 });
        await dialog.locator('[data-modal-close]').click();
        await dialog.waitFor({ state: 'hidden', timeout: 5000 });

        await thumb.click();
        await dialog.waitFor({ state: 'visible', timeout: 5000 });
        await dialog.click({ position: { x: 5, y: 5 } });
        await dialog.waitFor({ state: 'hidden', timeout: 5000 });
        await shot(ctx, '3-lightbox-closed');
      },
    },
    {
      name: 'Ctrl+clic en una miniatura no abre el visor: conserva el enlace de la imagen',
      fn: async (ctx) => {
        const page = ctx.page;
        await runningTaskWithImage(ctx, 'mod', `mod-${tag}.png`);
        const thumb = page.locator('[data-image-lightbox]').first();
        await thumb.waitFor({ timeout: 15000 });
        const popup = page.context().waitForEvent('page', { timeout: 5000 });
        await thumb.click({ modifiers: ['Control'] });
        const opened = await popup;
        ctx.assert(/\/api\/tasks\/[0-9a-f-]{36}\/attachments\//.test(opened.url()), `se abre la imagen en otra pestaña (${opened.url()})`);
        ctx.assert(!(await page.locator('[data-lightbox]').first().isVisible()), 'el visor sigue cerrado');
        await opened.close();
      },
    },
  ],
};
