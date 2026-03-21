package treegen

import (
	"FortressVision/shared/util"
	"math"
	"sync"
)

// =============================================================================
// PARTE 1: GENÉTICA (L-System & Espécies)
// =============================================================================

type Module struct {
	Symbol string
	Params []float32
}

type LSystem struct {
	Axiom      []Module
	Rules      map[string]func(m Module) []Module
	Alpha      float32
	Length     float32
	Iterations int
}

type TurtleState struct {
	Pos    util.Vector3
	Dir    util.Vector3
	Up     util.Vector3
	Radius float32
}

func (ls *LSystem) Interpret(startPos util.Vector3, baseRadius float32) *MeshData {
	mesh := GetMeshData()
	turtle := TurtleState{
		Pos:    startPos,
		Dir:    util.Vector3{X: 0, Y: 1, Z: 0},
		Up:     util.Vector3{X: 1, Y: 0, Z: 0},
		Radius: baseRadius,
	}

	var stack []TurtleState
	modules := ls.Iterate(ls.Iterations)

	for _, m := range modules {
		switch m.Symbol {
		case "F":
			lengthFactor := float32(1.0)
			if len(m.Params) > 0 {
				lengthFactor = m.Params[0]
			}

			currentRadius := turtle.Radius
			endRadius := currentRadius * 0.82 // Afunilamento progressivo
			length := ls.Length * lengthFactor

			end := util.Vector3{
				X: turtle.Pos.X + turtle.Dir.X*length,
				Y: turtle.Pos.Y + turtle.Dir.Y*length,
				Z: turtle.Pos.Z + turtle.Dir.Z*length,
			}

			// Variar segmentos pelo raio (troncos mais grossos precisam de mais suavidade)
			numSegments := 6
			if turtle.Radius > 0.3 { numSegments = 10 }
			if turtle.Radius > 0.5 { numSegments = 14 }

			segSeed := int(turtle.Pos.X*73 + turtle.Pos.Z*31)
			seg := GenerateCurvedCylinder(turtle.Pos, end, currentRadius, endRadius, numSegments, 0.1, segSeed)
			mergeMesh(mesh, seg)
			PutMeshData(seg)

			turtle.Pos = end
			turtle.Radius = endRadius

		case "L":
			// Folhagem terminal proporcional ao último galho
			size := turtle.Radius * 8.0
			if size < 1.0 {
				size = 1.0
			} // Tamanho mínimo para visibilidade
			leaf := GenerateLeafCluster(turtle.Pos, LeafTypeCross, size)
			mergeMesh(mesh, leaf)
			PutMeshData(leaf)

		case "[":
			stack = append(stack, turtle)
		case "]":
			if len(stack) > 0 {
				turtle = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case "+":
			angle := ls.Alpha
			if len(m.Params) > 0 {
				angle = m.Params[0]
			}
			right := normalize(cross(turtle.Dir, turtle.Up))
			turtle.Dir = normalize(rotateVector(turtle.Dir, right, angle))
			turtle.Up = normalize(rotateVector(turtle.Up, right, angle))
		case "-":
			angle := ls.Alpha
			if len(m.Params) > 0 {
				angle = m.Params[0]
			}
			right := normalize(cross(turtle.Dir, turtle.Up))
			turtle.Dir = normalize(rotateVector(turtle.Dir, right, -angle))
			turtle.Up = normalize(rotateVector(turtle.Up, right, -angle))
		case "&":
			angle := ls.Alpha
			if len(m.Params) > 0 {
				angle = m.Params[0]
			}
			// Twist/Roll: rotaciona o vetor 'Up' em torno da própria direção 'Dir'
			turtle.Up = normalize(rotateVector(turtle.Up, turtle.Dir, angle))
		}
	}
	return mesh
}

func rotateVector(v, axis util.Vector3, angleDeg float32) util.Vector3 {
	rad := float64(angleDeg) * math.Pi / 180.0
	c, s := float32(math.Cos(rad)), float32(math.Sin(rad))
	return util.Vector3{
		X: v.X*c + (axis.Y*v.Z-axis.Z*v.Y)*s,
		Y: v.Y*c + (axis.Z*v.X-axis.X*v.Z)*s,
		Z: v.Z*c + (axis.X*v.Y-axis.Y*v.X)*s,
	}
}

func (ls *LSystem) Iterate(n int) []Module {
	current := ls.Axiom
	for i := 0; i < n; i++ {
		next := make([]Module, 0)
		for _, m := range current {
			if rule, ok := ls.Rules[m.Symbol]; ok {
				next = append(next, rule(m)...)
			} else {
				next = append(next, m)
			}
		}
		current = next
	}
	return current
}

func NewSpeciesLSystem(speciesID uint32, seed int) *LSystem {
	angleJitter := float32(math.Sin(float64(seed)*3.7)) * 8.0
	switch speciesID {
	case 42: // Tower-cap
		return &LSystem{
			Axiom: []Module{{Symbol: "M", Params: []float32{0}}},
			Rules: map[string]func(m Module) []Module{
				"M": func(m Module) []Module {
					return []Module{{Symbol: "F", Params: []float32{1.5}}, {Symbol: "D", Params: []float32{3.5}}}
				},
			},
			Alpha: 0, Length: 1.0, Iterations: 2,
		}
	default:
		return &LSystem{
			Axiom: []Module{{Symbol: "A", Params: []float32{0, float32(seed)}}},
			Rules: map[string]func(m Module) []Module{
				"A": func(m Module) []Module {
					t := m.Params[0]
					s := m.Params[1]
					if t > 4 {
						return []Module{{Symbol: "L"}}
					} // Folhagem terminal no lugar de cilindro

					// Fatores de decaimento controlados pela profundidade
					// radiusFactor não é mais enviado para F; F usa turtle.Radius do pai
					lengthFactor := float32(math.Pow(0.8, float64(t)))

					return []Module{
						{Symbol: "F", Params: []float32{lengthFactor}},
						{Symbol: "&", Params: []float32{GetGoldenRotation(int(t))}},
						{Symbol: "[", Params: nil},
						{Symbol: "+", Params: []float32{25.0 + angleJitter}},
						{Symbol: "A", Params: []float32{t + 1, s * 1.3}},
						{Symbol: "]", Params: nil},

						{Symbol: "&", Params: []float32{-GetGoldenRotation(int(t))}},
						{Symbol: "[", Params: nil},
						{Symbol: "-", Params: []float32{20.0 - angleJitter}},
						{Symbol: "A", Params: []float32{t + 1, s * 0.7}},
						{Symbol: "]", Params: nil},

						{Symbol: "A", Params: []float32{t + 1, s + 1}},
					}
				},
			},
			Alpha: 25.0, Length: 2.0, Iterations: 4,
		}
	}
}

// =============================================================================
// PARTE 2: GEOMETRIA (Malhas & Cilindros)
// =============================================================================

type MeshData struct {
	Vertices []float32
	Indices  []uint32
}

var meshDataPool = sync.Pool{
	New: func() interface{} {
		return &MeshData{Vertices: make([]float32, 0, 1024), Indices: make([]uint32, 0, 2048)}
	},
}

func GetMeshData() *MeshData { return meshDataPool.Get().(*MeshData) }
func PutMeshData(md *MeshData) {
	if md == nil {
		return
	}
	md.Vertices = md.Vertices[:0]
	md.Indices = md.Indices[:0]
	meshDataPool.Put(md)
}

func GenerateCurvedCylinder(start, end util.Vector3, startRad, endRad float32, segments int, curvature float32, seed int) *MeshData {
	steps := 5 // Mais passos para maior detalhe fractal
	prev := start
	prevRad := startRad
	mesh := GetMeshData()

	for s := 1; s <= steps; s++ {
		t := float32(s) / float32(steps)

		// Ruído Fractal: Soma de oitavas para irregularidades em várias escalas
		f1 := math.Sin(float64(seed)*0.13 + float64(s)*1.9)
		f2 := math.Sin(float64(seed)*0.47+float64(s)*3.7) * 0.5
		noise := float32(f1+f2) * curvature * t

		// Gravitropismo sutil: galhos pesam levemente para baixo ou curvam para cima
		gravy := float32(math.Sin(float64(seed)*0.05)) * 0.05 * t * t

		mid := util.Vector3{
			X: lerp(start.X, end.X, t) + noise,
			Y: lerp(start.Y, end.Y, t) + noise*0.2 - gravy,
			Z: lerp(start.Z, end.Z, t) + noise*0.5,
		}

		// Micro-variação de raio (casca irregular)
		radiusJitter := 1.0 + float32(math.Sin(float64(seed)*0.8+float64(s)*4.1))*0.03
		nextRad := lerp(startRad, endRad, t) * radiusJitter

		seg := GenerateCylinder(prev, mid, prevRad, nextRad, segments, false)
		mergeMesh(mesh, seg)
		PutMeshData(seg)

		prev = mid
		prevRad = nextRad
	}

	return mesh
}

func GenerateCylinder(start, end util.Vector3, startRad, endRad float32, segments int, smoothEnds bool) *MeshData {
	mesh := GetMeshData()
	dir := util.Vector3{X: end.X - start.X, Y: end.Y - start.Y, Z: end.Z - start.Z}
	length := float32(math.Sqrt(float64(dir.X*dir.X + dir.Y*dir.Y + dir.Z*dir.Z)))
	if length < 0.001 { return mesh }
	up := util.Vector3{X: 0, Y: 1, Z: 0}
	if math.Abs(float64(dir.Y/length)) > 0.9 { up = util.Vector3{X: 1, Y: 0, Z: 0} }
	right := normalize(cross(up, dir))
	fwd := normalize(cross(dir, right))

	for i := 0; i <= segments; i++ {
		angle := float32(2.0 * math.Pi * float64(i) / float64(segments))
		cos, sin := float32(math.Cos(float64(angle))), float32(math.Sin(float64(angle)))
		vS := util.Vector3{
			X: start.X + (right.X*cos+fwd.X*sin)*startRad,
			Y: start.Y + (right.Y*cos+fwd.Y*sin)*startRad,
			Z: start.Z + (right.Z*cos+fwd.Z*sin)*startRad,
		}
		vE := util.Vector3{
			X: end.X + (right.X*cos+fwd.X*sin)*endRad,
			Y: end.Y + (right.Y*cos+fwd.Y*sin)*endRad,
			Z: end.Z + (right.Z*cos+fwd.Z*sin)*endRad,
		}
		nx, ny, nz := (right.X*cos + fwd.X*sin), (right.Y*cos + fwd.Y*sin), (right.Z*cos + fwd.Z*sin)
		u := float32(i) / float32(segments)
		mesh.Vertices = append(mesh.Vertices, vS.X, vS.Y, vS.Z, nx, ny, nz, u, 0)
		mesh.Vertices = append(mesh.Vertices, vE.X, vE.Y, vE.Z, nx, ny, nz, u, 1)
	}
	for i := 0; i < segments; i++ {
		b0, b1 := uint32(i*2), uint32(i*2+1)
		b2, b3 := uint32((i+1)*2), uint32((i+1)*2+1)
		mesh.Indices = append(mesh.Indices, b0, b2, b1, b1, b2, b3)
	}
	return mesh
}

type LeafType int

const (
	LeafTypeCross LeafType = 0
	LeafTypeStar  LeafType = 1
	LeafTypeDisc  LeafType = 2
)

func GenerateLeafCluster(center util.Vector3, lType LeafType, size float32) *MeshData {
	mesh := GetMeshData()
	switch lType {
	case LeafTypeCross:
		addLeafPlane(mesh, center, size, 0)
		addLeafPlane(mesh, center, size, 60)
		addLeafPlane(mesh, center, size, 120)
	case LeafTypeStar:
		addLeafPlane(mesh, center, size, 0)
		addLeafPlane(mesh, center, size, 45)
		addLeafPlane(mesh, center, size, 90)
		addLeafPlane(mesh, center, size, 135)
	case LeafTypeDisc:
		addLeafDisc(mesh, center, size)
	}
	return mesh
}

func mergeMesh(dst, src *MeshData) {
	offset := uint32(len(dst.Vertices) / 8)
	dst.Vertices = append(dst.Vertices, src.Vertices...)
	for _, idx := range src.Indices {
		dst.Indices = append(dst.Indices, idx+offset)
	}
}

func lerp(a, b, t float32) float32          { return a + (b-a)*t }
func lerpUint8(a, b uint8, t float32) uint8 { return uint8(float32(a) + (float32(b)-float32(a))*t) }

// =============================================================================
// PARTE 3: ARTE (Cores, Biomas & Variações)
// =============================================================================

func GetTrunkColor(speciesID uint32, baseColor [4]uint8, height int, radius float32) [4]uint8 {
	color := baseColor
	if speciesID == 42 {
		return [4]uint8{180, 160, 140, 255}
	}
	if radius > 0.3 {
		moss := float32(math.Min(float64(radius-0.3)/0.5, 1.0)) * 0.25
		color[1] = uint8(math.Min(float64(color[1])+float64(moss*40), 255))
		color[0] = uint8(float32(color[0]) * (1.0 - moss*0.4))
	}
	grey := float32(math.Max(0, float64(height%20-10))) / 10.0
	if grey > 0 {
		color[0] = lerpUint8(color[0], 160, grey)
		color[1] = lerpUint8(color[1], 155, grey)
		color[2] = lerpUint8(color[2], 150, grey)
	}
	return color
}

func GetLeafArt(speciesID uint32, baseColor [4]uint8, dist float32) ([4]uint8, string) {
	color := baseColor
	tex := "leaf_oak"
	if speciesID == 42 {
		return [4]uint8{100, 200, 255, 255}, "leaf_mushroom"
	}
	dF := 0.8 + 0.2*float32(math.Min(float64(dist/5.0), 1.0))
	color[0] = uint8(float32(color[0]) * dF)
	color[1] = uint8(float32(color[1]) * dF)
	color[2] = uint8(float32(color[2]) * dF)
	return color, tex
}

func cross(a, b util.Vector3) util.Vector3 {
	return util.Vector3{X: a.Y*b.Z - a.Z*b.Y, Y: a.Z*b.X - a.X*b.Z, Z: a.X*b.Y - a.Y*b.X}
}
func normalize(a util.Vector3) util.Vector3 {
	l := float32(math.Sqrt(float64(a.X*a.X + a.Y*a.Y + a.Z*a.Z)))
	if l < 0.0001 {
		return a
	}
	return util.Vector3{X: a.X / l, Y: a.Y / l, Z: a.Z / l}
}

func addLeafPlane(mesh *MeshData, center util.Vector3, size float32, rotDeg float32) {
	rad := rotDeg * math.Pi / 180.0
	cos, sin := float32(math.Cos(float64(rad))), float32(math.Sin(float64(rad)))
	h, half := size*0.8, size/2.0
	startIdx := uint32(len(mesh.Vertices) / 8)
	pts := []util.Vector3{{X: -half, Y: 0}, {X: half, Y: 0}, {X: half, Y: h}, {X: -half, Y: h}}
	for i, p := range pts {
		u, v := float32(0), float32(0)
		if i == 1 || i == 2 {
			u = 1
		}
		if i == 2 || i == 3 {
			v = 1
		}
		mesh.Vertices = append(mesh.Vertices, center.X+p.X*cos, center.Y+p.Y, center.Z+p.X*sin, 0, 1, 0, u, v)
	}
	mesh.Indices = append(mesh.Indices, startIdx+0, startIdx+2, startIdx+1, startIdx+0, startIdx+3, startIdx+2)
}

func addLeafDisc(mesh *MeshData, center util.Vector3, radius float32) {
	segments := 12
	startIdx := uint32(len(mesh.Vertices) / 8)
	mesh.Vertices = append(mesh.Vertices, center.X, center.Y, center.Z, 0, 1, 0, 0.5, 0.5)
	for i := 0; i <= segments; i++ {
		a := 2 * math.Pi * float64(i) / float64(segments)
		cos, sin := float32(math.Cos(a)), float32(math.Sin(a))
		mesh.Vertices = append(mesh.Vertices, center.X+radius*cos, center.Y, center.Z+radius*sin, 0, 1, 0, 0.5+0.5*cos, 0.5+0.5*sin)
	}
	for i := uint32(0); i < uint32(segments); i++ {
		mesh.Indices = append(mesh.Indices, startIdx, startIdx+1+i, startIdx+1+((i+1)%uint32(segments)))
	}
}

const GoldenAngle = 137.5 * math.Pi / 180.0

func GetGoldenRotation(n int) float32 {
	return float32(math.Mod(float64(n)*137.5, 360.0))
}
