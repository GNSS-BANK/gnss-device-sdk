package plot

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"

	"fyne.io/fyne/v2/canvas"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type verticalTextState struct {
	mu    sync.RWMutex
	text  string
	color color.Color
	font  FontFamily
}

var (
	verticalFonts [2]verticalFontCache
)

type verticalFontCache struct {
	once sync.Once
	font *opentype.Font
	err  error
}

func newVerticalText() (*canvas.Raster, *verticalTextState) {
	state := &verticalTextState{color: color.White}
	raster := canvas.NewRaster(state.render)
	return raster, state
}

func (state *verticalTextState) set(text string, value color.Color, font FontFamily) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.text == text && colorsEqual(state.color, value) && state.font == font {
		return false
	}
	state.text = text
	state.color = value
	state.font = font
	return true
}

func (state *verticalTextState) render(width int, height int) image.Image {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	state.mu.RLock()
	text := state.text
	value := state.color
	fontFamily := state.font
	state.mu.RUnlock()
	if text == "" || value == nil {
		return result
	}

	face, err := newVerticalTextFace(fontFamily, math.Max(8, 13*float64(width)/24))
	if err != nil {
		return result
	}
	defer face.Close()
	metrics := face.Metrics()
	drawer := font.Drawer{Face: face}
	textWidth := drawer.MeasureString(text).Ceil()
	textHeight := (metrics.Ascent + metrics.Descent).Ceil()
	if textWidth < 1 || textHeight < 1 {
		return result
	}

	horizontal := image.NewRGBA(image.Rect(0, 0, textWidth+4, textHeight+4))
	drawer.Dst = horizontal
	drawer.Src = image.NewUniform(value)
	drawer.Dot = fixed.P(2, 2+metrics.Ascent.Ceil())
	drawer.DrawString(text)

	rotated := image.NewRGBA(image.Rect(0, 0, horizontal.Bounds().Dy(), horizontal.Bounds().Dx()))
	for y := 0; y < horizontal.Bounds().Dy(); y++ {
		for x := 0; x < horizontal.Bounds().Dx(); x++ {
			rotated.SetRGBA(y, horizontal.Bounds().Dx()-1-x, horizontal.RGBAAt(x, y))
		}
	}
	offset := image.Pt((width-rotated.Bounds().Dx())/2, (height-rotated.Bounds().Dy())/2)
	draw.Draw(result, rotated.Bounds().Add(offset), rotated, image.Point{}, draw.Over)
	return result
}

func newVerticalTextFace(fontFamily FontFamily, size float64) (font.Face, error) {
	if err := validateFontFamily(fontFamily); err != nil {
		return nil, err
	}
	cache := &verticalFonts[int(fontFamily)]
	cache.once.Do(func() {
		data, err := fontData(fontFamily)
		if err != nil {
			cache.err = err
			return
		}
		cache.font, cache.err = opentype.Parse(data)
	})
	if cache.err != nil {
		return nil, cache.err
	}
	return opentype.NewFace(cache.font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

func colorsEqual(left color.Color, right color.Color) bool {
	if left == nil || right == nil {
		return left == right
	}
	leftR, leftG, leftB, leftA := left.RGBA()
	rightR, rightG, rightB, rightA := right.RGBA()
	return leftR == rightR && leftG == rightG && leftB == rightB && leftA == rightA
}
