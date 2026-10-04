package mapeditor

import (
	"cmp"
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/core"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
	"image/color"
	"math"
	"slices"
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
	pasteArea               *MapArea
	currentZone             string
	lastTileIndex           int
}

func (m *MapEditor) DefaultFillCondition(source geometry.Point, curr geometry.Point, next geometry.Point) bool {
	srcChar, _ := m.gameMap.CharAt(source)
	nextChar, _ := m.gameMap.CharAt(next)
	return srcChar == nextChar
}

func (m *MapEditor) SetDrawMode() {
	m.pasteArea = nil
	m.currentZone = ""
	//m.SetTileDrawing(m.lastTileIndex)
}

func NewMapEditor(
	palette textiles.ColorPalette,
	dimensions geometry.Point,
	app *cview.Application,
	panels *cview.Panels,
	tileResolver func(idx int16) NamedTile,
	items func(cat string) core.NamedColorChar,
	objects func(objectType string) core.NamedColorChar,
) *MapEditor {
	b := cview.NewBox()
	gameMap := NewGameMap(dimensions, tileResolver, items, objects)
	m := &MapEditor{
		Box:     b,
		palette: palette,
		gameMap: gameMap,
		gamma:   1.0,
		overlay: make(map[geometry.Point]rune),
		app:     app,
		panels:  panels,
	}

	b.SetInputCapture(m.handleKeyboard)
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
	pasteMode := m.pasteArea != nil

	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {

			mapPosX := col - x
			mapPosY := row - y

			if pasteMode {
				// check for mouse position (m.currentMouse) and paste area
				pasteSize := m.pasteArea.Dimensions
				pasteStart := m.currentMouse
				pasteEnd := geometry.Point{X: pasteStart.X + pasteSize.X, Y: pasteStart.Y + pasteSize.Y}
				if pasteStart.X <= mapPosX && mapPosX < pasteEnd.X && pasteStart.Y <= mapPosY && mapPosY < pasteEnd.Y {
					pastePos := geometry.Point{X: mapPosX - pasteStart.X, Y: mapPosY - pasteStart.Y}
					ch, style := m.renderMapPosition(m.pasteArea, pastePos, defaultMapStyle)
					screen.SetContent(col, row, ch, nil, style)
					continue
				}

			}

			mapPos := geometry.Point{X: mapPosX, Y: mapPosY}

			ch, style := m.renderMapPosition(m.gameMap, mapPos, defaultMapStyle)

			screen.SetContent(col, row, ch, nil, style)
		}
	}
	// Space for other content.
	return x, y, width, height
}

type MapLike interface {
	CharAt(pos geometry.Point) (core.NamedColorChar, bool)
}

func (m *MapEditor) renderMapPosition(mapArea MapLike, mapPos geometry.Point, style tcell.Style) (rune, tcell.Style) {
	var ch rune
	var textIcon core.NamedColorChar
	foundIcon := false

	if char, exist := m.overlay[mapPos]; exist {
		return char, tcell.StyleDefault
	}

	textIcon, foundIcon = mapArea.CharAt(mapPos)

	var fg, bg color.RGBA

	if foundIcon {
		ch, fg, bg = textIcon.Char, m.palette.Get(textIcon.Fg), m.palette.Get(textIcon.Bg)
	} else {
		ch = ' '
		fg = color.RGBA{255, 255, 255, 255}
		bg = color.RGBA{0, 0, 0, 255}
	}

	if m.currentZone != "" && mapArea == m.gameMap {
		gameMap := mapArea.(*GameMap)
		if gameMap.IsZoneAt(mapPos, m.currentZone) {
			bg = fxtools.NewColorFromRGBA(bg).Multiply(fxtools.HDRColor{R: 0.1, G: 0.9, B: 0.1}).ToRGBA()
		}
	}

	style = style.Foreground(tcell.NewRGBColor(int32(applyGamma(fg.R, m.gamma)), int32(applyGamma(fg.G, m.gamma)), int32(applyGamma(fg.B, m.gamma))))
	style = style.Background(tcell.NewRGBColor(int32(applyGamma(bg.R, m.gamma)), int32(applyGamma(bg.G, m.gamma)), int32(applyGamma(bg.B, m.gamma))))

	return ch, style
}

func (m *MapEditor) handleMouse(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
	if !m.HasFocus() {
		return cview.MouseLeftClick, event
	}
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
	isModified := event != nil && event.Modifiers() != 0
	if action == cview.MouseLeftDown {
		m.isLeftDown = true
		m.onMouseLeftDown(m.currentMouse)
	} else if action == cview.MouseLeftUp {
		m.isLeftDown = false
		m.onMouseLeftUp(m.currentMouse)
	}
	if action == cview.MouseRightClick {
		if isModified {
			if m.currentZone != "" {
				m.gameMap.RemoveZoneAt(m.currentMouse, m.currentZone)
			} else {
				m.removeMapObjectAt(m.currentMouse)
			}
		} else if m.gameMap.IsObjectAt(m.currentMouse) {
			m.gameMap.EditObjectAt(m.app, m.panels, m.currentMouse)
		} else if m.gameMap.IsItemAt(m.currentMouse) {
			m.gameMap.EditItemAt(m.app, m.panels, m.currentMouse)
		} else if m.gameMap.IsActorAt(m.currentMouse) {
			m.gameMap.EditActorAt(m.app, m.panels, m.currentMouse)
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

func (m *MapEditor) PlaceholderFields() []string {
	return m.gameMap.PlaceholderFields()
}

func (m *MapEditor) SaveMap(mapDirectory string) error {
	return m.gameMap.SaveMapToDisk(mapDirectory)
}

func (m *MapEditor) LoadMap(mapDirectory string, resolver func(idx int16) NamedTile, items func(cat string) core.NamedColorChar, objects func(objectType string) core.NamedColorChar) ([]NamedTile, error) {
	m.currentZone = ""
	gameMap, tileset, err := LoadMapFromDisk(mapDirectory, resolver, items, objects)
	if err != nil {
		return nil, err
	}
	m.gameMap = gameMap
	return tileset, nil
}

func (m *MapEditor) GetMapInfoAt(pos geometry.Point) []string {
	return m.gameMap.MapInfoAt(pos)
}

func (m *MapEditor) SetHoverHandler(onHoverChanged func(mapPos geometry.Point)) {
	m.onHoverChangedHandler = onHoverChanged
}

func (m *MapEditor) SetTileDrawing(idx int) {
	m.lastTileIndex = idx
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

func (m *MapEditor) SetItemDrawing(item CategoryObject) {
	m.currentSelectionHandler = func(points []geometry.Point) {
		for _, point := range points {
			m.gameMap.SetItem(point, item)
		}
	}
}

func (m *MapEditor) SetObjectDrawing(object CategoryObject) {
	m.currentSelectionHandler = func(points []geometry.Point) {
		for _, point := range points {
			m.gameMap.SetObject(point, object)
		}
	}
}

func (m *MapEditor) SetSelectionMode() {
	m.currentSelectionHandler = m.selectRect
	m.currentZone = ""
}

func (m *MapEditor) selectRect(selection []geometry.Point) {
	if len(selection) == 0 {
		return
	}
	minX := math.MaxInt
	minY := math.MaxInt
	maxX := 0
	maxY := 0

	for _, pos := range selection {
		if pos.X < minX {
			minX = pos.X
		}
		if pos.Y < minY {
			minY = pos.Y
		}
		if pos.X > maxX {
			maxX = pos.X
		}
		if pos.Y > maxY {
			maxY = pos.Y
		}
	}

	selectionRect := geometry.NewRect(minX, minY, maxX+1, maxY+1)
	m.gameMap.Copy(selectionRect)
}

func (m *MapEditor) removeMapObjectAt(pos geometry.Point) {
	if m.gameMap.IsObjectAt(pos) {
		m.gameMap.RemoveObject(pos)
	} else if m.gameMap.IsItemAt(pos) {
		m.gameMap.RemoveItem(pos)
	} else if m.gameMap.IsActorAt(pos) {
		m.gameMap.RemoveActor(pos)
	}
}

func (m *MapEditor) SetZoneDrawing(zoneName string) {
	m.currentZone = zoneName
	m.currentSelectionHandler = func(points []geometry.Point) {
		for _, point := range points {
			m.gameMap.SetZone(point, zoneName)
		}
	}
}
func (m *MapEditor) SetPasteMode() {
	if !m.gameMap.HasCopyBuffer() {
		return
	}
	m.pasteArea = m.gameMap.GetCopyBuffer()
	m.currentSelectionHandler = m.pasteSelection
}

func (m *MapEditor) PasteTilesAsHorizontalLine(tiles []int16) {
	pasteArea := m.gameMap.GetEmptyCopyBuffer(geometry.Point{X: len(tiles), Y: 1})
	pasteArea.Tiles = tiles
	m.pasteArea = pasteArea
	m.currentSelectionHandler = m.pasteSelection
}
func (m *MapEditor) pasteSelection(points []geometry.Point) {
	if len(points) == 0 || m.pasteArea == nil {
		return
	}
	targetPos := points[0]
	m.gameMap.pasteArea(m.pasteArea, targetPos)
}

func (m *MapEditor) handleKeyboard(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape {
		m.SetDrawMode()
		m.SetTileDrawing(m.lastTileIndex)
		return nil
	}

	if event.Key() == tcell.KeyCtrlC {
		m.SetBrush(fxtools.NewOutlinedRectangleBrush())
		m.SetSelectionMode()
	}

	if event.Key() == tcell.KeyCtrlV {
		m.SetBrush(fxtools.NewPencil())
		m.SetPasteMode()
		return nil
	}

	return event
}

func (m *MapEditor) GetZones() []string {
	var zones []string
	for zone := range m.gameMap.zones {
		zones = append(zones, zone)
	}
	slices.SortStableFunc(zones, func(i, j string) int {
		return cmp.Compare(i, j)
	})
	return zones
}

func (m *MapEditor) RemoveZone(zone string) {
	m.gameMap.RemoveZone(zone)
}

func applyGamma(colorChannel uint8, gamma float64) uint8 {
	colorAsFloat := float64(colorChannel) / 255.0
	gammaCorrected := fxtools.Clamp(0, 1, math.Pow(colorAsFloat, gamma))
	asEightBit := uint8(gammaCorrected * 255.0)
	return asEightBit
}
