package mapeditor

import (
	"errors"
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
	"slices"
	"sort"
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
	return WriteRecordsAtomic(filename, records)
}

func LoadTileSet(filename string) ([]NamedTile, error) {
	records, err := readRecords(filename)
	if err != nil {
		return nil, err
	}
	tiles := make([]NamedTile, len(records))
	for i, record := range records {
		tiles[i] = TileFromRecord(record)
	}
	return tiles, nil
}

// readRecords reads a rec file. A missing file yields no records, any other open error is returned.
func readRecords(filename string) ([]recfile.Record, error) {
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	records, _ := recfile.Read(file)
	return records, nil
}

// saveAtomically lets save write to a temp file next to path, then renames it over path.
func saveAtomically(path string, save func(tmpPath string) error) error {
	tmp := path + ".tmp"
	if err := save(tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// WriteRecordsAtomic writes records to path via a temp file and rename.
func WriteRecordsAtomic(path string, records []recfile.Record) error {
	return saveAtomically(path, func(tmp string) error {
		file, err := os.Create(tmp)
		if err != nil {
			return err
		}
		if err = recfile.Write(file, records); err != nil {
			file.Close()
			return err
		}
		return file.Close()
	})
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
	for _, pos := range sortedPoints(p) {
		records = append(records, p[pos].ToRecordWithPosition(pos))
	}
	return records
}

func sortedPoints[V any](m map[geometry.Point]V) []geometry.Point {
	points := make([]geometry.Point, 0, len(m))
	for pos := range m {
		points = append(points, pos)
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].Y != points[j].Y {
			return points[i].Y < points[j].Y
		}
		return points[i].X < points[j].X
	})
	return points
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
	return CategoryObject{
		Name:         o.Name,
		Category:     o.Category,
		IconOverride: o.IconOverride,
		Meta:         slices.Clone(o.Meta),
	}
}

func (o CategoryObject) WithCategory(newCat string) CategoryObject {
	return CategoryObject{
		Name:         o.Name,
		Category:     newCat,
		IconOverride: o.IconOverride,
		Meta:         slices.Clone(o.Meta),
	}
}

func (o CategoryObject) WithMeta(newMeta recfile.Record) CategoryObject {
	return CategoryObject{
		Name:         o.Name,
		Category:     o.Category,
		IconOverride: o.IconOverride,
		Meta:         slices.Clone(newMeta),
	}
}

type CategoryPositionMap map[geometry.Point]CategoryObject

func (m CategoryPositionMap) ToRecords() []recfile.Record {
	records := make([]recfile.Record, 0, len(m))
	for _, pos := range sortedPoints(m) {
		records = append(records, m[pos].ToRecordWithPosition(pos))
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
	// order of precedence (topmost first): object, item, actor, tile
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

func LoadMapFromDisk(mapDirectory string, resolver func(idx int16) NamedTile, items func(cat string) core.NamedColorChar, objects func(objectType string) core.NamedColorChar) (*GameMap, []NamedTile, error) {
	dimensions, tiles := textiles.ReadTileMap16(filepath.Join(mapDirectory, "tiles.bin"))
	if len(tiles) == 0 {
		return nil, nil, fmt.Errorf("no tiles.bin found in %s", mapDirectory)
	}
	area := NewMapArea(dimensions, resolver, items, objects)
	area.Tiles = tiles
	g := &GameMap{
		MapArea: area,
	}

	tileSetFile := filepath.Join(mapDirectory, "tileSet.rec")
	if !fxtools.FileExists(tileSetFile) {
		tileSetFile = filepath.Join(mapDirectory, "tileset.rec") // older maps
	}
	tileSet, err := LoadTileSet(tileSetFile)
	if err != nil {
		return nil, nil, err
	}

	if fxtools.FileExists(filepath.Join(mapDirectory, "zones.bin")) {
		g.zones = textiles.ReadZones(filepath.Join(mapDirectory, "zones.bin"))
	}

	objectRecords, err := readRecords(filepath.Join(mapDirectory, "objects.rec"))
	if err != nil {
		return nil, nil, err
	}
	itemRecords, err := readRecords(filepath.Join(mapDirectory, "items.rec"))
	if err != nil {
		return nil, nil, err
	}
	actorRecords, err := readRecords(filepath.Join(mapDirectory, "actors.rec"))
	if err != nil {
		return nil, nil, err
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
			obj := NewIconRecordSafe(record.WithoutFieldsIgnoreCase("Position"))
			if err == nil {
				g.actors[pos] = obj
			}
		}
	}

	return g, tileSet, nil
}

// PlaceholderFields lists "Category@x,y Field" for every field still holding a CHANGEME template value.
func (g *GameMap) PlaceholderFields() []string {
	var found []string
	for _, m := range []CategoryPositionMap{g.objects, g.items} {
		for _, pos := range sortedPoints(m) {
			for _, f := range m[pos].ToRecord() {
				if strings.Contains(strings.ToLower(f.Value), "changeme") {
					found = append(found, fmt.Sprintf("%s@%d,%d %s", m[pos].Category, pos.X, pos.Y, f.Name))
				}
			}
		}
	}
	for _, pos := range sortedPoints(g.actors) {
		for _, f := range g.actors[pos].ToRecord() {
			if strings.Contains(strings.ToLower(f.Value), "changeme") {
				found = append(found, fmt.Sprintf("actor@%d,%d %s", pos.X, pos.Y, f.Name))
			}
		}
	}
	return found
}

// uniqueIdField is the record field the game uses as the unique name of a placed object category.
// Elevator/NamedLocation: "identifier"; Transition: "Location" (registered as a named location).
func uniqueIdField(category string) string {
	switch strings.ToLower(category) {
	case "elevator", "namedlocation":
		return "Identifier"
	case "transition":
		return "Location"
	}
	return ""
}

func (g *GameMap) idTaken(category, field, id string) bool {
	for _, o := range g.objects {
		if uniqueIdField(o.Category) != "" && strings.EqualFold(uniqueIdField(o.Category), field) && o.Meta.FindValueForKeyIgnoreCase(field) == id {
			return true
		}
	}
	return false
}

func (g *GameMap) uniqueObject(o CategoryObject) CategoryObject {
	field := uniqueIdField(o.Category)
	id := o.Meta.FindValueForKeyIgnoreCase(field)
	if field == "" || id == "" || !g.idTaken(o.Category, field, id) {
		return o
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s_%d", id, n)
		if !g.idTaken(o.Category, field, candidate) {
			return o.WithMeta(slices.Clone(o.Meta).WithKeyValueIgnoreCase(field, candidate))
		}
	}
}

func (g *GameMap) SaveMapToDisk(mapDirectory string) error {
	if err := os.Mkdir(mapDirectory, 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	mapName := path.Base(mapDirectory)
	tiles, tileSet := g.generateTileSet()

	err := saveAtomically(filepath.Join(mapDirectory, "tiles.bin"), func(tmp string) error {
		return textiles.SaveTileMap16(tiles, g.Dimensions, tmp)
	})
	if err != nil {
		return err
	}

	err = SaveTileSet(filepath.Join(mapDirectory, "tileSet.rec"), tileSet)
	if err != nil {
		return err
	}

	zoneErr := saveAtomically(filepath.Join(mapDirectory, "zones.bin"), func(tmp string) error {
		return textiles.SaveZones(tmp, g.zones)
	})
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

		err = WriteRecordsAtomic(metaFile, []recfile.Record{metaRecord})
		if err != nil {
			return err
		}
	}

	err = WriteRecordsAtomic(filepath.Join(mapDirectory, "objects.rec"), g.objects.ToRecords())
	if err != nil {
		return err
	}
	err = WriteRecordsAtomic(filepath.Join(mapDirectory, "items.rec"), g.items.ToRecords())
	if err != nil {
		return err
	}
	return WriteRecordsAtomic(filepath.Join(mapDirectory, "actors.rec"), g.actors.ToRecords())
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
	area = area.Intersect(geometry.NewRect(0, 0, g.Dimensions.X, g.Dimensions.Y))
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
			for name, points := range g.zones {
				if points[orgPos] {
					if copyBuffer.zones == nil {
						copyBuffer.zones = make(map[string]map[geometry.Point]bool)
					}
					if copyBuffer.zones[name] == nil {
						copyBuffer.zones[name] = make(map[geometry.Point]bool)
					}
					copyBuffer.zones[name][pos] = true
				}
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
			for _, points := range g.zones {
				delete(points, newPos)
			}
			for name, points := range area.zones {
				if points[oldPos] {
					g.SetZone(newPos, name)
				}
			}
			g.Tiles[newPos.Y*g.Dimensions.X+newPos.X] = tileIdx
			if actor, ok := area.actors[oldPos]; ok {
				g.actors[newPos] = actor
			}
			if object, ok := area.objects[oldPos]; ok {
				g.objects[newPos] = g.uniqueObject(object)
			}
			if item, ok := area.items[oldPos]; ok {
				g.items[newPos] = item
			}
		}
	}
}

func (g *GameMap) inBounds(pos geometry.Point) bool {
	return pos.X >= 0 && pos.X < g.Dimensions.X && pos.Y >= 0 && pos.Y < g.Dimensions.Y
}

func (g *GameMap) SetTile(pos geometry.Point, tile int16) {
	if !g.inBounds(pos) {
		return
	}
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

// MoveTopmostAt moves the object, else item, else actor at from to to.
// It refuses when to already holds the same kind of entity.
func (g *GameMap) MoveTopmostAt(from, to geometry.Point) bool {
	if from == to || !to.In(geometry.NewRect(0, 0, g.Dimensions.X, g.Dimensions.Y)) {
		return false
	}
	if o, ok := g.objects[from]; ok {
		if _, taken := g.objects[to]; taken {
			return false
		}
		delete(g.objects, from)
		g.objects[to] = o
		return true
	}
	if i, ok := g.items[from]; ok {
		if _, taken := g.items[to]; taken {
			return false
		}
		delete(g.items, from)
		g.items[to] = i
		return true
	}
	if a, ok := g.actors[from]; ok {
		if _, taken := g.actors[to]; taken {
			return false
		}
		delete(g.actors, from)
		g.actors[to] = a
		return true
	}
	return false
}

func (g *GameMap) IsEntityAt(pos geometry.Point) bool {
	return g.IsObjectAt(pos) || g.IsItemAt(pos) || g.IsActorAt(pos)
}

func (g *GameMap) EditActorAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	actor, exists := g.actors[pos]
	if !exists {
		return
	}

	cview.EditRecord(app, panels, actor.ToRecord(), func(record recfile.Record) {
		obj := NewIconRecordSafe(record)
		g.actors[pos] = obj
	})
}
func (g *GameMap) EditObjectAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	object, exists := g.objects[pos]
	if !exists {
		return
	}

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
	if !g.inBounds(pos) {
		return nil
	}
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

// NewIconRecordSafe guards textiles.NewIconRecord, which panics on an empty icon value.
func NewIconRecordSafe(record recfile.Record) textiles.IconRecord {
	record = slices.Clone(record)
	for i, f := range record {
		if strings.EqualFold(f.Name, "icon") && f.Value == "" {
			record[i].Value = "?"
		}
	}
	return textiles.NewIconRecord(record)
}
