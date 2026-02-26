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
	Race int32

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
	if (u.Flags1&0x01) != 0 || (u.Flags1&0x02) != 0 {
		return false
	}
	return !u.IsDead
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
