# contractor_map_edit

A terminal (tcell) map editor for [contractor](https://github.com/memmaker/contractor).
It reads and writes the game's own data files, so maps saved here load directly in the game.

## Build

Requires Go 1.26+.

```bash
go build -o mapedit .
```

## Run

Point the editor at the game's data directory (the one containing `definitions/` and `maps/`):

```bash
./mapedit data=../contractor/data_atom
```

Optional arguments:

| Argument | Meaning | Default |
|---|---|---|
| `data=<dir>` | Game data directory | `.` |
| `<W>x<H>` | Canvas size, e.g. `120x40` | `80x23` |

## What it edits

**Global data** in `<data>/definitions/`:

- `tileSet.rec` – tile definitions (name, glyph, colors, walkable, transparent, flags)
- `palette.rec` – named colors
- `icons.rec`, `iconsForItems.rec`, `iconsForObjects.rec` – icons
- Read as templates: `actors.rec`, `objectTemplates.rec`, `weapons.rec`, `ammo.rec`, `armor.rec`, `food.rec`, `miscItems.rec`, `consumables.rec`

**Maps** in `<data>/maps/<name>/`:

| File | Content |
|---|---|
| `tiles.bin`, `tileSet.rec` | Tile layer and the tiles it uses |
| `zones.bin` | Zone membership per tile |
| `objects.rec`, `items.rec`, `actors.rec` | Placed objects, items and actors |
| `iconsForObjects.rec` | Object icons used by the map |
| `meta.rec` | Created with defaults if missing. The editor doesn't edit it. |

`zones.rec` and `initFlags.rec` are left untouched; edit them by hand.

Saves are atomic per file (written to a temp file, then renamed). The editor refuses to save
while any placed record still contains a `CHANGEME` placeholder from a template.

## Controls

The top menu bar switches between **Map mode** and **Icon mode** and holds New Tile, Zones,
Load/Save Tileset, Load/Save Map and Quit. The brush bar picks the tool.

| Input | Action |
|---|---|
| Left click / drag | Paint with the current brush |
| Left drag from an object, item or actor | Move it (object first, then item, then actor). Refused if the target already holds the same kind |
| Right click | Edit the topmost thing under the cursor (object, then item, then actor) |
| `F1`–`F5` | Pen, line, rectangle, circle, fill |
| `Ctrl+C` | Select an area to copy (tiles, zones, objects, items, actors) |
| `Ctrl+V` | Paste the copied area |
| `Esc` | Back to tile drawing |
| `Ctrl+S` | In the actor picker: save actor definitions. In the item picker: save item icons |

When pasting, Transitions, NamedLocations and Elevators whose identifier already exists on the
map get a `_2`, `_3`, … suffix, since the game needs those names to be unique. Fix up any
references to them by hand.

Canvas and icon tools:

| Input | Action |
|---|---|
| `Tab` | Pick a glyph from the canvas |
| `F7` | Pick a glyph and save it as an icon |
| `F8` | Save icons |
| `F10` | Quit and print the canvas to stdout |
| `F11` | Paste text onto the canvas |

## Test

```bash
go test ./...
```
