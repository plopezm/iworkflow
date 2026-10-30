// Run with: node --test web/static/js/
// The related projects list of a task form, driven through small stand-ins for the form and its rows: the row of
// the workspace the task belongs to is hidden and unchecked, and the other rows are left as they are.
import test from 'node:test';
import assert from 'node:assert/strict';

// The task form script installs its listeners on the document when it is imported, so the stand-in is set first.
globalThis.document = { addEventListener() {}, querySelectorAll: () => [] };
const { syncOwnProject } = await import('./task_form.js');

// One related project row: the label that wraps the checkbox, and the checkbox with its workspace id.
function projectRow(workspaceId, checked = false) {
  const row = { hidden: false };
  const input = {
    name: 'related_project_id',
    dataset: { workspaceId },
    checked,
    closest: (selector) => (selector === 'label' ? row : null),
  };
  return { row, input };
}

// A form holding the rows and the workspace select; the select's value is the chosen workspace.
function taskForm(value, rows) {
  const inputs = rows.map((r) => r.input);
  const form = {
    querySelectorAll: (selector) =>
      selector === '[data-related-projects] input[name="related_project_id"]' ? inputs : [],
  };
  const select = { name: 'workspace_id', value, closest: (selector) => (selector === 'form' ? form : null) };
  return select;
}

test('the row of the chosen workspace is hidden and unchecked, and the others stay as they were', () => {
  const app = projectRow('app', true);
  const api = projectRow('api', true);
  const web = projectRow('web');
  const select = taskForm('app', [app, api, web]);

  syncOwnProject(select);

  assert.equal(app.row.hidden, true, 'the own project is not offered');
  assert.equal(app.input.checked, false, 'a stale check of the own project is dropped');
  assert.equal(api.row.hidden, false);
  assert.equal(api.input.checked, true, 'the other checks are kept');
  assert.equal(web.row.hidden, false);
  assert.equal(web.input.checked, false);
});

test('changing the workspace shows the previous one again and hides the new one', () => {
  const app = projectRow('app');
  const api = projectRow('api');
  const select = taskForm('app', [app, api]);
  syncOwnProject(select);

  select.value = 'api';
  syncOwnProject(select);

  assert.equal(app.row.hidden, false, 'a workspace that is no longer the own one is offered again');
  assert.equal(api.row.hidden, true);
  assert.equal(api.input.checked, false);
});

test('no workspace chosen hides nothing', () => {
  const app = projectRow('app', true);
  const select = taskForm('', [app]);

  syncOwnProject(select);

  assert.equal(app.row.hidden, false);
  assert.equal(app.input.checked, true);
});

test('a form without a related projects list is left alone', () => {
  const select = taskForm('app', []);

  assert.doesNotThrow(() => syncOwnProject(select));
});
