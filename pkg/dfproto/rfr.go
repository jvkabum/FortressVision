// Package dfproto contém as structs protobuf manuais do RemoteFortressReader.
// Baseado em: dfhack-develop/plugins/remotefortressreader/proto/RemoteFortressReader.proto
package dfproto

import (
	"FortressVision/pkg/protowire"
)

// ---------- ENUMS essenciais ----------

// TiletypeShape - formato do tile
type TiletypeShape int32

const (
	ShapeNone          TiletypeShape = 0
	ShapeFloor         TiletypeShape = 1
	ShapeBoulder       TiletypeShape = 2
	ShapePebbles       TiletypeShape = 3
	ShapeWall          TiletypeShape = 4
	ShapeFortification TiletypeShape = 5
	ShapeStairUp       TiletypeShape = 6
	ShapeStairDown     TiletypeShape = 7
	ShapeStairUpDown   TiletypeShape = 8
	ShapeRamp          TiletypeShape = 9
	ShapeRampTop       TiletypeShape = 10
	ShapeBrookBed      TiletypeShape = 11
	ShapeBrookTop      TiletypeShape = 12
	ShapeTreeShape     TiletypeShape = 13
	ShapeSapling       TiletypeShape = 14
	ShapeShrub         TiletypeShape = 15
	ShapeEmpty         TiletypeShape = 16
	ShapeEndlessPit    TiletypeShape = 17
	ShapeBranch        TiletypeShape = 18
	ShapeTrunkBranch   TiletypeShape = 19
	ShapeTwig          TiletypeShape = 20
)

// TiletypeMaterial - material do tiletype
type TiletypeMaterial int32

const (
	TilematNone         TiletypeMaterial = 0
	TilematStone        TiletypeMaterial = 1
	TilematSoil         TiletypeMaterial = 2
	TilematGrass        TiletypeMaterial = 3 // grass_light, grass_dark, grass_dry, grass_dead
	TilematPlant        TiletypeMaterial = 4
	TilematTreeMaterial TiletypeMaterial = 5
	TilematLava         TiletypeMaterial = 6
	TilematMineral      TiletypeMaterial = 7
	TilematFrozenLiquid TiletypeMaterial = 8
	TilematConstruction TiletypeMaterial = 9
	TilematGrassDark    TiletypeMaterial = 10
	TilematGrassDry     TiletypeMaterial = 11
	TilematGrassDead    TiletypeMaterial = 12
	TilematHBM          TiletypeMaterial = 13 // hellstone, adamantine, etc
	TilematMagma        TiletypeMaterial = 14
	TilematDriftwood    TiletypeMaterial = 15
	TilematCampfire     TiletypeMaterial = 16
	TilematFire         TiletypeMaterial = 17
	TilematPool         TiletypeMaterial = 18
	TilematBrookShore   TiletypeMaterial = 19
	TilematRiverShore   TiletypeMaterial = 20
	TilematMushroom     TiletypeMaterial = 21
	TilematUnderworld   TiletypeMaterial = 22
	TilematFeatureStone TiletypeMaterial = 23
)

// TileDigDesignation
type TileDigDesignation int32

const (
	DigNone        TileDigDesignation = 0
	DigDefault     TileDigDesignation = 1
	DigUpDownStair TileDigDesignation = 2
	DigChannel     TileDigDesignation = 3
	DigRamp        TileDigDesignation = 4
	DigDownStair   TileDigDesignation = 5
	DigUpStair     TileDigDesignation = 6
)

// ---------- STRUCTS ----------

// MatPair representa um par (tipo_mat, indice_mat).
type MatPair struct {
	MatType  int32
	MatIndex int32
}

func (m *MatPair) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeVarintForce(1, int64(m.MatType))
	e.EncodeVarintForce(2, int64(m.MatIndex))
	return e.Bytes(), nil
}

func (m *MatPair) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.MatType = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.MatIndex = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// ColorDefinition - cor RGB float
type ColorDefinition struct {
	Red   int32
	Green int32
	Blue  int32
}

func (c *ColorDefinition) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			c.Red = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			c.Green = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			c.Blue = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// Tiletype - definição de um tiletype
type Tiletype struct {
	ID       int32
	Name     string
	Shape    TiletypeShape
	Material TiletypeMaterial
	Special  int32
	Variant  int32
	Dir      string
}

func (t *Tiletype) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			t.ID = int32(v)
		case 2:
			t.Name, err = d.ReadString()
			if err != nil {
				return err
			}
		case 3:
			_, err = d.ReadString() // caption (ignoramos mas consumimos)
			if err != nil {
				return err
			}
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			t.Shape = TiletypeShape(v)
		case 5:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			t.Special = int32(v)
		case 6:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			t.Material = TiletypeMaterial(v)
		case 7:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			t.Variant = int32(v)
		case 8:
			t.Dir, err = d.ReadString()
			if err != nil {
				return err
			}
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// TiletypeList - lista de tiletypes
type TiletypeList struct {
	TiletypeList []Tiletype
}

func (t *TiletypeList) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var tt Tiletype
			if err := tt.Unmarshal(subData); err != nil {
				return err
			}
			t.TiletypeList = append(t.TiletypeList, tt)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// MaterialDefinition - definição de material
type MaterialDefinition struct {
	MatPair    MatPair
	ID         string
	Name       string
	StateColor ColorDefinition
}

func (m *MaterialDefinition) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := m.MatPair.Unmarshal(subData); err != nil {
				return err
			}
		case 2:
			m.ID, err = d.ReadString()
			if err != nil {
				return err
			}
		case 3:
			m.Name, err = d.ReadString()
			if err != nil {
				return err
			}
		case 4:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := m.StateColor.Unmarshal(subData); err != nil {
				return err
			}
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// MaterialList - lista de materiais
type MaterialList struct {
	MaterialList []MaterialDefinition
}

func (m *MaterialList) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var mat MaterialDefinition
			if err := mat.Unmarshal(subData); err != nil {
				return err
			}
			m.MaterialList = append(m.MaterialList, mat)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// MapBlock - bloco 16x16x1 do mapa
type MapBlock struct {
	MapX               int32
	MapY               int32
	MapZ               int32
	Tiles              []int32
	Materials          []MatPair
	Water              []int32
	Magma              []int32
	Hidden             []bool
	TileDigDesignation []TileDigDesignation
	BaseMaterials      []MatPair
	LayerMaterials     []MatPair
	VeinMaterials      []MatPair
}

func (m *MapBlock) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.MapX = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.MapY = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.MapZ = int32(v)
		case 4:
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				m.Tiles = append(m.Tiles, vals...)
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.Tiles = append(m.Tiles, int32(v))
			}
		case 5:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var mp MatPair
			if err := mp.Unmarshal(subData); err != nil {
				return err
			}
			m.Materials = append(m.Materials, mp)
		case 6:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var mp MatPair
			if err := mp.Unmarshal(subData); err != nil {
				return err
			}
			m.LayerMaterials = append(m.LayerMaterials, mp)
		case 7:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var mp MatPair
			if err := mp.Unmarshal(subData); err != nil {
				return err
			}
			m.VeinMaterials = append(m.VeinMaterials, mp)
		case 8:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var mp MatPair
			if err := mp.Unmarshal(subData); err != nil {
				return err
			}
			m.BaseMaterials = append(m.BaseMaterials, mp)
		case 9:
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				m.Magma = append(m.Magma, vals...)
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.Magma = append(m.Magma, int32(v))
			}
		case 10:
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				m.Water = append(m.Water, vals...)
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.Water = append(m.Water, int32(v))
			}
		case 11:
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedBool()
				if err != nil {
					return err
				}
				m.Hidden = append(m.Hidden, vals...)
			} else {
				v, err := d.ReadBool()
				if err != nil {
					return err
				}
				m.Hidden = append(m.Hidden, v)
			}
		case 24:
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				for _, v := range vals {
					m.TileDigDesignation = append(m.TileDigDesignation, TileDigDesignation(v))
				}
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.TileDigDesignation = append(m.TileDigDesignation, TileDigDesignation(v))
			}
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// BlockList - lista de blocos retornada por GetBlockList
type BlockList struct {
	MapBlocks     []MapBlock
	MapX          int32
	MapY          int32
	EngravingInfo []int32 // ignoramos detalhes
}

func (b *BlockList) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1: // map_blocks
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var block MapBlock
			if err := block.Unmarshal(subData); err != nil {
				return err
			}
			b.MapBlocks = append(b.MapBlocks, block)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.MapX = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.MapY = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// BlockRequest - requisição de blocos
type BlockRequest struct {
	BlocksNeeded      int32
	MinX              int32
	MaxX              int32
	MinY              int32
	MaxY              int32
	MinZ              int32
	MaxZ              int32
	RequestTiles      bool
	RequestMaterials  bool
	RequestLiquid     bool
	RequestVegetation bool
}

func (r *BlockRequest) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeVarintForce(1, int64(r.BlocksNeeded))
	e.EncodeVarintForce(2, int64(r.MinX))
	e.EncodeVarintForce(3, int64(r.MaxX))
	e.EncodeVarintForce(4, int64(r.MinY))
	e.EncodeVarintForce(5, int64(r.MaxY))
	e.EncodeVarintForce(6, int64(r.MinZ))
	e.EncodeVarintForce(7, int64(r.MaxZ))
	if r.RequestTiles {
		e.EncodeBoolForce(8, true)
	}
	if r.RequestMaterials {
		e.EncodeBoolForce(9, true)
	}
	if r.RequestLiquid {
		e.EncodeBoolForce(11, true)
	}
	if r.RequestVegetation {
		e.EncodeBoolForce(18, true)
	}
	return e.Bytes(), nil
}

// MapInfo - informações do mapa
type MapInfo struct {
	BlockSizeX  int32
	BlockSizeY  int32
	BlockSizeZ  int32
	BlockPosX   int32
	BlockPosY   int32
	BlockPosZ   int32
	WorldName   string
	WorldNameEn string
	SaveName    string
}

func (m *MapInfo) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.BlockSizeX = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.BlockSizeY = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.BlockSizeZ = int32(v)
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.BlockPosX = int32(v)
		case 5:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.BlockPosY = int32(v)
		case 6:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.BlockPosZ = int32(v)
		case 7:
			m.WorldName, err = d.ReadString()
			if err != nil {
				return err
			}
		case 8:
			m.WorldNameEn, err = d.ReadString()
			if err != nil {
				return err
			}
		case 9:
			m.SaveName, err = d.ReadString()
			if err != nil {
				return err
			}
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// ViewInfo - informação da câmera do DF
type ViewInfo struct {
	ViewPosX     int32
	ViewPosY     int32
	ViewSizeX    int32
	ViewSizeY    int32
	CursorPosX   int32
	CursorPosY   int32
	CursorPosZ   int32
	ViewPosZ     int32
	FollowUnitID int32
	FollowItemID int32
}

func (v *ViewInfo) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.ViewPosX = int32(val)
		case 2:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.ViewPosY = int32(val)
		case 3:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.ViewPosZ = int32(val)
		case 4:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.ViewSizeX = int32(val)
		case 5:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.ViewSizeY = int32(val)
		case 6:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.CursorPosX = int32(val)
		case 7:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.CursorPosY = int32(val)
		case 8:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.CursorPosZ = int32(val)
		case 9:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.FollowUnitID = int32(val)
		case 10:
			val, err := d.ReadVarint()
			if err != nil {
				return err
			}
			v.FollowItemID = int32(val)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// UnitDefinition representa uma criatura/unidade no mundo.
type UnitDefinition struct {
	ID      int32
	IsValid bool
	PosX    int32
	PosY    int32
	PosZ    int32
}

func (u *UnitDefinition) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			u.ID = int32(v)
		case 2:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			u.IsValid = v
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			u.PosX = int32(v)
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			u.PosY = int32(v)
		case 5:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			u.PosZ = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// UnitList contém a lista de criaturas.
type UnitList struct {
	CreatureList []UnitDefinition
}

func (u *UnitList) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var unit UnitDefinition
			if err := unit.Unmarshal(subData); err != nil {
				return err
			}
			u.CreatureList = append(u.CreatureList, unit)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// SingleBool - mensagem com apenas um bool
type SingleBool struct {
	Value bool
}

func (s *SingleBool) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeBoolForce(1, s.Value)
	return e.Bytes(), nil
}

func (s *SingleBool) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			s.Value, err = d.ReadBool()
			if err != nil {
				return err
			}
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// DigCommand - comando de escavação
type DigCommand struct {
	Designation TileDigDesignation
	PosX        int32
	PosY        int32
	PosZ        int32
}

func (dc *DigCommand) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeVarintForce(1, int64(dc.Designation))
	e.EncodeVarintForce(2, int64(dc.PosX))
	e.EncodeVarintForce(3, int64(dc.PosY))
	e.EncodeVarintForce(4, int64(dc.PosZ))
	return e.Bytes(), nil
}
