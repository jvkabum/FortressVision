package render

import (
	"testing"

	"FortressVision/cliente/internal/mesher"
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

func TestFlowVisualSizeIsBoundedByDensity(t *testing.T) {
	small := flowVisualSize(dfproto.FlowInfo{Density: 0})
	large := flowVisualSize(dfproto.FlowInfo{Density: 1000})
	if small <= 0 || large <= small || large > 0.41 {
		t.Fatalf("unexpected flow sizes: small=%f large=%f", small, large)
	}
}
