package components

import "iworkflow/internal/core/i18n"

// PaletteStorageKey is the localStorage key of the color theme (shared with the init script and ui.js).
const PaletteStorageKey = "pi-palette"

// Palette is a color theme. The default keeps the original look; the others override the accent and tint
// the surfaces through the data-palette attribute of the document.
type Palette string

// Color themes. The values are the stored form and the data-palette attribute value.
const (
	PaletteDefault Palette = "default"
	PaletteOcean   Palette = "ocean"
	PaletteForest  Palette = "forest"
	PaletteSunset  Palette = "sunset"
	PaletteRose    Palette = "rose"
)

// ParsePalette reads a stored value; unknown values fall back to PaletteDefault.
func ParsePalette(value string) Palette {
	for _, option := range PaletteOptions() {
		if string(option.Palette) == value {
			return option.Palette
		}
	}
	return PaletteDefault
}

// PaletteOption is one choice of the color theme selector.
type PaletteOption struct {
	Palette Palette
	Label   i18n.Msg
}

// PaletteOptions lists the color themes in the order the selector shows them.
func PaletteOptions() []PaletteOption {
	return []PaletteOption{
		{Palette: PaletteDefault, Label: msgPaletteDefault},
		{Palette: PaletteOcean, Label: msgPaletteOcean},
		{Palette: PaletteForest, Label: msgPaletteForest},
		{Palette: PaletteSunset, Label: msgPaletteSunset},
		{Palette: PaletteRose, Label: msgPaletteRose},
	}
}

var (
	msgPaletteTitle = i18n.Msg{En: "Color theme", Es: "Tema de color", Zh: "配色主题", De: "Farbdesign", Fr: "Thème de couleurs"}
	msgPaletteHelp  = i18n.Msg{
		En: "Colors of the interface. The change applies at once and is kept on this device.",
		Es: "Colores de la interfaz. El cambio se aplica al instante y se guarda en este dispositivo.",
		Zh: "界面的颜色。更改会立即生效，并保存在此设备上。",
		De: "Farben der Oberfläche. Die Änderung gilt sofort und wird auf diesem Gerät gespeichert.",
		Fr: "Couleurs de l'interface. Le changement s'applique immédiatement et reste sur cet appareil.",
	}
	msgPaletteDefault = i18n.Msg{En: "Default", Es: "Predeterminado", Zh: "默认", De: "Standard", Fr: "Par défaut"}
	msgPaletteOcean   = i18n.Msg{En: "Ocean", Es: "Océano", Zh: "海洋", De: "Ozean", Fr: "Océan"}
	msgPaletteForest  = i18n.Msg{En: "Forest", Es: "Bosque", Zh: "森林", De: "Wald", Fr: "Forêt"}
	msgPaletteSunset  = i18n.Msg{En: "Sunset", Es: "Atardecer", Zh: "日落", De: "Sonnenuntergang", Fr: "Coucher de soleil"}
	msgPaletteRose    = i18n.Msg{En: "Rose", Es: "Rosa", Zh: "玫瑰", De: "Rose", Fr: "Rose"}
)
