// Create form of the Groups screen (Workspaces > Groups). The form posts to the group endpoint, which answers 204 and
// refreshes the list; the refresh keeps the typed text of an input, so the form is emptied here once the group exists.

// resetCreatedForm empties the create form after a successful request. It returns whether it did.
export function resetCreatedForm(event) {
  const target = event && event.target;
  if (!target || !target.matches || !target.matches('form[data-group-create]')) return false;
  if (!event.detail || !event.detail.successful) return false;
  target.reset();
  return true;
}

export function setupGroupManager(root = document) {
  if (!root.body) return;
  root.body.addEventListener('htmx:afterRequest', resetCreatedForm);
}

if (typeof document !== 'undefined') setupGroupManager(document);
