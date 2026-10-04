package mapeditor

import (
	"testing"

	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/recfile"
)

func TestPasteRenamesIdsAndCarriesZones(t *testing.T) {
	g := NewGameMap(geometry.Point{X: 4, Y: 1}, nil, nil, nil)
	g.SetObject(geometry.Point{X: 0, Y: 0}, NewCategoryObject(recfile.Record{{Name: "Category", Value: "NamedLocation"}, {Name: "Identifier", Value: "door"}}))
	g.SetZone(geometry.Point{X: 0, Y: 0}, "z")
	buf := g.copyArea(geometry.NewRect(0, 0, 1, 1))
	g.pasteArea(buf, geometry.Point{X: 2, Y: 0})
	g.pasteArea(buf, geometry.Point{X: 3, Y: 0})
	if got := g.objects[geometry.Point{X: 2, Y: 0}].Meta.FindValueForKeyIgnoreCase("Identifier"); got != "door_2" {
		t.Fatalf("got %q", got)
	}
	if got := g.objects[geometry.Point{X: 3, Y: 0}].Meta.FindValueForKeyIgnoreCase("Identifier"); got != "door_3" {
		t.Fatalf("got %q", got)
	}
	if !g.IsZoneAt(geometry.Point{X: 2, Y: 0}, "z") {
		t.Fatal("zone not pasted")
	}
}
