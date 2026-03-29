package engine

import (
	"errors"
	"prune/pkg/telemetry"
)

var (
	// ErrCRCValidationFailed indicates the CRC check on received frame failed.
	ErrCRCValidationFailed = errors.New("crc validation failed")
	// ErrPacketTooSmall indicates the packet doesn't have enough bits to decode.
	ErrPacketTooSmall = errors.New("packet too small")
)

// EncodedFrame is a complete sensor telemetry frame with header, payload, and CRC.
type EncodedFrame struct {
	Version  uint8  // 2 bits
	Sequence uint16 // 12 bits
	Flags    uint8  // 4 bits

	// Sensor values in quantized domain (what gets packed)
	BatteryQ  uint32
	AltitudeQ uint32
	AltMode   bool // true = keyframe, false = delta
	IMUQ      [3]uint32
	IMUModes  [3]bool // [0]=mode for imu_x, etc.

	// Previous quantized values for delta encoding state
	PrevAltitudeQ uint32
	PrevIMUQ      [3]uint32

	// Framed payload with CRC
	Bytes []byte
}

// FrameCodec provides deterministic encoding and decoding of telemetry frames.
type FrameCodec struct {
	writeBuf *BitBuffer
	readBuf  *BitBuffer
}

// NewFrameCodec returns a ready-to-use encoder/decoder.
func NewFrameCodec() *FrameCodec {
	return &FrameCodec{
		writeBuf: NewBitBuffer(),
		readBuf:  nil,
	}
}

// Encode takes a frame definition and returns the packed, CRC'd bytes.
func (fc *FrameCodec) Encode(frame *EncodedFrame) ([]byte, error) {
	fc.writeBuf.Reset()

	// Header
	if err := fc.writeBuf.WriteBits(uint64(frame.Version), telemetry.VersionBits); err != nil {
		return nil, err
	}
	if err := fc.writeBuf.WriteBits(uint64(frame.Sequence), telemetry.SequenceBits); err != nil {
		return nil, err
	}
	if err := fc.writeBuf.WriteBits(uint64(frame.Flags), telemetry.FlagsBits); err != nil {
		return nil, err
	}

	// Battery (6-bit absolute, not delta-able)
	if err := fc.writeBuf.WriteBits(uint64(frame.BatteryQ), telemetry.BatteryBits); err != nil {
		return nil, err
	}

	// Altitude (mode + payload)
	altMode := uint8(0)
	if frame.AltMode {
		altMode = 1
	}
	if err := fc.writeBuf.WriteBits(uint64(altMode), telemetry.AltitudeMode); err != nil {
		return nil, err
	}

	var altPayload uint32
	var altWidth uint8
	if frame.AltMode {
		// Keyframe mode
		altPayload = frame.AltitudeQ
		altWidth = telemetry.AltitudeBits
	} else {
		// Delta mode: pre-computed delta
		altPayload = frame.AltitudeQ
		altWidth = telemetry.DeltaBits
	}
	if err := fc.writeBuf.WriteBits(uint64(altPayload), altWidth); err != nil {
		return nil, err
	}

	// IMU (3-axis, each with mode + payload)
	for axis := 0; axis < 3; axis++ {
		imuMode := uint8(0)
		if frame.IMUModes[axis] {
			imuMode = 1
		}
		if err := fc.writeBuf.WriteBits(uint64(imuMode), telemetry.AltitudeMode); err != nil {
			return nil, err
		}

		var imuPayload uint32
		var imuWidth uint8
		if frame.IMUModes[axis] {
			imuPayload = frame.IMUQ[axis]
			imuWidth = telemetry.IMUBits
		} else {
			imuPayload = frame.IMUQ[axis]
			imuWidth = telemetry.DeltaBits
		}
		if err := fc.writeBuf.WriteBits(uint64(imuPayload), imuWidth); err != nil {
			return nil, err
		}
	}

	// Get payload bytes and append CRC
	payload := fc.writeBuf.Bytes()
	framed := AppendCRC(payload)
	frame.Bytes = framed
	return framed, nil
}

// Decode validates and unpacks a frame from raw bytes.
// Returns an EncodedFrame with all fields populated in quantized domain.
func (fc *FrameCodec) Decode(data []byte) (*EncodedFrame, error) {
	if !ValidateCRC(data) {
		return nil, ErrCRCValidationFailed
	}

	payload := data[:len(data)-1]
	fc.readBuf = NewBitBufferFromBytes(payload)

	frame := &EncodedFrame{}

	// Header
	v, err := fc.readBuf.ReadBits(telemetry.VersionBits)
	if err != nil {
		return nil, ErrPacketTooSmall
	}
	frame.Version = uint8(v)

	seq, err := fc.readBuf.ReadBits(telemetry.SequenceBits)
	if err != nil {
		return nil, ErrPacketTooSmall
	}
	frame.Sequence = uint16(seq)

	flgs, err := fc.readBuf.ReadBits(telemetry.FlagsBits)
	if err != nil {
		return nil, ErrPacketTooSmall
	}
	frame.Flags = uint8(flgs)

	// Battery
	bq, err := fc.readBuf.ReadBits(telemetry.BatteryBits)
	if err != nil {
		return nil, ErrPacketTooSmall
	}
	frame.BatteryQ = uint32(bq)

	// Altitude
	altMode, err := fc.readBuf.ReadBits(telemetry.AltitudeMode)
	if err != nil {
		return nil, ErrPacketTooSmall
	}
	frame.AltMode = altMode != 0

	var altPayload uint64
	if frame.AltMode {
		altPayload, err = fc.readBuf.ReadBits(telemetry.AltitudeBits)
	} else {
		altPayload, err = fc.readBuf.ReadBits(telemetry.DeltaBits)
	}
	if err != nil {
		return nil, ErrPacketTooSmall
	}
	frame.AltitudeQ = uint32(altPayload)

	// IMU
	for axis := 0; axis < 3; axis++ {
		imuMode, err := fc.readBuf.ReadBits(telemetry.AltitudeMode)
		if err != nil {
			return nil, ErrPacketTooSmall
		}
		frame.IMUModes[axis] = imuMode != 0

		var imuPayload uint64
		if frame.IMUModes[axis] {
			imuPayload, err = fc.readBuf.ReadBits(telemetry.IMUBits)
		} else {
			imuPayload, err = fc.readBuf.ReadBits(telemetry.DeltaBits)
		}
		if err != nil {
			return nil, ErrPacketTooSmall
		}
		frame.IMUQ[axis] = uint32(imuPayload)
	}

	frame.Bytes = data
	return frame, nil
}

// QuantizeAndEncode quantizes raw sensor values and prepares them for encoding.
// Handles delta encoding decisions (keyframe vs delta) based on prior state.
func QuantizeAndEncode(
	raw struct {
		Battery   float64
		Altitude  float64
		IMU       [3]float64
		Flags     uint8
		Sequence  uint16
	},
	priorState *struct {
		AltitudeQ uint32
		IMUQ      [3]uint32
	},
) (*EncodedFrame, error) {
	frame := &EncodedFrame{
		Version:       0,
		Sequence:      raw.Sequence,
		Flags:         raw.Flags,
		PrevAltitudeQ: priorState.AltitudeQ,
		PrevIMUQ:      priorState.IMUQ,
	}

	// Quantize battery (no delta)
	bq, err := Quantize(raw.Battery, telemetry.BatteryMin, telemetry.BatteryMax, telemetry.BatteryBits)
	if err != nil {
		return nil, err
	}
	frame.BatteryQ = bq

	// Quantize altitude and try delta
	aq, err := Quantize(raw.Altitude, telemetry.AltitudeMin, telemetry.AltitudeMax, telemetry.AltitudeBits)
	if err != nil {
		return nil, err
	}
	isKeyframe, deltaPayload, _ := EncodeDelta(aq, priorState.AltitudeQ)
	frame.AltMode = isKeyframe
	frame.AltitudeQ = deltaPayload

	// Quantize IMU and try delta for each axis
	for axis := 0; axis < 3; axis++ {
		iq, err := Quantize(raw.IMU[axis], telemetry.IMUMin, telemetry.IMUMax, telemetry.IMUBits)
		if err != nil {
			return nil, err
		}
		isKeyframe, deltaPayload, _ := EncodeDelta(iq, priorState.IMUQ[axis])
		frame.IMUModes[axis] = isKeyframe
		frame.IMUQ[axis] = deltaPayload
	}

	return frame, nil
}

// DecodeAndDequantize unpacks an encoded frame and reconstructs raw sensor values.
// Also returns the quantized domain values that the TUI will display as "reconstructed" for error metrics.
func DecodeAndDequantize(frame *EncodedFrame) (struct {
	Battery   float64
	Altitude  float64
	IMU       [3]float64
	Flags     uint8
	Sequence  uint16
}, struct {
	BatteryQ  uint32
	AltitudeQ uint32
	IMUQ      [3]uint32
}, error) {
	var raw struct {
		Battery   float64
		Altitude  float64
		IMU       [3]float64
		Flags     uint8
		Sequence  uint16
	}
	var qvals struct {
		BatteryQ  uint32
		AltitudeQ uint32
		IMUQ      [3]uint32
	}

	raw.Flags = frame.Flags
	raw.Sequence = frame.Sequence

	// Battery
	qvals.BatteryQ = frame.BatteryQ
	raw.Battery = Dequantize(frame.BatteryQ, telemetry.BatteryMin, telemetry.BatteryMax, telemetry.BatteryBits)

	// Altitude (decode delta if needed)
	qvals.AltitudeQ = DecodeDelta(frame.AltMode, frame.AltitudeQ, frame.PrevAltitudeQ)
	raw.Altitude = Dequantize(qvals.AltitudeQ, telemetry.AltitudeMin, telemetry.AltitudeMax, telemetry.AltitudeBits)

	// IMU (decode delta for each axis)
	for axis := 0; axis < 3; axis++ {
		qvals.IMUQ[axis] = DecodeDelta(frame.IMUModes[axis], frame.IMUQ[axis], frame.PrevIMUQ[axis])
		raw.IMU[axis] = Dequantize(qvals.IMUQ[axis], telemetry.IMUMin, telemetry.IMUMax, telemetry.IMUBits)
	}

	return raw, qvals, nil
}
