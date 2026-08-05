package mesher

import (
	"testing"

	"kaijuengine.com/matrix"
)

func TestBlendTileColorRevealsVeinWithoutReplacingBase(t *testing.T) {
	base := matrix.NewColor(0.8, 0.2, 0.1, 1)
	ore := matrix.NewColor(0.1, 0.8, 0.2, 1)
	mixed := blendTileColor(base, ore, 0.5)
	if mixed == base || mixed == ore {
		t.Fatalf("vein blend did not preserve both colors: %v", mixed)
	}
}

func TestMeshBoulderHasExternalVolume(t *testing.T) {
	mesh := NewMeshData()
	meshBoulder(mesh, 4, 7, matrix.ColorWhite())

	if len(mesh.Vertices) == 0 || len(mesh.Indices) == 0 {
		t.Fatal("expected boulder geometry")
	}
	if len(mesh.Indices)%3 != 0 {
		t.Fatalf("expected triangle indices, got %d", len(mesh.Indices))
	}

	minY, maxY := mesh.Vertices[0].Position.Y(), mesh.Vertices[0].Position.Y()
	for _, vertex := range mesh.Vertices[1:] {
		if vertex.Position.Y() < minY {
			minY = vertex.Position.Y()
		}
		if vertex.Position.Y() > maxY {
			maxY = vertex.Position.Y()
		}
	}
	if maxY-minY < 0.3 {
		t.Fatalf("boulder was flattened: vertical span %.3f", maxY-minY)
	}
}

func TestMeshPebblesHasSeveralVolumes(t *testing.T) {
	mesh := NewMeshData()
	meshPebbles(mesh, 0, 0, matrix.ColorWhite())

	if len(mesh.Vertices) < 3*8 {
		t.Fatalf("expected multiple pebble volumes, got %d vertices", len(mesh.Vertices))
	}
}

func TestMeshEndlessPitHasDepth(t *testing.T) {
	mesh := NewMeshData()
	meshEndlessPit(mesh, 0, 0, matrix.ColorWhite())

	if len(mesh.Indices) == 0 {
		t.Fatal("expected endless pit geometry")
	}
	minY, maxY := mesh.Vertices[0].Position.Y(), mesh.Vertices[0].Position.Y()
	for _, vertex := range mesh.Vertices[1:] {
		if vertex.Position.Y() < minY {
			minY = vertex.Position.Y()
		}
		if vertex.Position.Y() > maxY {
			maxY = vertex.Position.Y()
		}
	}
	if maxY-minY < 3.9 {
		t.Fatalf("endless pit was not deep enough: %.3f", maxY-minY)
	}
}
