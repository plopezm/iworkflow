// Package limits holds the business limits of an AGENTS.md file. It is a leaf package so the
// models and the errors (whose messages quote these limits) can both depend on it.
package limits

// MaxDescriptionChars is the maximum length of a description, counted in runes.
const MaxDescriptionChars = 200

// MaxContentBytes is the maximum size of an AGENTS.md file: 512 KiB.
const MaxContentBytes = 512 * 1024
