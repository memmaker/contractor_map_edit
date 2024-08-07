package mapeditor

import (
	"TextPainter/widgets"
	"fmt"
	"github.com/memmaker/go/core"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/recfile"
	"github.com/memmaker/go/textiles"
	"os"
	"path"
	"strconv"
	"strings"
)

type NamedTile struct {
	Icon          core.NamedColorChar
	Name          string
	IsWalkable    bool
	IsTransparent bool
	IsDamaging    bool
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
	records := recfile.Read(fxtools.MustOpen(filename))
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
		recfile.Field{Name: "IsDamaging", Value: strconv.FormatBool(t.IsDamaging)},
	}
}

func (t NamedTile) String() string {
	return fmt.Sprintf("%s (W: %c, T: %c, D: %c)", t.Name, boolAsChar(t.IsWalkable), boolAsChar(t.IsTransparent), boolAsChar(t.IsDamaging))
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
		case "isdamaging":
			tile.IsDamaging = field.AsBool()
		}
	}
	return tile
}

type PositionMap map[geometry.Point]textiles.IconRecord

func (p PositionMap) ToRecords() []recfile.Record {
	records := make([]recfile.Record, 0, len(p))
	for pos, obj := range p {
		records = append(records, obj.ToRecordWithPosition(pos))
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

type GameMap struct {
	Dimensions   geometry.Point
	Tiles        []int16
	resolveTile  func(int16) NamedTile
	SparseLayers map[Layer]PositionMap
}

func NewGameMap(dimensions geometry.Point, resolver func(idx int16) NamedTile) *GameMap {
	return &GameMap{
		Dimensions: dimensions,
		Tiles:      make([]int16, dimensions.X*dimensions.Y),
		SparseLayers: map[Layer]PositionMap{
			LayerObjects: make(PositionMap),
			LayerItems:   make(PositionMap),
			LayerActors:  make(PositionMap),
		},
		resolveTile: resolver,
	}
}

func LoadMapFromDisk(mapDirectory string, resolver func(idx int16) NamedTile) (*GameMap, []NamedTile) {
	dimensions, tiles := textiles.ReadTileMap16(path.Join(mapDirectory, "tiles.bin"))
	if len(tiles) == 0 {
		return nil, nil
	}
	g := &GameMap{
		Dimensions: dimensions,
		Tiles:      tiles,
		SparseLayers: map[Layer]PositionMap{
			LayerObjects: make(PositionMap),
			LayerItems:   make(PositionMap),
			LayerActors:  make(PositionMap),
		},
		resolveTile: resolver,
	}

	tileSet := LoadTileSet(path.Join(mapDirectory, "tileset.rec"))

	objectRecords := recfile.Read(fxtools.MustOpen(path.Join(mapDirectory, "objects.rec")))
	itemRecords := recfile.Read(fxtools.MustOpen(path.Join(mapDirectory, "items.rec")))
	actorRecords := recfile.Read(fxtools.MustOpen(path.Join(mapDirectory, "actors.rec")))

	g.SparseLayers[LayerObjects] = make(PositionMap)
	g.SparseLayers[LayerItems] = make(PositionMap)
	g.SparseLayers[LayerActors] = make(PositionMap)

	for _, record := range objectRecords {
		obj := textiles.NewIconRecord(record)
		if posField, hasPos := obj.Meta.FindField("Position"); hasPos {
			pos, err := geometry.NewPointFromEncodedString(posField.Value)
			if err == nil {
				g.SparseLayers[LayerObjects][pos] = obj
			}
		}
	}
	for _, record := range itemRecords {
		obj := textiles.NewIconRecord(record)
		if posField, hasPos := obj.Meta.FindField("Position"); hasPos {
			pos, err := geometry.NewPointFromEncodedString(posField.Value)
			if err == nil {
				g.SparseLayers[LayerItems][pos] = obj
			}
		}
	}
	for _, record := range actorRecords {
		obj := textiles.NewIconRecord(record)
		if posField, hasPos := obj.Meta.FindField("Position"); hasPos {
			pos, err := geometry.NewPointFromEncodedString(posField.Value)
			if err == nil {
				g.SparseLayers[LayerActors][pos] = obj
			}
		}
	}

	return g, tileSet
}

func (g *GameMap) SaveMapToDisk(mapDirectory string) error {
	os.Mkdir(mapDirectory, 0755)

	tiles, tileSet := g.generateTileSet()

	err := textiles.SaveTileMap16(tiles, g.Dimensions, path.Join(mapDirectory, "tiles.bin"))
	if err != nil {
		return err
	}

	err = SaveTileSet(path.Join(mapDirectory, "tileSet.rec"), tileSet)
	if err != nil {
		return err
	}

	objectRecords := g.SparseLayers[LayerObjects].ToRecords()
	itemRecords := g.SparseLayers[LayerItems].ToRecords()
	actorRecords := g.SparseLayers[LayerActors].ToRecords()

	err = recfile.Write(fxtools.MustCreate(path.Join(mapDirectory, "objects.rec")), objectRecords)
	if err != nil {
		return err
	}
	err = recfile.Write(fxtools.MustCreate(path.Join(mapDirectory, "items.rec")), itemRecords)
	if err != nil {
		return err
	}
	err = recfile.Write(fxtools.MustCreate(path.Join(mapDirectory, "actors.rec")), actorRecords)
	if err != nil {
		return err
	}
	return nil
}

func (g *GameMap) SetTile(pos geometry.Point, tile int16) {
	g.Tiles[pos.Y*g.Dimensions.X+pos.X] = tile
}

func (g *GameMap) SetActor(pos geometry.Point, actor textiles.IconRecord) {
	g.SparseLayers[LayerActors][pos] = actor
}

func (g *GameMap) SetObject(pos geometry.Point, object textiles.IconRecord) {
	g.SparseLayers[LayerObjects][pos] = object
}

func (g *GameMap) SetItem(pos geometry.Point, item textiles.IconRecord) {
	g.SparseLayers[LayerItems][pos] = item
}

func (g *GameMap) RemoveActor(pos geometry.Point) {
	delete(g.SparseLayers[LayerActors], pos)
}

func (g *GameMap) RemoveObject(pos geometry.Point) {
	delete(g.SparseLayers[LayerObjects], pos)
}

func (g *GameMap) RemoveItem(pos geometry.Point) {
	delete(g.SparseLayers[LayerItems], pos)
}

func (g *GameMap) EditActorAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	actor, exists := g.SparseLayers[LayerActors][pos]
	if !exists {
		return
	}

	widgets.EditRecord(app, panels, actor.ToRecord(), func(record recfile.Record) {
		obj := textiles.NewIconRecord(record)
		g.SparseLayers[LayerActors][pos] = obj
	})
}

func (g *GameMap) EditObjectAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	object, exists := g.SparseLayers[LayerObjects][pos]
	if !exists {
		return
	}

	widgets.EditRecord(app, panels, object.ToRecord(), func(record recfile.Record) {
		obj := textiles.NewIconRecord(record)
		g.SparseLayers[LayerObjects][pos] = obj
	})
}

func (g *GameMap) EditItemAt(app *cview.Application, panels *cview.Panels, pos geometry.Point) {
	item, exists := g.SparseLayers[LayerItems][pos]
	if !exists {
		return
	}

	widgets.EditRecord(app, panels, item.ToRecord(), func(record recfile.Record) {
		obj := textiles.NewIconRecord(record)
		g.SparseLayers[LayerItems][pos] = obj
	})
}

func (g *GameMap) CharAt(pos geometry.Point) (core.NamedColorChar, bool) {
	if pos.X < 0 || pos.X >= g.Dimensions.X || pos.Y < 0 || pos.Y >= g.Dimensions.Y {
		return core.NamedColorChar{}, false
	}
	layerIconFound := false
	layerIcon := core.NamedColorChar{}
	// order of precedence: actor, item, object, tile
	if actor, ok := g.SparseLayers[LayerActors][pos]; ok {
		layerIcon = actor.Icon
		layerIconFound = true
	}

	if item, ok := g.SparseLayers[LayerItems][pos]; ok {
		layerIcon = item.Icon
		layerIconFound = true
	}

	if object, ok := g.SparseLayers[LayerObjects][pos]; ok {
		layerIcon = object.Icon
		layerIconFound = true
	}

	if layerIconFound && layerIcon.HasBackground() {
		return layerIcon, true
	}

	idx := g.Tiles[pos.Y*g.Dimensions.X+pos.X]
	tile := g.resolveTile(idx)

	if layerIconFound {
		return layerIcon.WithBackground(tile.Icon.Bg), true
	}

	return tile.Icon, true
}

func (g *GameMap) MapInfoAt(pos geometry.Point) []string {
	var info []string
	if actor, ok := g.SparseLayers[LayerActors][pos]; ok {
		info = append(info, actor.String())
	}
	if item, ok := g.SparseLayers[LayerItems][pos]; ok {
		info = append(info, item.String())
	}
	if object, ok := g.SparseLayers[LayerObjects][pos]; ok {
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
	_, ok := g.SparseLayers[LayerActors][pos]
	return ok
}

func (g *GameMap) IsItemAt(pos geometry.Point) bool {
	_, ok := g.SparseLayers[LayerItems][pos]
	return ok
}

func (g *GameMap) IsObjectAt(pos geometry.Point) bool {
	_, ok := g.SparseLayers[LayerObjects][pos]
	return ok
}
