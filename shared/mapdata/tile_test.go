package mapdata

import (
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
