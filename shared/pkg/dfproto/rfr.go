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
	FlowCampFire      FlowType = -1
)

type Hair struct {
	Length int32
	Style  int32
}

func (h *Hair) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, _ := d.ReadVarint()
			h.Length = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			h.Style = int32(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type BodySizeInfo struct {
	SizeCur    int32
	SizeBase   int32
	AreaCur    int32
	AreaBase   int32
	LengthCur  int32
	LengthBase int32
}

func (b *BodySizeInfo) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, _ := d.ReadVarint()
			b.SizeCur = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			b.SizeBase = int32(v)
		case 3:
			v, _ := d.ReadVarint()
			b.AreaCur = int32(v)
		case 4:
			v, _ := d.ReadVarint()
			b.AreaBase = int32(v)
		case 5:
			v, _ := d.ReadVarint()
			b.LengthCur = int32(v)
		case 6:
			v, _ := d.ReadVarint()
			b.LengthBase = int32(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

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
			v, _ := d.ReadVarint()
			b.BuildingType = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			b.BuildingSubtype = int32(v)
		case 3:
			v, _ := d.ReadVarint()
			b.BuildingCustom = int32(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type ArtImageElementType int32

const (
	ImageCreature ArtImageElementType = 0
	ImagePlant    ArtImageElementType = 1
	ImageTree     ArtImageElementType = 2
	ImageShape    ArtImageElementType = 3
	ImageItem     ArtImageElementType = 4
)

type ArtImageElement struct {
	Count        int32
	Type         ArtImageElementType
	CreatureItem MatPair
	Material     MatPair
	ID           int32
}

func (a *ArtImageElement) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, _ := d.ReadVarint()
			a.Count = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			a.Type = ArtImageElementType(v)
		case 3:
			sub, _ := d.ReadBytes()
			a.CreatureItem.Unmarshal(sub)
		case 5:
			sub, _ := d.ReadBytes()
			a.Material.Unmarshal(sub)
		case 6:
			v, _ := d.ReadVarint()
			a.ID = int32(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type ArtImagePropertyType int32

const (
	TransitiveVerb   ArtImagePropertyType = 0
	IntransitiveVerb ArtImagePropertyType = 1
)

type ArtImageProperty struct {
	Subject int32
	Object  int32
	Verb    int32 // ArtImageVerb
	Type    ArtImagePropertyType
}

func (a *ArtImageProperty) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, _ := d.ReadVarint()
			a.Subject = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			a.Object = int32(v)
		case 3:
			v, _ := d.ReadVarint()
			a.Verb = int32(v)
		case 4:
			v, _ := d.ReadVarint()
			a.Type = ArtImagePropertyType(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type ArtImage struct {
	Elements   []ArtImageElement
	ID         MatPair
	Properties []ArtImageProperty
}

func (a *ArtImage) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			sub, _ := d.ReadBytes()
			var ae ArtImageElement
			ae.Unmarshal(sub)
			a.Elements = append(a.Elements, ae)
		case 2:
			sub, _ := d.ReadBytes()
			a.ID.Unmarshal(sub)
		case 3:
			sub, _ := d.ReadBytes()
			var ap ArtImageProperty
			ap.Unmarshal(sub)
			a.Properties = append(a.Properties, ap)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type Item struct {
	ID           int32
	Pos          Coord
	Flags1       uint32
	Flags2       uint32
	Type         MatPair
	Material     MatPair
	Dye          ColorDefinition   // ID 7
	StackSize    int32             // ID 8
	SubposX      float32           // ID 9
	SubposY      float32           // ID 10
	SubposZ      float32           // ID 11
	Projectile   bool              // ID 12
	VelocityX    float32           // ID 13
	VelocityY    float32           // ID 14
	VelocityZ    float32           // ID 15
	Volume       int32             // ID 16
	Improvements []ItemImprovement // ID 17
	Image        ArtImage          // ID 18
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
			v, _ := d.ReadVarint()
			i.ID = int32(v)
		case 2:
			sub, _ := d.ReadBytes()
			i.Pos.Unmarshal(sub)
		case 3:
			v, _ := d.ReadVarint()
			i.Flags1 = uint32(v)
		case 4:
			v, _ := d.ReadVarint()
			i.Flags2 = uint32(v)
		case 5:
			sub, _ := d.ReadBytes()
			i.Type.Unmarshal(sub)
		case 6:
			sub, _ := d.ReadBytes()
			i.Material.Unmarshal(sub)
		case 7:
			sub, _ := d.ReadBytes()
			i.Dye.Unmarshal(sub)
		case 8:
			v, _ := d.ReadVarint()
			i.StackSize = int32(v)
		case 9:
			i.SubposX, _ = d.ReadFixed32()
		case 10:
			i.SubposY, _ = d.ReadFixed32()
		case 11:
			i.SubposZ, _ = d.ReadFixed32()
		case 12:
			i.Projectile, _ = d.ReadBool()
		case 13:
			i.VelocityX, _ = d.ReadFixed32()
		case 14:
			i.VelocityY, _ = d.ReadFixed32()
		case 15:
			i.VelocityZ, _ = d.ReadFixed32()
		case 16:
			v, _ := d.ReadVarint()
			i.Volume = int32(v)
		case 17:
			sub, _ := d.ReadBytes()
			var imp ItemImprovement
			imp.Unmarshal(sub)
			i.Improvements = append(i.Improvements, imp)
		case 18:
			sub, _ := d.ReadBytes()
			i.Image.Unmarshal(sub)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type ItemImprovement struct {
	Material     MatPair
	Shape        int32
	SpecificType int32
	Image        ArtImage
	Type         int32
}

func (i *ItemImprovement) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			sub, _ := d.ReadBytes()
			i.Material.Unmarshal(sub)
		case 3:
			v, _ := d.ReadVarint()
			i.Shape = int32(v)
		case 4:
			v, _ := d.ReadVarint()
			i.SpecificType = int32(v)
		case 5:
			sub, _ := d.ReadBytes()
			i.Image.Unmarshal(sub)
		case 6:
			v, _ := d.ReadVarint()
			i.Type = int32(v)
		default:
			d.SkipField(wireType)
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
			v, _ := d.ReadVarint()
			b.Index = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			b.PosXMin = int32(v)
		case 3:
			v, _ := d.ReadVarint()
			b.PosYMin = int32(v)
		case 4:
			v, _ := d.ReadVarint()
			b.PosZMin = int32(v)
		case 5:
			v, _ := d.ReadVarint()
			b.PosXMax = int32(v)
		case 6:
			v, _ := d.ReadVarint()
			b.PosYMax = int32(v)
		case 7:
			v, _ := d.ReadVarint()
			b.PosZMax = int32(v)
		case 8:
			sub, _ := d.ReadBytes()
			b.BuildingType.Unmarshal(sub)
		case 9:
			sub, _ := d.ReadBytes()
			b.Material.Unmarshal(sub)
		case 10:
			v, _ := d.ReadVarint()
			b.BuildingFlags = uint32(v)
		case 11:
			b.IsRoom, _ = d.ReadBool()
		case 13:
			v, _ := d.ReadVarint()
			b.Direction = BuildingDirection(v)
		case 14:
			sub, _ := d.ReadBytes()
			var bi BuildingItem
			bi.Unmarshal(sub)
			b.Items = append(b.Items, bi)
		case 15:
			v, _ := d.ReadVarint()
			b.Active = int32(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

// Engraving representa uma gravura no mapa.
type Engraving struct {
	Pos       Coord
	Quality   int32
	Tile      int32
	Image     ArtImage
	IsFloor   bool
	West      bool
	East      bool
	North     bool
	South     bool
	Hidden    bool
	Northwest bool
	Northeast bool
	Southwest bool
	Southeast bool
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
			sub, _ := d.ReadBytes()
			e.Pos.Unmarshal(sub)
		case 2:
			v, _ := d.ReadVarint()
			e.Quality = int32(v)
		case 3:
			v, _ := d.ReadVarint()
			e.Tile = int32(v)
		case 4:
			sub, _ := d.ReadBytes()
			e.Image.Unmarshal(sub)
		case 5:
			e.IsFloor, _ = d.ReadBool()
		case 6:
			e.West, _ = d.ReadBool()
		case 7:
			e.East, _ = d.ReadBool()
		case 8:
			e.North, _ = d.ReadBool()
		case 9:
			e.South, _ = d.ReadBool()
		case 10:
			e.Hidden, _ = d.ReadBool()
		case 11:
			e.Northwest, _ = d.ReadBool()
		case 12:
			e.Northeast, _ = d.ReadBool()
		case 13:
			e.Southwest, _ = d.ReadBool()
		case 14:
			e.Southeast, _ = d.ReadBool()
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

// FlowInfo representa um fluxo de gás, vapor ou líquido.
type FlowInfo struct {
	Index     int32
	Type      FlowType
	Density   int32
	Pos       Coord
	Dest      Coord
	Expanding bool
	GuideID   int32
	Material  MatPair
	Item      MatPair
	Dead      bool
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
			v, _ := d.ReadVarint()
			f.Index = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			f.Type = FlowType(v)
		case 3:
			v, _ := d.ReadVarint()
			f.Density = int32(v)
		case 4:
			sub, _ := d.ReadBytes()
			f.Pos.Unmarshal(sub)
		case 5:
			sub, _ := d.ReadBytes()
			f.Dest.Unmarshal(sub)
		case 6:
			f.Expanding, _ = d.ReadBool()
		case 8:
			v, _ := d.ReadVarint()
			f.GuideID = int32(v)
		case 9:
			sub, _ := d.ReadBytes()
			f.Material.Unmarshal(sub)
		case 10:
			sub, _ := d.ReadBytes()
			f.Item.Unmarshal(sub)
		case 11:
			f.Dead, _ = d.ReadBool()
		case 12:
			f.Fast, _ = d.ReadBool()
		case 13:
			f.Creeping, _ = d.ReadBool()
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

// Wave representa uma onda marinha.
type Wave struct {
	Dest Coord
	Pos  Coord
}

func (w *Wave) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			sub, _ := d.ReadBytes()
			w.Dest.Unmarshal(sub)
		case 2:
			sub, _ := d.ReadBytes()
			w.Pos.Unmarshal(sub)
		default:
			d.SkipField(wireType)
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
	MapBlocks  []MapBlock
	MapX       int32
	MapY       int32
	Engravings []Engraving // ID 4
	OceanWaves []Wave      // ID 5
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
		case 4: // engravings
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var eng Engraving
			if err := eng.Unmarshal(subData); err != nil {
				return err
			}
			b.Engravings = append(b.Engravings, eng)
		case 5: // ocean_waves
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var wave Wave
			if err := wave.Unmarshal(subData); err != nil {
				return err
			}
			b.OceanWaves = append(b.OceanWaves, wave)
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

type UnitAppearance struct {
	BodyModifiers       []int32
	BpModifiers         []int32
	SizeModifier        int32
	Colors              []int32
	Hair                Hair
	Beard               Hair
	Moustache           Hair
	Sideburns           Hair
	PhysicalDescription string
}

func (u *UnitAppearance) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			if wireType == protowire.WireLengthDelimited {
				vals, _ := d.ReadPackedVarint()
				for _, v := range vals {
					u.BodyModifiers = append(u.BodyModifiers, int32(v))
				}
			} else {
				v, _ := d.ReadVarint()
				u.BodyModifiers = append(u.BodyModifiers, int32(v))
			}
		case 2:
			if wireType == protowire.WireLengthDelimited {
				vals, _ := d.ReadPackedVarint()
				for _, v := range vals {
					u.BpModifiers = append(u.BpModifiers, int32(v))
				}
			} else {
				v, _ := d.ReadVarint()
				u.BpModifiers = append(u.BpModifiers, int32(v))
			}
		case 3:
			v, _ := d.ReadVarint()
			u.SizeModifier = int32(v)
		case 4:
			if wireType == protowire.WireLengthDelimited {
				vals, _ := d.ReadPackedVarint()
				for _, v := range vals {
					u.Colors = append(u.Colors, int32(v))
				}
			} else {
				v, _ := d.ReadVarint()
				u.Colors = append(u.Colors, int32(v))
			}
		case 5:
			sub, _ := d.ReadBytes()
			u.Hair.Unmarshal(sub)
		case 6:
			sub, _ := d.ReadBytes()
			u.Beard.Unmarshal(sub)
		case 7:
			sub, _ := d.ReadBytes()
			u.Moustache.Unmarshal(sub)
		case 8:
			sub, _ := d.ReadBytes()
			u.Sideburns.Unmarshal(sub)
		case 9:
			u.PhysicalDescription, _ = d.ReadString()
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type InventoryItem struct {
	Mode       int32
	Item       Item
	BodyPartID int32
}

func (i *InventoryItem) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, _ := d.ReadVarint()
			i.Mode = int32(v)
		case 2:
			sub, _ := d.ReadBytes()
			i.Item.Unmarshal(sub)
		case 3:
			v, _ := d.ReadVarint()
			i.BodyPartID = int32(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type WoundPart struct {
	GlobalLayerIdx int32
	BodyPartID     int32
	LayerIdx       int32
}

func (w *WoundPart) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, _ := d.ReadVarint()
			w.GlobalLayerIdx = int32(v)
		case 2:
			v, _ := d.ReadVarint()
			w.BodyPartID = int32(v)
		case 3:
			v, _ := d.ReadVarint()
			w.LayerIdx = int32(v)
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

type UnitWound struct {
	Parts       []WoundPart
	SeveredPart bool
}

func (u *UnitWound) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			sub, _ := d.ReadBytes()
			var wp WoundPart
			wp.Unmarshal(sub)
			u.Parts = append(u.Parts, wp)
		case 2:
			v, _ := d.ReadBool()
			u.SeveredPart = v
		default:
			d.SkipField(wireType)
		}
	}
	return nil
}

// UnitDefinition representa a definição de uma unidade (anão, animal, etc).
type UnitDefinition struct {
	ID             int32
	IsValid        bool
	PosX           int32
	PosY           int32
	PosZ           int32
	Race           MatPair
	ProfessionCol  ColorDefinition
	Flags1         uint32
	Flags2         uint32
	Flags3         uint32
	IsSoldier      bool
	SizeInfo       BodySizeInfo
	Name           string
	BloodMax       int32
	BloodCount     int32
	Appearance     UnitAppearance
	ProfessionID   int32
	NoblePositions []string
	RiderID        int32
	Inventory      []InventoryItem
	SubposX        float32
	SubposY        float32
	SubposZ        float32
	Facing         Coord
	Age            int32
	Wounds         []UnitWound
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
			u.IsValid, _ = d.ReadBool()
		case 3:
			v, _ := d.ReadVarint()
			u.PosX = int32(v)
		case 4:
			v, _ := d.ReadVarint()
			u.PosY = int32(v)
		case 5:
			v, _ := d.ReadVarint()
			u.PosZ = int32(v)
		case 6:
			sub, _ := d.ReadBytes()
			u.Race.Unmarshal(sub)
		case 7:
			sub, _ := d.ReadBytes()
			u.ProfessionCol.Unmarshal(sub)
		case 8:
			v, _ := d.ReadVarint()
			u.Flags1 = uint32(v)
		case 9:
			v, _ := d.ReadVarint()
			u.Flags2 = uint32(v)
		case 10:
			v, _ := d.ReadVarint()
			u.Flags3 = uint32(v)
		case 11:
			u.IsSoldier, _ = d.ReadBool()
		case 12:
			sub, _ := d.ReadBytes()
			u.SizeInfo.Unmarshal(sub)
		case 13:
			u.Name, _ = d.ReadString()
		case 14:
			v, _ := d.ReadVarint()
			u.BloodMax = int32(v)
		case 15:
			v, _ := d.ReadVarint()
			u.BloodCount = int32(v)
		case 16:
			sub, _ := d.ReadBytes()
			u.Appearance.Unmarshal(sub)
		case 17:
			v, _ := d.ReadVarint()
			u.ProfessionID = int32(v)
		case 18:
			s, _ := d.ReadString()
			u.NoblePositions = append(u.NoblePositions, s)
		case 19:
			v, _ := d.ReadVarint()
			u.RiderID = int32(v)
		case 20:
			sub, _ := d.ReadBytes()
			var inv InventoryItem
			inv.Unmarshal(sub)
			u.Inventory = append(u.Inventory, inv)
		case 21:
			v, _ := d.ReadFixed32()
			u.SubposX = math.Float32frombits(uint32(v))
		case 22:
			v, _ := d.ReadFixed32()
			u.SubposY = math.Float32frombits(uint32(v))
		case 23:
			v, _ := d.ReadFixed32()
			u.SubposZ = math.Float32frombits(uint32(v))
		case 24:
			sub, _ := d.ReadBytes()
			u.Facing.Unmarshal(sub)
		case 25:
			v, _ := d.ReadVarint()
			u.Age = int32(v)
		case 26:
			sub, _ := d.ReadBytes()
			var w UnitWound
			w.Unmarshal(sub)
			u.Wounds = append(u.Wounds, w)
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
