package plot

import (
	_ "embed"
	"errors"
	"image/color"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
	"golang.org/x/image/font/gofont/goregular"
)

//go:embed assets/fonts/OpenGostTypeA-Regular.ttf
var gostTypeAData []byte

var gostTypeAResource = fyne.NewStaticResource("OpenGostTypeA-Regular.ttf", gostTypeAData)

func validateFontFamily(font FontFamily) error {
	if font != FontDefault && font != FontGOSTTypeA {
		return errors.New("unknown plot font")
	}
	return nil
}

func fontData(font FontFamily) ([]byte, error) {
	switch font {
	case FontDefault:
		return goregular.TTF, nil
	case FontGOSTTypeA:
		return gostTypeAData, nil
	default:
		return nil, errors.New("unknown plot font")
	}
}

func applyApplicationFont(application fyne.App, font FontFamily) error {
	if application == nil {
		return errors.New("fyne application is nil")
	}
	if err := validateFontFamily(font); err != nil {
		return err
	}
	if font == FontDefault {
		return nil
	}
	settings := application.Settings()
	base := settings.Theme()
	if base == nil {
		base = fynetheme.DefaultTheme()
	}
	settings.SetTheme(&fontTheme{
		base: base,
		font: gostTypeAResource,
	})
	return nil
}

type fontTheme struct {
	base fyne.Theme
	font fyne.Resource
}

func (theme *fontTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.base.Color(name, variant)
}

func (theme *fontTheme) Font(fyne.TextStyle) fyne.Resource {
	return theme.font
}

func (theme *fontTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.base.Icon(name)
}

func (theme *fontTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.base.Size(name)
}

var _ fyne.Theme = (*fontTheme)(nil)
