package mapdata

import (
	"strings"
	"testing"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

func TestFindItemByRayReturnsNearestLooseItem(t *testing.T) {
	store := NewMapDataStore()
	store.Chunks[util.NewDFCoord(0, 0, 0)] = &Chunk{
		Origin: util.NewDFCoord(0, 0, 0),
		Items: []dfproto.Item{
			{ID: 10, Pos: dfproto.Coord{X: 2, Y: 3, Z: 0}},
			{ID: 11, Pos: dfproto.Coord{X: 8, Y: 3, Z: 0}},
		},
	}

	item, found := store.FindItemByRay(util.Ray{
		Origin:    util.Vector3{X: 2.5, Y: 6, Z: -3.5},
		Direction: util.Vector3{X: 0, Y: -1, Z: 0},
	}, 100)
	if !found {
		t.Fatal("expected the ray to hit an item")
	}
	if item.ID != 10 {
		t.Fatalf("hit item %d, want 10", item.ID)
	}
}

func TestFindItemByRayUsesBuildingItemFallbackPosition(t *testing.T) {
	store := NewMapDataStore()
	store.Chunks[util.NewDFCoord(0, 0, 0)] = &Chunk{
		Origin: util.NewDFCoord(0, 0, 0),
		Buildings: []dfproto.BuildingInstance{{
			PosXMin: 4, PosXMax: 6,
			PosYMin: 7, PosYMax: 9,
			PosZMin: 0,
			Items:   []dfproto.BuildingItem{{Item: &dfproto.Item{ID: 20}}},
		}},
	}

	item, found := store.FindItemByRay(util.Ray{
		Origin:    util.Vector3{X: 5.5, Y: 6, Z: -8.5},
		Direction: util.Vector3{X: 0, Y: -1, Z: 0},
	}, 100)
	if !found || item.ID != 20 {
		t.Fatalf("building item selection = (%d, %v), want (20, true)", item.ID, found)
	}
}

func TestItemDisplayNameIncludesMaterialAndType(t *testing.T) {
	store := NewMapDataStore()
	material := dfproto.MatPair{MatType: 2, MatIndex: 7}
	store.MatStore.Names[material] = "Granito"

	name := store.ItemDisplayName(dfproto.Item{
		Type:     dfproto.MatPair{MatType: 4},
		Material: material,
	})
	if !strings.Contains(name, "Granito") || !strings.Contains(name, "Pedra") {
		t.Fatalf("item name = %q, want material and type", name)
	}
}

func TestItemSupportRejectsVegetationAndAcceptsGround(t *testing.T) {
	store := NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeBranch}
	store.Tiletypes[2] = dfproto.Tiletype{ID: 2, Shape: dfproto.ShapeFloor}

	branch := NewTile(store, util.DFCoord{})
	branch.TileType = 1
	if itemSupportTile(branch) {
		t.Fatal("branch must not be accepted as item ground support")
	}

	floor := NewTile(store, util.DFCoord{})
	floor.TileType = 2
	if !itemSupportTile(floor) {
		t.Fatal("floor must be accepted as item ground support")
	}
}

func TestFindTileByRayReturnsVisibleBlock(t *testing.T) {
	store := NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeFloor, Material: dfproto.TilematStone}
	chunk := &Chunk{Origin: util.NewDFCoord(0, 0, 0)}
	tile := NewTile(store, util.NewDFCoord(2, 3, 0))
	tile.TileType = 1
	chunk.Tiles[2][3] = tile
	store.Chunks[chunk.Origin] = chunk

	_, coord, found := store.FindTileByRay(util.Ray{
		Origin:    util.Vector3{X: 2.5, Y: 6, Z: -3.5},
		Direction: util.Vector3{X: 0, Y: -1, Z: 0},
	}, 100)
	if !found || coord != (util.DFCoord{X: 2, Y: 3, Z: 0}) {
		t.Fatalf("tile selection = (%v, %v), want ((2, 3, 0), true)", coord, found)
	}
}

func TestFindTileByRayCanInspectEmptyCellWhenNoSolidTileIsHit(t *testing.T) {
	store := NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeEmpty}
	chunk := &Chunk{Origin: util.NewDFCoord(0, 0, 0)}
	tile := NewTile(store, util.NewDFCoord(2, 3, 0))
	tile.TileType = 1
	chunk.Tiles[2][3] = tile
	store.Chunks[chunk.Origin] = chunk

	_, coord, found := store.FindTileByRay(util.Ray{
		Origin:    util.Vector3{X: 2.5, Y: 6, Z: -3.5},
		Direction: util.Vector3{X: 0, Y: -1, Z: 0},
	}, 100)
	if !found || coord != (util.DFCoord{X: 2, Y: 3, Z: 0}) {
		t.Fatalf("empty tile selection = (%v, %v), want ((2, 3, 0), true)", coord, found)
	}
}

func TestTileDisplayNameIncludesMaterial(t *testing.T) {
	store := NewMapDataStore()
	store.Tiletypes[1] = dfproto.Tiletype{ID: 1, Shape: dfproto.ShapeFloor, Material: dfproto.TilematStone}
	material := dfproto.MatPair{MatType: 2, MatIndex: 7}
	store.MatStore.Names[material] = "Granito"
	tile := NewTile(store, util.NewDFCoord(0, 0, 0))
	tile.TileType = 1
	tile.Material = material

	name := store.TileDisplayName(tile)
	if !strings.Contains(name, "Granito") {
		t.Fatalf("tile name = %q, want material", name)
	}
}
