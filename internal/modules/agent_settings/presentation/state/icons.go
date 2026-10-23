package state

// Icons of the bar selectors, copied from the Rust components. They are inline SVG fragments that
// use currentColor, so they follow the theme.
const (
	// IconRuntime is a terminal window, for the agent implementation selector.
	IconRuntime = `<svg viewBox="0 0 20 20" fill="none"><rect x="2.5" y="3.5" width="15" height="13" rx="2" stroke="currentColor" stroke-width="1.5"></rect><path d="M6 8L9 10.5L6 13M10.5 13H14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"></path></svg>`
	// IconModel is a chip, for the model selector.
	IconModel = `<svg viewBox="0 0 20 20" fill="none"><rect x="6" y="6" width="8" height="8" rx="1.5" stroke="currentColor" stroke-width="1.5"></rect><path d="M8.5 2.5V6M11.5 2.5V6M8.5 14V17.5M11.5 14V17.5M2.5 8.5H6M2.5 11.5H6M14 8.5H17.5M14 11.5H17.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"></path></svg>`
	// IconProfile is a stack of layers, for the profile selector.
	IconProfile = `<svg viewBox="0 0 20 20" fill="none"><path d="M10 3L17 7L10 11L3 7L10 3Z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"></path><path d="M3 10.5L10 14.5L17 10.5M3 14L10 18L17 14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"></path></svg>`
	// IconThinking is a gauge, for the reasoning selector.
	IconThinking = `<svg viewBox="0 0 20 20" fill="none"><path d="M3 14A7 7 0 0 1 17 14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"></path><path d="M10 14L13 9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"></path><circle cx="10" cy="14" r="1" fill="currentColor"></circle></svg>`
)
