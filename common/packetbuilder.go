package common

import "encoding/binary"

type PacketBuilder struct {
	Buf []byte
}

func (p *PacketBuilder) WriteUint8(v uint8) {
	p.Buf = append(p.Buf, v)
}

func (p *PacketBuilder) WriteBool(v bool) {
	if v {
		p.Buf = append(p.Buf, 1)
	} else {
		p.Buf = append(p.Buf, 0)
	}
}

func (p *PacketBuilder) WriteUint16(v uint16) {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	p.Buf = append(p.Buf, b...)
}

func (p *PacketBuilder) WriteUint32(v uint32) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	p.Buf = append(p.Buf, b...)
}
