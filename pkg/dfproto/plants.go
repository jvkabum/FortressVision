package dfproto

import (
	"FortressVision/pkg/protowire"
)

// PlantRaw - definição de uma espécie de planta
type PlantRaw struct {
	ID      string
	Name    string
	Growths []TreeGrowth
	Tile    int32
}

func (p *PlantRaw) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadString()
			if err != nil {
				return err
			}
			p.ID = v
		case 2:
			v, err := d.ReadString()
			if err != nil {
				return err
			}
			p.Name = v
		case 4: // growths
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var g TreeGrowth
			if err := g.Unmarshal(subData); err != nil {
				return err
			}
			p.Growths = append(p.Growths, g)
		case 5:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			p.Tile = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// TreeGrowth - crescimento de uma árvore/planta
type TreeGrowth struct {
	ID          string
	TimingStart int32
	TimingEnd   int32
	Prints      []GrowthPrint
}

func (g *TreeGrowth) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			v, err := d.ReadString()
			if err != nil {
				return err
			}
			g.ID = v
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			g.TimingStart = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			g.TimingEnd = int32(v)
		case 4: // prints
			subData, err := d.ReadBytes()
			if err != nil {
				return err
			}
			var p GrowthPrint
			if err := p.Unmarshal(subData); err != nil {
				return err
			}
			g.Prints = append(g.Prints, p)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// GrowthPrint - aparência visual de um crescimento
type GrowthPrint struct {
	TimingStart int32
	TimingEnd   int32
	Tile        int32
}

func (p *GrowthPrint) Unmarshal(data []byte) error {
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
			p.TimingStart = int32(v)
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			p.TimingEnd = int32(v)
		case 3:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			p.Tile = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// PlantRawList - lista de definições de plantas
type PlantRawList struct {
	PlantRaws []PlantRaw
}

func (l *PlantRawList) Unmarshal(data []byte) error {
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
			var p PlantRaw
			if err := p.Unmarshal(subData); err != nil {
				return err
			}
			l.PlantRaws = append(l.PlantRaws, p)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}
