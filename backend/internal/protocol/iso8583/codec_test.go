package iso8583

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodecPacksBitmapFixedLLVARLLLVARAndLengthHeader(t *testing.T) {
	codec := NewInternalCodec()
	profile := testProfile()

	got, err := codec.Pack(profile, map[string]any{
		"mti": "0100",
		"2":   "4111111111111111",
		"3":   "000000",
		"4":   "000000010000",
		"48":  "HELLO",
	})

	require.NoError(t, err)
	require.Equal(t, len(got)-2, int(binary.BigEndian.Uint16(got[:2])))
	require.Equal(t, "0100", string(got[2:6]))
	require.True(t, bitmapSet(got[6:14], 2))
	require.True(t, bitmapSet(got[6:14], 3))
	require.True(t, bitmapSet(got[6:14], 4))
	require.True(t, bitmapSet(got[6:14], 48))
	require.Contains(t, string(got[14:]), "164111111111111111")
	require.Contains(t, string(got[14:]), "000000")
	require.Contains(t, string(got[14:]), "000000010000")
	require.Contains(t, string(got[14:]), "005HELLO")
}

func TestCodecUnpacksMessage(t *testing.T) {
	codec := NewInternalCodec()
	profile := testProfile()

	packed, err := codec.Pack(profile, map[string]any{
		"mti": "0110",
		"3":   "000000",
		"4":   "000000010000",
		"39":  "00",
	})
	require.NoError(t, err)

	fields, err := codec.Unpack(profile, packed)

	require.NoError(t, err)
	require.Equal(t, "0110", fields["mti"])
	require.Equal(t, "000000", fields["3"])
	require.Equal(t, "000000010000", fields["4"])
	require.Equal(t, "00", fields["39"])
}

func TestCodecRejectsUndefinedProfileField(t *testing.T) {
	_, err := NewInternalCodec().Pack(testProfile(), map[string]any{
		"99": "not-defined",
	})

	require.ErrorContains(t, err, "field 99 is not supported")
}

func testProfile() Profile {
	return Profile{
		ID:           "default",
		MTI:          "0100",
		ResponseMTI:  "0110",
		LengthHeader: true,
		Fields: map[int]FieldSpec{
			2:  {ID: 2, Type: FieldLLVAR, Length: 19, Sensitive: true},
			3:  {ID: 3, Type: FieldFixed, Length: 6},
			4:  {ID: 4, Type: FieldFixed, Length: 12},
			11: {ID: 11, Type: FieldFixed, Length: 6},
			38: {ID: 38, Type: FieldFixed, Length: 6},
			39: {ID: 39, Type: FieldFixed, Length: 2},
			41: {ID: 41, Type: FieldFixed, Length: 8},
			48: {ID: 48, Type: FieldLLLVAR, Length: 999},
			49: {ID: 49, Type: FieldFixed, Length: 3},
		},
		SensitiveKeys: []string{"2"},
	}
}
