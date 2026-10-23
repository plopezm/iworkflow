// Shared Playwright harness for the e2e suite (CommonJS, local install only).
// Requires the server from end2end/run_tests.sh to already be up on SMOKE_BASE_URL.
const { chromium } = require(
  process.env.PLAYWRIGHT_MODULE ||
  '/home/pablolm/.cache/iwf-playwright/node_modules/playwright',
);

const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:7777';
const LANG = process.env.SMOKE_LANG || 'es';

let browser;

// Launch the shared browser (call once; runner owns its lifetime).
async function launchBrowser() {
  browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH
      ? { executablePath: process.env.SMOKE_CHROMIUM_PATH }
      : {},
  );
  return browser;
}

// Create a fresh page+context for one test, with shared boilerplate.
async function makePage() {
  const ctx = await browser.newContext({
    viewport: { width: 1440, height: 860 },
    deviceScaleFactor: 1,
  });
  // Without the language cookie the app shows a blocking language prompt.
  await ctx.addCookies([{ name: 'iw-lang', value: LANG, url: BASE }]);

  const page = await ctx.newPage();
  const problems = [];
  // The suite auto-accepts confirm dialogs (matches the smoke). A test that needs
  // to reject/inspect one removes the default listener and sets its own (below).
  let dialogMode = 'accept';
  page.on('dialog', (d) => (dialogMode === 'accept' ? d.accept() : d.dismiss()));
  const setDialog = (mode) => { dialogMode = mode === 'accept' ? 'accept' : 'reject'; };
  const acceptAll = () => { page.on('dialog', (d) => d.accept()); };

  // Filter the expected noise (favicon, hydration resource warnings, known 404s).
  const ignorable = (t) =>
    /favicon|reading a resource in `hydrate`|status of 404|status of 500/.test(t);

  page.on('console', (m) => {
    if ((m.type() === 'error' || m.type() === 'warning') && !ignorable(m.text()))
      problems.push(`[console:${m.type()}] ${m.text()}`);
  });
  page.on('pageerror', (e) => problems.push(`[pageerror] ${e.message}`));
  page.on('response', (r) => {
    if (r.status() >= 400 && !ignorable(r.url()) && !/favicon|list_model_options|list_agent|list_profile|list_workspace/.test(r.url()))
      problems.push(`[http] ${r.status()} ${r.url()}`);
  });

  const assert = (cond, msg) => {
    if (!cond) throw new Error(msg);
  };

  const goto = async (path) => {
    const abs = BASE + (path.startsWith('/') ? path : '/' + path);
    await page.goto(abs, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(400);
    return page;
  };

  const nav = (name) => page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name });

  const fill = (selector, value) => page.locator(selector).fill(value);
  const click = (selector) => page.locator(selector).click();

  const screenshot = async (name) => {
    try {
      await page.screenshot({ path: name });
    } catch (e) {
      problems.push(`[screenshot] failed: ${e.message}`);
    }
  };

  return {
    base: BASE,
    page,
    assert,
    goto,
    nav,
    fill,
    click,
    setDialog,
    acceptAll,
    screenshot,
    problems,
    getProblems: () => problems,
    close: async () => {
      // Best-effort cleanup: the runner closes the context.
      await ctx.close();
    },
  };
}

module.exports = { BASE, launchBrowser, makePage, LANG };
