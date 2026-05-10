package iso8583

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
)

type Codec interface {
	Pack(profile Profile, fields map[string]any) ([]byte, error)
	Unpack(profile Profile, data []byte) (map[string]any, error)
}

type InternalCodec struct{}

func NewInternalCodec() *InternalCodec {
	return &InternalCodec{}
}

func (c *InternalCodec) Pack(profile Profile, fields map[string]any) ([]byte, error) {
	mti := profile.MTI
	if value, ok := fields["mti"]; ok {
		mti = fmt.Sprintf("%v", value)
	}
	if len(mti) != 4 {
		return nil, fmt.Errorf("mti must be 4 characters")
	}

	fieldIDs := sortedFieldIDs(fields)
	bitmap := make([]byte, 8)
	payload := []byte(mti)

	for _, fieldID := range fieldIDs {
		if fieldID == 1 || fieldID > 64 {
			return nil, fmt.Errorf("field %d is not supported by primary bitmap", fieldID)
		}
		spec, ok := profile.Fields[fieldID]
		if !ok {
			return nil, fmt.Errorf("field %d is not defined in profile", fieldID)
		}
		value := fmt.Sprintf("%v", fields[strconv.Itoa(fieldID)])
		packed, err := packField(spec, value)
		if err != nil {
			return nil, err
		}
		setBitmap(bitmap, fieldID)
		payload = append(payload, packed...)
	}

	message := append(payload[:4], append(bitmap, payload[4:]...)...)
	if profile.LengthHeader {
		header := make([]byte, 2)
		binary.BigEndian.PutUint16(header, uint16(len(message)))
		message = append(header, message...)
	}

	return message, nil
}

func (c *InternalCodec) Unpack(profile Profile, data []byte) (map[string]any, error) {
	if profile.LengthHeader {
		if len(data) < 2 {
			return nil, fmt.Errorf("message too short for length header")
		}
		length := int(binary.BigEndian.Uint16(data[:2]))
		if len(data[2:]) != length {
			return nil, fmt.Errorf("length header mismatch")
		}
		data = data[2:]
	}
	if len(data) < 12 {
		return nil, fmt.Errorf("message too short")
	}

	fields := map[string]any{"mti": string(data[:4])}
	bitmap := data[4:12]
	offset := 12

	for fieldID := 2; fieldID <= 64; fieldID++ {
		if !bitmapSet(bitmap, fieldID) {
			continue
		}
		spec, ok := profile.Fields[fieldID]
		if !ok {
			return nil, fmt.Errorf("field %d is not defined in profile", fieldID)
		}
		value, consumed, err := unpackField(spec, data[offset:])
		if err != nil {
			return nil, err
		}
		fields[strconv.Itoa(fieldID)] = value
		offset += consumed
	}

	return fields, nil
}

func packField(spec FieldSpec, value string) ([]byte, error) {
	switch spec.Type {
	case FieldFixed:
		if len(value) != spec.Length {
			return nil, fmt.Errorf("field %d must be length %d", spec.ID, spec.Length)
		}
		return []byte(value), nil
	case FieldLLVAR:
		if len(value) > spec.Length || len(value) > 99 {
			return nil, fmt.Errorf("field %d exceeds LLVAR length", spec.ID)
		}
		return append([]byte(fmt.Sprintf("%02d", len(value))), []byte(value)...), nil
	case FieldLLLVAR:
		if len(value) > spec.Length || len(value) > 999 {
			return nil, fmt.Errorf("field %d exceeds LLLVAR length", spec.ID)
		}
		return append([]byte(fmt.Sprintf("%03d", len(value))), []byte(value)...), nil
	default:
		return nil, fmt.Errorf("field %d has unsupported type %q", spec.ID, spec.Type)
	}
}

func unpackField(spec FieldSpec, data []byte) (string, int, error) {
	switch spec.Type {
	case FieldFixed:
		if len(data) < spec.Length {
			return "", 0, fmt.Errorf("field %d is truncated", spec.ID)
		}
		return string(data[:spec.Length]), spec.Length, nil
	case FieldLLVAR:
		if len(data) < 2 {
			return "", 0, fmt.Errorf("field %d is missing LLVAR length", spec.ID)
		}
		length, err := strconv.Atoi(string(data[:2]))
		if err != nil {
			return "", 0, fmt.Errorf("field %d has invalid LLVAR length", spec.ID)
		}
		if len(data[2:]) < length {
			return "", 0, fmt.Errorf("field %d is truncated", spec.ID)
		}
		return string(data[2 : 2+length]), 2 + length, nil
	case FieldLLLVAR:
		if len(data) < 3 {
			return "", 0, fmt.Errorf("field %d is missing LLLVAR length", spec.ID)
		}
		length, err := strconv.Atoi(string(data[:3]))
		if err != nil {
			return "", 0, fmt.Errorf("field %d has invalid LLLVAR length", spec.ID)
		}
		if len(data[3:]) < length {
			return "", 0, fmt.Errorf("field %d is truncated", spec.ID)
		}
		return string(data[3 : 3+length]), 3 + length, nil
	default:
		return "", 0, fmt.Errorf("field %d has unsupported type %q", spec.ID, spec.Type)
	}
}

func sortedFieldIDs(fields map[string]any) []int {
	var ids []int
	for key := range fields {
		if key == "mti" {
			continue
		}
		id, err := strconv.Atoi(key)
		if err == nil {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

func setBitmap(bitmap []byte, fieldID int) {
	index := fieldID - 1
	bitmap[index/8] |= 1 << uint(7-(index%8))
}

func bitmapSet(bitmap []byte, fieldID int) bool {
	index := fieldID - 1
	return bitmap[index/8]&(1<<uint(7-(index%8))) != 0
}
