package mapeditor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/core"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
	"image/color"
	"math"
)

type MapEditor struct {
	*cview.Box
	gamma         float64
	currentBrush  fxtools.Brush
	palette       textiles.ColorPalette
	gameMap       *GameMap
	currentMouse  geometry.Point
	isLeftDown    bool
	lastDrawStart geometry.Point
	lastDrawStop  geometry.Point
	overlay       map[geometry.Point]rune

	currentSelectionHandler func([]geometry.Point)
	onHoverChangedHandler   func(mapPos geometry.Point)
	app                     *cview.Application
	panels                  *cview.Panels
}

func (m *MapEditor) DefaultFillCondition(source geometry.Point, curr geometry.Point, next geometry.Point) bool {
	srcChar, _ := m.gameMap.CharAt(source)
	nextChar, _ := m.gameMap.CharAt(next)
	return srcChar == nextChar
}

func (m *MapEditor) SetDrawMode() {

}

func NewMapEditor(
	palette textiles.ColorPalette,
	dimensions geometry.Point,
	tileResolver func(idx int16) NamedTile,
	app *cview.Application,
	panels *cview.Panels,
) *MapEditor {
	b := cview.NewBox()
	gameMap := NewGameMap(dimensions, tileResolver)
	m := &MapEditor{
		Box:     b,
		palette: palette,
		gameMap: gameMap,
		gamma:   1.0,
		overlay: make(map[geometry.Point]rune),
		app:     app,
		panels:  panels,
	}
	b.SetMouseCapture(m.handleMouse)
	return m
}

func (m *MapEditor) SetBrush(brush fxtools.Brush) {
	m.currentBrush = brush
}

func (m *MapEditor) SetSelectionHandler(handler func([]geometry.Point)) {
	m.currentSelectionHandler = handler
}

func (m *MapEditor) Draw(screen tcell.Screen) {
	m.Box.Draw(screen)
	x, y, width, height := m.GetInnerRect()
	m.drawMap(screen, x, y, width, height)
}

func (m *MapEditor) drawMap(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
	defaultMapStyle := tcell.StyleDefault
	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {

			mapPosX := col - x
			mapPosY := row - y

			mapPos := geometry.Point{X: mapPosX, Y: mapPosY}

			ch, style := m.renderMapPosition(mapPos, defaultMapStyle)

			screen.SetContent(col, row, ch, nil, style)
		}
	}
	// Space for other content.
	return x, y, width, height
}
func (m *MapEditor) renderMapPosition(mapPos geometry.Point, style tcell.Style) (rune, tcell.Style) {
	var ch rune
	var textIcon core.NamedColorChar
	foundIcon := false

	if char, exist := m.overlay[mapPos]; exist {
		return char, tcell.StyleDefault
	}

	textIcon, foundIcon = m.gameMap.CharAt(mapPos)

	var fg, bg color.RGBA

	if foundIcon {
		ch, fg, bg = textIcon.Char, m.palette.Get(textIcon.Fg), m.palette.Get(textIcon.Bg)
	} else {
		ch = ' '
		fg = color.RGBA{255, 255, 255, 255}
		bg = color.RGBA{0, 0, 0, 255}
	}

	style = style.Foreground(tcell.NewRGBColor(int32(applyGamma(fg.R, m.gamma)), int32(applyGamma(fg.G, m.gamma)), int32(applyGamma(fg.B, m.gamma))))
	style = style.Background(tcell.NewRGBColor(int32(applyGamma(bg.R, m.gamma)), int32(applyGamma(bg.G, m.gamma)), int32(applyGamma(bg.B, m.gamma))))

	return ch, style
}

func (m *MapEditor) handleMouse(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
	if event != nil {
		newX, newY := event.Position()
		xOff, yOff, _, _ := m.GetInnerRect()
		newX -= xOff
		newY -= yOff
		if newX != m.currentMouse.X || newY != m.currentMouse.Y {
			m.currentMouse = geometry.Point{X: newX, Y: newY}
			if !m.isLeftDown && action == cview.MouseMove && m.onHoverChangedHandler != nil {
				m.onHoverChangedHandler(m.currentMouse)
			} else {
				m.onMouseMoved(m.currentMouse)
			}
		}
	}
	isModified := event.Modifiers() != 0
	if action == cview.MouseLeftDown {
		m.isLeftDown = true
		m.onMouseLeftDown(m.currentMouse)
	} else if action == cview.MouseLeftUp {
		m.isLeftDown = false
		m.onMouseLeftUp(m.currentMouse)
	}
	if action == cview.MouseRightClick {
		if isModified {
			m.removeMapObjectAt(m.currentMouse)
		} else if m.gameMap.IsActorAt(m.currentMouse) {
			m.gameMap.EditActorAt(m.app, m.panels, m.currentMouse)
		} else if m.gameMap.IsItemAt(m.currentMouse) {
			m.gameMap.EditItemAt(m.app, m.panels, m.currentMouse)
		} else if m.gameMap.IsObjectAt(m.currentMouse) {
			m.gameMap.EditObjectAt(m.app, m.panels, m.currentMouse)
		}
	}

	return cview.MouseLeftClick, event
}

func (m *MapEditor) overlayRunes(i rune, selection []geometry.Point) {
	clear(m.overlay)
	for _, pos := range selection {
		m.overlay[pos] = i
	}
}
func (m *MapEditor) onMouseMoved(mouse geometry.Point) {
	if m.isLeftDown {
		selectedPositions := m.currentBrush.DraggedOver(mouse)
		m.overlayRunes('*', selectedPositions)
	}
}

func (m *MapEditor) onMouseLeftDown(mouse geometry.Point) {
	m.currentBrush.StartDrawing(mouse)
	m.lastDrawStart = mouse
}

func (m *MapEditor) onMouseLeftUp(mouse geometry.Point) {
	selectedPositions := m.currentBrush.StopDrawing(mouse)
	m.lastDrawStop = mouse
	clear(m.overlay)
	if m.currentSelectionHandler != nil {
		m.currentSelectionHandler(selectedPositions)
	}
}

func (m *MapEditor) SetTile(point geometry.Point, t int16) {
	m.gameMap.SetTile(point, t)
}

func (m *MapEditor) SetActor(point geometry.Point, actor textiles.IconRecord) {
	m.gameMap.SetActor(point, actor)
}

func (m *MapEditor) SetItem(point geometry.Point, item textiles.IconRecord) {
	m.gameMap.SetItem(point, item)
}

func (m *MapEditor) SetObject(point geometry.Point, object textiles.IconRecord) {
	m.gameMap.SetObject(point, object)
}

func (m *MapEditor) SaveMap(mapDirectory string) error {
	return m.gameMap.SaveMapToDisk(mapDirectory)
}

func (m *MapEditor) LoadMap(mapDirectory string, resolver func(idx int16) NamedTile) []NamedTile {
	gameMap, tileset := LoadMapFromDisk(mapDirectory, resolver)
	if gameMap == nil {
		return nil
	}
	m.gameMap = gameMap
	return tileset
}

func (m *MapEditor) GetMapInfoAt(pos geometry.Point) []string {
	return m.gameMap.MapInfoAt(pos)
}

func (m *MapEditor) SetHoverHandler(onHoverChanged func(mapPos geometry.Point)) {
	m.onHoverChangedHandler = onHoverChanged
}

func (m *MapEditor) SetTileDrawing(idx int) {
	m.currentSelectionHandler = func(points []geometry.Point) {
		for _, point := range points {
			m.gameMap.SetTile(point, int16(idx))
		}
	}
}

func (m *MapEditor) SetActorDrawing(actor textiles.IconRecord) {
	m.currentSelectionHandler = func(points []geometry.Point) {
		for _, point := range points {
			m.gameMap.SetActor(point, actor)
		}
	}
}

func (m *MapEditor) SetItemDrawing(item textiles.IconRecord) {
	m.currentSelectionHandler = func(points []geometry.Point) {
		for _, point := range points {
			m.gameMap.SetItem(point, item)
		}
	}
}

func (m *MapEditor) SetObjectDrawing(object textiles.IconRecord) {
	m.currentSelectionHandler = func(points []geometry.Point) {
		for _, point := range points {
			m.gameMap.SetObject(point, object)
		}
	}
}

func (m *MapEditor) removeMapObjectAt(pos geometry.Point) {
	if m.gameMap.IsActorAt(pos) {
		m.gameMap.RemoveActor(pos)
	} else if m.gameMap.IsItemAt(pos) {
		m.gameMap.RemoveItem(pos)
	} else if m.gameMap.IsObjectAt(pos) {
		m.gameMap.RemoveObject(pos)
	}
}

func applyGamma(colorChannel uint8, gamma float64) uint8 {
	colorAsFloat := float64(colorChannel) / 255.0
	gammaCorrected := fxtools.Clamp(0, 1, math.Pow(colorAsFloat, gamma))
	asEightBit := uint8(gammaCorrected * 255.0)
	return asEightBit
}
