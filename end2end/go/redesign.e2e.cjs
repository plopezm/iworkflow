// end2end/go/redesign.e2e.cjs
// Go-mode e2e of the redesign (odd/tasks/engram-integration.md, T4, S7): the PWA head
// (manifest and icons), the sidebar header and current-page marker, the board of a workspace with seeded tasks,
// and the status badges. Every check goes through the public UI: roles, visible text, attributes and computed
// styles. Nothing reads internal storage. The suite runs in Spanish (the harness language cookie).
//
// The workspace and the tasks are created with the app's own forms: a temp git repo with one commit is registered
// from the Workspaces screen, and the tasks come from the "Nueva tarea" dialog of the board.

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';

// Creates a git repo with one commit and returns its real path (the server stores the path it was given).
function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `redesign-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `redesign ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return fs.realpathSync(dir);
}

// Registers a workspace from the Workspaces screen and waits until its row is listed.
async function registerWs(ctx, name, dir) {
  await ctx.goto('/workspaces/new');
  await ctx.page.getByLabel('Nombre').first().fill(name);
  await ctx.page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await ctx.page.getByLabel(/Rama base/).first().fill('main');
  await ctx.page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await ctx.page.getByText(dir).first().waitFor({ timeout: 12000 });
}

// Opens the board of a workspace from the sidebar and waits for its list.
async function openBoard(ctx, name) {
  await ctx.goto('/');
  await ctx.page.locator('#board-sidebar').getByRole('link', { name }).first().click();
  await ctx.page.locator('[id^="board-list-"]:visible').waitFor({ timeout: 10000 });
}

// Creates a backlog task from the "Nueva tarea" dialog with the given priority ("" keeps the default).
async function createTask(ctx, ws, { title, priority }) {
  const page = ctx.page;
  const form = 'form:has(select[name=workspace_id]):visible';
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  await page.locator(`${form} select[name=workspace_id]`).selectOption({ label: `${ws} (main)` });
  await page.locator(`${form} input[name=title]`).fill(title);
  await page.locator(`${form} textarea[name=prompt]`).fill(`prompt ${title}`);
  if (priority) await page.locator(`${form} select[name=priority]`).selectOption(priority);
  await page.locator(`${form} button[type=submit]`, { hasText: 'Crear tarea' }).click();
  await page.locator(form).waitFor({ state: 'detached', timeout: 10000 });
}

// The Settings page heading is the proof that the Settings tab is open in the dock.
const settingsHeading = (page) => page.locator('#dock').getByRole('heading', { name: 'Configuración', level: 1 });

module.exports = {
  name: 'redesign',
  tests: [
    // ---- (1) PWA head ----
    {
      name: 'pwa: el documento enlaza el manifiesto y declara el color de tema',
      fn: async (ctx) => {
        await ctx.goto('/');
        const manifest = await ctx.page.locator('link[rel=manifest]').getAttribute('href');
        ctx.assert(manifest === '/static/manifest.webmanifest', `href del manifiesto: ${manifest}`);
        ctx.assert((await ctx.page.locator('meta[name=theme-color]').count()) > 0, 'falta meta theme-color');
      },
    },
    {
      name: 'pwa: el manifiesto responde application/manifest+json con nombre iworkflow y display standalone',
      fn: async (ctx) => {
        const res = await ctx.page.request.get(`${ctx.base}/static/manifest.webmanifest`);
        ctx.assert(res.status() === 200, `manifiesto: estado ${res.status()}`);
        const type = res.headers()['content-type'] || '';
        ctx.assert(type.startsWith('application/manifest+json'), `content-type del manifiesto: ${type}`);
        const json = JSON.parse(await res.text());
        ctx.assert(json.name === 'iworkflow', `name del manifiesto: ${json.name}`);
        ctx.assert(json.display === 'standalone', `display del manifiesto: ${json.display}`);
      },
    },
    {
      name: 'pwa: cada icono del manifiesto responde 200 con una imagen',
      fn: async (ctx) => {
        const res = await ctx.page.request.get(`${ctx.base}/static/manifest.webmanifest`);
        const manifest = JSON.parse(await res.text());
        ctx.assert(Array.isArray(manifest.icons) && manifest.icons.length > 0, 'el manifiesto no declara iconos');
        for (const icon of manifest.icons) {
          const iconRes = await ctx.page.request.get(`${ctx.base}${icon.src}`);
          ctx.assert(iconRes.status() === 200, `icono ${icon.src}: estado ${iconRes.status()}`);
          const type = iconRes.headers()['content-type'] || '';
          ctx.assert(type.startsWith('image/'), `icono ${icon.src}: content-type ${type}`);
        }
      },
    },

    // ---- (2) sidebar header and current page ----
    {
      name: 'barra lateral: la versión va junto a la marca, encima de la navegación',
      fn: async (ctx) => {
        await ctx.goto('/');
        const version = ctx.page.locator('[data-app-version]');
        await version.waitFor({ state: 'visible', timeout: 5000 });
        const brandRow = await version.evaluate((el) => el.parentElement.textContent || '');
        ctx.assert(brandRow.includes('iworkflow'), 'la versión no está junto a la marca');
        const versionBox = await version.boundingBox();
        const firstNav = await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link').first().boundingBox();
        ctx.assert(versionBox && firstNav, 'no se pudo medir la cabecera de la barra lateral');
        ctx.assert(versionBox.y < firstNav.y, 'la versión no está encima del primer enlace de la navegación');
      },
    },
    {
      name: 'barra lateral: el enlace de la página enfocada lleva aria-current="page"',
      fn: async (ctx) => {
        await ctx.goto('/settings');
        const current = ctx.page.locator('a.sidebar-nav[aria-current="page"]');
        await current.first().waitFor({ state: 'attached', timeout: 10000 });
        ctx.assert((await current.count()) === 1, 'debe haber exactamente un enlace con aria-current="page"');
        const marked = await current.first().evaluate((link) => {
          const copy = link.cloneNode(true);
          copy.querySelectorAll('[aria-hidden="true"]').forEach((icon) => icon.remove());
          return copy.textContent.replace(/\s+/g, ' ').trim();
        });
        ctx.assert(marked === 'Configuración', `el enlace marcado es "${marked}", no Configuración`);
      },
    },

    // ---- (3) board redesign with seeded data ----
    {
      name: 'tablero: dos tareas en backlog muestran cabecera con contador, prioridad alta y sin insignia de etapa en la tarjeta',
      fn: async (ctx) => {
        const name = `redesign-board-${tag}`;
        const dir = uniqueGitRepo('board');
        await registerWs(ctx, name, dir);
        await openBoard(ctx, name);
        const high = `alta-${tag}`;
        const low = `baja-${tag}`;
        await createTask(ctx, name, { title: high, priority: 'high' });
        await createTask(ctx, name, { title: low, priority: 'low' });

        const list = ctx.page.locator('[id^="board-list-"]:visible');
        const heading = list.locator('.stage-heading[data-stage=backlog]');
        await heading.waitFor({ state: 'visible', timeout: 10000 });
        // The pill counts the cards of its own group, whatever else the shared test database holds.
        const group = list.locator('[data-stage-group=backlog]');
        // Each card arrives with the board refresh after its creation: wait for both before counting.
        await group.locator('[data-loose-task]').nth(1).waitFor({ state: 'attached', timeout: 10000 });
        const cards = await group.locator('[data-loose-task]').count();
        const counted = Number((await heading.locator('.stage-heading-count').innerText()).trim());
        ctx.assert(cards >= 2, `el grupo de backlog tiene ${cards} tarjetas, se esperaban al menos las 2 creadas`);
        ctx.assert(counted === cards, `el contador de backlog (${counted}) no coincide con las tarjetas del grupo (${cards})`);

        const highCard = list.locator('[data-loose-task]', { hasText: high });
        await highCard.waitFor({ state: 'visible', timeout: 10000 });
        ctx.assert((await highCard.getAttribute('data-priority')) === 'high', 'la tarjeta de prioridad alta no tiene data-priority="high"');

        // The stage is the heading of the group, so the cards of the grouped list do not repeat it. "Shown"
        // means visible: a badge hidden by CSS does not count.
        const shown = await list.locator('[data-loose-task] .badge-backlog:visible').count();
        const inDom = await list.locator('[data-loose-task] .badge-backlog').count();
        ctx.assert(shown === 0,
          `las tarjetas del listado agrupado muestran la insignia de etapa (${shown} visibles, ${inDom} en el DOM)`);
      },
    },
    {
      name: 'tablero: el contador de "Requieren atención" está en alerta solo cuando hay tareas esperando',
      fn: async (ctx) => {
        await ctx.goto('/');
        const section = ctx.page.locator('[data-section]').filter({ hasText: 'Requieren atención' }).first();
        await section.waitFor({ state: 'attached', timeout: 10000 });
        const waiting = await section.locator('[data-attention-task]').count();
        const pill = section.locator('.sidebar-count');
        ctx.assert(Number((await pill.innerText()).trim()) === waiting,
          `el contador de atención no coincide con las ${waiting} tareas listadas`);
        const shadow = await pill.evaluate((el) => getComputedStyle(el).boxShadow);
        // The alert state paints a glow (box-shadow) on top of the error background; the idle pill has none.
        if (waiting === 0) {
          ctx.assert(shadow === 'none', `el contador vacío tiene sombra de alerta: ${shadow}`);
        } else {
          ctx.assert(shadow !== 'none', 'el contador con tareas esperando no está en estado de alerta');
        }
      },
    },

    // ---- (5) badges ----
    {
      name: 'insignias: "En ejecución" del panel tiene un único punto visible',
      fn: async (ctx) => {
        // The dashboard is a dock tab opened from the sidebar entry "Tablero"; a fresh "/" shows an empty dock.
        await ctx.goto('/');
        await ctx.page.getByRole('navigation', { name: 'Navegación' }).getByRole('link', { name: 'Tablero' }).first().click();
        const badge = ctx.page.locator('span.badge-working', { hasText: 'En ejecución' }).first();
        await badge.waitFor({ state: 'visible', timeout: 10000 });
        const own = badge.locator(':scope > span:first-child');
        ctx.assert((await own.count()) === 1, 'la insignia no trae su propio marcador');
        ctx.assert(await own.first().isVisible(), 'el marcador propio de la insignia no es visible');
        const pseudoDisplay = await badge.evaluate((el) => getComputedStyle(el, '::before').display);
        ctx.assert(pseudoDisplay === 'none', `el punto del pseudoelemento se ve (display: ${pseudoDisplay})`);
      },
    },
  ],
};
