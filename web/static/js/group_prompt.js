// Carries the answer of an hx-prompt into the form field that the element names in data-prompt-field. htmx sends
// the answer only as the HX-Prompt header, while the workspace group endpoints read their fields from the form.

// carryPromptAnswer copies the HX-Prompt answer into the request parameters under the element's field. It returns
// whether it did: a request without an answer, or an element without a field, is left as it is.
export function carryPromptAnswer(detail, element) {
  const answer = detail && detail.headers ? detail.headers['HX-Prompt'] : undefined;
  const field = promptField(element);
  if (answer === undefined || answer === null || !field) return false;
  detail.parameters[field] = answer;
  return true;
}

// isBlankPromptAnswer reports whether a group prompt was answered with nothing but spaces. Such a request has no
// name to send, so it is cancelled instead of creating or renaming a group with an empty name.
export function isBlankPromptAnswer(detail, element) {
  const answer = detail && detail.headers ? detail.headers['HX-Prompt'] : undefined;
  return typeof answer === 'string' && answer.trim() === '' && Boolean(promptField(element));
}

function promptField(element) {
  return element && element.getAttribute ? element.getAttribute('data-prompt-field') : null;
}

export function setupGroupPrompt(root = document) {
  if (!root.body) return;
  root.body.addEventListener('htmx:configRequest', (event) => {
    const element = event.detail.elt || event.target;
    if (isBlankPromptAnswer(event.detail, element)) {
      event.preventDefault();
      return;
    }
    carryPromptAnswer(event.detail, element);
  });
}

if (typeof document !== 'undefined') setupGroupPrompt(document);
