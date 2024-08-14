package widgets

import "github.com/memmaker/go/cview"

type MenuItem struct {
	Label      string
	Action     func()
	CloseMenus bool
}

func OpenModalMenu(app *cview.Application, panels *cview.Panels, menuItems []MenuItem) *cview.List {
	list := cview.NewList()
	closeModal := func() {
		panels.RemovePanel("modal")
		_, frontPanel := panels.GetFrontPanel()
		// Reset focus changes
		app.SetBeforeFocusFunc(nil)
		app.SetFocus(frontPanel)
	}
	list.SetSelectedFunc(func(index int, listItem *cview.ListItem) {
		action := menuItems[index]
		if action.CloseMenus {
			closeModal()
		}
		action.Action()
	})

	setListItemsFromMenuItems(list, menuItems)

	panels.AddPanel("modal", list, false, true)
	app.SetFocus(list)

	// deny any focus change
	app.SetBeforeFocusFunc(func(p cview.Primitive) bool {
		if p == list {
			return true
		}
		x, y, w, h := p.GetRect()
		if list.InRect(x, y) && list.InRect(x+w, y+h) {
			return true
		}
		return false
	})
	return list
}

func setListItemsFromMenuItems(list *cview.List, menuItems []MenuItem) int {
	list.Clear()
	longestItem := 0
	for index, a := range menuItems {
		action := a
		shortcut := ShortCutFromIndex(index)
		listItem := cview.NewListItem(action.Label)
		listItem.SetShortcut(shortcut)
		list.AddItem(listItem)
		itemLength := len(action.Label) + 4
		longestItem = max(longestItem, itemLength)
	}
	return longestItem
}
func ShortCutFromIndex(itemIndex int) rune {
	shortcut := rune(97 + itemIndex)
	if itemIndex > 25 {
		shortcut = rune(39 + itemIndex)
	}
	return shortcut
}
