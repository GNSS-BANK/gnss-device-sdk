package plot

import (
	_ "embed"
	"errors"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
	"golang.org/x/image/font/gofont/goregular"
)

//go:embed assets/fonts/OpenGostTypeA-Regular.ttf
var gostTypeAData []byte

var gostTypeAResource = fyne.NewStaticResource("OpenGostTypeA-Regular.ttf", gostTypeAData)

const (
	defaultFontSize float32 = 14
	minFontSize     float32 = 6
	maxFontSize     float32 = 72
)

type plotLayoutMetrics struct {
	scale        float32
	marginLeft   float32
	marginRight  float32
	marginTop    float32
	marginBottom float32
	tickSize     float32
	legendSize   float32
	hoverSize    float32
}

func newPlotLayoutMetrics(fontSize float32) plotLayoutMetrics {
	scale := fontSize / defaultFontSize
	return plotLayoutMetrics{
		scale:        scale,
		marginLeft:   plotMarginLeft * scale,
		marginRight:  plotMarginRight * scale,
		marginTop:    plotMarginTop * scale,
		marginBottom: plotMarginBottom * scale,
		tickSize:     11 * scale,
		legendSize:   12 * scale,
		hoverSize:    12 * scale,
	}
}

func (metrics plotLayoutMetrics) value(defaultValue float32) float32 {
	return defaultValue * metrics.scale
}

func validateFontSize(size float32) error {
	if math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) || size < minFontSize || size > maxFontSize {
		return errors.New("plot font size must be finite and between 6 and 72")
	}
	return nil
}

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

func applyApplicationTypography(application fyne.App, font FontFamily, fontSize float32) error {
	if application == nil {
		return errors.New("fyne application is nil")
	}
	if err := validateFontFamily(font); err != nil {
		return err
	}
	if err := validateFontSize(fontSize); err != nil {
		return err
	}
	if font == FontDefault && fontSize == defaultFontSize {
		return nil
	}
	settings := application.Settings()
	base := settings.Theme()
	if base == nil {
		base = fynetheme.DefaultTheme()
	}
	var resource fyne.Resource
	if font == FontGOSTTypeA {
		resource = gostTypeAResource
	}
	settings.SetTheme(&fontTheme{
		base:     base,
		font:     resource,
		fontSize: fontSize,
	})
	return nil
}

type fontTheme struct {
	base     fyne.Theme
	font     fyne.Resource
	fontSize float32
}

func (theme *fontTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.base.Color(name, variant)
}

func (theme *fontTheme) Font(style fyne.TextStyle) fyne.Resource {
	if theme.font == nil {
		return theme.base.Font(style)
	}
	return theme.font
}

func (theme *fontTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.base.Icon(name)
}

func (theme *fontTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case fynetheme.SizeNameText:
		return theme.fontSize
	case fynetheme.SizeNameCaptionText:
		return theme.fontSize * 0.85
	case fynetheme.SizeNameSubHeadingText:
		return theme.fontSize * 1.15
	case fynetheme.SizeNameHeadingText:
		return theme.fontSize * 1.5
	}
	return theme.base.Size(name)
}

var _ fyne.Theme = (*fontTheme)(nil)
