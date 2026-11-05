// Browser check of the "move the data folder" card (Settings -> Resources) against a server started WITHOUT
// IWF_HOME, driven by end2end/go/move_home_smoke.sh (which owns the isolated HOME and the server restart).
// Env: SMOKE_BASE_URL, MOVE_DEST (a free absolute path), PLAYWRIGHT_MODULE, SMOKE_CHROMIUM_PATH (optional).
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const BASE = process.env.SMOKE_BASE_URL;
const DEST = process.env.MOVE_DEST;
if (!BASE || !DEST) {
  console.error('SMOKE_BASE_URL and MOVE_DEST are required');
  process.exit(2);
}

(async () => {
  const browser = await chromium.launch(process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {});
  const context = await browser.newContext();
  await context.addCookies([{ name: 'iw-lang', value: 'es', url: BASE }]);
  const page = await context.newPage();
  const step = (s) => console.log('✔', s);
  const check = (ok, message) => {
    if (!ok) throw new Error(message);
  };

  try {
    await page.goto(BASE + '/settings/resources');
    await page.getByText('Carpeta de datos').first().waitFor({ timeout: 15000 });
    const order = await page.evaluate(() => {
      const titles = Array.from(document.querySelectorAll('h2, h3, .card-title, [data-card-title]')).map((el) => el.textContent.trim());
      return { home: titles.findIndex((t) => t.includes('Carpeta de datos')), limits: titles.findIndex((t) => t.includes('Límites de recursos')) };
    });
    check(order.home >= 0 && order.limits >= 0 && order.home < order.limits, `the card must come before the limits: ${JSON.stringify(order)}`);
    step('la tarjeta aparece encima de Límites de recursos');

    const field = page.getByLabel('Nueva carpeta (ruta absoluta)');
    const moveButton = page.getByRole('button', { name: 'Mover carpeta' });
    check((await field.count()) === 1 && (await moveButton.count()) === 1, 'without IWF_HOME the card offers the destination field and the Mover carpeta button');
    step('sin IWF_HOME la tarjeta ofrece campo de destino y botón');

    await field.fill('relative/dir');
    await moveButton.click();
    await page.locator('#data-home-card [role=alert], #data-home-card .text-error').first().waitFor({ timeout: 8000 });
    check((await page.getByText('Reiniciando el servidor').count()) === 0, 'a refused destination must not show the restarting card');
    check((await field.count()) === 1, 'the form stays after a refusal');
    step('un destino relativo se rechaza con aviso y sin reiniciar');

    await field.fill(DEST);
    await moveButton.click();
    await page.getByText('Reiniciando el servidor').first().waitFor({ timeout: 8000 });
    check((await moveButton.count()) === 0, 'the form is hidden while the server restarts');
    step('un destino válido muestra la tarjeta «Reiniciando»');

    // The card polls; once the server is back and the move applied, it shows the new folder and the form again.
    await page.getByText(DEST).first().waitFor({ timeout: 90000 });
    await page.getByRole('button', { name: 'Mover carpeta' }).waitFor({ timeout: 15000 });
    check((await page.getByText('Reiniciando el servidor').count()) === 0, 'the restarting text must be gone');
    step('tras el reinicio la tarjeta muestra la carpeta nueva y el formulario');
  } catch (error) {
    const card = await page.locator('#data-home-card').innerText().catch(() => '(card not found)');
    console.error('card text at the failure:\n' + card);
    throw error;
  } finally {
    await browser.close();
  }
  console.log('OK: move_home_ui_smoke');
})().catch((error) => {
  console.error('FAIL:', error.message);
  process.exit(1);
});
