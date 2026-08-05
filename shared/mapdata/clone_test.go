package mapdata

import (
	"testing"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestChunkCloneDoesNotShareTiles(t *testing.T) {
	store := NewMapDataStore()
	chunk := &Chunk{Origin: util.DFCoord{X: 16, Y: 32, Z: 4}}
	chunk.Tiles[2][3] = NewTile(store, util.DFCoord{X: 18, Y: 35, Z: 4})
	chunk.Tiles[2][3].TileType = 9
	chunk.Tiles[2][3].WaterLevel = 4
	chunk.Tiles[2][3].Material = dfproto.MatPair{MatType: 2, MatIndex: 7}

	clone := chunk.Clone()
	if clone == nil || clone.Tiles[2][3] == nil {
		t.Fatalf("clone lost the populated tile")
	}
	clone.Tiles[2][3].TileType = 12
	clone.Tiles[2][3].WaterLevel = 0
	if chunk.Tiles[2][3].TileType != 9 || chunk.Tiles[2][3].WaterLevel != 4 {
		t.Fatalf("clone shares mutable tile state with source")
	}
	if clone.Tiles[2][3].GetStore() != store {
		t.Fatalf("clone tile lost the material store reference")
	}
}
