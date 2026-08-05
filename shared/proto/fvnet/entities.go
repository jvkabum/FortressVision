package fvnet

import (
	"bytes"
	"encoding/gob"

	"FortressVision/shared/mapdata"
)

// CreatureUpdateMessage transporta o snapshot das unidades ativas do DFHack.
// O snapshot completo permite remover unidades que deixaram o mapa sem manter
// fantasmas no cliente.
type CreatureUpdateMessage struct {
	Units []mapdata.UnitInstance
}

func (m *CreatureUpdateMessage) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(m.Units); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *CreatureUpdateMessage) Unmarshal(data []byte) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(&m.Units)
}
