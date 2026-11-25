// The "select all" box of the branch manager ticks every branch checkbox that can be deleted (protected rows are
// disabled and stay out). The list is swapped by htmx, so the listeners sit on the document; the box follows the rows.

const ALL = '[data-branch-select-all]';
const ROWS = '#branch-delete-form input[type=checkbox][name=branch]';

// deletableRows are the checkboxes of the rows the user can pick.
export function deletableRows(root = document) {
  return Array.from(root.querySelectorAll(ROWS)).filter((box) => !box.disabled);
}

// toggleAll ticks or clears every deletable row.
export function toggleAll(checked, root = document) {
  for (const box of deletableRows(root)) box.checked = checked;
}

// syncAll keeps the select-all box in step with the rows: ticked when all are, mixed when some are.
export function syncAll(root = document) {
  const all = root.querySelector(ALL);
  if (!all) return;
  const rows = deletableRows(root);
  const picked = rows.filter((box) => box.checked).length;
  all.checked = rows.length > 0 && picked === rows.length;
  all.indeterminate = picked > 0 && picked < rows.length;
}

export function setupBranchSelectAll(doc = document) {
  doc.addEventListener('change', (event) => {
    const target = event.target;
    if (!target || !target.matches) return;
    if (target.matches(ALL)) {
      toggleAll(target.checked, doc);
      syncAll(doc);
    } else if (target.matches(ROWS)) {
      syncAll(doc);
    }
  });
}

if (typeof document !== 'undefined') setupBranchSelectAll(document);
