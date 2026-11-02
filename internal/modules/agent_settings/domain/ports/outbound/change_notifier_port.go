package outbound

// ChangeNotifierPort announces that the stored settings changed, so every open page refreshes what
// it shows from them.
type ChangeNotifierPort interface {
	// SettingsChanged is called after a change was saved. It must not block.
	SettingsChanged()
}
