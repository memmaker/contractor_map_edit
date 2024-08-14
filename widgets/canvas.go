package widgets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/core"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
)

type SelectionHandler func(selection []geometry.Point)

type Canvas struct {
	*cview.Box

	// backing storage
	canvasSize   geometry.Point
	screenBuffer []core.NamedColorChar

	// color palette
	palette textiles.ColorPalette

	// sparse overlay
	overlay map[geometry.Point]rune

	// drawing state
	currentChar             core.NamedColorChar
	currentBrush            fxtools.Brush
	currentSelectionHandler SelectionHandler
	lastDrawPos             geometry.Point
	lastDrawStart           geometry.Point
	lastDrawStop            geometry.Point

	// mouse
	currentMouse geometry.Point
	isLeftDown   bool

	// text input
	textInputMode bool
	currentCursor geometry.Point
}

func NewCanvas() *Canvas {
	b := cview.NewBox()
	c := &Canvas{
		Box:     b,
		overlay: make(map[geometry.Point]rune),
	}
	b.SetMouseCapture(c.handleMouse)
	b.SetInputCapture(c.handleKeyboard)
	return c
}

func (c *Canvas) SetBrush(brush fxtools.Brush) {
	c.currentBrush = brush
}

func (c *Canvas) SetChar(char core.NamedColorChar) {
	c.currentChar = char
}

func (c *Canvas) SetDrawMode() {
	c.currentSelectionHandler = c.drawRunes()
}

func (c *Canvas) Draw(screen tcell.Screen) {
	c.Box.Draw(screen)
	x, y, width, height := c.GetInnerRect()
	// draw the canvas
	c.drawCanvas(screen, x, y, width, height)
}

func (c *Canvas) drawCanvas(screen tcell.Screen, x int, y int, width int, height int) {
	if width < c.canvasSize.X || height < c.canvasSize.Y {
		cview.Print(screen, []byte("Canvas doesn't fit"), x, y, width, cview.AlignLeft, tcell.ColorWhite)
		return
	}

	index := 0
	for drawY := 0; drawY < c.canvasSize.Y; drawY++ {
		for drawX := 0; drawX < c.canvasSize.X; drawX++ {
			var drawIcon rune
			drawStyle := tcell.StyleDefault
			if overlayRune, exists := c.overlay[geometry.Point{X: drawX, Y: drawY}]; exists {
				drawIcon = overlayRune
			} else {
				coloredChar := c.screenBuffer[index]
				drawIcon = coloredChar.Char
				drawStyle = c.styleFromChar(coloredChar)
			}
			screen.SetContent(drawX+x, drawY+y, drawIcon, nil, drawStyle)
			index++
		}
	}

	if c.textInputMode && c.HasFocus() {
		screen.SetCursorStyle(tcell.CursorStyleBlinkingUnderline)
		screen.ShowCursor(c.currentCursor.X, c.currentCursor.Y)
	} else {
		screen.HideCursor()
	}
}

func (c *Canvas) styleFromChar(char core.NamedColorChar) tcell.Style {
	if char.IsEmpty() {
		return tcell.StyleDefault
	}
	fgIndex := char.Fg
	bgIndex := char.Bg
	if !c.palette.Has(fgIndex) {
		fgIndex = "white"
	}
	if !c.palette.Has(bgIndex) {
		bgIndex = "black"
	}
	return tcell.StyleDefault.Foreground(toTcellColor(c.palette.Get(fgIndex))).Background(toTcellColor(c.palette.Get(bgIndex)))
}
func (c *Canvas) handleMouse(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
	if event != nil {
		newX, newY := event.Position()
		xOff, yOff, _, _ := c.GetInnerRect()
		newX -= xOff
		newY -= yOff
		if newX != c.currentMouse.X || newY != c.currentMouse.Y {
			c.currentMouse = geometry.Point{X: newX, Y: newY}
			c.onMouseMoved(c.currentMouse)
		}
	}
	if action == cview.MouseLeftDown {
		c.isLeftDown = true
		c.onMouseLeftDown(c.currentMouse)
	}
	if action == cview.MouseLeftUp {
		c.isLeftDown = false
		c.onMouseLeftUp(c.currentMouse)
	}
	return cview.MouseLeftClick, event
}

func (c *Canvas) onMouseMoved(mouse geometry.Point) {
	if c.isLeftDown {
		selectedPositions := c.currentBrush.DraggedOver(mouse)
		c.overlayRunes(c.currentChar.Char, selectedPositions)
	}
}

func (c *Canvas) onMouseLeftDown(mouse geometry.Point) {
	c.currentBrush.StartDrawing(mouse)
	c.lastDrawStart = mouse
}

func (c *Canvas) onMouseLeftUp(mouse geometry.Point) {
	selectedPositions := c.currentBrush.StopDrawing(mouse)
	c.lastDrawStop = mouse
	clear(c.overlay)
	c.currentSelectionHandler(selectedPositions)
}

func (c *Canvas) SetCanvasSize(size geometry.Point) {
	if len(c.screenBuffer) != size.X*size.Y {
		c.resizeBuffer(size)
	}
}

func (c *Canvas) resizeBuffer(size geometry.Point) {
	newBuffer := make([]core.NamedColorChar, size.X*size.Y)
	if len(c.screenBuffer) > 0 {
		for y := 0; y < size.Y; y++ {
			for x := 0; x < size.X; x++ {
				if x < c.canvasSize.X && y < c.canvasSize.Y {
					newBuffer[y*size.X+x] = c.screenBuffer[y*c.canvasSize.X+x]
				}
			}
		}
	}
	c.screenBuffer = newBuffer
	c.canvasSize = size
}
func (c *Canvas) Contains(point geometry.Point) bool {
	return point.X >= 0 && point.Y >= 0 && point.X < c.canvasSize.X && point.Y < c.canvasSize.Y
}
func (c *Canvas) DefaultFillCondition(source, current, neighbor geometry.Point) bool {
	if !c.Contains(neighbor) || !c.Contains(source) || !c.Contains(current) {
		return false
	}

	sourceIndex := XYToIndex(source.X, source.Y, c.canvasSize.X)
	destIndex := XYToIndex(neighbor.X, neighbor.Y, c.canvasSize.X)

	if destIndex < 0 || destIndex >= len(c.screenBuffer) {
		return false
	}
	if sourceIndex < 0 || sourceIndex >= len(c.screenBuffer) {
		return false
	}

	sourceRune := c.screenBuffer[sourceIndex]
	destRune := c.screenBuffer[destIndex]

	return destRune == sourceRune
}

func (c *Canvas) drawRunes() func(selection []geometry.Point) {
	return func(selection []geometry.Point) {
		for _, pos := range selection {
			c.drawRune(pos, c.currentChar)
		}
	}
}
func (c *Canvas) drawRune(pos geometry.Point, icon core.NamedColorChar) {
	screenBufferIndex := pos.Y*c.canvasSize.X + pos.X
	if screenBufferIndex < 0 || screenBufferIndex >= len(c.screenBuffer) {
		return
	}
	c.screenBuffer[screenBufferIndex] = icon
	c.lastDrawPos = pos
}

func (c *Canvas) overlayRunes(i rune, selection []geometry.Point) {
	//c.application.
	clear(c.overlay)
	for _, pos := range selection {
		c.overlay[pos] = i
	}
}

func (c *Canvas) startTextInput(selection []geometry.Point) {
	if len(selection) > 0 {
		c.textInputMode = true
		c.currentCursor = selection[0]
		c.currentSelectionHandler = c.drawRunes()
	}
}

func XYToIndex(x int, y int, width int) int {
	return y*width + x
}
func (c *Canvas) handleTextInput(r rune) {
	c.screenBuffer[XYToIndex(c.currentCursor.X, c.currentCursor.Y, c.canvasSize.X)] = core.NamedColorChar{
		Char: r,
		Fg:   c.currentChar.Fg,
		Bg:   c.currentChar.Bg,
	}
	if c.currentCursor.X < c.canvasSize.X-1 {
		c.currentCursor.X++
	}
}

func (c *Canvas) SetCharRune(charRune rune) {
	c.currentChar.Char = charRune
}

func (c *Canvas) SetSelectionHandler(handler SelectionHandler) {
	c.currentSelectionHandler = handler
}

func (c *Canvas) GetCharAt(samplePoint geometry.Point) core.NamedColorChar {
	return c.screenBuffer[XYToIndex(samplePoint.X, samplePoint.Y, c.canvasSize.X)]
}

func (c *Canvas) SetScreenBuffer(buffer []core.NamedColorChar, canvasSize geometry.Point) {
	c.screenBuffer = buffer
	c.canvasSize = canvasSize
}

func (c *Canvas) handleKeyboard(event *tcell.EventKey) *tcell.EventKey {

	if !c.textInputMode && event.Key() == tcell.KeyEnter {
		c.currentSelectionHandler = c.startTextInput
	}

	if c.textInputMode && (event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter) {
		c.textInputMode = false
	}

	if (event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyDEL) && c.textInputMode {
		if c.currentCursor.X > 0 {
			c.drawRune(c.currentCursor, core.NamedColorChar{Char: ' ',
				Fg: c.currentChar.Fg,
				Bg: c.currentChar.Bg,
			})
			c.currentCursor.X--
		}
	}
	if event.Key() == tcell.KeyRune {
		if c.textInputMode {
			c.handleTextInput(event.Rune())
		} else if c.currentChar.Char != event.Rune() {
			c.currentChar.Char = event.Rune()
			c.currentSelectionHandler = c.drawRunes()
		}
	}
	return event
}

func (c *Canvas) SetPalette(palette textiles.ColorPalette) {
	c.palette = palette
}

func (c *Canvas) SetCharFg(name string) {
	c.currentChar.Fg = name
}

func (c *Canvas) SetCharBg(name string) {
	c.currentChar.Bg = name
}
