// Run with: node --test web/static/js/
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  applySuggestion,
  detectTrigger,
  matchingCommands,
  references,
  removeReference,
} from './composer.js';

// The rules of the message box, ported from the Rust composer_completion tests. Indexes are UTF-16 code units, the
// unit the browser reports for the caret, so no byte conversion is needed.

const atEnd = (text) => detectTrigger(text, text.length);

test('a slash command only triggers at the start of the message', () => {
  assert.deepEqual(atEnd('/pl'), { kind: 'command', start: 0, end: 3, query: 'pl' });
  assert.equal(atEnd('hello /pl'), null);
  assert.equal(atEnd('/plan now'), null);
});

test('an @ triggers after whitespace and reports the typed query', () => {
  const trigger = atEnd('look at @src/ma');
  assert.equal(trigger.kind, 'file');
  assert.equal(trigger.query, 'src/ma');
  assert.equal(trigger.start, 8);
  assert.equal(atEnd('mail me@host'), null, 'an @ inside a word is not a reference');
});

test('the caret in the middle of the text only sees the text before it', () => {
  const trigger = detectTrigger('fix @ab cd', 7);
  assert.equal(trigger.query, 'ab');
  assert.equal(trigger.end, 7);
});

test('a caret after a surrogate pair is read in UTF-16 units', () => {
  const text = 'ñ😀 @a';
  assert.equal(text.indexOf('@'), 4);
  const trigger = atEnd(text);
  assert.equal(trigger.kind, 'file');
  assert.equal(trigger.start, 4);
  assert.equal(trigger.query, 'a');
});

test('applying a suggestion replaces the token, keeps the tail and places the caret after the space', () => {
  const text = 'fix @ab now';
  const { text: out, caret } = applySuggestion(text, detectTrigger(text, 7), '@src/ab.rs');
  assert.equal(out, 'fix @src/ab.rs  now');
  assert.equal(out.slice(0, caret), 'fix @src/ab.rs ');
});

// The server lists the commands with their descriptions in the language of the request.
const COMMANDS = ['plan', 'agent', 'compact', 'clear', 'model', 'review', 'help'].map((name) => ({
  insert: `/${name}`,
  label: `/${name}`,
  hint: '',
}));

test('commands are filtered by a case-insensitive prefix', () => {
  assert.deepEqual(matchingCommands('C', COMMANDS).map((c) => c.label), ['/compact', '/clear']);
  assert.equal(matchingCommands('', COMMANDS).length, 7);
  assert.deepEqual(matchingCommands('zzz', COMMANDS), []);
});

test('references are distinct, in order of appearance, without the @', () => {
  assert.deepEqual(
    references('see @a.rs and @b/c.rs again @a.rs @ alone'),
    ['a.rs', 'b/c.rs'],
  );
});

test('removing a reference keeps the rest of the message', () => {
  assert.equal(removeReference('see @a.rs and @b.rs now', 'a.rs'), 'see and @b.rs now');
  assert.equal(removeReference('@a.rs', 'a.rs'), '');
});
