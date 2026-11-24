// end2end/go/content_search.e2e.cjs
// Búsqueda por contenido del explorador de proyecto contra el servidor Go, en navegador real
// (odd/tasks/content-search.md, T4). Cubre:
//   1. Filtro del árbol (S1, S5): escribir en form[data-content-filter] deja solo los ficheros con
//      coincidencias, con su badge; los toggles cs/ww cambian el resultado; una regex rota muestra el
//      error sin romper la página; borrar la caja restaura el árbol normal.
//   2. Salto desde el árbol (S2, S3): pulsar una línea de coincidencia abre el fichero con la barra
//      de búsqueda abierta, prellenada con la query del árbol, contador "n / total", resaltado y
//      scroll hasta la línea pulsada.
//   3. Navegación (S2, S4): flechas con vuelta, Enter/Shift+Enter, F3/Shift+F3, Ctrl+F y Escape.
//   4. La búsqueda del visor no cambia el filtro ni el resultado del árbol.
//
// Como project_tree.e2e.cjs, cada test crea SU PROPIO repo git y su workspace bajo TMP (el runner
// run_go.sh lo aísla). Los helpers se copian a propósito: este fichero no depende de project_tree.e2e.cjs.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

const FILTER_INPUT = 'form[data-content-filter] input[name="q"]';
const FILTER_FLAG = (name) => `form[data-content-filter] input[name="${name}"]`;
const HIT_FILE = 'a[data-hit-file]';
const BIG = 'big.txt';
const BIG_LINE = 215; // hits en 205, 210 (mayúsculas) y 215: cerca del final de un fichero de 220 líneas

// ---- helpers ----

// Repo git único con el fixture de búsqueda. Devuelve su ruta.
//   alpha.txt       "needle" x3 (una de ellas dentro de "needles")
//   beta.txt        sin coincidencias
//   src/gamma.go    "Needle" x1 (mayúscula)
//   big.txt         220 líneas; "needle" en 205 y 215, "NEEDLE" en 210
function fixtureRepo(prefix) {
  const dir = path.join(TMP, `cs-repo-${prefix}-${tag}`);
  fs.mkdirSync(path.join(dir, 'src'), { recursive: true });
  fs.writeFileSync(path.join(dir, 'README.md'), `readme ${prefix}\n`);
  fs.writeFileSync(path.join(dir, 'alpha.txt'), 'needle one\nneedles plural\nthe needle again\n');
  fs.writeFileSync(path.join(dir, 'beta.txt'), 'nothing to see here\n');
  fs.writeFileSync(path.join(dir, 'src', 'gamma.go'), 'package src\n\n// Needle is here\n');
  const lines = [];
  for (let i = 1; i <= 220; i++) {
    if (i === 205) lines.push('needle here 205');
    else if (i === 210) lines.push('NEEDLE here 210');
    else if (i === BIG_LINE) lines.push('needle here 215');
    else lines.push(`line ${i}`);
  }
  fs.writeFileSync(path.join(dir, BIG), lines.join('\n') + '\n');
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=smoke -c user.email=smoke@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=smoke -c user.email=smoke@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Registra un workspace desde /workspaces/new y devuelve el id (UUID) del workspace JUSTO registrado.
async function registerWs(ctx, wsName, dir) {
  await ctx.goto('/workspaces/new');
  await ctx.page.getByLabel('Nombre').fill(wsName);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
  const id = await ctx.page.evaluate((name) => {
    const cards = [...document.querySelectorAll('article.card')];
    for (const card of cards) {
      if (!card.textContent.includes(name)) continue;
      const link = card.querySelector('a[href*="/workspaces/"]');
      if (!link) continue;
      const m = link.href.match(/\/workspaces\/([0-9a-f-]{36})/);
      if (m) return m[1];
    }
    return null;
  }, wsName);
  ctx.assert(id, `no se pudo extraer el id del workspace "${wsName}"`);
  return id;
}

// Espera a que el valor del input contenga `needle`; devuelve el valor final.
async function waitForValue(locator, needle, { timeout = 10000 } = {}) {
  const start = Date.now();
  let last = '';
  while (Date.now() - start < timeout) {
    try {
      last = await locator.first().inputValue();
    } catch {
      last = '';
    }
    if (last.includes(needle)) return last;
    await new Promise((r) => setTimeout(r, 250));
  }
  return last;
}

// Espera al texto exacto (string) o que cumpla la expresión regular (RegExp) del contador.
async function waitForCounter(locator, expected, timeout = 8000) {
  const start = Date.now();
  let last = '';
  while (Date.now() - start < timeout) {
    last = ((await locator.first().textContent()) || '').trim();
    const ok = typeof expected === 'string' ? last === expected : expected.test(last);
    if (ok) return last;
    await new Promise((r) => setTimeout(r, 150));
  }
  return last;
}

// Escribe la query del árbol de contenido (dispara el hx-get con delay de 250 ms).
async function filterTree(ctx, query) {
  await ctx.page.locator(FILTER_INPUT).first().fill(query);
}

// Mapa fichero -> nº de coincidencias de la lista filtrada del árbol.
async function hitMap(page) {
  return page.evaluate(() => {
    const out = {};
    for (const a of document.querySelectorAll('a[data-hit-file]')) {
      out[a.dataset.hitPath] = Number(a.querySelector('[data-hit-count]')?.textContent.trim());
    }
    return out;
  });
}

function sameMap(a, b) {
  const pairs = (m) => JSON.stringify(Object.keys(m).sort().map((k) => [k, m[k]]));
  return pairs(a) === pairs(b);
}

// Espera a que la lista filtrada del árbol tenga exactamente el mapa esperado (o agota el tiempo).
async function waitForHitMap(ctx, expected, timeout = 10000) {
  const start = Date.now();
  let last = {};
  while (Date.now() - start < timeout) {
    last = await hitMap(ctx.page);
    if (sameMap(last, expected)) return last;
    await new Promise((r) => setTimeout(r, 200));
  }
  return last;
}

// Pulsa el triángulo de un fichero de la lista filtrada, espera a la línea `line` y la pulsa.
// Devuelve el visor (la raíz [data-file-search-root] de ese fichero, ya visible).
async function openFromHit(ctx, relPath, line) {
  const page = ctx.page;
  const section = page.locator(`li[data-section]:has(a[data-hit-file][data-hit-path="${relPath}"])`).first();
  await section.locator('[data-section-toggle]').first().click();
  const hit = section.locator(`a[data-hit-line="${line}"]`).first();
  await hit.waitFor({ state: 'visible', timeout: 5000 });
  await hit.click();
  const root = page.locator(`[data-file-search-root][data-path="${relPath}"]:visible`).first();
  await root.waitFor({ state: 'visible', timeout: 12000 });
  return root;
}

// Estado de los resaltados del visor: rangos de coincidencias y de la actual.
function highlightState(page) {
  return page.evaluate(() => ({
    all: CSS.highlights.get('iwf-search')?.size ?? 0,
    current: CSS.highlights.get('iwf-search-current')?.size ?? 0,
  }));
}

function pageErrors(ctx) {
  return ctx.getProblems().filter((p) => p.startsWith('[pageerror]'));
}

module.exports = {
  name: 'content_search',
  tests: [
    {
      name: 'content_tree_filter: el árbol muestra solo los ficheros con coincidencias, con su conteo',
      fn: async (ctx) => {
        const dir = fixtureRepo('filter');
        const id = await registerWs(ctx, `csfilter-${tag}`, dir);
        await ctx.goto(`/workspaces/${id}`);
        await ctx.page.locator(FILTER_INPUT).first().waitFor({ state: 'visible', timeout: 12000 });
        const normal = ctx.page.getByRole('link', { name: 'beta.txt', exact: true });
        await normal.first().waitFor({ timeout: 12000 });

        // Literal, sin distinguir mayúsculas: alpha 3, big 3 (incluye NEEDLE), gamma 1.
        await filterTree(ctx, 'needle');
        let got = await waitForHitMap(ctx, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 });
        ctx.assert(sameMap(got, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 }), `filtro "needle": ${JSON.stringify(got)}`);
        ctx.assert(
          await ctx.page.locator('a[data-hit-file][data-hit-path="beta.txt"]').count() === 0,
          'beta.txt no tiene coincidencias y no debe aparecer en el filtro'
        );
        ctx.assert(
          await ctx.page.getByRole('link', { name: 'beta.txt', exact: true }).count() === 0,
          'beta.txt no debe aparecer como fila normal mientras hay filtro'
        );
        ctx.assert(await ctx.page.locator('[data-content-summary]').count() === 1, 'falta el resumen de ficheros con coincidencias');

        // cs (mayúsculas): NEEDLE de big.txt y Needle de gamma.go dejan de contar.
        await ctx.page.locator(FILTER_FLAG('cs')).first().check();
        got = await waitForHitMap(ctx, { 'alpha.txt': 3, 'big.txt': 2 });
        ctx.assert(sameMap(got, { 'alpha.txt': 3, 'big.txt': 2 }), `filtro "needle" con cs: ${JSON.stringify(got)}`);
        await ctx.page.locator(FILTER_FLAG('cs')).first().uncheck();

        // ww (palabra completa): "needles" deja de contar en alpha.txt.
        await ctx.page.locator(FILTER_FLAG('ww')).first().check();
        got = await waitForHitMap(ctx, { 'alpha.txt': 2, 'big.txt': 3, 'src/gamma.go': 1 });
        ctx.assert(sameMap(got, { 'alpha.txt': 2, 'big.txt': 3, 'src/gamma.go': 1 }), `filtro "needle" con ww: ${JSON.stringify(got)}`);
        await ctx.page.locator(FILTER_FLAG('ww')).first().uncheck();
        got = await waitForHitMap(ctx, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 });
        ctx.assert(sameMap(got, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 }), `al quitar ww: ${JSON.stringify(got)}`);

        // Regex rota: error visible en el árbol, sin resultados, la página sigue viva y la caja conserva el texto.
        await ctx.page.locator(FILTER_FLAG('re')).first().check();
        await filterTree(ctx, '(needle');
        const err = ctx.page.locator('p.text-error', { hasText: 'búsqueda inválida' }).first();
        await err.waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert(await ctx.page.locator(HIT_FILE).count() === 0, 'una regex rota no debe listar coincidencias');
        ctx.assert(await ctx.page.locator(FILTER_INPUT).first().inputValue() === '(needle', 'la caja debe conservar lo escrito');
        // Regex válida otra vez: el filtro vuelve a funcionar.
        await filterTree(ctx, 'needle');
        got = await waitForHitMap(ctx, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 });
        ctx.assert(sameMap(got, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 }), `tras la regex rota: ${JSON.stringify(got)}`);
        await ctx.page.locator(FILTER_FLAG('re')).first().uncheck();

        // Borrar la caja restaura el árbol normal.
        await filterTree(ctx, '');
        await normal.first().waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert(await ctx.page.locator(HIT_FILE).count() === 0, 'al borrar la caja no deben quedar coincidencias');
        ctx.assert(
          await ctx.page.getByRole('button', { name: 'src', exact: true }).count() > 0,
          'el árbol normal debe volver a mostrar la carpeta src'
        );
        const errors = pageErrors(ctx);
        ctx.assert(errors.length === 0, `la página lanzó errores de JS: ${errors.join(' | ')}`);
      },
    },
    {
      name: 'content_handoff: pulsar una coincidencia abre el fichero con la búsqueda y la línea',
      fn: async (ctx) => {
        const dir = fixtureRepo('handoff');
        const id = await registerWs(ctx, `cshandoff-${tag}`, dir);
        await ctx.goto(`/workspaces/${id}`);
        await ctx.page.locator(FILTER_INPUT).first().waitFor({ state: 'visible', timeout: 12000 });
        await filterTree(ctx, 'needle');
        const got = await waitForHitMap(ctx, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 });
        ctx.assert(sameMap(got, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 }), `filtro previo: ${JSON.stringify(got)}`);

        const root = await openFromHit(ctx, BIG, BIG_LINE);
        const ta = root.locator('textarea[data-code-editor]');
        await waitForValue(ta, 'needle here 215', { timeout: 12000 });

        // Barra abierta, prellenada con la query del árbol y con el contador de la línea pulsada.
        await root.locator('[data-file-search]').waitFor({ state: 'visible', timeout: 8000 });
        const query = await root.locator('[data-fs-input]').inputValue();
        ctx.assert(query === 'needle', `la caja del visor debe traer la query del árbol (got "${query}")`);
        // Coincidencia 3 de 3: la primera de la línea 215 es la última del fichero.
        const counter = await waitForCounter(root.locator('[data-fs-count]'), '3 / 3');
        ctx.assert(counter === '3 / 3', `contador esperado "3 / 3" (got "${counter}")`);

        // Resaltado: las demás coincidencias en iwf-search y la actual en iwf-search-current.
        await ctx.page.waitForFunction(
          () => ((CSS.highlights.get('iwf-search')?.size ?? 0) + (CSS.highlights.get('iwf-search-current')?.size ?? 0)) > 0,
          null,
          { timeout: 8000 }
        );
        const hl = await highlightState(ctx.page);
        ctx.assert(hl.all === 2 && hl.current === 1, `resaltado esperado 2 + 1 (got ${JSON.stringify(hl)})`);

        // La línea del rango actual es la pulsada.
        const line = await ctx.page.evaluate(() => {
          const viewer = document.querySelector('[data-file-search-root][data-path="big.txt"]');
          const range = [...(CSS.highlights.get('iwf-search-current') || [])][0];
          if (!viewer || !range) return -1;
          const code = viewer.querySelector('[data-code-layer] code');
          const walker = document.createTreeWalker(code, NodeFilter.SHOW_TEXT);
          let offset = 0;
          for (let n = walker.nextNode(); n; n = walker.nextNode()) {
            if (n === range.startContainer) {
              offset += range.startOffset;
              break;
            }
            offset += n.textContent.length;
          }
          const text = viewer.querySelector('textarea[data-code-editor]').value;
          return text.slice(0, offset).split('\n').length;
        });
        ctx.assert(line === BIG_LINE, `el rango actual debe caer en la línea ${BIG_LINE} (got ${line})`);

        // El editor se desplaza hasta esa línea: scrollTop > 0 y la línea dentro del área visible.
        const scroll = await ctx.page.evaluate((target) => {
          const ta = document.querySelector('[data-file-search-root][data-path="big.txt"] textarea[data-code-editor]');
          const style = getComputedStyle(ta);
          const lh = parseFloat(style.lineHeight) || 1.25 * (parseFloat(getComputedStyle(document.documentElement).fontSize) || 16);
          const pad = parseFloat(style.paddingTop) || 0;
          const top = pad + (target - 1) * lh;
          return { scrollTop: ta.scrollTop, height: ta.clientHeight, top, bottom: top + lh };
        }, BIG_LINE);
        ctx.assert(scroll.scrollTop > 0, `el editor no se desplazó (scrollTop=${scroll.scrollTop})`);
        ctx.assert(
          scroll.top >= scroll.scrollTop - 1 && scroll.bottom <= scroll.scrollTop + scroll.height + 1,
          `la línea ${BIG_LINE} no está visible: ${JSON.stringify(scroll)}`
        );
        const errors = pageErrors(ctx);
        ctx.assert(errors.length === 0, `la página lanzó errores de JS: ${errors.join(' | ')}`);
      },
    },
    {
      name: 'content_viewer_nav: flechas, Enter, F3 con vuelta, Ctrl+F y Escape en el visor',
      fn: async (ctx) => {
        const dir = fixtureRepo('nav');
        const id = await registerWs(ctx, `csnav-${tag}`, dir);
        // Abre big.txt directamente (sin filtro) y busca con Ctrl+F.
        await ctx.goto(`/workspaces/${id}/files/${BIG}`);
        const root = ctx.page.locator(`[data-file-search-root][data-path="${BIG}"]`).first();
        const ta = root.locator('textarea[data-code-editor]');
        await waitForValue(ta, 'needle here 205', { timeout: 12000 });
        await ta.click();
        await ctx.page.keyboard.press('Control+f');
        const bar = root.locator('[data-file-search]');
        await bar.waitFor({ state: 'visible', timeout: 5000 });
        const input = root.locator('[data-fs-input]');
        ctx.assert(await input.evaluate((el) => document.activeElement === el), 'Ctrl+F debe enfocar la caja del visor');
        await input.fill('needle');
        const counter = root.locator('[data-fs-count]');
        const expectAt = async (text) => {
          const got = await waitForCounter(counter, text);
          ctx.assert(got === text, `contador esperado "${text}" (got "${got}")`);
        };
        await expectAt('1 / 3');

        // Teclado con el foco en la caja: F3 / Shift+F3 con vuelta, y Enter / Shift+Enter.
        await input.focus();
        await ctx.page.keyboard.press('F3');
        await expectAt('2 / 3');
        await ctx.page.keyboard.press('F3');
        await expectAt('3 / 3');
        await ctx.page.keyboard.press('F3');
        await expectAt('1 / 3');
        await ctx.page.keyboard.press('Shift+F3');
        await expectAt('3 / 3');
        await ctx.page.keyboard.press('Enter');
        await expectAt('1 / 3');
        await ctx.page.keyboard.press('Shift+Enter');
        await expectAt('3 / 3');

        // Flechas del ratón desde la 3.ª: la anterior va a la 2.ª. Al pulsar un botón la caja pierde el foco;
        // ese cambio no debe reiniciar la posición (si lo hace, el primer clic cae en la 3.ª).
        await root.locator('[data-fs-prev]').click();
        await expectAt('2 / 3');
        await root.locator('[data-fs-next]').click();
        await expectAt('3 / 3');
        await root.locator('[data-fs-next]').click();
        await expectAt('1 / 3');

        // Escape cierra la barra y quita los resaltados.
        await ctx.page.keyboard.press('Escape');
        await bar.waitFor({ state: 'hidden', timeout: 5000 });
        const hl = await highlightState(ctx.page);
        ctx.assert(hl.all === 0 && hl.current === 0, `Escape debe quitar los resaltados (got ${JSON.stringify(hl)})`);
        const errors = pageErrors(ctx);
        ctx.assert(errors.length === 0, `la página lanzó errores de JS: ${errors.join(' | ')}`);
      },
    },
    {
      name: 'content_viewer_isolated: buscar dentro del visor no cambia la caja ni los resultados del árbol',
      fn: async (ctx) => {
        const dir = fixtureRepo('isolated');
        const id = await registerWs(ctx, `csiso-${tag}`, dir);
        await ctx.goto(`/workspaces/${id}`);
        await ctx.page.locator(FILTER_INPUT).first().waitFor({ state: 'visible', timeout: 12000 });
        await filterTree(ctx, 'needle');
        const before = await waitForHitMap(ctx, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 });
        ctx.assert(sameMap(before, { 'alpha.txt': 3, 'big.txt': 3, 'src/gamma.go': 1 }), `filtro previo: ${JSON.stringify(before)}`);

        const root = await openFromHit(ctx, BIG, BIG_LINE);
        await waitForValue(root.locator('textarea[data-code-editor]'), 'needle here 215', { timeout: 12000 });
        await root.locator('[data-file-search]').waitFor({ state: 'visible', timeout: 8000 });
        const storageSnapshot = () => ctx.page.evaluate(() => JSON.stringify(Object.entries(sessionStorage)));
        const storedBefore = await storageSnapshot();

        // La query del visor: 217 líneas contienen "line" (las 220 menos las 3 con "needle"/"NEEDLE").
        await root.locator('[data-fs-input]').fill('line');
        const counter = await waitForCounter(root.locator('[data-fs-count]'), '1 / 217');
        ctx.assert(counter === '1 / 217', `el visor debe contar sus propias coincidencias (got "${counter}")`);

        await ctx.page.waitForTimeout(800); // por si el árbol fuera a recargarse (delay 250 ms)
        const treeInput = await ctx.page.locator(FILTER_INPUT).first().inputValue();
        ctx.assert(treeInput === 'needle', `la caja del árbol cambió a "${treeInput}"`);
        const after = await hitMap(ctx.page);
        ctx.assert(sameMap(after, before), `los resultados del árbol cambiaron: ${JSON.stringify(after)}`);
        const storedAfter = await storageSnapshot();
        ctx.assert(storedAfter === storedBefore, 'la búsqueda del visor no debe escribir en sessionStorage');
      },
    },
  ],
};
