// Unit test of the shared runtime helper against a fake page (no browser, no server):
//   node --test end2end/agent_settings_runtime_test.cjs
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { setAgentRuntime } = require('./agent_settings_runtime.cjs');

// fakePage keeps a stored runtime. selectOption stores the value unless storeAs overrides it (a server that does
// not keep the change). options are the select's options; status is the answer of the settings read.
function fakePage({ current, options = ['', 'claude', 'pi', 'cursor'], status = 200, storeAs = null }) {
  const calls = [];
  let stored = current;
  let shown = current;
  const select = {
    options: options.map((v) => ({ value: v })),
    async inputValue() {
      return shown;
    },
    async selectOption(v) {
      calls.push(['selectOption', v]);
      shown = v;
      stored = storeAs === null ? v : storeAs;
    },
    async evaluate(fn, target) {
      return fn({ options: select.options }, target);
    },
  };
  return {
    calls,
    url: () => 'http://127.0.0.1:1/workspaces/1',
    locator(selector) {
      calls.push(['locator', selector]);
      return select;
    },
    request: {
      async get(url) {
        calls.push(['get', url]);
        return {
          ok: () => status >= 200 && status < 300,
          status: () => status,
          json: async () => ({ runtime: stored === '' ? null : stored }),
        };
      },
    },
    async waitForTimeout() {},
  };
}

const selected = (page) => page.calls.filter((c) => c[0] === 'selectOption').map((c) => c[1]);

test('a different value is selected and read back from the settings API', async () => {
  const page = fakePage({ current: '' });
  await setAgentRuntime(page, 'pi');
  assert.deepEqual(selected(page), ['pi']);
  assert.ok(page.calls.some((c) => c[0] === 'get' && c[1].endsWith('/api/agent-settings')));
});

test('a value that is already the target is changed to another runtime first, so a save really happens', async () => {
  const page = fakePage({ current: 'pi' });
  await setAgentRuntime(page, 'pi');
  assert.deepEqual(selected(page), ['claude', 'pi']);
});

test('a failed settings read fails with the HTTP status', async () => {
  const page = fakePage({ current: '', status: 500 });
  await assert.rejects(setAgentRuntime(page, 'pi'), /HTTP 500/);
});

test('a runtime the server does not store fails and names the stored one', async () => {
  const page = fakePage({ current: '', storeAs: 'claude' });
  await assert.rejects(setAgentRuntime(page, 'pi'), /guardado es «claude», no «pi»/);
});

test('without another runtime to change to, the helper says so', async () => {
  const page = fakePage({ current: 'pi', options: ['', 'pi'] });
  await assert.rejects(setAgentRuntime(page, 'pi'), /no hay otro agente/);
});
