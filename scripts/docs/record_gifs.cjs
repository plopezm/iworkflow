// scripts/docs/record_gifs.cjs
// Records the scenes of the documentation GIFs against the Go server that end2end/go/run_go.sh starts with the
// demo agent (scripts/docs/demo_agent.sh). Run it through scripts/docs/record_gifs.sh, which sets the environment.
//
// Each scene is one browser context with video recording. The setup a scene needs (workspace registration, the
// tasks it starts from) runs in a separate context that is not recorded, so the videos only show the actions of
// the scene. Output: $DOCS_GIF_OUT/<scene>.webm. A scene that fails keeps its partial video as <scene>.failed.webm
// (record_gifs.sh does not convert those) and the process exits non-zero after the other scenes ran.
//
// Environment (set by run_go.sh and record_gifs.sh):
//   SMOKE_BASE_URL   server address             PLAYWRIGHT_MODULE  playwright package path
//   DOCS_GIF_OUT     output directory (required)  WORK_TMP_DIR       scratch directory for demo repositories
//   DOCS_GIF_SCENES  optional comma list of scene names to record (default: all)
//   SMOKE_LANG       must be "en" (lang_shim.cjs reads it when preloaded; this script sets the cookie too)
'use strict';

const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const BASE = process.env.SMOKE_BASE_URL || '';
const OUT = process.env.DOCS_GIF_OUT || '';
const TMP = process.env.WORK_TMP_DIR || process.env.SMOKE_TMP_DIR || '/tmp';
const LANG = 'en';
const VIEW = Object.freeze({ width: 1280, height: 760 });
const TAG = Date.now().toString(36);

// A small dot that follows the pointer: Playwright video does not draw the cursor, and the viewer needs to see
// which control a step clicks. It is added to every page of the recording contexts.
const CURSOR_INIT = `(() => {
  const dot = document.createElement('div');
  dot.id = 'docs-cursor';
  dot.style.cssText = 'position:fixed;z-index:2147483647;width:18px;height:18px;margin:-9px 0 0 -9px;border-radius:50%;' +
    'background:rgba(239,68,68,.85);border:2px solid #fff;box-shadow:0 0 0 2px rgba(0,0,0,.25);pointer-events:none;' +
    'left:-40px;top:-40px;transition:left .06s linear,top .06s linear,transform .12s';
  const attach = () => document.documentElement.appendChild(dot);
  if (document.documentElement) attach(); else document.addEventListener('DOMContentLoaded', attach);
  document.addEventListener('mousemove', (e) => { dot.style.left = e.clientX + 'px'; dot.style.top = e.clientY + 'px'; }, true);
  document.addEventListener('mousedown', () => { dot.style.transform = 'scale(.7)'; }, true);
  document.addEventListener('mouseup', () => { dot.style.transform = 'scale(1)'; }, true);
})();`;

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
// A human-paced pause: the viewer needs time to read what changed.
const beat = (ms = 700) => sleep(ms);

async function newContext(browser, recordDir) {
  const options = { viewport: VIEW, deviceScaleFactor: 1 };
  if (recordDir) options.recordVideo = { dir: recordDir, size: VIEW };
  const ctx = await browser.newContext(options);
  await ctx.addCookies([{ name: 'iw-lang', value: LANG, url: BASE }]);
  await ctx.addInitScript(CURSOR_INIT);
  return ctx;
}

// Types slowly, the way a person writes into a form: a click, then one key at a time.
async function typeSlowly(locator, text) {
  await locator.click();
  await locator.pressSequentially(text, { delay: 40 });
  await beat(200);
}

// The first visible element of a locator: pages keep hidden copies of some controls (mobile menus, other tabs).
async function firstVisible(locator, timeout) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const count = await locator.count();
    for (let index = 0; index < count; index += 1) {
      const candidate = locator.nth(index);
      if (await candidate.isVisible()) return candidate;
    }
    await sleep(200);
  }
  throw new Error(`no visible element for ${locator}`);
}

// Hovers and clicks: the pointer moves to the control first so the viewer sees the target.
async function clickVisible(locator, { timeout = 15000 } = {}) {
  const target = await firstVisible(locator, timeout);
  await target.hover();
  await beat(150);
  await target.click();
}

async function goto(page, route) {
  await page.goto(BASE + route, { waitUntil: 'domcontentloaded' });
  await beat(250);
}

// The dashboard (running and pending tasks plus the graph) is the "Board" tab of the dock: opened from the sidebar.
async function openDashboard(page) {
  await goto(page, '/');
  await clickVisible(page.locator('a.sidebar-nav[data-dock-path="/"]'));
  await page.locator('[data-graph-canvas]').first().waitFor({ state: 'visible', timeout: 15000 });
  await beat(200);
}

// A git repository with one commit: a workspace needs a real one.
function makeRepo(label) {
  const dir = path.join(TMP, `docs-demo-${label}-${TAG}`);
  fs.mkdirSync(dir, { recursive: true });
  const git = (...args) => execFileSync('git', ['-C', dir, '-c', 'user.name=docs', '-c', 'user.email=docs@localhost', ...args], { stdio: 'ignore' });
  git('init', '-q', '-b', 'main');
  fs.writeFileSync(path.join(dir, 'README.md'), '# Demo project\n\nA small project used for the documentation GIFs.\n');
  git('add', '.');
  git('commit', '-qm', 'init');
  return dir;
}

// Registers a workspace through the UI (unrecorded setup) and returns its name and repository path.
async function registerWorkspace(browser, label) {
  const repo = makeRepo(label);
  const ctx = await newContext(browser, null);
  const page = await ctx.newPage();
  // /workspaces opens as a dock tab from the sidebar; its "+" button shows the registration form.
  await goto(page, '/');
  // Two links point to /workspaces once a workspace exists: the menu item "Workspaces" is the one to use.
  await clickVisible(page.locator('a[href="/workspaces"].menu-item').filter({ hasText: /Workspaces\s*$/ }));
  await clickVisible(page.locator('span[aria-hidden="true"]').filter({ hasText: /^\+$/ }));
  await page.getByLabel(/^Name/).first().fill(label);
  await page.getByLabel(/^Absolute path/).first().fill(repo);
  await page.getByLabel(/^Base branch/).first().fill('main');
  await page.getByRole('button', { name: /^Register( workspace)?$/ }).first().click();
  await page.locator('#board-sidebar').getByRole('link', { name: label }).first().waitFor({ timeout: 15000 });
  await ctx.close();
  return { name: label, repo };
}

// Creates a task from the "New task" form of the board of a workspace (unrecorded setup helper).
async function createTaskUnrecorded(browser, workspace, prompt, { start = false } = {}) {
  const ctx = await newContext(browser, null);
  const page = await ctx.newPage();
  await openBoardOf(page, workspace);
  await createTaskForm(page, workspace, prompt, { start });
  await ctx.close();
}

async function openBoardOf(page, workspace) {
  await goto(page, '/');
  await clickVisible(page.locator('#board-sidebar').getByRole('link', { name: workspace }));
  await page.locator('[id^="board-list-"]:visible').first().waitFor({ timeout: 15000 });
  await beat(300);
}

async function createTaskForm(page, workspace, prompt, { start }) {
  await clickVisible(page.getByRole('button', { name: /New task/ }));
  await page.getByLabel(/^Workspace/).first().selectOption({ label: `${workspace} (main)` });
  await typeSlowly(page.getByLabel('Prompt', { exact: true }).first(), prompt);
  if (start) await page.getByLabel('Start now', { exact: true }).first().check();
  await beat(200);
  await clickVisible(page.getByRole('button', { name: /Create task/ }));
  await page.getByRole('button', { name: /Create task/ }).first().waitFor({ state: 'detached', timeout: 15000 });
  await page.locator('[data-task-id]:visible', { hasText: prompt }).first().waitFor({ state: 'attached', timeout: 15000 });
}

// The id of the task whose card shows a prompt, read from its link.
async function taskIdOf(page, prompt) {
  const link = page.locator('[data-task-id]:visible', { hasText: prompt }).locator('a[href*="/tasks/"]').first();
  await link.waitFor({ state: 'attached', timeout: 15000 });
  const href = await link.getAttribute('href');
  const match = href && href.match(/\/tasks\/([0-9a-f-]{36})/);
  if (!match) throw new Error(`no task id in link "${href}"`);
  return match[1];
}

// Finish and review, read the diff and merge: the review path shared by the task scenes. The task is awaiting review
// when its status reads "Awaiting review" (the same signal the e2e suite waits for).
async function finishReviewAndMerge(page, { showDiff, taskTitle }) {
  // The Actions panel is the default one; it is opened only when its controls are not on screen yet.
  const finish = page.getByRole('button', { name: /Finish and review/ }).first();
  if (!(await finish.isVisible().catch(() => false))) {
    await clickVisible(page.getByText('Actions', { exact: true }));
  }
  await clickVisible(page.getByRole('button', { name: /Finish and review/ }), { timeout: 40000 });
  await page.getByText('Awaiting review', { exact: true }).first().waitFor({ state: 'visible', timeout: 25000 });
  await beat(400);
  if (showDiff) {
    await clickVisible(page.getByRole('button', { name: /View diff/ }), { timeout: 15000 });
    await beat(1300);
    // The diff opens as its own tab: go back to the task tab (its title is the exact prompt) to merge.
    await clickVisible(page.getByText(taskTitle, { exact: true }), { timeout: 10000 });
    await beat(400);
  }
  await page.getByText('Actions', { exact: true }).first().click({ timeout: 3000 }).catch(() => {});
  await beat(200);
  await clickVisible(page.getByRole('button', { name: 'Merge', exact: true }), { timeout: 15000 });
  await page.getByRole('button', { name: 'Merge', exact: true }).first().waitFor({ state: 'detached', timeout: 30000 });
  await beat(300);
}

const SCENES = {
  // The workflow graph: three tasks chained on the board; the first one runs, is reviewed and merged, and the
  // successors start by themselves.
  async 'task-graph'(page, browser) {
    const ws = await registerWorkspace(browser, `graph-${TAG}`);
    const titles = ['Parse the input file', 'Write the unit tests', 'Update the docs'];
    for (const title of titles) await createTaskUnrecorded(browser, ws.name, title);

    await openDashboard(page);
    const canvas = page.locator('[data-graph-canvas]').first();
    for (const [index, title] of titles.entries()) {
      const card = page.locator('[data-dashboard-task]', { hasText: title }).first();
      await card.hover();
      await beat(150);
      await clickVisible(card.getByRole('button', { name: /Add to graph/ }));
      const node = page.locator('.graph-node', { hasText: title }).first();
      await node.waitFor({ state: 'visible', timeout: 15000 });
      // Adding a node re-lays out the dashboard and can scroll the page: measure the canvas again for each node.
      await canvas.scrollIntoViewIfNeeded();
      const box = await canvas.boundingBox();
      if (!box) throw new Error('graph canvas has no box');
      await dragNode(page, node, box.x + 60 + index * 340, box.y + 60);
    }

    for (let index = 0; index < titles.length - 1; index += 1) {
      await connect(page, page.locator('.graph-node', { hasText: titles[index] }).first(),
        page.locator('.graph-node', { hasText: titles[index + 1] }).first());
      await page.locator(`.graph-edges g[data-edge-key]`).nth(index).waitFor({ state: 'attached', timeout: 15000 });
    }
    await beat(900);

    // Start the first task from its dashboard card, follow its run, then finish, review and merge it.
    await clickVisible(page.locator('[data-dashboard-task]', { hasText: titles[0] }).first().getByRole('button', { name: /^Start$/ }));
    await beat(200);
    const firstId = await page.locator('.graph-node', { hasText: titles[0] }).first().getAttribute('data-task-id');
    await goto(page, `/tasks/${firstId}`);
    await page.getByText(/^Done\./).first().waitFor({ state: 'visible', timeout: 40000 });
    await beat(150);
    await finishReviewAndMerge(page, { showDiff: false, taskTitle: titles[0] });

    // The successors start on their own once their predecessor is merged.
    await openDashboard(page);
    await page.locator('[data-dashboard-task]', { hasText: titles[1] }).first().waitFor({ state: 'visible', timeout: 15000 });
    await beat(400);
  },

  // The review path: a task is started from the board, followed live, finished, its diff is read and it is merged.
  async 'task-review'(page, browser) {
    const ws = await registerWorkspace(browser, `review-${TAG}`);
    const prompt = 'Add a slugify helper for task titles';
    await openBoardOf(page, ws.name);
    await createTaskForm(page, ws.name, prompt, { start: true });
    const id = await taskIdOf(page, prompt);
    await goto(page, `/tasks/${id}`);
    // The agent streams its reasoning, tool calls and reply; the reply closes the run.
    await page.getByText(/^Done\./).first().waitFor({ state: 'visible', timeout: 40000 });
    // The reasoning is folded by default: open it to show what the agent thought (best effort).
    await page.getByText('Reasoning', { exact: true }).first().click({ timeout: 3000 }).catch(() => {});
    await beat(800);
    await finishReviewAndMerge(page, { showDiff: true, taskTitle: prompt });
    await beat(600);
  },

  // The usage bar in the sidebar footer: Claude at the normal level, Codex at warn and critical; zoomed for reading.
  async 'usage-limits'(page) {
    await goto(page, '/');
    await page.locator('#usage-bar [data-usage-provider]').first().waitFor({ state: 'visible', timeout: 20000 });
    await beat(900);
    // Claude at the normal level, then Codex at warn (5 h) and critical (week); then the zoomed view to read it.
    await page.locator('[data-usage-provider="claude"]').first().hover();
    await beat(1400);
    await page.locator('[data-usage-provider="codex"]').first().hover();
    await beat(1600);
    // Zoom the page a little and keep the footer bar in view (the sidebar scrolls when the zoom makes it taller).
    await page.evaluate(() => {
      document.documentElement.style.zoom = '1.25';
      document.getElementById('usage-bar')?.scrollIntoView({ block: 'end' });
    });
    await beat(3200);
  },

  // Task hooks: a script hook is created, assigned to a project before a task starts, and its run is shown in the task.
  async 'task-hooks'(page, browser) {
    const ws = await registerWorkspace(browser, `hooks-${TAG}`);
    const hookName = `lint-check-${TAG}`;
    const prompt = 'Fix the flaky login test';

    await goto(page, '/hooks');
    await page.locator('[data-hooks-screen]').waitFor({ state: 'visible', timeout: 15000 });
    await clickVisible(page.getByRole('button', { name: 'New hook +', exact: true }));
    const editor = page.locator('[data-hook-editor]');
    await editor.waitFor({ state: 'visible', timeout: 15000 });
    await typeSlowly(editor.getByLabel('Name', { exact: true }), hookName);
    await typeSlowly(editor.getByLabel('Command', { exact: true }), 'echo "starting {{.TaskTitle}}"');
    await clickVisible(editor.getByRole('button', { name: 'Save', exact: true }));
    const row = page.locator('[data-hook]', { hasText: hookName }).first();
    await row.waitFor({ state: 'visible', timeout: 15000 });
    await beat(500);

    await clickVisible(row.getByRole('button', { name: 'Assignments', exact: true }));
    await page.locator('[data-hook-assignments]').waitFor({ state: 'visible', timeout: 15000 });
    await clickVisible(page.getByRole('button', { name: /Assign to a project/ }));
    const form = page.locator('[data-assignment-form]');
    await form.waitFor({ state: 'visible', timeout: 15000 });
    await form.locator('select[name="project"]').selectOption({ label: ws.name });
    await form.locator('select[name="event"]').selectOption('task_start');
    await form.locator('select[name="phase"]').selectOption('pre');
    await beat(500);
    await clickVisible(form.locator('button[type="submit"]'));
    await page.locator('[data-assignment]', { hasText: ws.name }).first().waitFor({ state: 'visible', timeout: 15000 });
    await beat(700);

    // "Test" runs the hook on the chosen project from the assignments view and shows its output.
    await clickVisible(page.getByRole('button', { name: 'Test', exact: true }));
    const testForm = page.locator('[data-hook-test-form]');
    await testForm.waitFor({ state: 'visible', timeout: 15000 });
    await testForm.locator('select[name="project"]').selectOption({ label: ws.name });
    await beat(500);
    await clickVisible(testForm.getByRole('button', { name: 'Test', exact: true }));
    await page.locator('[data-hook-test]').first().waitFor({ state: 'visible', timeout: 20000 });
    await beat(1500);

    // Start a task of the project: the hook runs before it and its run shows in the task.
    await openBoardOf(page, ws.name);
    await createTaskForm(page, ws.name, prompt, { start: true });
    const id = await taskIdOf(page, prompt);
    await goto(page, `/tasks/${id}`);
    const strip = page.locator('[data-task-hooks]');
    await strip.waitFor({ state: 'visible', timeout: 20000 });
    // The strip lists the hook of the task start with its command and its result.
    await strip.getByText('OK', { exact: true }).first().waitFor({ state: 'visible', timeout: 30000 });
    await beat(2500);
  },
};

// Moves a node so that its top-left corner reaches a screen point. The canvas drags a node from its card (not from
// its buttons), so the press is on the title area of the card, with the same offset for the move.
async function dragNode(page, node, targetX, targetY) {
  const card = await firstVisible(node.locator('.task-node-card, article.card'), 15000);
  const box = await card.boundingBox();
  if (!box) throw new Error('node card has no box');
  const fromX = box.x + 40;
  const fromY = box.y + 14;
  const dx = targetX - box.x;
  const dy = targetY - box.y;
  await page.mouse.move(fromX, fromY);
  await beat(100);
  await page.mouse.down();
  await page.mouse.move(fromX + dx, fromY + dy, { steps: 12 });
  await beat(80);
  await page.mouse.up();
  await beat(250);
}

// Drags from the output port of a node to another node: the dependency is saved by the canvas.
async function connect(page, from, to) {
  await from.hover();
  await beat(150);
  const port = from.locator('.graph-port-out').first();
  const portBox = await port.boundingBox();
  const targetBox = await to.boundingBox();
  if (!portBox || !targetBox) throw new Error('port or target has no box');
  await page.mouse.move(portBox.x + portBox.width / 2, portBox.y + portBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, { steps: 12 });
  await beat(120);
  await page.mouse.up();
  await beat(250);
}

async function main() {
  if (!BASE || !OUT) throw new Error('SMOKE_BASE_URL and DOCS_GIF_OUT are required (run scripts/docs/record_gifs.sh)');
  fs.mkdirSync(OUT, { recursive: true });
  const only = process.env.DOCS_GIF_SCENES ? new Set(process.env.DOCS_GIF_SCENES.split(',').map((s) => s.trim())) : null;
  const browser = await chromium.launch(process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH } : {});
  const failed = [];
  try {
    for (const [name, run] of Object.entries(SCENES)) {
      if (only && !only.has(name)) continue;
      const dir = path.join(OUT, `.${name}`);
      fs.mkdirSync(dir, { recursive: true });
      const ctx = await newContext(browser, dir);
      const page = await ctx.newPage();
      // Finish and Merge ask for confirmation first: the demo accepts them, as the e2e suite does.
      page.on('dialog', (dialog) => dialog.accept());
      const problems = [];
      page.on('pageerror', (err) => problems.push(`pageerror: ${err.message}`));
      page.on('response', (res) => { if (res.status() >= 500) problems.push(`HTTP ${res.status()} ${res.url()}`); });
      console.log(`▶ scene ${name}`);
      let error = null;
      try {
        await run(page, browser);
      } catch (err) {
        error = err;
      }
      const video = page.video();
      await ctx.close();
      const webm = video ? await video.path() : null;
      if (problems.length) console.log(`  ⚠ ${name}: ${problems.join(' | ')}`);
      if (error) {
        failed.push(name);
        console.error(`  ✗ ${name} failed: ${error.message.split('\n').filter((l) => l.trim()).slice(0, 4).join(' | ')}`);
        if (webm) fs.renameSync(webm, path.join(OUT, `${name}.failed.webm`));
      } else {
        fs.renameSync(webm, path.join(OUT, `${name}.webm`));
        console.log(`  ✓ ${name}`);
      }
      fs.rmSync(dir, { recursive: true, force: true });
    }
  } finally {
    await browser.close();
  }
  if (failed.length) {
    console.error(`scenes failed: ${failed.join(', ')}`);
    process.exitCode = 1;
  }
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
