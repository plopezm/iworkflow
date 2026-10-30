// Preloaded with `node -r end2end/go/lang_shim.cjs <script>` for the standalone smoke scripts that do not
// set the language cookie. Without it the first-run language prompt covers the page. Every browser
// context opened through chromium.launch gets the cookie that the harness sets (iw-lang=es).
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const LANG = process.env.SMOKE_LANG || 'es';
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
const launch = chromium.launch.bind(chromium);

chromium.launch = async (options) => {
  const browser = await launch(options);
  const newContext = browser.newContext.bind(browser);
  browser.newContext = async (contextOptions) => {
    const context = await newContext(contextOptions);
    await context.addCookies([{ name: 'iw-lang', value: LANG, url: BASE }]);
    return context;
  };
  return browser;
};
