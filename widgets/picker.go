package widgets

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/textiles"
	"math"
)

type PickerItem struct {
	Name      string
	Icon      rune
	IconStyle tcell.Style
}

func (i PickerItem) HasIcon() bool {
	return i.Icon != 0
}

type Picker struct {
	*cview.Box
	listMode bool
	items    []PickerItem

	primarySelected   int
	secondarySelected int

	primarySelectIndicator   rune
	secondarySelectIndicator rune

	indicateWithColors   bool
	primarySelectColor   tcell.Color
	secondarySelectColor tcell.Color

	onModifiedClick            func(isRight bool, clickedIndex int, clickedItem PickerItem)
	onSelectionChange          func(primaryIndex int, secondaryIndex int, primarySelected PickerItem, secondarySelected PickerItem)
	primaryIndicatorDisabled   bool
	secondaryIndicatorDisabled bool
	onHoverHandler             func(index int, item PickerItem)
	lastHoverIndex             int

	currentPage int
	isModal     bool
}

func OpenModalPicker(app *cview.Application, panels *cview.Panels, elements []PickerItem, onSelected func(selectedIndex int)) *Picker {
	modal := NewPicker()
	modal.SetModal(true)
	modal.SetBorder(true)
	w, h := 18, 18
	screenW, screenH := app.GetScreen().Size()
	x, y := (screenW-w)/2, (screenH-h)/2
	modal.SetRect(x, y, w, h)

	closePicker := func() {
		panels.RemovePanel("modal")
		_, frontPanel := panels.GetFrontPanel()
		app.SetBeforeFocusFunc(nil)
		app.SetFocus(frontPanel)
	}
	modal.SetItems(elements)
	modal.SetChangeHandler(func(primaryIndex int, secondaryIndex int, primarySelected PickerItem, secondarySelected PickerItem) {
		if primaryIndex >= 0 && primaryIndex < len(elements) {
			closePicker()
			onSelected(primaryIndex)
		}
	})
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closePicker()
		}
		return event
	})
	panels.AddPanel("modal", modal, false, true)
	app.SetFocus(modal)
	app.SetBeforeFocusFunc(func(p cview.Primitive) bool { return false })
	return modal
}

func (p *Picker) needsPaging() bool {
	if !p.listMode {
		return false
	}
	return len(p.items) > p.itemsPerPage()
}

func (p *Picker) pageCount() int {
	if p.itemsPerPage() == 0 {
		return 1
	}
	return int(math.Ceil(float64(len(p.items)) / float64(p.itemsPerPage())))
}

func (p *Picker) itemsPerPage() int {
	_, _, _, height := p.GetInnerRect()
	return height
}

func (p *Picker) firstItemIndexForPage(pageNumber int) int {
	return pageNumber * p.itemsPerPage()
}

func (p *Picker) NextPage() {
	if p.currentPage < p.pageCount()-1 {
		p.currentPage++
	} else {
		p.currentPage = 0
	}
}

func (p *Picker) PreviousPage() {
	if p.currentPage > 0 {
		p.currentPage--
	} else {
		p.currentPage = p.pageCount() - 1
	}
}

func (p *Picker) SetModClickHandler(handler func(isRight bool, clickedIndex int, clickedItem PickerItem)) {
	p.onModifiedClick = handler
}
func (p *Picker) SetOnHover(handler func(index int, item PickerItem)) {
	p.onHoverHandler = handler
}
func (p *Picker) Draw(screen tcell.Screen) {
	p.Box.Draw(screen)
	x, y, width, height := p.GetInnerRect()
	if p.listMode {
		p.drawList(screen, x, y, width, height)
	} else {
		p.drawGrid(screen, x, y, width, height)
	}
}

func NewPalettePicker() *Picker {
	p := NewPicker()
	p.SetIndicators('f', 'b')
	return p
}

func NewCharPicker() *Picker {
	p := NewPicker()
	p.SetCodepage437()
	p.SetIndicateWithColors(true)
	return p
}

func NewPicker() *Picker {
	box := cview.NewBox()
	p := &Picker{
		Box:                  box,
		listMode:             false,
		primarySelected:      -1,
		secondarySelected:    -1,
		primarySelectColor:   tcell.ColorDarkGoldenrod,
		secondarySelectColor: tcell.ColorRed,
	}
	box.SetMouseCapture(p.handleMouse)

	return p
}
func (p *Picker) SetItems(items []PickerItem) *Picker {
	p.items = items
	return p
}

func (p *Picker) drawList(screen tcell.Screen, x int, y int, width int, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	// OLD: for i, item := range p.items {
	for i := p.firstItemIndexForPage(p.currentPage); i < p.firstItemIndexForPage(p.currentPage+1); i++ {
		if i >= len(p.items) || i < 0 {
			break
		}
		item := p.items[i]
		drawStyle := tcell.StyleDefault

		if i == p.primarySelected && !p.primaryIndicatorDisabled {
			if p.indicateWithColors {
				drawStyle = drawStyle.Background(p.primarySelectColor).Foreground(tcell.ColorWhite)
			}
		}
		if i == p.secondarySelected && !p.secondaryIndicatorDisabled {
			if p.indicateWithColors {
				drawStyle = drawStyle.Background(p.secondarySelectColor).Foreground(tcell.ColorWhite)
			}
		}
		drawX := x
		drawY := y + p.absoluteIndexToPageIndex(i)

		screen.SetContent(drawX, drawY, item.Icon, nil, item.IconStyle)
		cview.PrintStyle(screen, []byte(item.Name), drawX+2, drawY, width-2, cview.AlignLeft, drawStyle)
	}
	// draw x/y at the bottom
	if p.needsPaging() {
		pageText := fmt.Sprintf("P. %d/%d", p.currentPage+1, p.pageCount())
		cview.PrintStyle(screen, []byte(pageText), x, y+height, width, cview.AlignCenter, tcell.StyleDefault)
	}
}

func (p *Picker) drawGrid(screen tcell.Screen, x int, y int, width int, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	for i, item := range p.items {
		drawRune := item.Icon
		drawStyle := item.IconStyle
		_, bg, _ := drawStyle.Decompose()
		if i == p.primarySelected && !p.primaryIndicatorDisabled {
			if p.indicateWithColors {
				drawStyle = drawStyle.Background(p.primarySelectColor)
			} else {
				drawRune = p.primarySelectIndicator
				if IsColorMoreDarkThanLight(bg) {
					drawStyle = drawStyle.Foreground(tcell.ColorWhite)
				} else {
					drawStyle = drawStyle.Foreground(tcell.ColorBlack)
				}
			}
		}
		if i == p.secondarySelected && !p.secondaryIndicatorDisabled {
			if p.indicateWithColors {
				drawStyle = drawStyle.Background(p.secondarySelectColor)
			} else {
				drawRune = p.secondarySelectIndicator
				if IsColorMoreDarkThanLight(bg) {
					drawStyle = drawStyle.Foreground(tcell.ColorWhite)
				} else {
					drawStyle = drawStyle.Foreground(tcell.ColorBlack)
				}
			}
		}
		drawX := x + i%width
		drawY := y + i/width
		if drawX >= x && drawX < x+width && drawY >= y && drawY < y+height {
			screen.SetContent(drawX, drawY, drawRune, nil, drawStyle)
		}
	}
}

func (p *Picker) SetPalette(palette textiles.ColorPalette) {
	items := PickerItemsFromPalette(palette)
	p.SetItems(items)
}

func PickerItemsFromPalette(palette textiles.ColorPalette) []PickerItem {
	items := make([]PickerItem, palette.Count())
	for i, color := range palette.AsNamedColors() {
		items[i] = PickerItem{
			Name:      color.Name,
			Icon:      ' ',
			IconStyle: tcell.StyleDefault.Background(toTcellColor(color.Color)),
		}
	}
	return items
}

func (p *Picker) SetCodepage437() {
	items := make([]PickerItem, 256)
	for i := 0; i < 256; i++ {
		items[i] = PickerItem{
			Name:      string(rune(i)),
			Icon:      fxtools.CP437ToRune(byte(i)),
			IconStyle: tcell.StyleDefault.Foreground(tcell.ColorWhite),
		}
	}
	p.SetItems(items)
}

func (p *Picker) SetIndicators(primary rune, secondary rune) {
	p.primarySelectIndicator = primary
	p.secondarySelectIndicator = secondary
}

func (p *Picker) SetIndicateWithColors(value bool) {
	p.indicateWithColors = value
}

func (p *Picker) handleMouse(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
	x, y := event.Position()
	xOff, yOff, width, height := p.GetInnerRect()

	isLeftClick := action == cview.MouseLeftClick
	isRightClick := action == cview.MouseRightClick

	if isLeftClick && p.needsPaging() {
		isOnBorder := x == xOff-1 || x == xOff+width || y == yOff-1 || y == yOff+height
		if isOnBorder {
			centerOfBounds := xOff + width/2
			if x < centerOfBounds {
				p.PreviousPage()
				return cview.MouseLeftClick, event
			} else {
				p.NextPage()
				return cview.MouseLeftClick, event
			}
		}
	}

	if x < xOff || x >= xOff+width || y < yOff || y >= yOff+height {
		return action, event
	}
	x -= xOff
	y -= yOff

	isModClick := event.Modifiers() != 0 && (isLeftClick || isRightClick)

	hoverIndex := y

	if p.needsPaging() {
		hoverIndex = p.relativePageIndexToAbsoluteIndex(hoverIndex)
	}

	if !p.listMode {
		hoverIndex = y*width + x
	}

	if hoverIndex < 0 || hoverIndex >= len(p.items) {
		return action, event
	}

	if isModClick && p.onModifiedClick != nil {
		p.onModifiedClick(isRightClick, hoverIndex, p.byIndex(hoverIndex))
		return cview.MouseLeftClick, event
	}

	if isLeftClick {
		p.primarySelected = hoverIndex
		p.selectionChanged(hoverIndex, -1)
		if p.isModal {
			return 0, nil
		}
		return cview.MouseLeftClick, event
	} else if isRightClick {
		p.secondarySelected = hoverIndex
		p.selectionChanged(-1, hoverIndex)
		return cview.MouseLeftClick, event
	}

	if hoverIndex != p.lastHoverIndex {
		p.onHover(hoverIndex, p.byIndex(hoverIndex))
		p.lastHoverIndex = hoverIndex
	}

	return action, event
}

func (p *Picker) selectionChanged(primary, secondary int) {
	if p.onSelectionChange != nil {
		p.onSelectionChange(primary, secondary, p.byIndex(primary), p.byIndex(secondary))
	}
}

func (p *Picker) byIndex(index int) PickerItem {
	if index < 0 || index >= len(p.items) {
		return PickerItem{}
	}
	return p.items[index]
}

func (p *Picker) SetChangeHandler(handler func(primaryIndex int, secondaryIndex int, primarySelected PickerItem, secondarySelected PickerItem)) {
	p.onSelectionChange = handler
}

func (p *Picker) SetPrimarySelectedIndex(index int) {
	p.primarySelected = index
}

func (p *Picker) SetSecondarySelectedIndex(index int) {
	p.secondarySelected = index
}

func (p *Picker) SetNoIndicators() {
	p.primaryIndicatorDisabled = true
	p.secondaryIndicatorDisabled = true
}

func (p *Picker) SetListMode() {
	p.listMode = true
}

func (p *Picker) SetNoSecondaryIndicator() {
	p.secondaryIndicatorDisabled = true
}

func (p *Picker) onHover(index int, item PickerItem) {
	if p.onHoverHandler != nil {
		p.onHoverHandler(index, item)
	}
}

func (p *Picker) absoluteIndexToPageIndex(i int) int {
	return i - p.firstItemIndexForPage(p.currentPage)
}

func (p *Picker) relativePageIndexToAbsoluteIndex(index int) int {
	return index + p.firstItemIndexForPage(p.currentPage)
}

func (p *Picker) SetModal(value bool) {
	p.isModal = value
}
