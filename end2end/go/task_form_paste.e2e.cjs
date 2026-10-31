// end2end/go/task_form_paste.e2e.cjs
// Pasting images into the prompt of the create dialog (web/static/js/task_form.js, Rust parity: image_paste).
// An image on the clipboard joins the file input of the dialog and is previewed; plain text pastes as usual.
// The suite runs in Spanish (the language cookie of the harness).
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const PNG_BASE64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `paste-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `paste ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Registers a workspace, opens the board for it and opens the create dialog.
async function openDialog(ctx, name) {
  const dir = uniqueGitRepo(name);
  await ctx.goto('/workspaces');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
  await sleep(300);
  await ctx.page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await ctx.page.locator('form:has(select[name=workspace_id]):visible textarea[name=prompt]').waitFor({ timeout: 10000 });
}

// Dispatches a paste on the prompt with the given clipboard files and text; returns whether the script took it.
function paste(ctx, files, text) {
  return ctx.page.evaluate(({ files, text, base64 }) => {
    const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
    const data = new DataTransfer();
    for (const name of files) data.items.add(new File([bytes], name, { type: 'image/png' }));
    if (text) data.setData('text/plain', text);
    const event = new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true });
    document.querySelector('form:has(select[name=workspace_id]):visible textarea[name=prompt]').dispatchEvent(event);
    return event.defaultPrevented;
  }, { files, text, base64: PNG_BASE64 });
}

module.exports = {
  name: 'task_form_paste',
  tests: [
    {
      name: 'pegar una imagen en el prompt la añade al selector y la previsualiza',
      fn: async (ctx) => {
        await openDialog(ctx, `tp-image-${tag}`);
        const taken = await paste(ctx, [`pegada-${tag}.png`], '');
        ctx.assert(taken, 'el pegado de una imagen debe interceptarse');
        await ctx.page.locator(`form:has(select[name=workspace_id]):visible img[alt="pegada-${tag}.png"]`).first().waitFor({ timeout: 10000 });
        const count = await ctx.page.locator('form:has(select[name=workspace_id]):visible input[type=file]').first().evaluate((input) => input.files.length);
        ctx.assert(count === 1, `el selector debe tener 1 imagen y tiene ${count}`);
      },
    },
    {
      name: 'pegar otra imagen conserva las anteriores',
      fn: async (ctx) => {
        await openDialog(ctx, `tp-two-${tag}`);
        await paste(ctx, [`uno-${tag}.png`], '');
        await paste(ctx, [`dos-${tag}.png`], '');
        await ctx.page.locator(`form:has(select[name=workspace_id]):visible img[alt="dos-${tag}.png"]`).first().waitFor({ timeout: 10000 });
        const count = await ctx.page.locator('form:has(select[name=workspace_id]):visible input[type=file]').first().evaluate((input) => input.files.length);
        ctx.assert(count === 2, `el selector debe tener 2 imágenes y tiene ${count}`);
      },
    },
    {
      name: 'pegar solo texto no se intercepta',
      fn: async (ctx) => {
        await openDialog(ctx, `tp-text-${tag}`);
        const taken = await paste(ctx, [], 'texto normal');
        ctx.assert(!taken, 'el pegado de texto debe seguir su curso normal');
        const count = await ctx.page.locator('form:has(select[name=workspace_id]):visible input[type=file]').first().evaluate((input) => input.files.length);
        ctx.assert(count === 0, `no debe añadirse ninguna imagen y hay ${count}`);
      },
    },
  ],
};
