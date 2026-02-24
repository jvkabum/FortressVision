// Package dfproto contém as structs protobuf manuais do DFHack core protocol.
// Baseado em: dfhack-develop/library/proto/CoreProtocol.proto
package dfproto

import (
	"FortressVision/shared/pkg/protowire"
)

// EmptyMessage representa uma mensagem vazia (sem campos).
type EmptyMessage struct{}

func (m *EmptyMessage) Marshal() ([]byte, error) {
	return []byte{}, nil
}

func (m *EmptyMessage) Unmarshal(data []byte) error {
	return nil
}

// CoreBindRequest é a mensagem para vincular um método RPC.
//
//	message CoreBindRequest {
//	  required string method = 1;
//	  required string input_msg = 2;
//	  required string output_msg = 3;
//	  optional string plugin = 4;
//	}
type CoreBindRequest struct {
	Method    string
	InputMsg  string
	OutputMsg string
	Plugin    string
}

func (m *CoreBindRequest) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeStringForce(1, m.Method)
	e.EncodeStringForce(2, m.InputMsg)
	e.EncodeStringForce(3, m.OutputMsg)
	if m.Plugin != "" {
		e.EncodeString(4, m.Plugin)
	}
	return e.Bytes(), nil
}

// CoreBindReply é a resposta do bind.
//
//	message CoreBindReply {
//	  required int32 assigned_id = 1;
//	}
type CoreBindReply struct {
	AssignedID int32
}

func (m *CoreBindReply) Unmarshal(data []byte) error {
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
			m.AssignedID = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// CoreRunCommandRequest executa um comando no DFHack.
//
//	message CoreRunCommandRequest {
//	  required string command = 1;
//	  repeated string arguments = 2;
//	}
type CoreRunCommandRequest struct {
	Command   string
	Arguments []string
}

func (m *CoreRunCommandRequest) Marshal() ([]byte, error) {
	e := protowire.NewEncoder()
	e.EncodeStringForce(1, m.Command)
	for _, arg := range m.Arguments {
		e.EncodeString(2, arg)
	}
	return e.Bytes(), nil
}

// CoreTextNotification é uma notificação de texto do servidor.
//
//	message CoreTextNotification {
//	  optional CoreTextFragment fragments = 1;
//	}
type CoreTextNotification struct {
	Fragments []CoreTextFragment
}

func (m *CoreTextNotification) Unmarshal(data []byte) error {
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
			var frag CoreTextFragment
			if err := frag.Unmarshal(subData); err != nil {
				return err
			}
			m.Fragments = append(m.Fragments, frag)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// CoreTextFragment é um fragmento de texto colorido.
type CoreTextFragment struct {
	Text  string
	Color int32
}

func (f *CoreTextFragment) Unmarshal(data []byte) error {
	d := protowire.NewDecoder(data)
	for !d.Done() {
		fieldNum, wireType, err := d.ReadTag()
		if err != nil {
			return err
		}
		switch fieldNum {
		case 1:
			f.Text, err = d.ReadString()
			if err != nil {
				return err
			}
		case 2:
			v, err := d.ReadVarint()
			if err != nil {
				return err
			}
			f.Color = int32(v)
		default:
			if err := d.SkipField(wireType); err != nil {
				return err
			}
		}
	}
	return nil
}
