package render

import (
	"testing"

	"FortressVision/cliente/internal/mesher"
	"FortressVision/shared/mapdata"
	"FortressVision/shared/pkg/dfproto"
	"kaijuengine.com/matrix"
	"kaijuengine.com/rendering"
)

func TestNextPowerOfTwo(t *testing.T) {
	for value, want := range map[int]int{0: 1, 1: 1, 2: 2, 3: 4, 7: 8, 8: 8, 9: 16} {
		if got := nextPowerOfTwo(value); got != want {
			t.Fatalf("nextPowerOfTwo(%d) = %d, want %d", value, got, want)
		}
	}
}

func TestPadMeshDataKeepsTrianglesAndAddsDegenerateSlots(t *testing.T) {
	data := &mesher.MeshData{
		Vertices: []rendering.Vertex{
			{Position: matrix.NewVec3(0, 0, 0)},
			{Position: matrix.NewVec3(1, 0, 0)},
			{Position: matrix.NewVec3(0, 1, 0)},
		},
		Indices: []uint32{0, 1, 2},
	}

	verts, indices := padMeshData(data, 2)
	if len(verts) != 6 || len(indices) != 6 {
		t.Fatalf("padded mesh lengths = %d/%d, want 6/6", len(verts), len(indices))
	}
	if indices[0] != 0 || indices[1] != 1 || indices[2] != 2 {
		t.Fatalf("original triangle indices changed: %v", indices[:3])
	}
	if indices[3] != indices[4] || indices[4] != indices[5] {
		t.Fatalf("unused slot is not degenerate: %v", indices[3:])
	}
	if verts[0].Position != data.Vertices[0].Position || verts[2].Position != data.Vertices[2].Position {
		t.Fatalf("original triangle vertices changed")
	}
}

func TestMakeDetailTextureIsOpaqueAndNonUniform(t *testing.T) {
	data := makeDetailTexture(8, 8)
	if len(data) != 8*8*4 {
		t.Fatalf("texture byte length = %d, want %d", len(data), 8*8*4)
	}
	first := data[0]
	var different bool
	for i := 0; i < len(data); i += 4 {
		if data[i+3] != 255 || data[i] != data[i+1] || data[i+1] != data[i+2] {
			t.Fatalf("pixel %d is not opaque grayscale RGBA", i/4)
		}
		if data[i] != first {
			different = true
		}
	}
	if !different {
		t.Fatalf("detail texture is uniform")
	}
}

func TestTileSurfaceYStartsAtVoxelTop(t *testing.T) {
	if got := tileSurfaceY(12, 0); got != 13 {
		t.Fatalf("tile surface y = %f, want 13", got)
	}
	if got := tileSurfaceY(-3, 0.02); got != -1.98 {
		t.Fatalf("tile surface offset y = %f, want -1.98", got)
	}
}

func TestItemMeshKindUsesRemoteItemState(t *testing.T) {
	if got := itemMeshKind(dfproto.Item{Projectile: true}); got != "cone" {
		t.Fatalf("projectile mesh = %q, want cone", got)
	}
	if got := itemMeshKind(dfproto.Item{Volume: 10, StackSize: 1}); got != "cylinder" {
		t.Fatalf("voluminous item mesh = %q, want cylinder", got)
	}
	if got := itemMeshKind(dfproto.Item{StackSize: 3}); got != "cube" {
		t.Fatalf("stacked item mesh = %q, want cube", got)
	}
	if got := itemMeshKind(dfproto.Item{}); got != "sphere" {
		t.Fatalf("default item mesh = %q, want sphere", got)
	}
}

func TestItemVisualScaleReflectsProjectileVelocity(t *testing.T) {
	base := itemVisualScale(dfproto.Item{}, 0.16, 0.14)
	projectile := itemVisualScale(dfproto.Item{
		Projectile: true,
		VelocityX:  2,
		VelocityY:  1,
		VelocityZ:  3,
	}, 0.16, 0.14)
	if projectile.X() <= base.X() || projectile.Y() <= base.Y() || projectile.Z() <= base.Z() {
		t.Fatalf("projectile scale did not reflect velocity: base=%v projectile=%v", base, projectile)
	}
	tooFast := itemVisualScale(dfproto.Item{Projectile: true, VelocityX: 100, VelocityY: 100, VelocityZ: 100}, 0.16, 0.14)
	if tooFast.X() > 0.4 || tooFast.Y() > 0.4 || tooFast.Z() > 0.4 {
		t.Fatalf("projectile scale exceeded safety bound: %v", tooFast)
	}
}

func TestFlowVisualSizeIsBoundedByDensity(t *testing.T) {
	small := flowVisualSize(dfproto.FlowInfo{Density: 0})
	large := flowVisualSize(dfproto.FlowInfo{Density: 1000})
	if small <= 0 || large <= small || large > 0.41 {
		t.Fatalf("unexpected flow sizes: small=%f large=%f", small, large)
	}
}

func TestWaveVisualScaleIsFlatAndBounded(t *testing.T) {
	scale := waveVisualScale(dfproto.Wave{
		Pos:  dfproto.Coord{X: 10, Y: 10, Z: 4},
		Dest: dfproto.Coord{X: 30, Y: 12, Z: 4},
	})
	if scale.Y() <= 0 || scale.Y() >= scale.X() || scale.Y() >= scale.Z() {
		t.Fatalf("wave marker is not flat: %v", scale)
	}
	if scale.X() > 0.45 || scale.Z() > 0.45 {
		t.Fatalf("wave marker exceeded visual bound: %v", scale)
	}
}

func TestArtImageElementColorUsesSemanticFallback(t *testing.T) {
	color := artImageElementColor(mapdata.ChunkSnapshot{}, dfproto.ArtImageElement{Type: dfproto.ImagePlant})
	if color.G() <= color.R() || color.G() <= color.B() {
		t.Fatalf("plant engraving marker was not green-biased: %v", color)
	}
}

func TestLerpUnitPositionMovesTowardTarget(t *testing.T) {
	current := unitPosition{x: 0, y: 2, z: -4}
	target := unitPosition{x: 10, y: 4, z: 6}
	got := lerpUnitPosition(current, target, 0.25)
	want := unitPosition{x: 2.5, y: 2.5, z: -1.5}
	if got != want {
		t.Fatalf("lerped unit position = %+v, want %+v", got, want)
	}
}

func TestLerpUnitPositionClampsAlpha(t *testing.T) {
	current := unitPosition{x: 1, y: 2, z: 3}
	target := unitPosition{x: 4, y: 5, z: 6}
	if got := lerpUnitPosition(current, target, -1); got != current {
		t.Fatalf("negative alpha changed position: %+v", got)
	}
	if got := lerpUnitPosition(current, target, 2); got != target {
		t.Fatalf("alpha above one did not reach target: %+v", got)
	}
}
