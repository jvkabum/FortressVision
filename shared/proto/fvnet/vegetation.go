package fvnet

import (
	"FortressVision/shared/pkg/protowire"
)

// VegetationUpdateMessage representa uma atualização leve de plantas em um chunk
type VegetationUpdateMessage struct {
	ChunkX int32
	ChunkY int32
	ChunkZ int32
	Plants []PlantDetail
}

// PlantDetail representa uma planta individual no protocolo fvnet
type PlantDetail struct {
	X        int32
	Y        int32
	MatType  int32
	MatIndex int32
}

func (m *VegetationUpdateMessage) Marshal() []byte {
	e := protowire.NewEncoder()
	e.EncodeVarint(1, int64(m.ChunkX))
	e.EncodeVarint(2, int64(m.ChunkY))
	e.EncodeVarint(3, int64(m.ChunkZ))
	for _, p := range m.Plants {
		pe := protowire.NewEncoder()
		pe.EncodeVarint(1, int64(p.X))
		pe.EncodeVarint(2, int64(p.Y))
		pe.EncodeVarint(3, int64(p.MatType))
		pe.EncodeVarint(4, int64(p.MatIndex))
		e.EncodeSubmessage(4, pe.Bytes())
	}
	return e.Bytes()
}

func (m *VegetationUpdateMessage) Unmarshal(data []byte) error {
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
			m.ChunkX = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.ChunkY = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			m.ChunkZ = int32(v)
		case 4:
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var p PlantDetail
			if err := p.Unmarshal(subData); err != nil {
				return err
			}
			m.Plants = append(m.Plants, p)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
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
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			p.X = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			p.Y = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			p.MatType = int32(v)
		case 4:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			p.MatIndex = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}
