package mesher

import (
	"testing"

	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestIsolatedFloorHasExternalFaces(t *testing.T) {
	store := mapdata.NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{
		ID:       1,
		Shape:    dfproto.ShapeFloor,
		Material: dfproto.TilematStone,
	}

	chunk := &mapdata.Chunk{Origin: util.DFCoord{X: 0, Y: 0, Z: 0}}
	tile := mapdata.NewTile(store, util.DFCoord{X: 0, Y: 0, Z: 0})
	tile.TileType = 1
	chunk.Tiles[0][0] = tile

	mesh := GenerateChunkMesh(chunk)
	main := mesh.SubMeshes[0]
	if main == nil {
		t.Fatal("expected the solid terrain submesh")
	}

	// An isolated floor must have its top and four exposed vertical sides.
	if got, want := len(main.Vertices), 20; got != want {
		t.Fatalf("isolated floor vertices = %d, want %d (top + four sides)", got, want)
	}
	if got, want := len(main.Indices), 30; got != want {
		t.Fatalf("isolated floor indices = %d, want %d", got, want)
	}
}
