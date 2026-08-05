package mesher

import (
	"os"
	"path/filepath"
	"testing"

	"kaijuengine.com/matrix"
)

func TestLoadSimpleObjKeepsExternalWinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ramp.obj")
	obj := []byte("v 1 0 1\n" +
		"v -1 0 1\n" +
		"v -1 0 -1\n" +
		"f 1 2 3\n")
	if err := os.WriteFile(path, obj, 0o600); err != nil {
		t.Fatalf("write test OBJ: %v", err)
	}

	geom := loadSimpleObj(path)
	if geom == nil || len(geom.Indices) != 6 {
		t.Fatalf("expected the original triangle plus its external/back-face copy")
	}

	a := geom.Verts[geom.Indices[0]].Position
	b := geom.Verts[geom.Indices[1]].Position
	c := geom.Verts[geom.Indices[2]].Position
	abX, abZ := b.X()-a.X(), b.Z()-a.Z()
	acX, acZ := c.X()-a.X(), c.Z()-a.Z()
	if abZ*acX-abX*acZ >= 0 {
		t.Fatalf("ramp base winding was inverted; expected outward downward face")
	}

	backA := geom.Verts[geom.Indices[3]].Position
	backB := geom.Verts[geom.Indices[4]].Position
	backC := geom.Verts[geom.Indices[5]].Position
	backABX, backABZ := backB.X()-backA.X(), backB.Z()-backA.Z()
	backACX, backACZ := backC.X()-backA.X(), backC.Z()-backA.Z()
	if backABZ*backACX-backABX*backACZ <= 0 {
		t.Fatalf("ramp external copy was not wound in the opposite direction")
	}
}

func TestLoadSimpleObjRebuildsMissingRampShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ramp_without_sides.obj")
	obj := []byte("v -1 0 -1\n" +
		"v 1 0 -1\n" +
		"v 1 0 1\n" +
		"v -1 0 1\n" +
		"v -1 2 -1\n" +
		"v 1 2 -1\n" +
		"v 1 2 1\n" +
		"v -1 2 1\n" +
		"f 1 2 3 4\n" +
		"f 5 8 7 6\n")
	if err := os.WriteFile(path, obj, 0o600); err != nil {
		t.Fatalf("write test OBJ: %v", err)
	}

	geom := loadSimpleObj(path)
	if geom == nil {
		t.Fatal("expected ramp geometry")
	}

	for i := 0; i+2 < len(geom.Indices); i += 3 {
		a := geom.Verts[geom.Indices[i]].Position
		b := geom.Verts[geom.Indices[i+1]].Position
		c := geom.Verts[geom.Indices[i+2]].Position
		if a.X() == 0.5 && b.X() == 0.5 && c.X() == 0.5 {
			minY, maxY := a.Y(), a.Y()
			for _, p := range []matrix.Float{b.Y(), c.Y()} {
				if p < minY {
					minY = p
				}
				if p > maxY {
					maxY = p
				}
			}
			if maxY-minY > 0.9 {
				return
			}
		}
	}
	t.Fatal("expected the missing external side to be reconstructed")
}
