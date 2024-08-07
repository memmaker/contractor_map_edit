package widgets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
)

type LabelItem struct {
	Label             string
	Action            func()
	HighlightOnSelect bool
}

type DropMenu struct {
	Label string
	Items []LabelItem
}

func (m DropMenu) DropDownLabels() []string {
	var labels []string
	for _, item := range m.Items {
		labels = append(labels, item.Label)
	}
	return labels
}

type MenuElement interface {
	SetLabelColor(fg tcell.Color)
	SetBackgroundColor(bg tcell.Color)
	Draw(screen tcell.Screen)
	SetRect(x int, y int, width int, height int)
	InRect(x int, y int) bool
	MouseHandler() func(action cview.MouseAction, event *tcell.EventMouse, setFocus func(p cview.Primitive)) (consumed bool, capture cview.Primitive)
}

type ButtonBar struct {
	*cview.Box
	buttons      []MenuElement
	highlight    int
	isHorizontal bool
	fg           tcell.Color
	bg           tcell.Color
}

func NewButtonBar(labelItems []LabelItem) *ButtonBar {
	box := cview.NewBox()
	bb := &ButtonBar{
		Box:          box,
		isHorizontal: true,
		fg:           tcell.ColorWhite,
		bg:           tcell.ColorBlack,
		highlight:    -1,
	}
	bb.SetButtonsFromLabelItems(labelItems)
	box.SetMouseCapture(bb.handleMouse)
	return bb
}
func NewMenuBar(menuItems []DropMenu) *ButtonBar {
	box := cview.NewBox()
	bb := &ButtonBar{
		Box:          box,
		isHorizontal: true,
		fg:           tcell.ColorWhite,
		bg:           tcell.ColorBlack,
	}
	bb.SetMenuFromDropMenuItems(menuItems)
	box.SetMouseCapture(bb.handleMouse)
	return bb
}

func (bb *ButtonBar) SetHighlight(index int) {
	bb.highlight = index
}
func (bb *ButtonBar) SetColors(fg, bg tcell.Color) {
	bb.fg = fg
	bb.bg = bg
}

func (bb *ButtonBar) SetRect(x int, y int, width int, height int) {
	bb.Box.SetRect(x, y, width, height)
	if bb.isHorizontal {
		bb.layoutButtonsHorizontally()
	} else {
		bb.layoutButtonsVertically()
	}
}
func (bb *ButtonBar) Draw(screen tcell.Screen) {
	bb.Box.Draw(screen)
	for i, button := range bb.buttons {
		if i == bb.highlight {
			button.SetLabelColor(bb.bg)
			button.SetBackgroundColor(bb.fg)
		} else {
			button.SetLabelColor(bb.fg)
			button.SetBackgroundColor(bb.bg)
		}
		button.Draw(screen)
	}
}

func (bb *ButtonBar) layoutButtonsHorizontally() {
	x, y, width, height := bb.GetInnerRect()
	for i, button := range bb.buttons {
		buttonWidth := width / len(bb.buttons)
		buttonHeight := height
		buttonX := x + (i * buttonWidth)
		buttonY := y
		button.SetRect(buttonX, buttonY, buttonWidth, buttonHeight)
	}
}

func (bb *ButtonBar) layoutButtonsVertically() {
	x, y, width, height := bb.GetInnerRect()
	for i, button := range bb.buttons {
		buttonWidth := width
		buttonHeight := height / len(bb.buttons)
		buttonX := x
		buttonY := y + (i * buttonHeight)
		button.SetRect(buttonX, buttonY, buttonWidth, buttonHeight)
	}
}

func (bb *ButtonBar) handleMouse(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
	for _, button := range bb.buttons {
		if button.InRect(event.Position()) {
			button.MouseHandler()(action, event, func(p cview.Primitive) {})
		}
	}
	return action, event
}

func (bb *ButtonBar) SetButtonsFromLabelItems(items []LabelItem) {
	buttons := make([]MenuElement, len(items))
	for i, item := range items {
		button := cview.NewButton(item.Label)
		if item.HighlightOnSelect {
			button.SetSelectedFunc(func() {
				item.Action()
				bb.SetHighlight(i)
			})
		} else {
			button.SetSelectedFunc(item.Action)
		}
		buttons[i] = button
	}
	bb.buttons = buttons
}

func (bb *ButtonBar) SetMenuFromDropMenuItems(items []DropMenu) {
	buttons := make([]MenuElement, len(items))
	for i, item := range items {
		button := cview.NewDropDown()
		button.SetLabel(item.Label)
		button.SetOptionsSimple(func(index int, option *cview.DropDownOption) {
			if index >= len(item.Items) {
				return
			}
			item.Items[index].Action()
		}, item.DropDownLabels()...)
		buttons[i] = button
	}
	bb.buttons = buttons
}
