// Runner for the e2e suite (CommonJS). Starts the shared browser once, runs
// every end2end/*.e2e.cjs module in a fresh page/context, and reports a summary.
// Usage:
//   node end2end/run.e2e.cjs                 # all modules
//   node end2end/run.e2e.cjs workspaces      # only modules whose name/file matches
// Requires the server to be up on SMOKE_BASE_URL (started by end2end/run_tests.sh).
const fs = require('fs');
const path = require('path');
const harness = require('./harness.cjs');

const FILTER = process.argv[2] || '';
const SMOKE_TMP = process.env.SMOKE_TMP_DIR || '/tmp/iwf-e2e-fail';

function collectModules() {
  const dir = __dirname;
  const modules = fs.readdirSync(dir)
    .filter((f) => f.endsWith('.e2e.cjs'))
    .filter((f) => !f.startsWith('_') && f !== 'run.e2e.cjs') // throwaway probes + this runner
    .map((f) => ({ file: f, name: f.replace(/.e2e.cjs$/, '') }))
    .sort((a, b) => {
      // workspaces runs first: its "empty state" test needs a fresh (empty)
      // shared DB; the other modules may have registered workspaces.
      const aW = a.name === 'workspaces' ? 0 : 1;
      const bW = b.name === 'workspaces' ? 0 : 1;
      return aW === bW ? a.name.localeCompare(b.name) : aW - bW;
    });
  return modules.filter((m) => !FILTER || m.name.includes(FILTER) || m.file.includes(FILTER));
}

async function main() {
  const modules = collectModules();
  if (modules.length === 0) {
    console.log('No test modules found (end2end/*.e2e.cjs).');
    process.exit(0);
  }

  console.log(`\n▶ Running ${modules.length} module(s)${FILTER ? ` (filter: ${FILTER})` : ''} against ${harness.BASE}`);
  const browser = await harness.launchBrowser();
  const results = [];

  for (const mod of modules) {
    console.log(`\n── ${mod.name} (${mod.file})`);
    const spec = require(path.join(__dirname, mod.file));
    const tests = spec.tests || [];
    if (!Array.isArray(tests) || tests.length === 0) {
      console.log('  (no tests declared)');
      continue;
    }
    for (const t of tests) {
      const started = Date.now();
      try {
        const ctx = await harness.makePage();
        await t.fn(ctx);
        // A test that declares expectNoProblems() can force-fail on console noise.
        if (t.noProblems && ctx.getProblems().length > 0) {
          throw new Error(
            `test asserted no console/pageerror/HTTP problems but ${ctx.getProblems().length} found`,
          );
        }
        const ms = Date.now() - started;
        console.log(`  ✔ ${t.name}  (${ms} ms)`);
        results.push({ name: mod.name, test: t.name, status: 'pass', ms, errors: [] });
      } catch (e) {
        const ms = Date.now() - started;
        const err = [
          e.message,
          ...(typeof e.stack === 'string' ? e.stack.split('\n').slice(0, 4) : []),
        ];
        console.log(`  ✖ ${t.name}  (${ms} ms)`);
        err.forEach((l) => console.log('    ' + l));
        results.push({ name: mod.name, test: t.name, status: 'fail', ms, errors: err });
      }
    }
  }

  // Summary.
  const pass = results.filter((r) => r.status === 'pass').length;
  const fail = results.filter((r) => r.status === 'fail').length;
  const total = pass + fail;
  console.log(
    `\n\n════ Summary ═════\n  ${pass}/${total} passed${fail ? ` · ${fail} failed` : ''}`,
  );
  if (fail) {
    console.log('  Failed:');
    results
      .filter((r) => r.status === 'fail')
      .forEach((r) => console.log(`    - ${r.name}.${r.test}`));
    process.exit(1);
  }
  console.log('  All good.');
  await browser.close();
  process.exit(0);
}

main().catch((e) => {
  console.error('Runner failed:', e && e.message ? e.message : e);
  process.exit(2);
});
