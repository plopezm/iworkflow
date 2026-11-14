// end2end/go/chat_links.e2e.cjs
// URLs and paths of the disk in the agent chat are links: URLs open in a new tab, paths are asked to the server.
// The fake agent echoes the prompt ("eco: <prompt>"), so the prompt carries the links the agent "writes".
// Screenshots: set E2E_SHOTS_DIR (default /tmp/iwf-chat-links-shots).
const fs = require('fs');
const path = require('path');
const { taskDetail } = require('./tasks_actions.e2e.cjs').helpers;

const tag = Date.now().toString();
const SHOTS = process.env.E2E_SHOTS_DIR || '/tmp/iwf-chat-links-shots';
let runs = 0;
const WINDOWS_PATH = 'C:\\Users\\me\\app\\main.go';
const PROMPT = [
  `enlaces-${tag}:`,
  'web https://example.com/docs/intro,',
  'sin esquema www.example.org,',
  'ficheros ./README.md:1 y ./no-existe.txt y ./run.sh,',
  'absoluto /etc/hostname, home ~/notas/a.md,',
  `windows \`${WINDOWS_PATH}\` y \`\\\\server\\share\\doc.txt\`,`,
  'prosa y/o 1/2 sin enlace.',
].join(' ');

async function shot(ctx, name) {
  fs.mkdirSync(SHOTS, { recursive: true });
  await ctx.screenshot(path.join(SHOTS, `${name}.png`));
}

// Opens a task whose agent message holds the links and waits for them to be rendered.
async function chatWithLinks(ctx) {
  runs += 1;
  // Each test registers its own workspace (one git repo per key), and the card is found by the start of the prompt.
  const t = await taskDetail(ctx, `chatlinks${runs}`, PROMPT, { start: true, match: `enlaces-${tag}` });
  const links = ctx.page.locator('.msg-assistant .markdown a');
  await links.first().waitFor({ timeout: 30000 });
  return t;
}

const fileLink = (ctx, value) => ctx.page.locator(`.msg-assistant .markdown a[data-file-path="${value}"]`);

module.exports = {
  name: 'chat_links',
  tests: [
    {
      name: 'chat links: URLs open in a new tab and every kind of path is highlighted as a link',
      fn: async (ctx) => {
        await chatWithLinks(ctx);
        const external = ctx.page.locator('.msg-assistant .markdown a[target="_blank"]');
        const hrefs = await external.evaluateAll((els) => els.map((a) => a.getAttribute('href')));
        ctx.assert(hrefs.includes('https://example.com/docs/intro'), `falta la URL con esquema: ${hrefs}`);
        ctx.assert(hrefs.includes('http://www.example.org'), `falta la URL www: ${hrefs}`);
        ctx.assert(
          (await external.first().getAttribute('rel')) === 'noopener noreferrer',
          'los enlaces externos deben llevar rel="noopener noreferrer"',
        );
        const paths = await ctx.page.locator('.msg-assistant .markdown a.file-link')
          .evaluateAll((els) => els.map((a) => a.getAttribute('data-file-path')));
        for (const expected of [
          './README.md:1', './no-existe.txt', './run.sh', '/etc/hostname', '~/notas/a.md',
          WINDOWS_PATH, '\\\\server\\share\\doc.txt',
        ]) {
          ctx.assert(paths.includes(expected), `ruta sin enlace: ${expected} (hay: ${JSON.stringify(paths)})`);
        }
        ctx.assert(paths.length === 7, `la prosa "y/o 1/2" no debe enlazarse: ${JSON.stringify(paths)}`);
        const decoration = await fileLink(ctx, './README.md:1').evaluate((a) => getComputedStyle(a).textDecorationLine);
        ctx.assert(decoration.includes('underline'), 'el enlace de fichero debe estar resaltado (subrayado)');
        const color = (locator) => locator.evaluate((a) => getComputedStyle(a).color);
        const urlColor = await color(external.first());
        const fileColor = await color(fileLink(ctx, './README.md:1'));
        const textColor = await ctx.page.locator('.msg-assistant .markdown p').first().evaluate((p) => getComputedStyle(p).color);
        ctx.assert(urlColor !== textColor && fileColor !== textColor, `los enlaces deben tener color propio (${urlColor}, ${fileColor} vs ${textColor})`);
        ctx.assert(urlColor !== fileColor, 'las URLs y las rutas deben distinguirse por color');
        await shot(ctx, '1-chat-links-highlighted');
      },
    },
    {
      name: 'chat links: clicking a path asks the server to open it, relative to the worktree, and stays on the page',
      fn: async (ctx) => {
        await chatWithLinks(ctx);
        const url = ctx.page.url();
        // The success path would start the real opener of the machine: the request is answered by the test.
        let posted = null;
        await ctx.page.route('**/open-path', async (route) => {
          posted = { url: route.request().url(), body: route.request().postData() };
          await route.fulfill({ status: 204 });
        });
        await fileLink(ctx, './README.md:1').click();
        await ctx.page.waitForTimeout(500);
        ctx.assert(posted, 'el clic no pidió abrir la ruta');
        ctx.assert(/\/tasks\/[0-9a-f-]{36}\/open-path$/.test(posted.url), `URL inesperada: ${posted.url}`);
        ctx.assert(new URLSearchParams(posted.body).get('path') === './README.md:1', `cuerpo inesperado: ${posted.body}`);
        ctx.assert(ctx.page.url() === url, 'el clic en una ruta no debe navegar');
        ctx.assert(
          (await fileLink(ctx, './README.md:1').getAttribute('class')).includes('file-link-error') === false,
          'una apertura correcta no debe marcar error',
        );
      },
    },
    {
      name: 'chat links: a missing path and a program are refused with the reason as tooltip',
      fn: async (ctx) => {
        await chatWithLinks(ctx);
        const missing = fileLink(ctx, './no-existe.txt');
        await missing.click();
        await ctx.page.locator('a[data-file-path="./no-existe.txt"].file-link-error').waitFor({ timeout: 8000 });
        ctx.assert((await missing.getAttribute('title')) === 'Esta ruta no existe.', 'falta el motivo de la ruta inexistente');

        const program = fileLink(ctx, './run.sh');
        await program.click();
        await ctx.page.locator('a[data-file-path="./run.sh"].file-link-error').waitFor({ timeout: 8000 });
        ctx.assert(
          (await program.getAttribute('title')) === 'Los programas y scripts no se abren desde el chat.',
          'falta el motivo del script bloqueado',
        );
        await shot(ctx, '2-chat-links-refused');
      },
    },
    {
      name: 'chat links: a URL link opens in a new tab',
      fn: async (ctx) => {
        await chatWithLinks(ctx);
        // The new tab is not loaded: the test only needs to see that the click opened one.
        await ctx.page.context().route('https://example.com/**', (route) => route.fulfill({ status: 200, body: 'ok' }));
        const [popup] = await Promise.all([
          ctx.page.context().waitForEvent('page', { timeout: 10000 }),
          ctx.page.locator('.msg-assistant .markdown a[href="https://example.com/docs/intro"]').click(),
        ]);
        ctx.assert(popup.url().startsWith('https://example.com/docs/intro'), `pestaña inesperada: ${popup.url()}`);
        fs.mkdirSync(SHOTS, { recursive: true });
        await popup.screenshot({ path: path.join(SHOTS, '3-chat-links-url-opened-in-new-tab.png') });
        await popup.close();
      },
    },
  ],
};
