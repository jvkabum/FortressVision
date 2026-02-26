// Package dfproto contém as structs protobuf manuais do RemoteFortressReader.
// Baseado em: dfhack-develop/plugins/remotefortressreader/proto/RemoteFortressReader.proto
package dfproto

import (
	"FortressVision/shared/pkg/protowire"
	"math"
)

// ---------- ENUMS essenciais ----------

// TiletypeShape - formato do tile
type TiletypeShape int32

const (
	ShapeNoShape       TiletypeShape = 0
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
	TilematNoMaterial   TiletypeMaterial = 0
	TilematStone        TiletypeMaterial = 1
	TilematSoil         TiletypeMaterial = 2
	TilematGrassLight   TiletypeMaterial = 3 // grass_light, grass_dark, grass_dry, grass_dead
	TilematPlant        TiletypeMaterial = 4
	TilematTreeMaterial TiletypeMaterial = 5
	TilematLavaStone    TiletypeMaterial = 6
	TilematMineral      TiletypeMaterial = 7
	TilematFrozenLiquid TiletypeMaterial = 8
	TilematConstruction TiletypeMaterial = 9
	TilematGrassDark    TiletypeMaterial = 10
	TilematGrassDry     TiletypeMaterial = 11
	TilematGrassDead    TiletypeMaterial = 12
	TilematHFS          TiletypeMaterial = 13 // hellstone, adamantine, etc
	TilematMagma        TiletypeMaterial = 14
	TilematDriftwood    TiletypeMaterial = 15
	TilematCampfire     TiletypeMaterial = 16
	TilematFire         TiletypeMaterial = 17
	TilematPool         TiletypeMaterial = 18
	TilematBrookShore   TiletypeMaterial = 19
	TilematRiverShore   TiletypeMaterial = 20
	TilematMushroom     TiletypeMaterial = 21
	TilematUnderworld   TiletypeMaterial = 22
	TilematFeature      TiletypeMaterial = 23
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

// MatterState - estado da matéria para spatters
type MatterState int32

const (
	MatterSolid   MatterState = 0
	MatterLiquid  MatterState = 1
	MatterGas     MatterState = 2
	MatterPowder  MatterState = 3
	MatterPaste   MatterState = 4
	MatterPressed MatterState = 5
)

// ---------- STRUCTS ----------

type Coord struct {
	X int32
	Y int32
	Z int32
}

func (c *Coord) Unmarshal(data []byte) error {
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
			c.X = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			c.Y = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			c.Z = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

type FlowType int32

const (
	FlowMiasma        FlowType = 0
	FlowSteam         FlowType = 1
	FlowMist          FlowType = 2
	FlowMaterialDust  FlowType = 3
	FlowMagmaMist     FlowType = 4
	FlowSmoke         FlowType = 5
	FlowDragonfire    FlowType = 6
	FlowFire          FlowType = 7
	FlowWeb           FlowType = 8
	FlowMaterialGas   FlowType = 9
	FlowMaterialVapor FlowType = 10
	FlowOceanWave     FlowType = 11
	FlowSeaFoam       FlowType = 12
	FlowItemCloud     FlowType = 13
)

// Spatter representa uma mancha ou sujeira no mapa.
type Spatter struct {
	Material MatPair
	Amount   int32
	State    MatterState
	Item     MatPair
}

func (s *Spatter) Unmarshal(data []byte) error {
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
			if err := s.Material.Unmarshal(subData); err != nil {
				return err
			}
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			s.Amount = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			s.State = MatterState(v)
		case 4:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := s.Item.Unmarshal(subData); err != nil {
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

// SpatterPile é uma coleção de spatters em um único tile.
type SpatterPile struct {
	Spatters []Spatter
}

func (s *SpatterPile) Unmarshal(data []byte) error {
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
			var sp Spatter
			if err := sp.Unmarshal(subData); err != nil {
				return err
			}
			s.Spatters = append(s.Spatters, sp)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// Engraving representa uma gravura ou entalhe em um tile.
type Engraving struct {
	Pos       Coord
	ItemIndex int32
	ArtID     int32
	TileType  int32
	Quality   int32
}

func (e *Engraving) Unmarshal(data []byte) error {
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
			if err := e.Pos.Unmarshal(subData); err != nil {
				return err
			}
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			e.ItemIndex = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			e.ArtID = int32(v)
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			e.TileType = int32(v)
		case 5:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			e.Quality = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

type FlowInfo struct {
	Index     int32
	Type      FlowType
	Density   int32
	Pos       Coord
	Dest      Coord
	Expanding bool
	Fast      bool
	Creeping  bool
}

func (f *FlowInfo) Unmarshal(data []byte) error {
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
			f.Index = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			f.Type = FlowType(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			f.Density = int32(v)
		case 4:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := f.Pos.Unmarshal(subData); err != nil {
				return err
			}
		case 5:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := f.Dest.Unmarshal(subData); err != nil {
				return err
			}
		case 6:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			f.Expanding = v
		case 12:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			f.Fast = v
		case 13:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			f.Creeping = v
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

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

func (c *ColorDefinition) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeVarintForce(1, int64(c.Red))
	e.EncodeVarintForce(2, int64(c.Green))
	e.EncodeVarintForce(3, int64(c.Blue))
	return e.Bytes(), nil
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

func (t *Tiletype) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeVarintForce(1, int64(t.ID))
	e.EncodeStringForce(2, t.Name)
	e.EncodeString(3, t.Name) // caption
	e.EncodeVarintForce(4, int64(t.Shape))
	e.EncodeVarintForce(5, int64(t.Special))
	e.EncodeVarintForce(6, int64(t.Material))
	e.EncodeVarintForce(7, int64(t.Variant))
	e.EncodeString(8, t.Dir)
	return e.Bytes(), nil
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

func (t *TiletypeList) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	for _, tt := range t.TiletypeList {
		data, err := tt.Marshal()
		if err != nil {
			return nil, err
		}
		e.EncodeBytes(1, data)
	}
	return e.Bytes(), nil
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

func (m *MaterialDefinition) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	data, err := m.MatPair.Marshal()
	if err == nil {
		e.EncodeBytes(1, data)
	}
	e.EncodeStringForce(2, m.ID)
	e.EncodeStringForce(3, m.Name)
	data, err = m.StateColor.Marshal()
	if err == nil {
		e.EncodeBytes(4, data)
	}
	return e.Bytes(), nil
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

func (m *MaterialList) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	for _, md := range m.MaterialList {
		data, err := md.Marshal()
		if err != nil {
			return nil, err
		}
		e.EncodeBytes(1, data)
	}
	return e.Bytes(), nil
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
			var md MaterialDefinition
			if err := md.Unmarshal(subData); err != nil {
				return err
			}
			m.MaterialList = append(m.MaterialList, md)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// PlantDetail representa uma planta individual (shrub, sapling) no mapa
type PlantDetail struct {
	Pos      Coord
	Material MatPair
}

func (p *PlantDetail) Unmarshal(data []byte) error {
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
			if err := p.Pos.Unmarshal(subData); err != nil {
				return err
			}
		case 2:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := p.Material.Unmarshal(subData); err != nil {
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

type BuildingType struct {
	BuildingType    int32
	BuildingSubtype int32
	BuildingCustom  int32
}

func (b *BuildingType) Unmarshal(data []byte) error {
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
			b.BuildingType = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.BuildingSubtype = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.BuildingCustom = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

type BuildingItem struct {
	Item *Item
	Mode int32
}

func (b *BuildingItem) Unmarshal(data []byte) error {
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
			b.Item = &Item{}
			if err := b.Item.Unmarshal(subData); err != nil {
				return err
			}
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.Mode = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

type BuildingInstance struct {
	Index         int32
	PosXMin       int32
	PosYMin       int32
	PosZMin       int32
	PosXMax       int32
	PosYMax       int32
	PosZMax       int32
	BuildingType  BuildingType
	Material      MatPair
	BuildingFlags uint32
	IsRoom        bool
	Direction     BuildingDirection // BuildingDirection
	Items         []BuildingItem
	Active        int32
}

func (b *BuildingInstance) Unmarshal(data []byte) error {
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
			b.Index = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.PosXMin = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.PosYMin = int32(v)
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.PosZMin = int32(v)
		case 5:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.PosXMax = int32(v)
		case 6:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.PosYMax = int32(v)
		case 7:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.PosZMax = int32(v)
		case 8:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := b.BuildingType.Unmarshal(subData); err != nil {
				return err
			}
		case 9:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := b.Material.Unmarshal(subData); err != nil {
				return err
			}
		case 10:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.BuildingFlags = uint32(v)
		case 11:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			b.IsRoom = v
		case 13:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.Direction = BuildingDirection(v)
		case 14:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var item BuildingItem
			if err := item.Unmarshal(subData); err != nil {
				return err
			}
			b.Items = append(b.Items, item)
		case 15:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			b.Active = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

type Item struct {
	ID       int32
	Pos      Coord
	Flags1   uint32
	Flags2   uint32
	Type     MatPair
	Material MatPair
}

func (i *Item) Unmarshal(data []byte) error {
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
			i.ID = int32(v)
		case 2:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := i.Pos.Unmarshal(subData); err != nil {
				return err
			}
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			i.Flags1 = uint32(v)
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			i.Flags2 = uint32(v)
		case 5:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := i.Type.Unmarshal(subData); err != nil {
				return err
			}
		case 6:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			if err := i.Material.Unmarshal(subData); err != nil {
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

type MapBlock struct {
	MapX                 int32
	MapY                 int32
	MapZ                 int32
	Tiles                []int32
	Materials            []MatPair
	Water                []int32
	Magma                []int32
	Hidden               []bool
	Light                []bool
	Subterranean         []bool
	Outside              []bool
	Aquifer              []bool
	WaterStagnant        []bool
	WaterSalt            []bool
	ConstructionItems    []MatPair          // ID 18
	Buildings            []BuildingInstance // ID 19
	TileDigDesignation   []TileDigDesignation
	BaseMaterials        []MatPair
	LayerMaterials       []MatPair
	VeinMaterials        []MatPair
	Flows                []FlowInfo
	TreePercent          []int32
	TreeX                []int32
	TreeY                []int32
	TreeZ                []int32
	Items                []Item  // ID 26
	DigDesignationMarker []bool  // ID 27
	DigDesignationAuto   []bool  // ID 28
	GrassPercent         []int32 // ID 29
	Plants               []PlantDetail
	SpatterPile          []SpatterPile // ID 25
	Engravings           []Engraving   // ID 31
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
				for _, v := range vals {
					m.Tiles = append(m.Tiles, int32(v))
				}
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
				for _, v := range vals {
					m.Magma = append(m.Magma, int32(v))
				}
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
				for _, v := range vals {
					m.Water = append(m.Water, int32(v))
				}
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.Water = append(m.Water, int32(v))
			}
		case 11:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.Hidden = append(m.Hidden, v)
		case 12:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.Light = append(m.Light, v)
		case 13:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.Subterranean = append(m.Subterranean, v)
		case 14:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.Outside = append(m.Outside, v)
		case 15:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.Aquifer = append(m.Aquifer, v)
		case 16:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.WaterStagnant = append(m.WaterStagnant, v)
		case 17:
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.WaterSalt = append(m.WaterSalt, v)
		case 18: // construction_items
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var mp MatPair
			if err := mp.Unmarshal(subData); err != nil {
				return err
			}
			m.ConstructionItems = append(m.ConstructionItems, mp)
		case 19: // buildings
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var bi BuildingInstance
			if err := bi.Unmarshal(subData); err != nil {
				return err
			}
			m.Buildings = append(m.Buildings, bi)
		case 20: // tree_percent
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				for _, v := range vals {
					m.TreePercent = append(m.TreePercent, int32(v))
				}
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.TreePercent = append(m.TreePercent, int32(v))
			}
		case 21: // tree_x
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				for _, v := range vals {
					m.TreeX = append(m.TreeX, int32(v))
				}
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.TreeX = append(m.TreeX, int32(v))
			}
		case 22: // tree_y
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				for _, v := range vals {
					m.TreeY = append(m.TreeY, int32(v))
				}
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.TreeY = append(m.TreeY, int32(v))
			}
		case 23: // tree_z
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				for _, v := range vals {
					m.TreeZ = append(m.TreeZ, int32(v))
				}
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.TreeZ = append(m.TreeZ, int32(v))
			}
		case 24: // tile_dig_designation
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
		case 25: // spatter_pile
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var sp SpatterPile
			if err := sp.Unmarshal(subData); err != nil {
				return err
			}
			m.SpatterPile = append(m.SpatterPile, sp)
		case 26: // items
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var bit Item
			if err := bit.Unmarshal(subData); err != nil {
				return err
			}
			m.Items = append(m.Items, bit)
		case 27: // tile_dig_designation_marker
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.DigDesignationMarker = append(m.DigDesignationMarker, v)
		case 28: // tile_dig_designation_auto
			v, err := d.ReadBool()
			if err != nil {
				return err
			}
			m.DigDesignationAuto = append(m.DigDesignationAuto, v)
		case 29: // grass_percent
			if wireType == protowire.WireLengthDelimited {
				vals, err := d.ReadPackedVarint()
				if err != nil {
					return err
				}
				for _, v := range vals {
					m.GrassPercent = append(m.GrassPercent, int32(v))
				}
			} else {
				v, err := d.ReadVarint()
				if err != nil {
					return err
				}
				m.GrassPercent = append(m.GrassPercent, int32(v))
			}
		case 30: // flows
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var flow FlowInfo
			if err := flow.Unmarshal(subData); err != nil {
				return err
			}
			m.Flows = append(m.Flows, flow)
		case 31: // plants
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var plant PlantDetail
			if err := plant.Unmarshal(subData); err != nil {
				return err
			}
			m.Plants = append(m.Plants, plant)
		case 32: // engravings
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var eng Engraving
			if err := eng.Unmarshal(subData); err != nil {
				return err
			}
			m.Engravings = append(m.Engravings, eng)
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
	BlocksNeeded int32
	MinX         int32
	MaxX         int32
	MinY         int32
	MaxY         int32
	MinZ         int32
	MaxZ         int32
	ForceReload  bool
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
	if r.ForceReload {
		e.EncodeBoolForce(8, true)
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

// UnitDefinition representa uma unidade (criatura) no DF.
type UnitDefinition struct {
	ID      int32
	Name    string
	Race    int32
	PosX    int32
	PosY    int32
	PosZ    int32
	SubposX float32
	SubposY float32
	SubposZ float32
	Flags1  uint32
	Flags2  uint32
	Flags3  uint32
	IsValid bool
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
			v, _ := d.ReadVarint()
			u.ID = int32(v)
		case 2:
			u.Name, _ = d.ReadString()
		case 3:
			v, _ := d.ReadVarint()
			u.Race = int32(v)
		case 4:
			v, _ := d.ReadVarint()
			u.PosX = int32(v)
		case 5:
			v, _ := d.ReadVarint()
			u.PosY = int32(v)
		case 6:
			v, _ := d.ReadVarint()
			u.PosZ = int32(v)
		case 7:
			v, _ := d.ReadFixed32()
			u.SubposX = math.Float32frombits(uint32(v))
		case 8:
			v, _ := d.ReadFixed32()
			u.SubposY = math.Float32frombits(uint32(v))
		case 9:
			v, _ := d.ReadFixed32()
			u.SubposZ = math.Float32frombits(uint32(v))
		case 10:
			v, _ := d.ReadVarint()
			u.Flags1 = uint32(v)
		case 11:
			v, _ := d.ReadVarint()
			u.Flags2 = uint32(v)
		case 12:
			v, _ := d.ReadVarint()
			u.Flags3 = uint32(v)
		case 13:
			v, _ := d.ReadVarint()
			u.IsValid = v != 0
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

// UnitList contém a lista de unidades.
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
			sub, _ := d.ReadBytes()
			var ud UnitDefinition
			ud.Unmarshal(sub)
			u.CreatureList = append(u.CreatureList, ud)
		default:
			d.SkipField(wireType)
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

// WorldMap - informações globais do mundo
type WorldMap struct {
	WorldWidth  int32
	WorldHeight int32
	Name        string
	NameEn      string
	CenterX     int32
	CenterY     int32
	CenterZ     int32
	CurYear     int32
	CurYearTick int32
}

func (w *WorldMap) Unmarshal(data []byte) error {
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
			w.WorldWidth = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			w.WorldHeight = int32(v)
		case 3:
			w.Name, err = d.ReadString()
			if err != nil {
				return err
			}
		case 4:
			w.NameEn, err = d.ReadString()
			if err != nil {
				return err
			}
		case 17:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			w.CenterX = int32(v)
		case 18:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			w.CenterY = int32(v)
		case 19:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			w.CenterZ = int32(v)
		case 20:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			w.CurYear = int32(v)
		case 21:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			w.CurYearTick = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// BuildingDefinition representa uma instância ou definição de construção.
type BuildingDefinition struct {
	BuildingType BuildingType
	ID           string
	Name         string
}

func (b *BuildingDefinition) Unmarshal(data []byte) error {
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
			if err := b.BuildingType.Unmarshal(subData); err != nil {
				return err
			}
		case 2:
			b.ID, err = d.ReadString()
			if err != nil {
				return err
			}
		case 3:
			b.Name, err = d.ReadString()
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

// BuildingList é a resposta para GetBuildingDefList.
type BuildingList struct {
	BuildingList []BuildingDefinition
}

func (b *BuildingList) Unmarshal(data []byte) error {
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
			var bd BuildingDefinition
			if err := bd.Unmarshal(subData); err != nil {
				return err
			}
			b.BuildingList = append(b.BuildingList, bd)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// Language contém os termos e nomes do jogo.
type Language struct {
	WordEntries []string
}

func (l *Language) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			s, err := d.ReadString()
			if err != nil {
				return err
			}
			l.WordEntries = append(l.WordEntries, s)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// BuildingExtents - área de uma construção
type BuildingExtents struct {
	X      int32
	Y      int32
	Width  int32
	Height int32
}

func (e *BuildingExtents) Unmarshal(data []byte) error {
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
			e.X = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			e.Y = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			e.Width = int32(v)
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			e.Height = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// BuildingInstanceList - lista de instâncias de construção
type BuildingInstanceList struct {
	BuildingList []BuildingInstance
}

func (l *BuildingInstanceList) Unmarshal(data []byte) error {
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
			var bit BuildingInstance
			if err := bit.Unmarshal(subData); err != nil {
				return err
			}
			l.BuildingList = append(l.BuildingList, bit)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// BuildingDirection - direção da construção
type BuildingDirection int32

const (
	BuildingDirNorth     BuildingDirection = 0
	BuildingDirEast      BuildingDirection = 1
	BuildingDirSouth     BuildingDirection = 2
	BuildingDirWest      BuildingDirection = 3
	BuildingDirNortheast BuildingDirection = 4
	BuildingDirSoutheast BuildingDirection = 5
	BuildingDirSouthwest BuildingDirection = 6
	BuildingDirNorthwest BuildingDirection = 7
	BuildingDirNone      BuildingDirection = 8
)
