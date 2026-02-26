// Package protowire implementa encoding/decoding protobuf wire format minimalista.
// Suficiente para comunicação com DFHack RPC.
// Wire types: 0=Varint, 1=64bit, 2=LengthDelimited, 5=32bit
package protowire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// WireType constantes do protobuf
const (
	WireVarint          = 0
	Wire64Bit           = 1
	WireLengthDelimited = 2
	Wire32Bit           = 5
)

// ---------- ENCODER ----------

// Encoder acumula bytes no formato protobuf.
type Encoder struct {
	buf []byte
}

// NewEncoder cria um encoder vazio.
func NewEncoder() *Encoder {
	return &Encoder{buf: make([]byte, 0, 256)}
}

// Bytes retorna o buffer serializado.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// Reset limpa o buffer.
func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
}

// appendVarint adiciona um varint ao buffer.
func (e *Encoder) appendVarint(v uint64) {
	for v >= 0x80 {
		e.buf = append(e.buf, byte(v)|0x80)
		v >>= 7
	}
	e.buf = append(e.buf, byte(v))
}

// appendTag adiciona field tag (field_number << 3 | wire_type).
func (e *Encoder) appendTag(fieldNum int, wireType int) {
	e.appendVarint(uint64(fieldNum<<3 | wireType))
}

// EncodeVarint codifica um campo varint (int32, int64, uint32, uint64, bool, enum).
func (e *Encoder) EncodeVarint(fieldNum int, v int64) {
	if v == 0 {
		return // proto3: zero é valor default, não serializa
	}
	e.appendTag(fieldNum, WireVarint)
	e.appendVarint(uint64(v))
}

// EncodeVarintForce codifica varint mesmo que seja zero (proto2 required).
func (e *Encoder) EncodeVarintForce(fieldNum int, v int64) {
	e.appendTag(fieldNum, WireVarint)
	e.appendVarint(uint64(v))
}

// EncodeUvarint codifica uint64.
func (e *Encoder) EncodeUvarint(fieldNum int, v uint64) {
	if v == 0 {
		return
	}
	e.appendTag(fieldNum, WireVarint)
	e.appendVarint(v)
}

// EncodeBool codifica um boolean.
func (e *Encoder) EncodeBool(fieldNum int, v bool) {
	if !v {
		return
	}
	e.appendTag(fieldNum, WireVarint)
	e.appendVarint(1)
}

// EncodeBoolForce codifica um boolean mesmo que false.
func (e *Encoder) EncodeBoolForce(fieldNum int, v bool) {
	e.appendTag(fieldNum, WireVarint)
	if v {
		e.appendVarint(1)
	} else {
		e.appendVarint(0)
	}
}

// EncodeBytes codifica bytes raw (length-delimited).
func (e *Encoder) EncodeBytes(fieldNum int, v []byte) {
	if len(v) == 0 {
		return
	}
	e.appendTag(fieldNum, WireLengthDelimited)
	e.appendVarint(uint64(len(v)))
	e.buf = append(e.buf, v...)
}

// EncodeString codifica uma string.
func (e *Encoder) EncodeString(fieldNum int, v string) {
	if v == "" {
		return
	}
	e.appendTag(fieldNum, WireLengthDelimited)
	e.appendVarint(uint64(len(v)))
	e.buf = append(e.buf, v...)
}

// EncodeStringForce codifica uma string mesmo que vazia.
func (e *Encoder) EncodeStringForce(fieldNum int, v string) {
	e.appendTag(fieldNum, WireLengthDelimited)
	e.appendVarint(uint64(len(v)))
	e.buf = append(e.buf, v...)
}

// EncodeSubmessage codifica uma submensagem (length-delimited).
func (e *Encoder) EncodeSubmessage(fieldNum int, sub []byte) {
	if len(sub) == 0 {
		return
	}
	e.appendTag(fieldNum, WireLengthDelimited)
	e.appendVarint(uint64(len(sub)))
	e.buf = append(e.buf, sub...)
}

// EncodeFixed32 codifica um float32 como fixed32.
func (e *Encoder) EncodeFixed32(fieldNum int, v float32) {
	e.appendTag(fieldNum, Wire32Bit)
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
	e.buf = append(e.buf, b...)
}

// EncodePackedVarint codifica um repeated field como packed varint.
func (e *Encoder) EncodePackedVarint(fieldNum int, values []int32) {
	if len(values) == 0 {
		return
	}
	sub := NewEncoder()
	for _, v := range values {
		sub.appendVarint(uint64(v))
	}
	e.EncodeBytes(fieldNum, sub.Bytes())
}

// ---------- DECODER ----------

// Decoder lê campos protobuf de um buffer.
type Decoder struct {
	buf []byte
	pos int
}

// NewDecoder cria um decoder sobre um buffer.
func NewDecoder(buf []byte) *Decoder {
	return &Decoder{buf: buf, pos: 0}
}

// Done retorna true se não há mais bytes.
func (d *Decoder) Done() bool {
	return d.pos >= len(d.buf)
}

// Remaining retorna os bytes restantes.
func (d *Decoder) Remaining() int {
	return len(d.buf) - d.pos
}

// readVarint lê um varint do buffer.
func (d *Decoder) readVarint() (uint64, error) {
	var result uint64
	var shift uint
	for {
		if d.pos >= len(d.buf) {
			return 0, errors.New("protowire: varint truncado")
		}
		b := d.buf[d.pos]
		d.pos++
		result |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return result, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, errors.New("protowire: varint overflow")
		}
	}
}

// ReadTag lê o número do campo e o tipo de wire do próximo campo.
func (d *Decoder) ReadTag() (fieldNum int, wireType int, err error) {
	v, err := d.readVarint()
	if err != nil {
		return 0, 0, err
	}
	fieldNum = int(v >> 3)
	wireType = int(v & 0x07)
	return
}

// ReadVarint lê um valor varint (após o tag já ter sido lido).
func (d *Decoder) ReadVarint() (int64, error) {
	v, err := d.readVarint()
	return int64(v), err
}

// ReadBool lê um boolean.
func (d *Decoder) ReadBool() (bool, error) {
	v, err := d.readVarint()
	return v != 0, err
}

// ReadBytes lê um campo length-delimited.
func (d *Decoder) ReadBytes() ([]byte, error) {
	length, err := d.readVarint()
	if err != nil {
		return nil, err
	}

	// Verificação de segurança: evita overflow e pânicos de slice bounds (ex: [152:151])
	remaining := uint64(len(d.buf) - d.pos)
	if length > remaining {
		return nil, fmt.Errorf("protowire: comprimento excessivo: precisa %d, tem %d", length, remaining)
	}

	intLen := int(length)
	data := d.buf[d.pos : d.pos+intLen]
	d.pos += intLen
	return data, nil
}

// ReadString lê uma string.
func (d *Decoder) ReadString() (string, error) {
	b, err := d.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadFixed32 lê um float32 / fixed32.
func (d *Decoder) ReadFixed32() (float32, error) {
	if d.pos+4 > len(d.buf) {
		return 0, errors.New("protowire: fixed32 truncado")
	}
	v := binary.LittleEndian.Uint32(d.buf[d.pos:])
	d.pos += 4
	return math.Float32frombits(v), nil
}

// ReadFixed64 lê um fixed64.
func (d *Decoder) ReadFixed64() (uint64, error) {
	if d.pos+8 > len(d.buf) {
		return 0, errors.New("protowire: fixed64 truncado")
	}
	v := binary.LittleEndian.Uint64(d.buf[d.pos:])
	d.pos += 8
	return v, nil
}

// SkipField pula um campo baseado no wire type.
func (d *Decoder) SkipField(wireType int) error {
	switch wireType {
	case WireVarint:
		_, err := d.readVarint()
		return err
	case Wire64Bit:
		if d.pos+8 > len(d.buf) {
			return errors.New("protowire: 64-bit truncado")
		}
		d.pos += 8
		return nil
	case WireLengthDelimited:
		b, err := d.ReadBytes()
		_ = b
		return err
	case Wire32Bit:
		if d.pos+4 > len(d.buf) {
			return errors.New("protowire: 32-bit truncado")
		}
		d.pos += 4
		return nil
	default:
		return fmt.Errorf("protowire: wire type desconhecido: %d", wireType)
	}
}

// ReadPackedVarint lê um packed repeated varint field.
func (d *Decoder) ReadPackedVarint() ([]int32, error) {
	data, err := d.ReadBytes()
	if err != nil {
		return nil, err
	}
	sub := NewDecoder(data)
	var result []int32
	for !sub.Done() {
		v, err := sub.ReadVarint()
		if err != nil {
			return result, err
		}
		result = append(result, int32(v))
	}
	return result, nil
}

// ReadPackedBool lê um packed repeated bool field.
func (d *Decoder) ReadPackedBool() ([]bool, error) {
	data, err := d.ReadBytes()
	if err != nil {
		return nil, err
	}
	sub := NewDecoder(data)
	var result []bool
	for !sub.Done() {
		v, err := sub.ReadBool()
		if err != nil {
			return result, err
		}
		result = append(result, v)
	}
	return result, nil
}
