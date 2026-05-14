package iso8583

import (
	"context"
	"encoding/binary"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/protocol"
)

func TestAdapterSendsMessageToTCPUpstreamAndUnpacksResponse(t *testing.T) {
	profile := testProfile()
	codec := NewInternalCodec()
	requestCh := make(chan []byte, 1)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		requestCh <- append([]byte(nil), buffer[:n]...)

		response, _ := codec.Pack(profile, map[string]any{
			"mti": "0110",
			"11":  "123456",
			"38":  "A12345",
			"39":  "00",
		})
		_, _ = conn.Write(response)
	}()

	adapter := NewAdapter(codec, NewInMemoryProfileStore(profile), nil)
	msg, err := adapter.Call(context.Background(), protocol.UpstreamTarget{
		ID:       "switch",
		TenantID: "tenant_1",
		Protocol: Name,
		BaseURL:  listener.Addr().String(),
		Metadata: map[string]string{
			"iso8583ProfileId": profile.ID,
		},
	}, protocol.CanonicalMessage{
		Fields: map[string]any{
			"2":  "4111111111111111",
			"3":  "000000",
			"4":  "000000010000",
			"11": "123456",
			"41": "ATM00101",
			"49": "360",
		},
	})

	require.NoError(t, err)
	request := <-requestCh
	require.Equal(t, len(request)-2, int(binary.BigEndian.Uint16(request[:2])))
	require.Equal(t, "0100", string(request[2:6]))
	require.Equal(t, "00", msg.Fields["39"])
	require.Equal(t, "A12345", msg.Fields["38"])
	require.Equal(t, "123456", msg.Fields["11"])
}
