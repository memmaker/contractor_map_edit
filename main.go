package main

import (
	"TextPainter/mapeditor"
	"TextPainter/widgets"
	"bufio"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/core"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/recfile"
	"github.com/memmaker/go/textiles"
	"image/color"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type BrushHolder interface {
	SetBrush(brush fxtools.Brush)
	SetDrawMode()
	DefaultFillCondition(source geometry.Point, curr geometry.Point, next geometry.Point) bool
}

type TextPainter struct {
	application *cview.Application

	screenSize geometry.Point
	isReady    bool

	drawCommands []string

	canvasSize geometry.Point
	mainGrid   *cview.Grid

	palette textiles.ColorPalette

	colorPicker *widgets.ColorPicker

	currentlyEditedColorIndex int

	icons []core.NamedColorChar

	definedTiles []mapeditor.NamedTile

	panels        *cview.Panels
	iconPicker    *widgets.Picker
	palettePicker *widgets.Picker
	charPicker    *widgets.Picker
	drawLayer     *widgets.Canvas
	iconMenuBar   *widgets.ButtonBar
	mapMenuBar    *widgets.ButtonBar
	brushBar      *widgets.ButtonBar
	isMapMode     bool
	mapEditor     *mapeditor.MapEditor
	tilePicker    *widgets.Picker

	currentBrushHolder BrushHolder
	logWindow          *cview.TextView
	gameDataDirectory  string
	actorRecords       []recfile.Record

	// itemTemplates have all the information to create an item.
	// They don't have an icon defined and only include a "Category" field.
	itemTemplates []recfile.Record

	// objectTemplates are example records for the creation of an object.
	// They don't have an icon defined and only include a "Name" field.
	objectTemplates []recfile.Record

	actorPicker  *widgets.Picker
	itemPicker   *widgets.Picker
	objectPicker *widgets.Picker

	itemIcons map[string]core.NamedColorChar
	// itemCategoryRecords Will map an ItemCategory to an Icon with Named Colors
	// There fields are: Name, Icon, Foreground, Background (Opt), InventoryColor
	itemCategoryRecords []recfile.Record

	objectIcons map[string]core.NamedColorChar
	// objectTypeRecords Will map an ObjectType to an Icon with Named Colors
	// There fields are: Name, Icon, Foreground, Background (Opt), InventoryColor
	objectTypeRecords []recfile.Record
	currentMapName    string
}

func NewTextPainter(gameDataDir string, canvasSize geometry.Point) *TextPainter {
	u := &TextPainter{
		canvasSize:        canvasSize,
		gameDataDirectory: gameDataDir,
		palette:           textiles.NewPaletteFromFileOrDefault(path.Join(gameDataDir, "definitions", "palette.rec")),
	}
	u.init()
	return u
}

func (u *TextPainter) toggleMapMode() {
	u.isMapMode = !u.isMapMode

	if u.isMapMode {
		u.setupMapModeLayout()
	} else {
		u.setupIconLayout()
	}

	u.currentBrushHolder.SetBrush(fxtools.NewPencil())
	u.currentBrushHolder.SetDrawMode()
}

func (u *TextPainter) setupIconLayout() {
	if u.iconMenuBar == nil {
		u.initIconUI()
	}

	u.mainGrid.Clear()

	u.mainGrid.SetMinSize(1, 18)
	u.mainGrid.SetColumns(u.canvasSize.X, 18, 18, -1)
	u.mainGrid.SetRows(1, 1, u.canvasSize.Y, 5, -1)

	u.mainGrid.AddItem(u.iconMenuBar, 0, 0, 1, 4, 0, 0, false)
	u.mainGrid.AddItem(u.brushBar, 1, 0, 1, 4, 0, 0, false)
	u.mainGrid.AddItem(u.drawLayer, 2, 0, 1, 1, 0, 0, true)
	u.mainGrid.AddItem(u.charPicker, 2, 1, 1, 1, 0, 0, false)
	u.mainGrid.AddItem(u.palettePicker, 2, 2, 1, 1, 0, 0, false)
	u.mainGrid.AddItem(u.iconPicker, 2, 3, 1, 1, 0, 0, false)
	u.mainGrid.AddItem(u.colorPicker, 3, 0, 1, 4, 0, 0, false)
	u.mainGrid.AddItem(u.logWindow, 4, 0, 1, 4, 0, 0, false)

	u.currentBrushHolder = u.drawLayer
	u.isMapMode = false
}

func (u *TextPainter) setupMapModeLayout() {
	if u.mapMenuBar == nil {
		u.initMapModeUI()
	}

	u.mainGrid.Clear()

	u.mainGrid.SetColumns(u.canvasSize.X, 18, 18, 18, -1)
	u.mainGrid.SetRows(1, 1, u.canvasSize.Y, -1)

	u.mainGrid.AddItem(u.mapMenuBar, 0, 0, 1, 5, 0, 0, false)
	u.mainGrid.AddItem(u.brushBar, 1, 0, 1, 5, 0, 0, false)
	u.mainGrid.AddItem(u.mapEditor, 2, 0, 1, 1, 0, 0, true)
	u.mainGrid.AddItem(u.tilePicker, 2, 1, 1, 1, 0, 0, false)
	u.mainGrid.AddItem(u.actorPicker, 2, 2, 1, 1, 0, 0, false)
	u.mainGrid.AddItem(u.objectPicker, 2, 3, 1, 1, 0, 0, false)
	u.mainGrid.AddItem(u.itemPicker, 2, 4, 1, 1, 0, 0, false)
	u.mainGrid.AddItem(u.logWindow, 3, 0, 1, 5, 0, 0, false)

	u.currentBrushHolder = u.mapEditor
	u.isMapMode = true
}

func (u *TextPainter) initMapModeUI() {
	menuBar := widgets.NewButtonBar([]widgets.LabelItem{
		{
			Label: "New NamedTile",
			Action: func() {
				widgets.OpenModalPicker(u.application, u.panels, pickerItemsFromIcons(u.icons, u.palette), func(selectedIndex int) {
					u.mergeTileSet([]mapeditor.NamedTile{
						mapeditor.NamedTile{
							Icon:          u.icons[selectedIndex],
							Name:          "none",
							IsWalkable:    true,
							IsTransparent: true,
						},
					})
				})
			},
		},
		{
			Label:  "Icon Mode",
			Action: u.toggleMapMode,
		},
		{
			Label:  "Load GameData",
			Action: u.loadGameData,
		},
		{
			Label:  "Load Tileset",
			Action: u.loadTileSet,
		},
		{
			Label:  "Save Tileset",
			Action: u.saveTileSet,
		},
		{
			Label: "Save Map",
			Action: func() {
				prefill := "testmap"
				if u.currentMapName != "" {
					prefill = u.currentMapName
				}
				u.askForString("Map Name?", prefill, func(entered string) {
					mapDir := path.Join(u.gameDataDirectory, "maps", entered)
					u.saveMap(mapDir)
					u.currentMapName = entered
				})
			},
		},
		{
			Label: "Load testmap",
			Action: func() {
				prefill := "testmap"
				if u.currentMapName != "" {
					prefill = u.currentMapName
				}
				u.askForString("Map Name?", prefill, func(entered string) {
					u.loadMap(path.Join(u.gameDataDirectory, "maps", entered))
					u.currentMapName = entered
				})
			},
		},
		{
			Label: "Save & Quit",
			Action: func() {
				u.application.Stop()
			},
		},
	})
	u.mapMenuBar = menuBar

	// Map Viewer
	mapEditor := mapeditor.NewMapEditor(u.palette, u.canvasSize, u.application, u.panels, u.tileResolver, u.itemIconResolver, u.objectIconResolver)
	mapEditor.SetBrush(fxtools.NewPencil())
	mapEditor.SetHoverHandler(func(mapPos geometry.Point) {
		mapInfo := u.mapEditor.GetMapInfoAt(mapPos)
		u.logWindow.SetText(fmt.Sprintf("%s\n%s", mapPos.String(), strings.Join(mapInfo, "\n")))
	})
	u.mapEditor = mapEditor

	// TilePicker
	tilePicker := widgets.NewPicker()
	tilePicker.SetBorder(true)
	tilePicker.SetTitle("Tiles")
	tilePicker.SetIndicateWithColors(true)
	tilePicker.SetListMode()
	tilePicker.SetNoSecondaryIndicator()
	tilePicker.SetItems(pickerItemsFromTiles(u.definedTiles, u.palette))
	tilePicker.SetChangeHandler(func(primaryIdx int, secondaryIdx int, primary widgets.PickerItem, secondary widgets.PickerItem) {
		if primaryIdx >= 0 && primaryIdx < len(u.definedTiles) {
			u.mapEditor.SetTileDrawing(primaryIdx)
			tile := u.definedTiles[primaryIdx]
			u.Print(tile.String())
		}
		if secondaryIdx >= 0 && secondaryIdx < len(u.definedTiles) {
			widgets.OpenModalForm(u.application, u.panels, []widgets.FormElementDescription{
				{
					FieldName:    "name",
					Label:        "Name",
					PrefillValue: widgets.TypedString(u.definedTiles[secondaryIdx].Name),
					FieldType:    widgets.FieldTypeString,
				},
				{
					FieldName:    "isTransparent",
					Label:        "Is Transparent?",
					PrefillValue: widgets.TypedStringFromBool(u.definedTiles[secondaryIdx].IsTransparent),
					FieldType:    widgets.FieldTypeBool,
				},
				{
					FieldName:    "isWalkable",
					Label:        "Is Walkable?",
					PrefillValue: widgets.TypedStringFromBool(u.definedTiles[secondaryIdx].IsWalkable),
					FieldType:    widgets.FieldTypeBool,
				},
			}, func(m map[string]widgets.TypedString) {
				if name, exists := m["name"]; exists {
					u.definedTiles[secondaryIdx].Name = name.AsString()
				}
				if isTransparent, exists := m["isTransparent"]; exists {
					u.definedTiles[secondaryIdx].IsTransparent = isTransparent.AsBool()
				}
				if isWalkable, exists := m["isWalkable"]; exists {
					u.definedTiles[secondaryIdx].IsWalkable = isWalkable.AsBool()
				}
				tilePicker.SetItems(pickerItemsFromTiles(u.definedTiles, u.palette))
			})
		}
	})

	u.tilePicker = tilePicker

	actorPicker := widgets.NewPicker()
	actorPicker.SetBorder(true)
	actorPicker.SetTitle("Actors")
	actorPicker.SetIndicateWithColors(true)
	actorPicker.SetListMode()
	actorPicker.SetNoSecondaryIndicator()
	actorPicker.SetModClickHandler(func(isRight bool, clickedIndex int, clickedItem widgets.PickerItem) {
		if clickedIndex >= 0 && clickedIndex < len(u.actorRecords) {
			u.editActorColor(clickedIndex)
		}
	})
	actorPicker.SetChangeHandler(func(primaryIdx int, secondaryIdx int, primary widgets.PickerItem, secondary widgets.PickerItem) {
		if primaryIdx >= 0 && primaryIdx < len(u.actorRecords) {
			actorRecord := u.actorRecords[primaryIdx]
			u.Print(actorRecord.String())
			mapObject := textiles.NewIconRecord(actorRecord)
			u.mapEditor.SetActorDrawing(mapObject)
		}
		if secondaryIdx >= 0 && secondaryIdx < len(u.actorRecords) {
			u.editActorRecord(secondaryIdx)
		}
	})
	actorPicker.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// capture Ctrl-S
		if event.Key() == tcell.KeyCtrlS {
			u.saveActorRecords()
			return nil
		}
		return event
	})
	u.actorPicker = actorPicker

	u.updateActorPicker()

	itemPicker := widgets.NewPicker()
	itemPicker.SetBorder(true)
	itemPicker.SetTitle("Items")
	itemPicker.SetIndicateWithColors(true)
	itemPicker.SetListMode()
	itemPicker.SetNoSecondaryIndicator()
	itemPicker.SetModClickHandler(func(isRight bool, clickedIndex int, clickedItem widgets.PickerItem) {
		if clickedIndex >= 0 && clickedIndex < len(u.itemTemplates) {
			record := u.itemTemplates[clickedIndex]
			catName := record.FindValueForKeyIgnoreCase("category")
			u.editItemCategoryIcon(catName)
		}
	})
	itemPicker.SetChangeHandler(func(primaryIdx int, secondaryIdx int, primary widgets.PickerItem, secondary widgets.PickerItem) {
		if primaryIdx >= 0 && primaryIdx < len(u.itemTemplates) {
			itemRecord := u.itemTemplates[primaryIdx]
			u.Print(itemRecord.String())
			mapObject := mapeditor.NewCategoryObject(itemRecord)
			u.mapEditor.SetItemDrawing(mapObject)
		}
	})
	itemPicker.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// capture Ctrl-S
		if event.Key() == tcell.KeyCtrlS {
			u.saveIconsForItemsGlobally()
			u.Print("Saved Item Icons")
			return nil
		}
		return event
	})

	u.itemPicker = itemPicker

	u.updateItemPicker()

	// CURRENT PROBLEM
	// The type of Object and the icon of the object is the same field.
	// We want these two things to be independent of each other.
	// They have to be two different fields

	// ALSO
	// For some Objects, we want them to just reference specific icons.
	// While for other objects, we want them have unique icons.
	// This is something each object decides for itself
	// Containers, Signs -> Will always use the individually assigned icon
	// Doors -> Will always use the pre-defined icons for DoorStates

	objectPicker := widgets.NewPicker()
	objectPicker.SetBorder(true)
	objectPicker.SetTitle("Objects")
	objectPicker.SetIndicateWithColors(true)
	objectPicker.SetListMode()
	objectPicker.SetNoSecondaryIndicator()
	objectPicker.SetModClickHandler(func(isRight bool, clickedIndex int, clickedItem widgets.PickerItem) {
		if clickedIndex >= 0 && clickedIndex < len(u.objectTemplates) {
			record := u.objectTemplates[clickedIndex]
			objectName := record.FindValueForKeyIgnoreCase("category")
			if nameField, exists := record.FindFieldIgnoreCase("name"); exists {
				objectName = nameField.Value
			}
			u.editObjectTypeIcon(objectName)
		}
	})

	objectPicker.SetChangeHandler(func(primaryIdx int, secondaryIdx int, primary widgets.PickerItem, secondary widgets.PickerItem) {
		if primaryIdx >= 0 && primaryIdx < len(u.objectTemplates) {
			objectRecord := u.objectTemplates[primaryIdx]
			u.Print(objectRecord.String())
			//objectRecord = append(objectRecord, recfile.Field{Name: "category", Value: objectRecord.FindValueForKeyIgnoreCase("name")})
			mapObject := mapeditor.NewCategoryObject(objectRecord)
			u.mapEditor.SetObjectDrawing(mapObject)
		}
	})

	u.objectPicker = objectPicker

	u.updateObjectPicker()
}

func (u *TextPainter) saveMap(mapDir string) {
	u.mapEditor.SaveMap(mapDir)
	u.saveIconsForObjects(path.Join(mapDir, "iconsForObjects.rec"))
}

func (u *TextPainter) loadMap(mapDirectory string) {
	tiles := u.mapEditor.LoadMap(mapDirectory, u.tileResolver, u.itemIconResolver, u.objectIconResolver)
	if len(tiles) > 0 {
		u.definedTiles = tiles
		u.tilePicker.SetItems(pickerItemsFromTiles(u.definedTiles, u.palette))
	}
	u.loadIconsForObjects(path.Join(mapDirectory, "iconsForObjects.rec"))

	u.updateItemPicker()
	u.updateObjectPicker()
}

func (u *TextPainter) saveTileSet() {
	mapeditor.SaveTileSet(path.Join(u.gameDataDirectory, "definitions", "tileSet.rec"), u.definedTiles)
}

func (u *TextPainter) loadTileSet() {
	u.mergeTileSet(mapeditor.LoadTileSet(path.Join(u.gameDataDirectory, "definitions", "tileSet.rec")))
}

func pickerItemsFromTiles(tiles []mapeditor.NamedTile, palette textiles.ColorPalette) []widgets.PickerItem {
	pickerItems := make([]widgets.PickerItem, len(tiles))
	for i, tile := range tiles {
		pickerItems[i] = widgets.PickerItem{
			Name:      tile.Name,
			Icon:      tile.Icon.Char,
			IconStyle: GetStyleForNamedChar(tile.Icon, palette),
		}
	}
	return pickerItems
}

func (u *TextPainter) initCoreUI() {
	u.application = cview.NewApplication()
	u.application.SetAfterResizeFunc(u.onTerminalResized)

	u.application.EnableMouse(true)

	drawLayer := widgets.NewCanvas()
	drawLayer.SetChar(core.NamedColorChar{
		Char: '#',
		Fg:   u.palette.GetNamedColorByIndex(0).Name,
		Bg:   u.palette.GetNamedColorByIndex(1).Name,
	})
	drawLayer.SetDrawMode()
	drawLayer.SetBrush(fxtools.NewPencil())
	drawLayer.SetPalette(u.palette)
	u.drawLayer = drawLayer

	u.mainGrid = cview.NewGrid()

	u.currentBrushHolder = u.drawLayer

	u.panels = cview.NewPanels()
	u.panels.AddPanel("main", u.mainGrid, true, true)
	u.panels.SetInputCapture(u.handleMainInput)

	u.application.SetRoot(u.panels, true)
	u.application.SetFocus(u.panels)

	// Log window
	logWindow := cview.NewTextView()
	logWindow.SetBorder(true)

	u.logWindow = logWindow

	u.initIconUI()
	u.initMapModeUI()

	u.setupMapModeLayout()
}

func (u *TextPainter) initIconUI() {

	menuBar := widgets.NewButtonBar([]widgets.LabelItem{
		{
			Label: "Pick",
			Action: func() {
				u.drawLayer.SetSelectionHandler(u.pickRune)
			},
		},
		{
			Label: "To Icon",
			Action: func() {
				u.drawLayer.SetSelectionHandler(u.pickRuneAndSaveAsIcon)
			},
		},
		{
			Label:  "Map Mode",
			Action: u.toggleMapMode,
		},
		{
			Label:  "Save Icons",
			Action: u.saveIcons,
		},
		{
			Label: "Save & Quit",
			Action: func() {
				u.application.Stop()
				u.writeToStdOut()
			},
		},
	})
	u.iconMenuBar = menuBar

	charPicker := widgets.NewCharPicker()
	charPicker.SetBorder(true)
	charPicker.SetTitle("Chars")
	charPicker.SetNoSecondaryIndicator()
	charPicker.SetChangeHandler(func(primaryIdx int, secondaryIdx int, primary widgets.PickerItem, secondary widgets.PickerItem) {
		if primary.HasIcon() {
			u.drawLayer.SetCharRune(primary.Icon)
		}
	})
	charPicker.SetPrimarySelectedIndex(int(fxtools.UnicodeToCP437Byte('#')))

	u.charPicker = charPicker

	palettePicker := widgets.NewPalettePicker()
	palettePicker.SetTitle("Colors")
	palettePicker.SetPalette(u.palette)
	palettePicker.SetBorder(true)
	palettePicker.SetPrimarySelectedIndex(0)
	palettePicker.SetSecondarySelectedIndex(1)

	palettePicker.SetChangeHandler(func(primaryIdx int, secondaryIdx int, primary widgets.PickerItem, secondary widgets.PickerItem) {
		if u.palette.IsValidIndex(primaryIdx) {
			colorName := u.palette.GetNamedColorByIndex(primaryIdx).Name
			u.drawLayer.SetCharFg(colorName)
			u.logWindow.SetText(fmt.Sprintf("Primary: %s", colorName))
		}
		if u.palette.IsValidIndex(secondaryIdx) {
			colorName := u.palette.GetNamedColorByIndex(secondaryIdx).Name
			u.drawLayer.SetCharBg(colorName)
			u.logWindow.SetText(fmt.Sprintf("Secondary: %s", colorName))
		}
	})
	palettePicker.SetModClickHandler(func(isRight bool, clickedIndex int, clickedItem widgets.PickerItem) {
		if isRight {
			u.editColorName(clickedIndex)
		} else {
			u.currentlyEditedColorIndex = clickedIndex
			u.colorPicker.SetColor(u.palette.GetByIndex(clickedIndex))
		}
	})
	u.palettePicker = palettePicker

	iconPicker := widgets.NewPicker()
	iconPicker.SetTitle("Icons")
	iconPicker.SetBorder(true)
	iconPicker.SetNoIndicators()
	iconPicker.SetModClickHandler(func(isRight bool, clickedIndex int, clickedItem widgets.PickerItem) {
		// disable background of icon
		if clickedIndex >= 0 && clickedIndex < len(u.icons) {
			u.icons = append(u.icons[:clickedIndex], u.icons[clickedIndex+1:]...)
			u.updateIconPicker()
		}
	})
	iconPicker.SetChangeHandler(func(primaryIdx int, secondaryIdx int, primary widgets.PickerItem, secondary widgets.PickerItem) {
		if primaryIdx >= 0 && primaryIdx < len(u.icons) {
			selectedIcon := u.icons[primaryIdx]
			u.drawLayer.SetChar(selectedIcon)
			u.palettePicker.SetPrimarySelectedIndex(u.palette.GetColorIndex(selectedIcon.Fg))
			u.palettePicker.SetSecondarySelectedIndex(u.palette.GetColorIndex(selectedIcon.Bg))
			u.charPicker.SetPrimarySelectedIndex(int(fxtools.UnicodeToCP437Byte(selectedIcon.Char)))
		}
		if secondaryIdx >= 0 && secondaryIdx < len(u.icons) {
			u.icons[secondaryIdx] = u.icons[secondaryIdx].WithBackground("")
			u.updateIconPicker()
		}
	})

	u.iconPicker = iconPicker
	u.updateIconPicker()

	colorPicker := widgets.NewColorPicker()
	colorPicker.SetChangeHandler(u.onColorChanged)

	u.colorPicker = colorPicker
	brushBar := widgets.NewButtonBar([]widgets.LabelItem{
		{
			Label:             "Pen",
			HighlightOnSelect: true,
			Action: func() {
				u.currentBrushHolder.SetBrush(fxtools.NewPencil())
				u.drawLayer.SetDrawMode()
			},
		},
		{
			Label:             "Line",
			HighlightOnSelect: true,
			Action: func() {
				u.currentBrushHolder.SetBrush(fxtools.NewLineBrush())
				u.drawLayer.SetDrawMode()
			},
		},
		{
			Label:             "Rectangle",
			HighlightOnSelect: true,
			Action: func() {
				u.currentBrushHolder.SetBrush(fxtools.NewOutlinedRectangleBrush())
				u.drawLayer.SetDrawMode()
			},
		},
		{
			Label:             "Circle",
			HighlightOnSelect: true,
			Action: func() {
				u.currentBrushHolder.SetBrush(fxtools.NewOutlinedCircleBrush())
				u.drawLayer.SetDrawMode()
			},
		},
		{
			Label:             "Fill",
			HighlightOnSelect: true,
			Action: func() {
				u.currentBrushHolder.SetBrush(fxtools.NewPaintBucket(u.currentBrushHolder.DefaultFillCondition))
				u.drawLayer.SetDrawMode()
			},
		},
	})
	brushBar.SetHighlight(0)
	u.brushBar = brushBar
}

func (u *TextPainter) askForString(prompt, prefill string, onConfirm func(entered string)) {
	widgets.OpenModalForm(u.application, u.panels, []widgets.FormElementDescription{
		{
			FieldName:    "text",
			PrefillValue: widgets.TypedString(prefill),
			Label:        prompt,
			FieldType:    widgets.FieldTypeString,
		},
	}, func(values map[string]widgets.TypedString) {
		if text, exists := values["text"]; exists {
			onConfirm(text.AsString())
		}
	})
}

func (u *TextPainter) updateIconPicker() {
	pickerItems := pickerItemsFromIcons(u.icons, u.palette)
	u.iconPicker.SetItems(pickerItems)
}

func pickerItemsFromIcons(icons []core.NamedColorChar, palette textiles.ColorPalette) []widgets.PickerItem {
	pickerItems := make([]widgets.PickerItem, len(icons))
	for i, icon := range icons {
		style := styleFromChar(icon, palette)
		if !icon.HasBackground() {
			style = style.Background(tcell.ColorBlack)
		}
		pickerItems[i] = widgets.PickerItem{
			Name:      fmt.Sprintf("%d", i),
			Icon:      icon.Char,
			IconStyle: style,
		}
	}
	return pickerItems
}

func (u *TextPainter) editColorName(index int) {
	oldName := u.palette.GetNamedColorByIndex(index).Name
	widgets.OpenModalForm(u.application, u.panels, []widgets.FormElementDescription{
		{
			FieldName:    "name",
			Label:        "Name",
			FieldType:    widgets.FieldTypeString,
			PrefillValue: widgets.TypedString(oldName),
		},
	}, func(values map[string]widgets.TypedString) {
		if newName, exists := values["name"]; exists && newName.AsString() != oldName {
			u.palette = u.palette.WithColorRenamed(oldName, newName.AsString())
		}
		u.palettePicker.SetPalette(u.palette)
		u.drawLayer.SetPalette(u.palette)
	})
}

func styleFromChar(char core.NamedColorChar, palette textiles.ColorPalette) tcell.Style {
	if char.IsEmpty() {
		return tcell.StyleDefault
	}
	fgIndex := char.Fg
	bgIndex := char.Bg
	if !palette.Has(fgIndex) {
		fgIndex = "white"
	}
	if !palette.Has(bgIndex) {
		bgIndex = "black"
	}
	return tcell.StyleDefault.Foreground(toTcellColor(palette.Get(fgIndex))).Background(toTcellColor(palette.Get(bgIndex)))
}

func (u *TextPainter) Run() {
	if err := u.application.Run(); err != nil {
		panic(err)
	}
}
func (u *TextPainter) onTerminalResized(width int, height int) {
	if !u.isReady {
		u.tryReadFromStdIn()
		u.isReady = true
	}
	u.application.QueueUpdateDraw(func() {
		u.screenSize = geometry.Point{X: width, Y: height}
		if u.isMapMode {
			u.setupMapModeLayout()
		} else {
			u.setupIconLayout()
		}
	})
}

func (u *TextPainter) handleMainInput(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyF1 {
		u.currentBrushHolder.SetBrush(fxtools.NewPencil())
		u.currentBrushHolder.SetDrawMode()
	}
	if event.Key() == tcell.KeyF2 {
		u.currentBrushHolder.SetBrush(fxtools.NewLineBrush())
		u.currentBrushHolder.SetDrawMode()
	}
	if event.Key() == tcell.KeyF3 {
		u.currentBrushHolder.SetBrush(fxtools.NewOutlinedRectangleBrush())
		u.currentBrushHolder.SetDrawMode()
	}
	if event.Key() == tcell.KeyF4 {
		u.currentBrushHolder.SetBrush(fxtools.NewOutlinedCircleBrush())
		u.currentBrushHolder.SetDrawMode()
	}
	if event.Key() == tcell.KeyF5 {
		u.currentBrushHolder.SetBrush(fxtools.NewPaintBucket(u.currentBrushHolder.DefaultFillCondition))
		u.currentBrushHolder.SetDrawMode()
	}

	if event.Key() == tcell.KeyTab {
		u.drawLayer.SetSelectionHandler(u.pickRune)
	}

	if event.Key() == tcell.KeyF7 {
		u.drawLayer.SetSelectionHandler(u.pickRuneAndSaveAsIcon)
	}

	if event.Key() == tcell.KeyF8 {
		u.saveIcons()
	}

	if event.Key() == tcell.KeyF10 {
		u.application.Stop()
		u.writeToStdOut()
	}

	return event
}

func (u *TextPainter) writeToStdOut() {
	for y := 0; y < u.canvasSize.Y; y++ {
		for x := 0; x < u.canvasSize.X; x++ {
			runeAtBuffer := u.drawLayer.GetCharAt(geometry.Point{x, y}).Char
			os.Stdout.Write([]byte(string(runeAtBuffer)))
		}
		os.Stdout.Write([]byte("\n"))
	}
}

func readFromStdIn() ([]rune, int, int, bool) {
	stat, _ := os.Stdin.Stat()
	var width int
	var height int
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var stdin string
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			trimmedLine := strings.Trim(line, "\n")
			if width == 0 {
				width = utf8.RuneCountInString(trimmedLine)
			}
			if width == 0 {
				return []rune{}, 0, 0, false
			}
			if utf8.RuneCountInString(trimmedLine) != width {
				return []rune(stdin), width, height, true
			}
			if utf8.RuneCountInString(trimmedLine) == width {
				stdin += trimmedLine
				height++
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		return []rune(stdin), width, height, true
	} else {
		return []rune{}, 0, 0, false
	}
}

func (u *TextPainter) tryReadFromStdIn() {
	if runes, width, height, success := readFromStdIn(); success {
		screenBuffer := runesToColoredChars(runes)
		canvasSize := geometry.Point{X: width, Y: height}
		u.drawLayer.SetScreenBuffer(screenBuffer, canvasSize)
	}
}

func runesToColoredChars(runes []rune) []core.NamedColorChar {
	chars := make([]core.NamedColorChar, len(runes))
	for i, r := range runes {
		chars[i] = core.NamedColorChar{
			Char: r,
			Fg:   "white",
			Bg:   "black",
		}
	}
	return chars
}

func (u *TextPainter) pickRune(selection []geometry.Point) {
	if len(selection) > 0 {
		samplePoint := selection[0]
		charOnCanvas := u.drawLayer.GetCharAt(samplePoint)
		u.drawLayer.SetChar(charOnCanvas)
		u.drawLayer.SetDrawMode()
		u.palettePicker.SetPrimarySelectedIndex(u.palette.GetColorIndex(charOnCanvas.Fg))
		u.palettePicker.SetSecondarySelectedIndex(u.palette.GetColorIndex(charOnCanvas.Bg))
		u.charPicker.SetPrimarySelectedIndex(int(fxtools.UnicodeToCP437Byte(charOnCanvas.Char)))
	}
}

func (u *TextPainter) pickRuneAndSaveAsIcon(selection []geometry.Point) {
	if len(selection) > 0 {
		samplePoint := selection[0]
		runeFound := u.drawLayer.GetCharAt(samplePoint)
		u.icons = append(u.icons, runeFound)
		u.updateIconPicker()
		u.drawLayer.SetDrawMode()
	}
}

func (u *TextPainter) onColorChanged(rgba color.RGBA) {
	index := u.currentlyEditedColorIndex
	if u.palette.IsValidIndex(index) {
		u.palette = u.palette.WithColorChanged(index, rgba)
		u.palettePicker.SetPalette(u.palette)
		u.drawLayer.SetPalette(u.palette)
	}
}

func (u *TextPainter) savePalette() {
	writePalErr := u.palette.ToFile(path.Join(u.gameDataDirectory, "definitions", "palette.rec"))
	if writePalErr != nil {
		log.Fatal(writePalErr)
	}
}

func (u *TextPainter) saveIcons() {
	// use gob to write icons
	iconFile := fxtools.MustCreate(path.Join(u.gameDataDirectory, "definitions", "icons.rec"))
	defer iconFile.Close()
	records := make([]recfile.Record, len(u.icons))
	for i, icon := range u.icons {
		records[i] = namedColorCharToRecord(icon)
	}
	recfile.Write(iconFile, records)
}

func (u *TextPainter) tryLoadIcons() {
	iconFile := fxtools.MustOpen(path.Join(u.gameDataDirectory, "definitions", "icons.rec"))
	defer iconFile.Close()
	records := recfile.Read(iconFile)
	decodedIcons := make([]core.NamedColorChar, len(records))
	for i, record := range records {
		decodedIcons[i] = recordToNamedColorChar(record)
	}
	u.icons = decodedIcons
}

func (u *TextPainter) editTile(index int) {
	widgets.OpenModalForm(u.application, u.panels, []widgets.FormElementDescription{
		{
			FieldName:    "name",
			Label:        "Name",
			FieldType:    widgets.FieldTypeString,
			PrefillValue: widgets.TypedString(u.definedTiles[index].Name),
		},
		{
			FieldName:    "isWalkable",
			Label:        "Is Walkable",
			FieldType:    widgets.FieldTypeBool,
			PrefillValue: widgets.TypedStringFromBool(u.definedTiles[index].IsWalkable),
		},
		{
			FieldName:    "isTransparent",
			Label:        "Is Transparent",
			FieldType:    widgets.FieldTypeBool,
			PrefillValue: widgets.TypedStringFromBool(u.definedTiles[index].IsTransparent),
		},
	}, func(values map[string]widgets.TypedString) {
		if name, exists := values["name"]; exists {
			u.definedTiles[index].Name = name.AsString()
		}
		if isWalkable, exists := values["isWalkable"]; exists {
			u.definedTiles[index].IsWalkable = isWalkable.AsBool()
		}
		if isTransparent, exists := values["isTransparent"]; exists {
			u.definedTiles[index].IsTransparent = isTransparent.AsBool()
		}
	})
}

func (u *TextPainter) SetCanvasSize(point geometry.Point) {
	u.canvasSize = point
}

func (u *TextPainter) mergeTileSet(loadedSet []mapeditor.NamedTile) {
	if len(u.definedTiles) == 0 {
		u.definedTiles = loadedSet
	} else {
		tileSet := make(map[mapeditor.NamedTile]bool)
		for _, tile := range u.definedTiles {
			tileSet[tile] = true
		}
		for _, tile := range loadedSet {
			if _, exists := tileSet[tile]; !exists {
				u.definedTiles = append(u.definedTiles, tile)
				tileSet[tile] = true
			}

		}
	}
	u.tilePicker.SetItems(pickerItemsFromTiles(u.definedTiles, u.palette))
}

func (u *TextPainter) tileResolver(idx int16) mapeditor.NamedTile {
	if idx >= 0 && int(idx) < len(u.definedTiles) {
		return u.definedTiles[idx]
	}
	return mapeditor.NamedTile{}
}

func (u *TextPainter) init() {
	u.tryLoadIcons()

	u.initCoreUI()

	u.drawLayer.SetCanvasSize(u.canvasSize)
	u.onTerminalResized(u.screenSize.X, u.screenSize.Y)

	u.loadTileSet()
	u.loadGameData()
}

func (u *TextPainter) loadGameData() {
	u.loadIconsForItems(path.Join(u.gameDataDirectory, "definitions", "iconsForItems.rec"))
	u.loadIconsForObjects(path.Join(u.gameDataDirectory, "definitions", "iconsForObjects.rec"))

	filePath := path.Join(u.gameDataDirectory, "definitions")

	actorFilePath := path.Join(filePath, "actors.rec")
	weaponsFilePath := path.Join(filePath, "weapons.rec")
	ammoFilePath := path.Join(filePath, "ammo.rec")
	armorFilePath := path.Join(filePath, "armor.rec")
	foodFilePath := path.Join(filePath, "food.rec")
	miscFilePath := path.Join(filePath, "miscItems.rec")

	actors := fxtools.MustOpen(actorFilePath)
	defer actors.Close()
	u.actorRecords = recfile.Read(actors)

	weapons := fxtools.MustOpen(weaponsFilePath)
	defer weapons.Close()
	weaponRecords := recfile.Read(weapons)

	ammo := fxtools.MustOpen(ammoFilePath)
	defer ammo.Close()
	ammoRecords := recfile.Read(ammo)

	armors := fxtools.MustOpen(armorFilePath)
	defer armors.Close()
	armorRecords := recfile.Read(armors)

	food := fxtools.MustOpen(foodFilePath)
	defer food.Close()
	foodRecords := recfile.Read(food)

	misc := fxtools.MustOpen(miscFilePath)
	defer misc.Close()
	miscRecords := recfile.Read(misc)

	itemRecords := append(weaponRecords, ammoRecords...)
	itemRecords = append(itemRecords, armorRecords...)
	itemRecords = append(itemRecords, foodRecords...)
	itemRecords = append(itemRecords, miscRecords...)

	u.itemTemplates = itemRecords
	u.objectTemplates = recfile.Read(fxtools.MustOpen(path.Join(filePath, "objectTemplates.rec")))

	u.updateActorPicker()
	u.updateItemPicker()
	u.updateObjectPicker()

}

func (u *TextPainter) loadIconsForObjects(objectTypesFilePath string) {
	objTypes, opErr := os.Open(objectTypesFilePath)
	if opErr != nil {
		return
	}
	defer objTypes.Close()

	u.objectIcons = mapToColoredChar(ReadIconRecordsIntoLowerCaseMap(objTypes))
	objTypes.Seek(0, 0)
	u.objectTypeRecords = recfile.Read(objTypes)
}
func OpenOrEmpty(filename string) *os.File {
	open, err := os.Open(filename)
	if err != nil { // return fake file
		return &os.File{}
	}
	return open
}
func (u *TextPainter) loadIconsForItems(itemCategoriesFilePath string) {

	itemCats, opErr := os.Open(itemCategoriesFilePath)
	if opErr != nil {
		return
	}

	defer itemCats.Close()
	u.itemIcons = mapToColoredChar(ReadIconRecordsIntoLowerCaseMap(itemCats))

	itemCats.Seek(0, 0)
	u.itemCategoryRecords = recfile.Read(itemCats)
}

func mapToColoredChar(caseMap map[string]textiles.IconRecord) map[string]core.NamedColorChar {
	result := make(map[string]core.NamedColorChar)
	for k, v := range caseMap {
		result[k] = v.Icon
	}
	return result
}

func (u *TextPainter) updateActorPicker() {
	pickerItems := make([]widgets.PickerItem, len(u.actorRecords))
	for i, record := range u.actorRecords {
		pickerItems[i] = widgets.PickerItem{
			Name:      record.FindValueForKeyIgnoreCase("name"),
			Icon:      []rune(record.FindValueForKeyIgnoreCase("icon"))[0],
			IconStyle: tcell.StyleDefault.Foreground(toTcellColor(u.palette.Get(record.FindValueForKeyIgnoreCase("foreground")))),
		}
	}
	u.actorPicker.SetItems(pickerItems)
}
func (u *TextPainter) objectIconResolver(objectType string) core.NamedColorChar {
	if resolved, exists := u.objectIcons[strings.ToLower(objectType)]; exists {
		return resolved
	}
	return core.NamedColorChar{}
}

func (u *TextPainter) itemIconResolver(cat string) core.NamedColorChar {
	if category, exists := u.itemIcons[strings.ToLower(cat)]; exists {
		return category
	}
	return core.NamedColorChar{}
}
func (u *TextPainter) updateItemPicker() {
	if len(u.itemIcons) == 0 {
		return
	}

	pickerItems := make([]widgets.PickerItem, len(u.itemTemplates))
	for i, record := range u.itemTemplates {
		itemCategory := record.FindValueForKeyIgnoreCase("category")
		category := u.itemIconResolver(itemCategory)
		itemName := record.FindValueForKeyIgnoreCase("name")
		pickerItems[i] = widgets.PickerItem{
			Name:      itemName,
			Icon:      category.Char,
			IconStyle: GetStyleForNamedChar(category, u.palette),
		}
	}
	u.itemPicker.SetItems(pickerItems)
}

func (u *TextPainter) updateObjectPicker() {
	pickerItems := make([]widgets.PickerItem, 0)
	for _, record := range u.objectTemplates {
		objectName := record.FindValueForKeyIgnoreCase("category")
		if field, exists := record.FindFieldIgnoreCase("name"); exists {
			objectName = field.Value
		}
		typeIcon := u.objectIconResolver(objectName)
		pickerItems = append(pickerItems, widgets.PickerItem{
			Name:      objectName,
			Icon:      typeIcon.Char,
			IconStyle: GetStyleForNamedChar(typeIcon, u.palette),
		})
	}
	u.objectPicker.SetItems(pickerItems)
}
func GetStyle(icon textiles.TextIcon) tcell.Style {
	if icon.HasBackground() {
		return tcell.StyleDefault.Foreground(toTcellColor(icon.Fg)).Background(toTcellColor(icon.Bg))
	}
	return tcell.StyleDefault.Foreground(toTcellColor(icon.Fg))
}

func GetStyleForNamedChar(icon core.NamedColorChar, palette textiles.ColorPalette) tcell.Style {
	if icon.HasBackground() {
		return tcell.StyleDefault.Foreground(toTcellColor(palette.Get(icon.Fg))).Background(toTcellColor(palette.Get(icon.Bg)))
	}
	return tcell.StyleDefault.Foreground(toTcellColor(palette.Get(icon.Fg)))
}
func (u *TextPainter) editActorColor(idx int) {
	record := u.actorRecords[idx]
	widgets.OpenModalPicker(u.application, u.panels, widgets.PickerItemsFromPalette(u.palette), func(selectedIndex int) {
		u.actorRecords[idx] = record.WithKeyValue("foreground", u.palette.GetNamedColorByIndex(selectedIndex).Name)
		u.updateActorPicker()
	})
}
func (u *TextPainter) editActorRecord(idx int) {
	record := u.actorRecords[idx]
	widgets.EditRecord(u.application, u.panels, record, func(modifiedRecord recfile.Record) {
		u.actorRecords[idx] = modifiedRecord
		u.updateActorPicker()
		u.logWindow.SetText("Modified record")
	})
}

func (u *TextPainter) editItemCategoryIcon(catName string) {
	widgets.OpenModalPicker(u.application, u.panels, pickerItemsFromIcons(u.icons, u.palette), func(selectedIndex int) {
		u.itemIcons[strings.ToLower(catName)] = u.icons[selectedIndex]
		u.updateItemPicker()
	})
}

func (u *TextPainter) editObjectTypeIcon(objectName string) {
	widgets.OpenModalPicker(u.application, u.panels, pickerItemsFromIcons(u.icons, u.palette), func(selectedIndex int) {
		key := strings.ToLower(objectName)
		selectedIcon := u.icons[selectedIndex]
		if _, exists := u.objectIcons[key]; !exists {
			newEntry := recfile.Record{
				recfile.Field{Name: "Name", Value: objectName},
				recfile.Field{Name: "Icon", Value: string(selectedIcon.Char)},
				recfile.Field{Name: "Foreground", Value: selectedIcon.Fg},
			}
			if selectedIcon.HasBackground() {
				newEntry = append(newEntry, recfile.Field{Name: "Background", Value: selectedIcon.Bg})
			}
			u.objectTypeRecords = append(u.objectTypeRecords, newEntry)
		}
		u.objectIcons[key] = selectedIcon
		u.updateObjectPicker()
	})
}

func (u *TextPainter) Print(text string) {
	if u.logWindow != nil {
		u.logWindow.SetText(strings.TrimSpace(text))
		u.logWindow.ScrollToBeginning()
	}
}

func (u *TextPainter) saveActorRecords() {
	actorFilePath := path.Join(u.gameDataDirectory, "definitions", "actors.rec")
	actors := fxtools.MustCreate(actorFilePath)
	defer actors.Close()
	recfile.Write(actors, u.actorRecords)
	u.Print("Saved actors")
}

func (u *TextPainter) saveIconsForItemsGlobally() {
	itemCategoriesFilePath := path.Join(u.gameDataDirectory, "definitions", "iconsForItems.rec")
	u.saveIconsForItems(itemCategoriesFilePath)
}

func (u *TextPainter) saveIconsForItems(itemCategoriesFilePath string) {
	itemCats := fxtools.MustCreate(itemCategoriesFilePath)
	defer itemCats.Close()
	records := u.itemCategoryRecords

	for i, cat := range records {
		catName := cat.FindValueForKeyIgnoreCase("Name")
		icon := u.itemIconResolver(catName)
		records[i] = recfile.Record{
			recfile.Field{Name: "Name", Value: catName},
			recfile.Field{Name: "Icon", Value: string(icon.Char)},
			recfile.Field{Name: "Foreground", Value: icon.Fg},
			recfile.Field{Name: "InventoryColor", Value: cat.FindValueForKeyIgnoreCase("InventoryColor")},
		}
		if icon.HasBackground() {
			records[i] = append(records[i], recfile.Field{Name: "Background", Value: icon.Bg})
		}
	}

	recfile.Write(itemCats, records)
	u.Print("Saved item categories")
}

func (u *TextPainter) saveIconsForObjectsGlobally() {
	objectTypesFilePath := path.Join(u.gameDataDirectory, "definitions", "iconsForObjects.rec")
	u.saveIconsForObjects(objectTypesFilePath)
}

func (u *TextPainter) saveIconsForObjects(objectTypesFilePath string) {
	objectTypes := fxtools.MustCreate(objectTypesFilePath)
	defer objectTypes.Close()
	records := u.objectTypeRecords

	for i, obj := range records {
		objName := obj.FindValueForKeyIgnoreCase("name")
		icon := u.objectIconResolver(objName)
		records[i] = recfile.Record{
			recfile.Field{Name: "Name", Value: objName},
			recfile.Field{Name: "Icon", Value: string(icon.Char)},
			recfile.Field{Name: "Foreground", Value: icon.Fg},
		}
		if icon.HasBackground() {
			records[i] = append(records[i], recfile.Field{Name: "Background", Value: icon.Bg})
		}
	}

	recfile.Write(objectTypes, records)
	u.Print("Saved object types")
}

func IndexToXY(index int, width int) (int, int) {
	return index % width, index / width
}

func XYToIndex(x int, y int, width int) int {
	return y*width + x
}
func toTcellColor(rgba color.RGBA) tcell.Color {
	return tcell.NewRGBColor(int32(rgba.R), int32(rgba.G), int32(rgba.B))
}
func main() {
	gameDir := "."
	canvasSize := geometry.Point{X: 80, Y: 23}
	if len(os.Args) > 1 {
		dimensionRegex := "([0-9]+)x([0-9]+)"
		for _, arg := range os.Args[1:] {
			if dimensions, _ := regexp.MatchString(dimensionRegex, arg); dimensions {
				parts := strings.Split(arg, "x")
				width, _ := strconv.Atoi(parts[0])
				height, _ := strconv.Atoi(parts[1])
				canvasSize = geometry.Point{X: width, Y: height}
			} else if strings.HasPrefix(arg, "data=") {
				dataDir := strings.TrimSpace(strings.TrimPrefix(arg, "data="))
				if dataDir != "" {
					gameDir = dataDir
				}
			}
		}
	}
	app := NewTextPainter(gameDir, canvasSize)
	app.Run()
}

func ReadIconRecordsIntoLowerCaseMap(reader io.Reader) map[string]textiles.IconRecord {
	records := recfile.Read(reader)
	categories := make(map[string]textiles.IconRecord, len(records))
	for _, record := range records {
		category := textiles.NewIconRecord(record)
		categories[strings.ToLower(category.Name)] = category
	}
	return categories
}
func namedColorCharToRecord(c core.NamedColorChar) recfile.Record {
	return recfile.Record{
		recfile.Field{
			Name:  "Char",
			Value: string(c.Char),
		},
		recfile.Field{
			Name:  "Fg",
			Value: c.Fg,
		},
		recfile.Field{
			Name:  "Bg",
			Value: c.Bg,
		},
	}
}

func recordToNamedColorChar(record recfile.Record) core.NamedColorChar {
	var char core.NamedColorChar
	for _, field := range record {
		switch strings.ToLower(field.Name) {
		case "char":
			char.Char = field.AsRune()
		case "fg":
			char.Fg = field.Value
		case "bg":
			char.Bg = field.Value
		}
	}
	return char
}
