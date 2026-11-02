// end2end/go/board_refresh.e2e.cjs
// Go-mode regression for the refresh of the board sidebar: it listens to the task stream with a throttle and
// swaps its content with morph. The tests fire the same event the stream extension fires ("sse:changed" on the
// listening element) and count the requests the sidebar makes, so no agent or task is needed.
//  - a burst of events ends in one refetch that starts after the last event, so the sidebar always shows the final
//    state (htmx "throttle" drops the events of its window and would miss it);
//  - the refresh keeps the nodes of the sidebar (morph) instead of replacing them, so a click is not lost.
const SIDEBAR = '#board-sidebar';

async function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// Records, inside the page, when the sidebar starts a request (htmx:beforeRequest). Timestamps are taken in the page
// with performance.now(), so they compare with the time of the last notice without the delay of the test protocol.
async function recordSidebarRequests(page) {
  await page.evaluate((selector) => {
    window.__sidebarRequests = [];
    document.querySelector(selector).addEventListener('htmx:beforeRequest', (event) => {
      if (event.target.id === 'board-sidebar') window.__sidebarRequests.push(performance.now());
    });
  }, SIDEBAR);
}

async function sidebarRequests(page) {
  return page.evaluate(() => window.__sidebarRequests.slice());
}

// Waits until the sidebar has made no request for `quietMs` (or `timeoutMs` passes). It replaces a fixed sleep: the
// stream connects after the page loads and its "reset" notice refetches the sidebar once, at a time that depends on
// how fast the server is.
async function untilQuiet(page, quietMs, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let seen = (await sidebarRequests(page)).length;
  while (Date.now() < deadline) {
    await sleep(quietMs);
    const now = (await sidebarRequests(page)).length;
    if (now === seen) return;
    seen = now;
  }
  throw new Error('la barra lateral no se estabiliza: sigue pidiendo recargas');
}

// Waits until the sidebar has made at least `count` requests.
async function untilRequests(page, count, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if ((await sidebarRequests(page)).length >= count) return;
    await sleep(50);
  }
  throw new Error(`la barra lateral no llegó a ${count} recargas`);
}

// Opens the shell and waits until the sidebar has loaded and its stream-driven first refetch is over. The recorder is
// installed before the wait and emptied after it, so the tests count only the requests they cause.
async function openShell(ctx) {
  await ctx.goto('/');
  await ctx.page.locator(SIDEBAR).waitFor({ timeout: 10000 });
  await recordSidebarRequests(ctx.page);
  await untilQuiet(ctx.page, 500, 10000);
  await ctx.page.evaluate(() => { window.__sidebarRequests.length = 0; });
}

// Keeps a reference to the first node inside the sidebar, so the test can tell if a refresh replaced it. A morph
// removes attributes the server does not send, so the identity of the node is what is compared, not a marker.
async function markSidebarNode(page) {
  await page.evaluate((selector) => {
    window.__sidebarNode = document.querySelector(selector).firstElementChild;
  }, SIDEBAR);
}

// Fires the notices and returns performance.now() right after the last one.
async function fireChanged(page, times, gapMs) {
  return page.evaluate(async ({ selector, times, gapMs }) => {
    const element = document.querySelector(selector);
    for (let i = 0; i < times; i += 1) {
      element.dispatchEvent(new CustomEvent('sse:changed', { detail: { data: 'task' } }));
      if (gapMs > 0) await new Promise((resolve) => setTimeout(resolve, gapMs));
    }
    return performance.now();
  }, { selector: SIDEBAR, times, gapMs });
}

module.exports = {
  name: 'board_refresh',
  tests: [
    {
      name: 'sidebar_burst: diez avisos seguidos acaban en una recarga que empieza después del último',
      fn: async (ctx) => {
        await openShell(ctx);
        const lastNotice = await fireChanged(ctx.page, 10, 0);
        await untilRequests(ctx.page, 1, 5000);
        await untilQuiet(ctx.page, 600, 10000);
        const times = await sidebarRequests(ctx.page);
        ctx.assert(times.length >= 1, 'la ráfaga debe provocar una recarga');
        ctx.assert(times.length <= 2, `la ráfaga no debe pedir una recarga por aviso (hubo ${times.length})`);
        ctx.assert(times[times.length - 1] > lastNotice, 'la última recarga debe empezar después del último aviso, o el cambio final se pierde');
      },
    },
    {
      name: 'sidebar_spaced: avisos separados por más de la ventana provocan una recarga cada uno',
      fn: async (ctx) => {
        await openShell(ctx);
        for (let i = 0; i < 3; i += 1) {
          await fireChanged(ctx.page, 1, 0);
          await untilRequests(ctx.page, i + 1, 5000);
          await untilQuiet(ctx.page, 400, 10000);
        }
        const times = await sidebarRequests(ctx.page);
        ctx.assert(times.length === 3, `cada aviso aislado debe recargar la barra (hubo ${times.length})`);
      },
    },
    {
      name: 'sidebar_morph: la recarga conserva los nodos de la barra lateral',
      fn: async (ctx) => {
        await openShell(ctx);
        await markSidebarNode(ctx.page);
        await fireChanged(ctx.page, 1, 0);
        await untilRequests(ctx.page, 1, 5000);
        await untilQuiet(ctx.page, 500, 10000);
        const kept = await ctx.page.evaluate((selector) => {
          const first = document.querySelector(selector).firstElementChild;
          return first === window.__sidebarNode;
        }, SIDEBAR);
        ctx.assert(kept, 'la recarga debe actualizar los nodos en su sitio (morph), no reemplazarlos');
      },
    },
  ],
};
