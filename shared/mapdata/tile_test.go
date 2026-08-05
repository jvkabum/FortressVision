package mapdata

import (
	"bytes"
	"encoding/gob"
	"testing"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestCalculateRampTypeUsesWallAndFloorNeighbors(t *testing.T) {
	store := NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeRamp}
	store.Tiletypes[2] = dfproto.Tiletype{ID: 2, Shape: dfproto.ShapeWall}
	store.Tiletypes[3] = dfproto.Tiletype{ID: 3, Shape: dfproto.ShapeFloor}

	store.Mu.Lock()
	for _, pos := range []util.DFCoord{
		{X: 1, Y: 1, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 1},
	} {
		origin := pos.BlockCoord()
		chunk := store.Chunks[origin]
		if chunk == nil {
			chunk = &Chunk{Origin: origin}
			store.Chunks[origin] = chunk
		}
		tile := NewTile(store, pos)
		switch pos {
		case (util.DFCoord{X: 1, Y: 1, Z: 0}):
			tile.TileType = 1
		case (util.DFCoord{X: 1, Y: 0, Z: 0}):
			tile.TileType = 2
		case (util.DFCoord{X: 1, Y: 0, Z: 1}):
			tile.TileType = 3
		}
		local := pos.LocalCoord()
		chunk.Tiles[local.X][local.Y] = tile
	}
	ramp := store.getTileLocked(util.DFCoord{X: 1, Y: 1, Z: 0})
	store.Mu.Unlock()

	ramp.CalculateRampType()
	if ramp.RampType != 2 {
		t.Fatalf("RampType = %d, want 2 for a north wall with a floor above it", ramp.RampType)
	}
}

func TestBrookShapesAreRenderableFloors(t *testing.T) {
	store := NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeBrookBed}
	store.Tiletypes[2] = dfproto.Tiletype{ID: 2, Shape: dfproto.ShapeBrookTop}

	bed := NewTile(store, util.DFCoord{X: 0, Y: 0, Z: 0})
	bed.TileType = 1
	top := NewTile(store, util.DFCoord{X: 1, Y: 0, Z: 0})
	top.TileType = 2

	if !bed.IsFloor() || !top.IsFloor() {
		t.Fatalf("brook bed/top must be treated as renderable floors")
	}
}

func TestChunkSnapshotCarriesVisualEntities(t *testing.T) {
	chunk := &Chunk{
		Buildings: []dfproto.BuildingInstance{{Index: 7}},
		Items:     []dfproto.Item{{ID: 9}},
		Plants:    []dfproto.PlantDetail{{Pos: dfproto.Coord{X: 1, Y: 2}}},
		Flows:     []dfproto.FlowInfo{{Index: 4, Type: dfproto.FlowSmoke}},
	}

	snapshot := chunk.Snapshot()
	if len(snapshot.Buildings) != 1 || snapshot.Buildings[0].Index != 7 {
		t.Fatalf("building data was not copied to snapshot")
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].ID != 9 {
		t.Fatalf("item data was not copied to snapshot")
	}
	if len(snapshot.Plants) != 1 || snapshot.Plants[0].Pos.X != 1 {
		t.Fatalf("plant data was not copied to snapshot")
	}
	if len(snapshot.Flows) != 1 || snapshot.Flows[0].Type != dfproto.FlowSmoke {
		t.Fatalf("flow data was not copied to snapshot")
	}
}

func TestChunkSnapshotGobRoundTrip(t *testing.T) {
	want := ChunkSnapshot{
		Buildings: []dfproto.BuildingInstance{{Index: 7}},
		Items:     []dfproto.Item{{ID: 9, StackSize: 3}},
		Plants:    []dfproto.PlantDetail{{Pos: dfproto.Coord{X: 1, Y: 2}}},
		Flows:     []dfproto.FlowInfo{{Index: 4, Type: dfproto.FlowSteam}},
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(want); err != nil {
		t.Fatalf("encode chunk snapshot: %v", err)
	}
	var got ChunkSnapshot
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("decode chunk snapshot: %v", err)
	}
	if len(got.Buildings) != 1 || len(got.Items) != 1 || len(got.Plants) != 1 || len(got.Flows) != 1 || got.Flows[0].Type != dfproto.FlowSteam {
		t.Fatalf("chunk snapshot round trip lost visual entities: %#v", got)
	}
}
