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

func TestVegetationRenderingKeepsTreesAndShrubsButHidesLooseDetails(t *testing.T) {
	if !shouldRenderVegetationShape(dfproto.ShapeTrunkBranch) {
		t.Fatal("tree trunks should be enabled")
	}
	for _, shape := range []dfproto.TiletypeShape{dfproto.ShapeTreeShape, dfproto.ShapeSapling, dfproto.ShapeShrub} {
		if !shouldRenderVegetationShape(shape) {
			t.Fatalf("vegetation shape %v should be enabled", shape)
		}
	}
	for _, shape := range []dfproto.TiletypeShape{dfproto.ShapeBranch, dfproto.ShapeTwig} {
		if shouldRenderVegetationShape(shape) {
			t.Fatalf("vegetation shape %v should remain disabled", shape)
		}
	}
}

func TestIsolatedUndesignatedVoidGetsAVisualSupport(t *testing.T) {
	store := mapdata.NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeFloor}
	store.Tiletypes[2] = dfproto.Tiletype{ID: 2, Shape: dfproto.ShapeEmpty}
	chunk := &mapdata.Chunk{Origin: util.DFCoord{}}
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			tile := mapdata.NewTile(store, util.DFCoord{X: int32(x), Y: int32(y)})
			tile.TileType = 1
			chunk.Tiles[x][y] = tile
		}
	}
	center := chunk.Tiles[1][1]
	center.TileType = 2

	if !shouldPatchIsolatedVoid(chunk, 1, 1) {
		t.Fatal("isolated undesignated void should receive a visual support")
	}
	center.DigDesignation = dfproto.DigChannel
	if shouldPatchIsolatedVoid(chunk, 1, 1) {
		t.Fatal("designated channel must remain an open void")
	}
}

func TestBoulderAndPebblesIncludeFloorBase(t *testing.T) {
	store := mapdata.NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeBoulder, Material: dfproto.TilematStone}
	store.Tiletypes[2] = dfproto.Tiletype{ID: 2, Shape: dfproto.ShapePebbles, Material: dfproto.TilematStone}

	for _, tileType := range []int32{1, 2} {
		chunk := &mapdata.Chunk{Origin: util.DFCoord{}}
		tile := mapdata.NewTile(store, util.DFCoord{})
		tile.TileType = tileType
		chunk.Tiles[0][0] = tile

		mesh := GenerateChunkMesh(chunk)
		main := mesh.SubMeshes[0]
		if main == nil || len(main.Vertices) < 4 {
			t.Fatalf("shape %d did not produce a floor base", tileType)
		}
	}
}

func TestRampTopAndVegetationIncludeFloorBase(t *testing.T) {
	store := mapdata.NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeRampTop, Material: dfproto.TilematStone}
	store.Tiletypes[2] = dfproto.Tiletype{ID: 2, Shape: dfproto.ShapeTreeShape, Material: dfproto.TilematTreeMaterial}

	for _, tileType := range []int32{1, 2} {
		chunk := &mapdata.Chunk{Origin: util.DFCoord{}}
		tile := mapdata.NewTile(store, util.DFCoord{})
		tile.TileType = tileType
		chunk.Tiles[0][0] = tile

		mesh := GenerateChunkMesh(chunk)
		main := mesh.SubMeshes[0]
		if main == nil || len(main.Vertices) < 4 {
			t.Fatalf("shape %d did not produce a floor base", tileType)
		}
	}
}
