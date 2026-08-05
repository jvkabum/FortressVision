package mapdata

import (
	"fmt"
	"math"
	"strings"

	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

// itemTypeNames acompanha os tipos de item usados pelo Dwarf Fortress. O
// RemoteFortressReader envia o tipo no campo MatType do MatPair Type; o
// MatIndex permanece disponível no texto quando for um subtipo específico.
var itemTypeNames = map[int32]string{
	0:  "Barra",
	1:  "Gema lapidada",
	2:  "Bloco",
	3:  "Gema bruta",
	4:  "Pedra",
	5:  "Madeira",
	6:  "Porta",
	7:  "Comporta",
	8:  "Cama",
	9:  "Cadeira",
	10: "Corrente",
	11: "Frasco",
	12: "Cálice",
	13: "Instrumento musical",
	14: "Brinquedo",
	15: "Janela",
	16: "Gaiola",
	17: "Barril",
	18: "Balde",
	19: "Armadilha de animais",
	20: "Mesa",
	21: "Caixão",
	22: "Estátua",
	23: "Cadáver",
	24: "Arma",
	25: "Armadura",
	26: "Calçado",
	27: "Escudo",
	28: "Capacete",
	29: "Luvas",
	30: "Caixa",
	31: "Saco",
	32: "Lixeira",
	33: "Suporte de armadura",
	34: "Suporte de armas",
	35: "Armário",
	36: "Estatueta",
	37: "Amuleto",
	38: "Cetro",
	39: "Munição",
	40: "Coroa",
	41: "Anel",
	42: "Brinco",
	43: "Bracelete",
	44: "Gema",
	45: "Bigorna",
	46: "Parte de cadáver",
	47: "Restos",
	48: "Carne",
	49: "Peixe",
	50: "Peixe cru",
	51: "Vermin",
	52: "Animal doméstico",
	53: "Semente",
	54: "Planta",
	55: "Couro curtido",
	56: "Crescimento de planta",
	57: "Fio",
	58: "Tecido",
	59: "Totem",
	60: "Calça",
	61: "Mochila",
	62: "Aljava",
	63: "Peça de catapulta",
	64: "Peça de balista",
	65: "Munição de cerco",
	66: "Ponta de flecha de balista",
	67: "Mecanismo",
	68: "Componente de armadilha",
	69: "Bebida",
	70: "Pó",
	71: "Queijo",
	72: "Comida",
	73: "Líquido",
	74: "Moeda",
	75: "Massa",
	76: "Pedrinha",
	77: "Seção de tubo",
	78: "Tampa de alçapão",
	79: "Grade",
	80: "Mó manual",
	81: "Moinho",
	82: "Tala",
	83: "Muleta",
	84: "Banco de tração",
	85: "Gesso ortopédico",
	86: "Ferramenta",
	87: "Laje",
	88: "Ovo",
	89: "Livro",
	90: "Folha",
	91: "Galho",
	92: "Peça de lançador de virotes",
}

// FindItemByRay retorna o item visual mais próximo atingido pelo raio. A
// consulta usa caixas pequenas ao redor da posição renderizada, portanto não
// depende de MapSize nem do tile ter uma colisão perfeita para ser clicável.
// Isso é importante no modo de streaming, em que MapSize pode ainda estar 0.
func (s *MapDataStore) FindItemByRay(ray util.Ray, maxDistance float32) (dfproto.Item, bool) {
	if maxDistance <= 0 {
		maxDistance = 2000
	}

	s.Mu.RLock()
	defer s.Mu.RUnlock()

	bestDistance := maxDistance
	var selected dfproto.Item
	found := false
	visit := func(item dfproto.Item, fallback *util.DFCoord) {
		pos := itemWorldPosition(item, fallback)
		if pos == (util.DFCoord{}) && item.Pos != (dfproto.Coord{}) {
			pos = util.DFCoord{X: item.Pos.X, Y: item.Pos.Y, Z: item.Pos.Z}
		}
		distance, ok := itemRayDistance(ray, item, pos, bestDistance)
		if ok && (!found || distance < bestDistance) {
			bestDistance = distance
			selected = item
			found = true
		}
	}

	for _, chunk := range s.Chunks {
		if chunk == nil {
			continue
		}
		for _, item := range chunk.Items {
			visit(item, nil)
		}
		for _, building := range chunk.Buildings {
			fallback := util.DFCoord{
				X: (building.PosXMin + building.PosXMax) / 2,
				Y: (building.PosYMin + building.PosYMax) / 2,
				Z: building.PosZMin,
			}
			for _, buildingItem := range building.Items {
				if buildingItem.Item != nil {
					visit(*buildingItem.Item, &fallback)
				}
			}
		}
	}

	return selected, found
}

func itemWorldPosition(item dfproto.Item, fallback *util.DFCoord) util.DFCoord {
	if item.Pos != (dfproto.Coord{}) {
		return util.DFCoord{X: item.Pos.X, Y: item.Pos.Y, Z: item.Pos.Z}
	}
	if fallback != nil {
		return *fallback
	}
	return util.DFCoord{}
}

func itemRayDistance(ray util.Ray, item dfproto.Item, pos util.DFCoord, maxDistance float32) (float32, bool) {
	corner := util.DFToWorldPos(pos)
	center := util.Vector3{
		X: corner.X + 0.5 + item.SubposX,
		Y: corner.Y + 1.0 + 0.20 + item.SubposZ,
		Z: corner.Z - 0.5 - item.SubposY,
	}

	// O desenho real tem entre 0.16 e 0.2 de largura. A caixa um pouco mais
	// generosa deixa o item selecionável mesmo quando a câmera está distante.
	halfSize := float32(0.28)
	halfHeight := float32(0.42)
	if item.Projectile {
		halfSize = 0.34
		halfHeight = 0.48
	}
	minimum := util.Vector3{X: center.X - halfSize, Y: center.Y - halfHeight, Z: center.Z - halfSize}
	maximum := util.Vector3{X: center.X + halfSize, Y: center.Y + halfHeight, Z: center.Z + halfSize}

	return rayAABBDistance(ray, minimum, maximum, maxDistance)
}

// FindTileByRay retorna o bloco visível mais próximo atingido pelo raio.
// Diferentemente do Raycast de navegação, esta consulta percorre somente os
// chunks já carregados e não depende de MapSize estar preenchido.
func (s *MapDataStore) FindTileByRay(ray util.Ray, maxDistance float32) (*Tile, util.DFCoord, bool) {
	if maxDistance <= 0 {
		maxDistance = 2000
	}

	s.Mu.RLock()
	defer s.Mu.RUnlock()

	bestDistance := maxDistance
	var selected *Tile
	var selectedCoord util.DFCoord
	for origin, chunk := range s.Chunks {
		if chunk == nil {
			continue
		}
		for localX := 0; localX < util.BlockSize; localX++ {
			for localY := 0; localY < util.BlockSize; localY++ {
				tile := chunk.Tiles[localX][localY]
				if tile == nil || tile.Hidden {
					continue
				}
				shape := tile.Shape()
				if shape == dfproto.ShapeNoShape || shape == dfproto.ShapeEmpty {
					continue
				}

				coord := util.DFCoord{
					X: origin.X + int32(localX),
					Y: origin.Y + int32(localY),
					Z: origin.Z,
				}
				corner := util.DFToWorldPos(coord)
				minimum := util.Vector3{
					X: corner.X,
					Y: corner.Y,
					Z: corner.Z - util.GameScale,
				}
				maximum := util.Vector3{
					X: corner.X + util.GameScale,
					Y: corner.Y + util.GameScale,
					Z: corner.Z,
				}
				distance, ok := rayAABBDistance(ray, minimum, maximum, bestDistance)
				if ok && (selected == nil || distance < bestDistance) {
					bestDistance = distance
					selected = tile
					selectedCoord = coord
				}
			}
		}
	}
	return selected, selectedCoord, selected != nil
}

func rayAABBDistance(ray util.Ray, minimum, maximum util.Vector3, maxDistance float32) (float32, bool) {
	near := float32(0)
	far := maxDistance
	for _, axis := range [3]struct {
		origin    float32
		direction float32
		minimum   float32
		maximum   float32
	}{
		{ray.Origin.X, ray.Direction.X, minimum.X, maximum.X},
		{ray.Origin.Y, ray.Direction.Y, minimum.Y, maximum.Y},
		{ray.Origin.Z, ray.Direction.Z, minimum.Z, maximum.Z},
	} {
		if math.Abs(float64(axis.direction)) < 0.000001 {
			if axis.origin < axis.minimum || axis.origin > axis.maximum {
				return 0, false
			}
			continue
		}
		first := (axis.minimum - axis.origin) / axis.direction
		last := (axis.maximum - axis.origin) / axis.direction
		if first > last {
			first, last = last, first
		}
		if first > near {
			near = first
		}
		if last < far {
			far = last
		}
		if near > far {
			return 0, false
		}
	}
	return near, far >= 0
}

// ItemDisplayName monta um nome curto para o painel de inspeção. O material
// vem do cache de materiais do DFHack; o tipo vem do enum enviado pelo RFR.
func (s *MapDataStore) ItemDisplayName(item dfproto.Item) string {
	typeName, ok := itemTypeNames[item.Type.MatType]
	if !ok {
		typeName = fmt.Sprintf("Item tipo %d", item.Type.MatType)
	}

	name := typeName
	if s != nil && s.MatStore != nil && item.Material != (dfproto.MatPair{}) {
		material := s.MatStore.GetMaterialName(item.Material)
		if material != "" && !strings.HasPrefix(material, "Desconhecido (") {
			name = fmt.Sprintf("%s %s", material, typeName)
		}
	}
	if item.Type.MatIndex != 0 {
		name = fmt.Sprintf("%s #%d", name, item.Type.MatIndex)
	}
	return name
}

// TileDisplayName monta uma identificação curta para um bloco selecionado.
func (s *MapDataStore) TileDisplayName(tile *Tile) string {
	if tile == nil {
		return "Bloco"
	}
	main, category := tile.GetCategorization()
	name := fmt.Sprintf("%s — %s", main, category)

	material := tile.Material
	if material == (dfproto.MatPair{}) {
		material = tile.BaseMaterial
	}
	if material == (dfproto.MatPair{}) {
		material = tile.LayerMaterial
	}
	if s != nil && s.MatStore != nil && material != (dfproto.MatPair{}) {
		materialName := s.MatStore.GetMaterialName(material)
		if materialName != "" && !strings.HasPrefix(materialName, "Desconhecido (") {
			name = fmt.Sprintf("%s — %s", materialName, category)
		}
	}
	return name
}
