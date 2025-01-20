package mapeditor

import (
	"fmt"
	"github.com/memmaker/go/core"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/recfile"
	"github.com/memmaker/go/textiles"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type TileFlags uint8

func (f TileFlags) Has(flag TileFlags) bool {
	return f&flag != 0
}

func (f TileFlags) With(flag TileFlags) TileFlags {
	return f | flag
}

func (f TileFlags) WithFlagSetTo(flag TileFlags, asBool bool) TileFlags {
	if asBool {
		return f | flag
	}
	return f &^ flag
}

const TileNoFlags TileFlags = 0

const ( // bitwise flags
	TileFlagHazardous TileFlags = 1 << iota
	TileFlagWater
	TileFlagRadiated
	TileFlagMountable
	TileFlagCrawlable
	TileFlagDestroyable
)

type NamedTile struct {
	Icon          core.NamedColorChar
	Name          string
	IsWalkable    bool
	IsTransparent bool
	Flags         TileFlags
}

func SaveTileSet(filename string, tiles []NamedTile) error {
	records := make([]recfile.Record, len(tiles))
	for i, tile := range tiles {
		tileRecord := tile.ToRecord()
		records[i] = tileRecord
	}
	err := recfile.Write(fxtools.MustCreate(filename), records)
	if err != nil {
		return err
	}
	return nil
}

func LoadTileSet(filename string) []NamedTile {
	records, _ := recfile.Read(fxtools.MustOpen(filename))
	tiles := make([]NamedTile, len(records))
	for i, record := range records {
		tiles[i] = TileFromRecord(record)
	}
	return tiles
}
func (t NamedTile) ToRecord() recfile.Record {
	return recfile.Record{
		recfile.Field{Name: "Name", Value: t.Name},
		recfile.Field{Name: "Char", Value: string(t.Icon.Char)},
		recfile.Field{Name: "Foreground", Value: t.Icon.Fg},
		recfile.Field{Name: "Background", Value: t.Icon.Bg},
		recfile.Field{Name: "IsWalkable", Value: strconv.FormatBool(t.IsWalkable)},
		recfile.Field{Name: "IsTransparent", Value: strconv.FormatBool(t.IsTransparent)},
		recfile.Field{Name: "Flags", Value: strconv.FormatUint(uint64(t.Flags), 10)},
	}
}

func (t NamedTile) String() string {
	return fmt.Sprintf("%s (W: %c, T: %c, HA: %c, WT: %c, MT: %c, CR: %c, DS: %c)", t.Name, boolAsChar(t.IsWalkable), boolAsChar(t.IsTransparent), boolAsChar(t.IsHazardous()), boolAsChar(t.IsWater()), boolAsChar(t.Flags.Has(TileFlagMountable)), boolAsChar(t.Flags.Has(TileFlagCrawlable)), boolAsChar(t.Flags.Has(TileFlagDestroyable)))
}

func (t NamedTile) IsHazardous() bool {
	return t.Flags.Has(TileFlagHazardous)
}

func (t NamedTile) IsWater() bool {
	return t.Flags.Has(TileFlagWater)
}

func (t NamedTile) IsDestroyable() bool {
	return t.Flags.Has(TileFlagDestroyable)
}
func boolAsChar(b bool) rune {
	if b { // checkmark unicode
		return '✓'
	}
	return '✗'
}
func TileFromRecord(record recfile.Record) NamedTile {
	tile := NamedTile{}
	for _, field := range record {
		switch strings.ToLower(field.Name) {
		case "name":
			tile.Name = field.Value
		case "char":
			tile.Icon.Char = field.AsRune()
		case "foreground":
			tile.Icon.Fg = field.Value
		case "background":
			tile.Icon.Bg = field.Value
		case "iswalkable":
			tile.IsWalkable = field.AsBool()
		case "istransparent":
			tile.IsTransparent = field.AsBool()
		case "flags":
			tile.Flags = TileFlags(field.AsInt())
		}
	}
	return tile
}

type IconPositionMap map[geometry.Point]textiles.IconRecord

func (p IconPositionMap) ToRecords() []recfile.Record {
	records := make([]recfile.Record, 0, len(p))
	for pos, obj := range p {
		records = append(records, obj.ToRecordWithPosition(pos))
	}
	return records
}
func (p IconPositionMap) ToRecordsWithoutIcons() []recfile.Record {
	records := make([]recfile.Record, 0, len(p))
	for pos, obj := range p {
		record := obj.ToRecordWithPosition(pos)
		record = record.WithoutFieldsIgnoreCase("Icon", "Foreground", "Background")
		records = append(records, record)
	}
	return records
}

type Layer uint8

const (
	LayerTiles Layer = iota
	LayerObjects
	LayerItems
	LayerActors
)

type CategoryObject struct {
	Name         string
	Category     string
	IconOverride string
	Meta         recfile.Record
}

func NewCategoryObject(record recfile.Record) CategoryObject {
	return CategoryObject{
		Name:         record.FindValueForKeyIgnoreCase("Name"),
		Category:     record.FindValueForKeyIgnoreCase("Category"),
		IconOverride: record.FindValueForKeyIgnoreCase("IconOverride"),
		Meta:         record.WithoutFieldsIgnoreCase("Name", "Category", "IconOverride"),
	}
}

func (o CategoryObject) String() string {
	head := fmt.Sprintf("Category: %s", o.Category)
	if o.Name != "" {
		head = fmt.Sprintf("Name: %s\nCategory: %s", o.Name, o.Category)
	}
	return fmt.Sprintf("%s\n%s", head, o.Meta.String())
}

func (o CategoryObject) ToRecord() recfile.Record {
	record := recfile.Record{}
	record = append(record, recfile.Field{Name: "Category", Value: o.Category})
	if o.Name != "" {
		record = append(record, recfile.Field{Name: "Name", Value: o.Name})
	}
	if o.IconOverride != "" {
		record = append(record, recfile.Field{Name: "IconOverride", Value: o.IconOverride})
	}
	record = append(record, o.Meta...)
	return record
}

func (o CategoryObject) IconIdentifier() string {
	if o.IconOverride != "" {
		return o.IconOverride
	}
	return o.Category
}

func (o CategoryObject) ToRecordWithPosition(pos geometry.Point) recfile.Record {
	record := o.ToRecord()
	return record.WithKeyValueIgnoreCase("Position", pos.Encode())
}

func (o CategoryObject) Copy() CategoryObject {
	newMeta := make(recfile.Record, len(o.Meta))
	copy(newMeta, o.Meta)
	return CategoryObject{
		Name:     o.Name,
		Category: o.Category,
		Meta:     newMeta,
	}
}

func (o CategoryObject) WithCategory(newCat string) CategoryObject {
	return CategoryObject{
		Name:     o.Name,
		Category: newCat,
		Meta:     o.Meta,
	}
}

func (o CategoryObject) WithMeta(newMeta recfile.Record) CategoryObject {
	return CategoryObject{
		Name:     o.Name,
		Category: o.Category,
		Meta:     newMeta,
	}
}

type CategoryPositionMap map[geometry.Point]CategoryObject

func (m CategoryPositionMap) ToRecords() []recfile.Record {
	records := make([]recfile.Record, 0, len(m))
	for pos, obj := range m {
		records = append(records, obj.ToRecordWithPosition(pos))
	}
	return records
}

type MapArea struct {
	Dimensions geometry.Point
	Tiles      []int16
	actors     IconPositionMap
	objects    CategoryPositionMap
	items      CategoryPositionMap

	zones map[string]map[geometry.Point]bool

	resolveTile   func(int16) NamedTile
	resolveObject func(string) core.NamedColorChar
	resolveItem   func(string) core.NamedColorChar
}

func (m *MapArea) CharAt(pos geometry.Point) (core.NamedColorChar, bool) {
	if pos.X < 0 || pos.X >= m.Dimensions.X || pos.Y < 0 || pos.Y >= m.Dimensions.Y {
		return core.NamedColorChar{}, false
	}
	layerIconFound := false
	layerIcon := core.NamedColorChar{}
	// order of precedence: actor, item, object, tile
	if actor, ok := m.actors[pos]; ok {
		layerIcon = actor.Icon
		layerIconFound = true
	}

	if item, ok := m.items[pos]; ok {
		layerIcon = m.resolveItem(item.Category)
		layerIconFound = true
	}

	if object, ok := m.objects[pos]; ok {
		layerIcon = m.resolveObject(object.IconIdentifier())
		layerIconFound = true
	}

	if layerIconFound && layerIcon.HasBackground() {
		return layerIcon, true
	}

	idx := m.Tiles[pos.Y*m.Dimensions.X+pos.X]
	tile := m.resolveTile(idx)

	if layerIconFound {
		return layerIcon.WithBackground(tile.Icon.Bg), true
	}

	return tile.Icon, true
}

func (m *MapArea) RotateClockwise() {
	newTiles := make([]int16, m.Dimensions.X*m.Dimensions.Y)
	newActors := make(IconPositionMap)
	newObjects := make(CategoryPositionMap)
	newItems := make(CategoryPositionMap)
	for y := 0; y < m.Dimensions.Y; y++ {
		for x := 0; x < m.Dimensions.X; x++ {
			newX := m.Dimensions.Y - y - 1
			newY := x
			newPos := geometry.Point{X: newX, Y: newY}
			oldPos := geometry.Point{X: x, Y: y}
			newTiles[newY*m.Dimensions.X+newX] = m.Tiles[y*m.Dimensions.X+x]
			if actor, ok := m.actors[oldPos]; ok {
				newActors[newPos] = actor
			}
			if object, ok := m.objects[oldPos]; ok {
				newObjects[newPos] = object
			}
			if item, ok := m.items[oldPos]; ok {
				newItems[newPos] = item
			}
		}
	}
	m.Tiles = newTiles
	m.actors = newActors
	m.objects = newObjects
	m.items = newItems
	m.Dimensions = geometry.Point{X: m.Dimensions.Y, Y: m.Dimensions.X}
}

type GameMap struct {
	*MapArea

	copyBuffer *MapArea
}

func NewGameMap(dimensions geometry.Point, resolver func(idx int16) NamedTile, items func(cat string) core.NamedColorChar, objects func(objectType string) core.NamedColorChar) *GameMap {
	return &GameMap{
		MapArea: NewMapArea(dimensions, resolver, items, objects),
	}
}

func NewMapArea(dimensions geometry.Point, resolver func(idx int16) NamedTile, items func(cat string) core.NamedColorChar, objects func(objectType string) core.NamedColorChar) *MapArea {
	return &MapArea{
		Dimensions:    dimensions,
		Tiles:         make([]int16, dimensions.X*dimensions.Y),
		actors:        make(IconPositionMap),
		objects:       make(CategoryPositionMap),
		items:         make(CategoryPositionMap),
		resolveTile:   resolver,
		resolveObject: objects,
		resolveItem:   items,
	}
}

func LoadMapFromDisk(mapDirectory string, resolver func(idx int16) NamedTile, items func(cat string) core.NamedColorChar, objects func(objectType string) core.NamedColorChar) (*GameMap, []NamedTile) {
	dimensions, tiles := textiles.ReadTileMap16(filepath.Join(mapDirectory, "tiles.bin"))
	if len(tiles) == 0 {
		return nil, nil
	}
	area := NewMapArea(dimensions, resolver, items, objects)
	area.Tiles = tiles
	g := &GameMap{
		MapArea: area,
	}

	tileSet := LoadTileSet(filepath.Join(mapDirectory, "tileset.rec"))

	if fxtools.FileExists(filepath.Join(mapDirectory, "zones.bin")) {
		g.zones = textiles.ReadZones(filepath.Join(mapDirectory, "zones.bin"))
	}

	var objectRecords, itemRecords, actorRecords []recfile.Record
	if fxtools.FileExists(filepath.Join(mapDirectory, "objects.rec")) {
		objectRecords, _ = recfile.Read(fxtools.MustOpen(filepath.Join(mapDirectory, "objects.rec")))
	}
	if fxtools.FileExists(filepath.Join(mapDirectory, "items.rec")) {
		itemRecords, _ = recfile.Read(fxtools.MustOpen(filepath.Join(mapDirectory, "items.rec")))
	}
	if fxtools.FileExists(filepath.Join(mapDirectory, "actors.rec")) {
		actorRecords, _ = recfile.Read(fxtools.MustOpen(filepath.Join(mapDirectory, "actors.rec")))
	}

	g.actors = make(IconPositionMap)
	g.objects = make(CategoryPositionMap)
	g.items = make(CategoryPositionMap)

	for _, record := range objectRecords {
		if posField, hasPos := record.FindField("Position"); hasPos {
			pos, err := geometry.NewPointFromEncodedString(posField.Value)
			if err == nil {
				obj := NewCategoryObject(record.WithoutFieldsIgnoreCase("Position"))
				g.objects[pos] = obj
			}
		}
	}
	for _, record := range itemRecords {
		if posField, hasPos := record.FindField("Position"); hasPos {
			pos, err := geometry.NewPointFromEncodedString(posField.Value)
			if err == nil {
				obj := NewCategoryObject(record.WithoutFieldsIgnoreCase("Position"))
				g.items[pos] = obj
			}
		}
	}
	for _, record := range actorRecords {
		if posField, hasPos := record.FindField("Position"); hasPos {
			pos, err := geometry.NewPointFromEncodedString(posField.Value)
			obj := textiles.NewIconRecord(record.WithoutFieldsIgnoreCase("Position"))
			if err == nil {
				g.actors[pos] = obj
			}
		}
	}

	return g, tileSet
}

func (g *GameMap) SaveMapToDisk(mapDirectory string) error {
	os.Mkdir(mapDirectory, 0755)

	mapName := path.Base(mapDirectory)
	tiles, tileSet := g.generateTileSet()

	err := textiles.SaveTileMap16(tiles, g.Dimensions, filepath.Join(mapDirectory, "tiles.bin"))
	if err != nil {
		return err
	}

	err = SaveTileSet(filepath.Join(mapDirectory, "tileSet.rec"), tileSet)
	if err != nil {
		return err
	}

	zoneErr := textiles.SaveZones(filepath.Join(mapDirectory, "zones.bin"), g.zones)
	if zoneErr != nil {
		return zoneErr
	}

	metaFile := filepath.Join(mapDirectory, "meta.rec")

	if !fxtools.FileExists(metaFile) {
		metaRecord := recfile.Record{
			recfile.Field{Name: "Name", Value: mapName},
			recfile.Field{Name: "MusicFile", Value: ""},
			recfile.Field{Name: "IsOutdoor", Value: "false"},
			recfile.Field{Name: "IndoorAmbientLight", Value: "(1.0, 1.0, 1.0)"},
		}

		err = recfile.Write(fxtools.MustCreate(metaFile), []recfile.Record{metaRecord})
		if err != nil {
			return err
		}
	}

	objectRecords := g.objects.ToRecords()
	itemRecords := g.items.ToRecords()
	actorRecords := g.actors.ToRecords()

	err = recfile.Write(fxtools.MustCreate(filepath.Join(mapDirectory, "objects.rec")), objectRecords)
	if err != nil {
		return err
	}
	err = recfile.Write(fxtools.MustCreate(filepath.Join(mapDirectory, "items.rec")), itemRecords)
	if err != nil {
		return err
	}
	err = recfile.Write(fxtools.MustCreate(filepath.Join(mapDirectory, "actors.rec")), actorRecords)
	if err != nil {
		return err
	}
	return nil
}

func (g *GameMap) Copy(area geometry.Rect) {
	g.copyBuffer = g.copyArea(area)
}

func (g *GameMap) Paste(pos geometry.Point) {
	if g.copyBuffer == nil {
		return
	}
	g.pasteArea(g.copyBuffer, pos)
}

func (g *GameMap) GetCopyBuffer() *MapArea {
	return g.copyBuffer
}

func (g *GameMap) copyArea(area geometry.Rect) *MapArea {
	copyBuffer := NewMapArea(area.Size(), g.resolveTile, g.resolveItem, g.resolveObject)
	for y := 0; y < area.Size().Y; y++ {
		for x := 0; x < area.Size().X; x++ {
			orgPos := geometry.Point{X: area.Min.X + x, Y: area.Min.Y + y}
			pos := geometry.Point{X: x, Y: y}
			tileIdx := g.Tiles[(area.Min.Y+y)*g.Dimensions.X+area.Min.X+x]
			copyBuffer.Tiles[y*area.Size().X+x] = tileIdx
			if actor, ok := g.actors[orgPos]; ok {
				copyBuffer.actors[pos] = actor.Copy()
			}
			if object, ok := g.objects[orgPos]; ok {
				copyBuffer.objects[pos] = object.Copy()
			}
			if item, ok := g.items[orgPos]; ok {
				copyBuffer.items[pos] = item.Copy()
			}
		}
	}
	return copyBuffer
}

func (g *GameMap) pasteArea(area *MapArea, pos geometry.Point) {
	for y := 0; y < area.Dimensions.Y; y++ {
		for x := 0; x < area.Dimensions.X; x++ {
			newPos := geometry.Point{X: pos.X + x, Y: pos.Y + y}
			oldPos := geometry.Point{X: x, Y: y}
			tileIdx := area.Tiles[y*area.Dimensions.X+x]
			if newPos.X < 0 || newPos.X >= g.Dimensions.X || newPos.Y < 0 || newPos.Y >= g.Dimensions.Y {
				continue
			}
			delete(g.actors, newPos)
			delete(g.objects, newPos)
			delete(g.items, newPos)
			g.Tiles[newPos.Y*g.Dimensions.X+newPos.X] = tileIdx
			if actor, ok := area.actors[oldPos]; ok {
				g.actors[newPos] = actor
			}
			if object, ok := area.objects[oldPos]; ok {
				g.objects[newPos] = object
			}
			if item, ok := area.items[oldPos]; ok {
				g.items[newPos] = item
			}
		}
	}
}

func (g *GameMap) SetTile(pos geometry.Point, tile int16) {
	g.Tiles[pos.Y*g.Dimensions.X+pos.X] = tile
}

func (g *GameMap) SetActor(pos geometry.Point, actor textiles.IconRecord) {
	g.actors[pos] = actor
}

func (g *GameMap) SetObject(pos geometry.Point, object CategoryObject) {
	g.objects[pos] = object
}

func (g *GameMap) SetItem(pos geometry.Point, item CategoryObject) {
	g.items[pos] = item
}

func (g *GameMap) RemoveActor(pos geometry.Point) {
	delete(g.actors, pos)
}

func (g *GameMap) RemoveObject(pos geometry.Point) {
	delete(g.objects, pos)
}

func (g *GameMap) RemoveItem(pos geometry.Point) {
	delete(g.items, pos)
}

func (g *GameMap) EditActorAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	actor, exists := g.actors[pos]
	if !exists {
		return
	}

	cview.EditRecord(app, panels, actor.ToRecord(), func(record recfile.Record) {
		obj := textiles.NewIconRecord(record)
		g.actors[pos] = obj
	})
}
func (g *GameMap) EditDoorAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	door, objExists := g.objects[pos]
	if !objExists {
		return
	}
	if !strings.HasSuffix(strings.ToLower(door.Category), "door") {
		return
	}
	/*
			Category: LockedDoor
			AudioCue: wood_door
			Description: a door
			LockFlag: bar_door_security
			NumberLock: 1996
		    LockDifficulty: medium
	*/
	dmap := door.Meta.ToLowerMap(",")
	diffAsNumberString := diffToNumberString(dmap.GetStringOrDefault("lockdifficulty", "veryeasy"))
	cview.OpenModalForm(app, panels, []cview.FormElementDescription{
		{
			FieldName:    "category",
			PrefillValue: cview.TypedString(door.Category),
			Label:        "Category",
			FieldType:    cview.FieldTypeString, // should be select
		},
		{
			FieldName:    "audio_cue",
			PrefillValue: cview.TypedString(dmap.GetStringOrDefault("audiocue", "")),
			Label:        "Audio Cue",
			FieldType:    cview.FieldTypeString, // should be select
		},
		{
			FieldName:    "description",
			Label:        "Description",
			PrefillValue: cview.TypedString(dmap.GetStringOrDefault("description", "a door")),
		},
		{
			FieldName:    "lock_flag",
			Label:        "Lock Flag",
			PrefillValue: cview.TypedString(dmap.GetStringOrDefault("lockflag", "")),
			FieldType:    cview.FieldTypeString,
		},
		{
			FieldName:    "number_lock",
			Label:        "Number Lock",
			PrefillValue: cview.TypedString(dmap.GetStringOrDefault("numberlock", "")),
			FieldType:    cview.FieldTypeString,
		},
		{
			FieldName:    "lock_difficulty",
			Label:        "Lock Difficulty",
			PrefillValue: cview.TypedString(diffAsNumberString),
			FieldType:    cview.FieldTypeIntegerSlider, // should be select?
		},
	}, func(values map[string]cview.TypedString) {
		oldMeta := door.Meta
		for k, v := range values {
			if k == "category" {
				continue
			}
			value := v.AsString()
			if k == "lock_difficulty" {
				value = numberStringToDiff(value)
			}
			oldMeta = oldMeta.WithKeyValueIgnoreCase(k, value)
		}
		door = door.WithCategory(values["category"].AsString())
		door = door.WithMeta(oldMeta)
		g.objects[pos] = door
	})
}

/*
case "trivial": = "0"

	return Trivial

case "veryeasy":

	return VeryEasy

case "easy":

	return Easy

case "medium":

	return Medium

case "hard":

	return Hard

case "veryhard":

	return VeryHard

case "superhuman":

	return SuperHuman
*/
func diffToNumberString(orDefault string) string {
	switch strings.ToLower(orDefault) {
	case "trivial":
		return "0"
	case "veryeasy":
		return "1"
	case "easy":
		return "2"
	case "medium":
		return "3"
	case "hard":
		return "4"
	case "veryhard":
		return "5"
	case "superhuman":
		return "6"
	}
	return orDefault
}
func numberStringToDiff(orDefault string) string {
	switch orDefault {
	case "0":
		return "Trivial"
	case "1":
		return "VeryEasy"
	case "2":
		return "Easy"
	case "3":
		return "Medium"
	case "4":
		return "Hard"
	case "5":
		return "VeryHard"
	case "6":
		return "SuperHuman"
	}
	return orDefault
}
func (g *GameMap) EditObjectAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	object, exists := g.objects[pos]
	if !exists {
		return
	}

	/* TODO: bugfix
	   if strings.HasSuffix(strings.ToLower(object.Category), "door") {
	   	g.EditDoorAt(app, panels, pos)
	   	return
	   }

	*/

	cview.EditRecord(app, panels, object.ToRecord(), func(record recfile.Record) {
		obj := NewCategoryObject(record)
		g.objects[pos] = obj
	})
}

func (g *GameMap) EditItemAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	item, exists := g.items[pos]
	if !exists {
		return
	}

	cview.EditRecord(app, panels, item.ToRecord(), func(record recfile.Record) {
		obj := NewCategoryObject(record)
		g.items[pos] = obj
	})
}

func (g *GameMap) MapInfoAt(pos geometry.Point) []string {
	var info []string
	if actor, ok := g.actors[pos]; ok {
		info = append(info, actor.String())
	}
	if item, ok := g.items[pos]; ok {
		info = append(info, item.String())
	}
	if object, ok := g.objects[pos]; ok {
		info = append(info, object.String())
	}
	if tile := g.Tiles[pos.Y*g.Dimensions.X+pos.X]; g.resolveTile(tile).Name != "" {
		info = append(info, g.resolveTile(tile).String())
	}
	return info
}

func (g *GameMap) generateTileSet() ([]int16, []NamedTile) {
	inTileSet := make(map[int16]bool)
	tileSet := make([]NamedTile, 0)
	remapTable := make(map[int16]int16)
	tileMap := make([]int16, len(g.Tiles))
	for mapIdx, tileIdx := range g.Tiles {
		if _, ok := inTileSet[tileIdx]; !ok {
			oldIndex := tileIdx
			newIndex := int16(len(tileSet))
			inTileSet[tileIdx] = true

			tileSet = append(tileSet, g.resolveTile(tileIdx))
			remapTable[oldIndex] = newIndex
		}
		tileMap[mapIdx] = remapTable[tileIdx]
	}
	return tileMap, tileSet
}

func (g *GameMap) IsActorAt(pos geometry.Point) bool {
	_, ok := g.actors[pos]
	return ok
}

func (g *GameMap) IsItemAt(pos geometry.Point) bool {
	_, ok := g.items[pos]
	return ok
}

func (g *GameMap) IsObjectAt(pos geometry.Point) bool {
	_, ok := g.objects[pos]
	return ok
}

func (g *GameMap) HasCopyBuffer() bool {
	return g.copyBuffer != nil && g.copyBuffer.Dimensions.X > 0 && g.copyBuffer.Dimensions.Y > 0
}

func (g *GameMap) SetZone(point geometry.Point, zoneName string) {
	if g.zones == nil {
		g.zones = make(map[string]map[geometry.Point]bool)
	}
	if g.zones[zoneName] == nil {
		g.zones[zoneName] = make(map[geometry.Point]bool)
	}
	g.zones[zoneName][point] = true
}

func (g *GameMap) IsZoneAt(pos geometry.Point, zoneName string) bool {
	if g.zones[zoneName] == nil {
		return false
	}
	if _, ok := g.zones[zoneName][pos]; !ok {
		return false
	}
	return g.zones[zoneName][pos]
}

func (g *GameMap) RemoveZone(zone string) {
	delete(g.zones, zone)
}

func (g *GameMap) RemoveZoneAt(mouse geometry.Point, zone string) {
	delete(g.zones[zone], mouse)
	if len(g.zones[zone]) == 0 {
		delete(g.zones, zone)
	}
}

func (g *GameMap) GetEmptyCopyBuffer(dimensions geometry.Point) *MapArea {
	return NewMapArea(dimensions, g.resolveTile, g.resolveItem, g.resolveObject)
}
