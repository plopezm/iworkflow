// Smoke of the touch targets (sidebar toggle, dock tab bar) on an emulated phone: every control a
// finger has to hit measures at least 44px, and an inactive tab's hidden close button is not tappable.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:3000';
const MIN = 44;
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
(async () => {
  const browser = await chromium.launch(
    process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {},
  );
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
  await context.addCookies([{ name: 'iw-lang', value: 'es', url: BASE }]);
  const page = await context.newPage();
  const size = async (locator) => {
    const box = await locator.boundingBox();
    assert(box, 'the control is not rendered');
    return box;
  };
  const atLeast = async (locator, label) => {
    const box = await size(locator);
    assert(box.width >= MIN - 0.5 && box.height >= MIN - 0.5, `${label} measures ${box.width}x${box.height}, under ${MIN}px`);
    console.log('✔', `${label} ${Math.round(box.width)}x${Math.round(box.height)}`);
  };

  await page.goto(BASE + '/');
  await page.waitForLoadState('networkidle');
  assert(await page.evaluate(() => matchMedia('(pointer: coarse)').matches), 'the emulated device must be coarse-pointer');

  const burger = page.locator('button[data-mobile-open]').first();
  await atLeast(burger, 'sidebar open button');
  await burger.tap();
  await atLeast(page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Workspaces' }), 'sidebar nav link');
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Workspaces' }).tap();
  await page.locator('.dock-tab', { hasText: 'Workspaces' }).waitFor();
  await page.locator('button[data-mobile-open]').first().tap();
  await page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Modelos' }).tap();
  await page.locator('.dock-tab', { hasText: 'Modelos' }).waitFor();
  // Opening a tab closes the sidebar overlay, and its full-screen backdrop must not stay behind.
  await page.locator('#sidebar-panel[data-mobile-open]').waitFor({ state: 'detached' });
  assert(await page.locator('#sidebar-panel [data-overlay-close]').isHidden(), 'the sidebar backdrop stays after opening a tab');

  await atLeast(page.locator('.dock-tab').first(), 'dock tab');
  await atLeast(page.locator('.dock-icon-btn').first(), 'dock + / ⋯ button');
  const close = page.locator('.dock-tab-active .dock-tab-close');
  const closeBox = await size(close);
  // The glyph box is 32px; the transparent ::after adds 4px per side, so the tap area is 40px+ wide
  // and the tab itself (44px tall) frames it. Assert the hit area through elementFromPoint.
  const hit = await close.evaluate((el) => {
    const r = el.getBoundingClientRect();
    const at = (x, y) => document.elementFromPoint(x, y);
    const y = r.top + r.height / 2;
    return [at(r.left - 3, y), at(r.right + 3, y)].map((e) => e === el || `${e && e.tagName}.${e && e.className}`);
  });
  assert(closeBox.width >= 32 && hit.every((h) => h === true), `active close button too small (${closeBox.width}x${closeBox.height}, hit=${JSON.stringify(hit)})`);
  console.log('✔', `active tab close ${Math.round(closeBox.width)}x${Math.round(closeBox.height)} (+4px hit area)`);

  const inactiveClose = page.locator('.dock-tab:not(.dock-tab-active) .dock-tab-close').first();
  const pe = await inactiveClose.evaluate((el) => getComputedStyle(el).pointerEvents);
  assert(pe === 'none', `an inactive tab's hidden close button must ignore taps (pointer-events: ${pe})`);
  console.log('✔', 'inactive tab close ignores taps');

  const tabsBefore = await page.locator('.dock-tab').count();
  await page.locator('.dock-tab:not(.dock-tab-active)').first().tap();
  await page.waitForTimeout(300);
  assert((await page.locator('.dock-tab').count()) === tabsBefore, 'tapping an inactive tab must activate it, not close it');
  console.log('✔', 'tapping an inactive tab activates it without closing');
  await browser.close();
})().catch((e) => { console.error('✘', e.message); process.exit(1); });
