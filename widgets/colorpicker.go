package widgets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"image/color"
	"math"
)

type ColorMode bool

const (
	RGB ColorMode = false
	HSL ColorMode = true
)

type ColorPicker struct {
	*cview.Box
	currentColor color.RGBA
	currentMode  ColorMode
	onChange     func(color.RGBA)
}

func (p *ColorPicker) drawPicker(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
	if height < 3 || width < 3 {
		return x, y, width, height
	}
	// draw three rows of colors
	if p.currentMode == RGB {
		// draw the RGB color picker
		p.drawRGBPicker(screen, x, y, width, height)
	} else {
		// draw the HSL color picker
	}
	return x, y, width, height
}

func (p *ColorPicker) drawRGBPicker(screen tcell.Screen, x int, y int, width int, height int) {
	// three lines = RGB, from 0 to 255, scaled to width

	// scale current color to width
	// so we can mark it on the picker
	currentR := int(math.Floor(float64(p.currentColor.R) * float64(width-1) / float64(255)))
	currentG := int(math.Floor(float64(p.currentColor.G) * float64(width-1) / float64(255)))
	currentB := int(math.Floor(float64(p.currentColor.B) * float64(width-1) / float64(255)))

	for i := 0; i < width; i++ {
		r := uint8(i * 255 / width)
		g := uint8(i * 255 / width)
		b := uint8(i * 255 / width)
		drawX := x + i
		redColor := color.RGBA{R: r, G: 0, B: 0, A: 255}
		greenColor := color.RGBA{R: 0, G: g, B: 0, A: 255}
		blueColor := color.RGBA{R: 0, G: 0, B: b, A: 255}

		redFg := tcell.ColorBlack
		greenFg := tcell.ColorWhite
		blueFg := tcell.ColorWhite

		if IsMoreDarkThanLight(redColor) {
			redFg = tcell.ColorWhite
		}
		if IsMoreDarkThanLight(greenColor) {
			greenFg = tcell.ColorWhite
		}
		if IsMoreDarkThanLight(blueColor) {
			blueFg = tcell.ColorWhite
		}

		redMarker := ' '
		greenMarker := ' '
		blueMarker := ' '

		if i == currentR {
			redMarker = '▲'
		}

		if i == currentG {
			greenMarker = '▲'
		}

		if i == currentB {
			blueMarker = '▲'
		}

		screen.SetContent(drawX, y, redMarker, nil, tcell.StyleDefault.Background(toTcellColor(redColor)).Foreground(redFg))
		screen.SetContent(drawX, y+1, greenMarker, nil, tcell.StyleDefault.Background(toTcellColor(greenColor)).Foreground(greenFg))
		screen.SetContent(drawX, y+2, blueMarker, nil, tcell.StyleDefault.Background(toTcellColor(blueColor)).Foreground(blueFg))
	}
}

func (p *ColorPicker) SetColor(color color.RGBA) {
	p.currentColor = color
}

func (p *ColorPicker) SetChangeHandler(handler func(color.RGBA)) {
	p.onChange = handler
}

func (p *ColorPicker) OnColorChange() {
	if p.onChange != nil {
		p.onChange(p.currentColor)
	}
}
func (p *ColorPicker) Draw(screen tcell.Screen) {
	p.Box.Draw(screen)
	x, y, width, height := p.GetInnerRect()
	p.drawPicker(screen, x, y, width, height)
}
func (p *ColorPicker) handleMouse(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
	if action == cview.MouseLeftClick {
		// update current color
		x, y := event.Position()
		startX, startY, width, _ := p.GetInnerRect()

		x -= startX
		y -= startY

		if x < 0 || x >= width || y < 0 || y >= 3 {
			return action, event
		}
		// update current color
		if p.currentMode == RGB {
			// update current color
			// three lines = RGB, from 0 to 255, scaled to width
			value := uint8(math.Ceil(float64(x) / float64(width-1) * float64(255)))
			if y == 0 {
				p.currentColor.R = value
			} else if y == 1 {
				p.currentColor.G = value
			} else if y == 2 {
				p.currentColor.B = value
			}
		} else {
			// update current color (HSL)

		}
		p.OnColorChange()
	}
	return action, event
}

func NewColorPicker() *ColorPicker {
	box := cview.NewBox()
	box.SetBorder(true)
	c := &ColorPicker{
		Box: box,
	}
	box.SetMouseCapture(c.handleMouse)
	return c
}

func toTcellColor(rgba color.RGBA) tcell.Color {
	return tcell.NewRGBColor(int32(rgba.R), int32(rgba.G), int32(rgba.B))
}

func IsMoreDarkThanLight(color color.RGBA) bool {
	return int(color.R)+int(color.G)+int(color.B) < 384
}

func IsColorMoreDarkThanLight(color tcell.Color) bool {
	r, g, b := color.RGB()
	return int(r)+int(g)+int(b) < 384
}
