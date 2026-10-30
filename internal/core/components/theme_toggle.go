package components

// ThemeStorageKey is the localStorage key of the theme preference (shared with the init script).
const ThemeStorageKey = "pi-theme"

// ThemeMode is the user's theme preference.
type ThemeMode string

// Theme preferences. The values are the stored form.
const (
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
	ThemeSystem ThemeMode = "system"
)

// ParseThemeMode reads a stored preference; unknown values fall back to ThemeSystem.
func ParseThemeMode(value string) ThemeMode {
	switch ThemeMode(value) {
	case ThemeLight:
		return ThemeLight
	case ThemeDark:
		return ThemeDark
	default:
		return ThemeSystem
	}
}

// Resolve returns the effective theme ("light" or "dark") for the system preference.
func (m ThemeMode) Resolve(systemPrefersDark bool) string {
	switch {
	case m == ThemeDark, m == ThemeSystem && systemPrefersDark:
		return "dark"
	default:
		return "light"
	}
}

// ThemeToggleOption is one choice of the theme selector.
type ThemeToggleOption struct {
	Mode  ThemeMode
	Label string
}

// ThemeToggleOptions lists the choices in the order the selector shows them.
func ThemeToggleOptions(light, dark, auto string) []ThemeToggleOption {
	return []ThemeToggleOption{
		{Mode: ThemeLight, Label: light},
		{Mode: ThemeDark, Label: dark},
		{Mode: ThemeSystem, Label: auto},
	}
}

// ThemeToggleProps configures the light / dark / system selector. Selected is the stored mode.
type ThemeToggleProps struct {
	Selected ThemeMode
}

func themeChipClass(selected bool) string {
	if selected {
		return "chip chip-selected flex-1 justify-center"
	}
	return "chip flex-1 justify-center"
}
