// Run with: node --test web/static/js/
// The related projects list of a task form, driven through small stand-ins for the form and its rows: the row of
// the workspace the task belongs to is hidden and unchecked, and the other rows are left as they are.
import test from 'node:test';
import assert from 'node:assert/strict';

// The task form script installs its listeners on the document when it is imported, so the stand-in is set first.
globalThis.document = { addEventListener() {}, querySelectorAll: () => [] };

// The script tells the kinds of element apart with instanceof, so the stand-in elements are instances of these classes.
class FakeElement {}
class FakeSelect extends FakeElement {}
class FakeInput extends FakeElement {}
class FakeTextArea extends FakeElement {}
Object.assign(globalThis, {
  Element: FakeElement,
  HTMLSelectElement: FakeSelect,
  HTMLInputElement: FakeInput,
  HTMLTextAreaElement: FakeTextArea,
});

const { syncOwnProject, installTaskForm, filterRelatedProjects } = await import('./task_form.js');

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
  return Object.assign(new FakeSelect(), { name: 'workspace_id', value, closest: (selector) => (selector === 'form' ? form : null) });
}

// A document that records its listeners and dispatches events to them.
function documentWith() {
  const listeners = {};
  return {
    addEventListener(type, listener) {
      (listeners[type] ??= []).push(listener);
    },
    dispatch(type, event) {
      for (const listener of listeners[type] ?? []) listener(event);
    },
    listenerCount: (type) => (listeners[type] ?? []).length,
  };
}

// A fragment loaded by htmx: an element that answers the workspace selects it holds.
function fragment(selects) {
  const node = Object.create(FakeElement.prototype);
  node.querySelectorAll = (selector) => (selector === 'select[name="workspace_id"]' ? selects : []);
  return node;
}

// A checkbox or input that is not a related project row of the task form.
function plainInput(name) {
  return Object.assign(new FakeInput(), { name, matches: () => false });
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

test('choosing the workspace in the create dialog hides and unchecks its row through the change event', () => {
  const app = projectRow('app', true);
  const api = projectRow('api', true);
  const select = taskForm('app', [app, api]);
  const doc = documentWith();
  installTaskForm(doc);

  doc.dispatch('change', { target: select });

  assert.equal(app.row.hidden, true);
  assert.equal(app.input.checked, false);
  assert.equal(api.row.hidden, false);
  assert.equal(api.input.checked, true);
});

test('a create dialog loaded by htmx is synced when it loads', () => {
  const app = projectRow('app', true);
  const select = taskForm('app', [app]);
  const doc = documentWith();
  installTaskForm(doc);

  doc.dispatch('htmx:load', { target: fragment([select]) });

  assert.equal(app.row.hidden, true);
  assert.equal(app.input.checked, false);
});

// The edit page has no workspace select: the server does not render the own project, so loading it or changing
// one of its checkboxes must not throw nor touch the rows.
test('the edit page has no workspace select, so loading it and changing a checkbox sync nothing', () => {
  const api = projectRow('api', true);
  const doc = documentWith();
  installTaskForm(doc);

  assert.doesNotThrow(() => doc.dispatch('htmx:load', { target: fragment([]) }));
  assert.doesNotThrow(() => doc.dispatch('change', { target: plainInput('related_project_id') }));

  assert.equal(api.row.hidden, false);
  assert.equal(api.input.checked, true, 'the stored check is kept');
});

test('syncOwnProject does nothing without a select and does not throw', () => {
  const api = projectRow('api', true);
  taskForm('api', [api]);

  assert.doesNotThrow(() => syncOwnProject(null));
  assert.doesNotThrow(() => syncOwnProject(undefined));
  assert.equal(api.input.checked, true);
  assert.equal(api.row.hidden, false);
});

test('a non-element target of htmx:load is ignored', () => {
  const doc = documentWith();
  installTaskForm(doc);

  assert.doesNotThrow(() => doc.dispatch('htmx:load', { target: { notAnElement: true } }));
});

test('installing the task form twice on a document connects each listener once', () => {
  const doc = documentWith();
  installTaskForm(doc);
  installTaskForm(doc);

  const counts = {
    change: doc.listenerCount('change'),
    'htmx:load': doc.listenerCount('htmx:load'),
    paste: doc.listenerCount('paste'),
  };
  assert.deepEqual(counts, { change: 1, 'htmx:load': 1, paste: 1 });
});

// A related projects dialog with the given rows (name and path text) and its empty note.
function relatedDialog(texts, filterValue) {
  const rows = texts.map((text) => ({ dataset: { relatedText: text }, hidden: false, style: { display: '' } }));
  const empty = { hidden: true };
  const root = {
    querySelectorAll: (selector) => (selector === '[data-related-row]' ? rows : []),
    querySelector: (selector) => (selector === '[data-related-empty]' ? empty : null),
  };
  return { rows, empty, filter: { value: filterValue, closest: (selector) => (selector === '[data-related-projects]' ? root : null) } };
}

test('the related projects filter keeps the rows whose name or path contains the text, ignoring case', () => {
  const { rows, empty, filter } = relatedDialog(['api /repos/api', 'web /home/me/Sites'], '  SITES ');

  filterRelatedProjects(filter);

  assert.equal(rows[0].style.display, 'none');
  assert.equal(rows[1].style.display, '');
  assert.equal(empty.hidden, true, 'a match hides the empty note');
});

test('the related projects filter shows the empty note when nothing matches and every row when cleared', () => {
  const dialog = relatedDialog(['api /repos/api', 'web /repos/web'], 'zzz');

  filterRelatedProjects(dialog.filter);
  assert.equal(dialog.empty.hidden, false);
  assert.deepEqual(dialog.rows.map((row) => row.style.display), ['none', 'none']);

  dialog.filter.value = '';
  filterRelatedProjects(dialog.filter);
  assert.equal(dialog.empty.hidden, true);
  assert.deepEqual(dialog.rows.map((row) => row.style.display), ['', '']);
});

test('the related projects filter does not offer the row of the own project, which stays hidden', () => {
  const { rows, empty, filter } = relatedDialog(['app /repos/app'], 'app');
  rows[0].hidden = true;

  filterRelatedProjects(filter);

  assert.equal(rows[0].hidden, true, 'the filter never unhides it');
  assert.equal(empty.hidden, false, 'with nothing offered the empty note shows');
});
