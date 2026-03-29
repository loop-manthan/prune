package engine

const crc8Poly = uint8(0x07)

// CRC8 computes CRC-8 over data using polynomial 0x07,
// initial value 0x00, and no final xor.
func CRC8(data []byte) uint8 {
	var crc uint8
	for _, b := range data {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ crc8Poly
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// AppendCRC returns a new frame with CRC-8 appended as the final byte.
func AppendCRC(frame []byte) []byte {
	out := make([]byte, len(frame)+1)
	copy(out, frame)
	out[len(frame)] = CRC8(frame)
	return out
}

// ValidateCRC verifies the trailing byte equals CRC-8(payload).
func ValidateCRC(frame []byte) bool {
	if len(frame) < 1 {
		return false
	}
	payload := frame[:len(frame)-1]
	want := frame[len(frame)-1]
	return CRC8(payload) == want
}
