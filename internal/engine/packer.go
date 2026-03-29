package engine

import "errors"

var (
	// ErrInvalidBitWidth64 indicates bit width is outside supported limits.
	ErrInvalidBitWidth64 = errors.New("invalid bit width: must be in [1, 64]")
	// ErrReadPastEnd indicates the read cursor exceeded written payload size.
	ErrReadPastEnd = errors.New("read exceeds written bit length")
)

// BitBuffer performs non-byte-aligned bit writes and reads.
//
// Convention:
// - Fields are written MSB-first.
// - Bits are placed left-to-right across bytes.
// - Bit offset 0 is the MSB of byte 0.
type BitBuffer struct {
	buf            []byte
	writeBitOffset uint32
	readBitOffset  uint32
}

// NewBitBuffer returns an empty buffer ready for bit writes.
func NewBitBuffer() *BitBuffer {
	return &BitBuffer{}
}

// NewBitBufferFromBytes returns a reader/writer initialized from bytes.
// All bits in data are considered written.
func NewBitBufferFromBytes(data []byte) *BitBuffer {
	b := &BitBuffer{buf: append([]byte(nil), data...)}
	b.writeBitOffset = uint32(len(data) * 8)
	return b
}

// WriteBits appends the lower width bits of value into the buffer.
func (b *BitBuffer) WriteBits(value uint64, width uint8) error {
	if width == 0 || width > 64 {
		return ErrInvalidBitWidth64
	}

	value &= lowerBitMask(width)

	for i := int(width) - 1; i >= 0; i-- {
		bit := (value >> uint(i)) & 1
		byteIdx := int(b.writeBitOffset / 8)
		bitInByte := 7 - (b.writeBitOffset % 8)

		if byteIdx >= len(b.buf) {
			b.buf = append(b.buf, 0)
		}

		if bit == 1 {
			b.buf[byteIdx] |= 1 << bitInByte
		}
		b.writeBitOffset++
	}

	return nil
}

// ReadBits reads width bits from the current read cursor.
func (b *BitBuffer) ReadBits(width uint8) (uint64, error) {
	if width == 0 || width > 64 {
		return 0, ErrInvalidBitWidth64
	}
	if b.readBitOffset+uint32(width) > b.writeBitOffset {
		return 0, ErrReadPastEnd
	}

	var out uint64
	for i := 0; i < int(width); i++ {
		byteIdx := int(b.readBitOffset / 8)
		bitInByte := 7 - (b.readBitOffset % 8)
		bit := (b.buf[byteIdx] >> bitInByte) & 1
		out = (out << 1) | uint64(bit)
		b.readBitOffset++
	}

	return out, nil
}

// Bytes returns a copy of the underlying packed bytes.
func (b *BitBuffer) Bytes() []byte {
	return append([]byte(nil), b.buf...)
}

// BitString returns the written bits as a 0/1 string, trimmed to written bits.
// This is useful for future TUI visualization of raw frames.
func (b *BitBuffer) BitString() string {
	if b.writeBitOffset == 0 {
		return ""
	}
	out := make([]byte, 0, b.writeBitOffset)
	for i := uint32(0); i < b.writeBitOffset; i++ {
		byteIdx := int(i / 8)
		bitInByte := 7 - (i % 8)
		bit := (b.buf[byteIdx] >> bitInByte) & 1
		if bit == 1 {
			out = append(out, '1')
		} else {
			out = append(out, '0')
		}
	}
	return string(out)
}

// Reset clears bytes and both cursors.
func (b *BitBuffer) Reset() {
	b.buf = b.buf[:0]
	b.writeBitOffset = 0
	b.readBitOffset = 0
}

// RewindRead resets the read cursor to the first bit.
func (b *BitBuffer) RewindRead() {
	b.readBitOffset = 0
}

// WrittenBits returns how many bits have been written.
func (b *BitBuffer) WrittenBits() uint32 {
	return b.writeBitOffset
}

// ReadBitsOffset returns the current read cursor in bits.
func (b *BitBuffer) ReadBitsOffset() uint32 {
	return b.readBitOffset
}

func lowerBitMask(width uint8) uint64 {
	if width == 64 {
		return ^uint64(0)
	}
	return (uint64(1) << width) - 1
}
