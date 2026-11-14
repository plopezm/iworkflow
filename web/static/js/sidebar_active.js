// Marks the sidebar navigation link of the page that has the focus in the dock (aria-current="page"),
// which the stylesheet lights up. The dock publishes the focused page as data-focused-path.

// A link is the current page only when its path equals the focused one (exact match, no prefix match, so
// the home board "/" does not light up on every page). An empty path on either side marks nothing.
export function isCurrentPath(linkPath, focusedPath) {
  if (!linkPath || !focusedPath) return false;
  return linkPath === focusedPath;
}

export function markCurrent(root = document) {
  const dock = root.getElementById('dock');
  const focused = dock ? dock.getAttribute('data-focused-path') : '';
  root.querySelectorAll('a.sidebar-nav[data-dock-path]').forEach((link) => {
    if (isCurrentPath(link.getAttribute('data-dock-path'), focused)) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  });
}

export function setupSidebarActive(root = document) {
  if (!root.body) return;
  markCurrent(root);
  // The dock may swap its whole element, so watch the document, not the node that exists now.
  new MutationObserver(() => markCurrent(root)).observe(root.body, {
    subtree: true,
    childList: true,
    attributes: true,
    attributeFilter: ['data-focused-path'],
  });
}

if (typeof document !== 'undefined') setupSidebarActive(document);
