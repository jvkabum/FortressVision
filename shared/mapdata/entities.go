package mapdata

import (
	"FortressVision/shared/pkg/dfproto"
	"FortressVision/shared/util"
)

// BuildingInstance representa uma instância de construção no mapa.
// Baseado na lógica do BuildingManager.cs do Armok Vision.
type BuildingInstance struct {
	Index        int32
	BuildingType *dfproto.BuildingDefinition
	Material     dfproto.MatPair

	// Limites espaciais (DFCoords)
	MinPos util.DFCoord
	MaxPos util.DFCoord

	Center util.DFCoord

	Direction dfproto.BuildingDirection

	// Itens contidos na construção
	Items []dfproto.BuildingItem
}

// UnitInstance representa uma unidade (criatura) no mapa.
// Baseado na lógica do CreatureManager.cs do Armok Vision.
type UnitInstance struct {
	ID   int32
	Name string
	Race dfproto.MatPair
	// Appearance básica recebida do UnitList do DFHack. Mantê-la no snapshot
	// permite diferenciar criaturas sem carregar uma malha individual por raça.
	ProfessionColor dfproto.ColorDefinition
	Appearance      dfproto.UnitAppearance
	Inventory       []dfproto.InventoryItem
	SizeCurrent     int32
	SizeBase        int32
	IsSoldier       bool

	Pos    util.DFCoord
	SubPos util.Vector3 // Posição detalhada (subtile)

	Flags1 uint32
	Flags2 uint32
	Flags3 uint32

	IsDead   bool
	IsHidden bool
}

// IsValid verifica se a unidade deve ser processada/renderizada.
// Tradução de CreatureManager.IsValidCreature.
func (u *UnitInstance) IsValid() bool {
	// Flags típicas de morte ou remoção (baseado em UnitFlags.cs)
	// dead (1), left (2), caged (8), forest (16)
	// Flags como caged/forest descrevem o estado da criatura, mas nÃ£o devem
	// removÃª-la da visualizaÃ§Ã£o 3D.
	return !u.IsDead && !u.IsHidden
}

// ItemInstance representa um item solto no mapa.
type ItemInstance struct {
	ID       int32
	Type     int32
	Subtype  int32
	Material dfproto.MatPair

	Pos   util.DFCoord
	Flags uint32
}
