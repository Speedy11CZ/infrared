package login

import (
	"testing"

	"github.com/haveachin/infrared/internal/pkg/java/protocol"
)

func TestUnmarshalServerBoundLoginStart(t *testing.T) {
	tt := []struct {
		packet             protocol.Packet
		version            protocol.Version
		unmarshalledPacket ServerLoginStart
	}{
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x00},
			},
			version: protocol.Version_1_18_2,
			unmarshalledPacket: ServerLoginStart{
				Name: protocol.String(""),
			},
		},
		{
			packet: protocol.Packet{
				ID:   0x00,
				Data: []byte{0x0d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21},
			},
			version: protocol.Version_1_18_2,
			unmarshalledPacket: ServerLoginStart{
				Name: protocol.String("Hello, World!"),
			},
		},
	}

	for _, tc := range tt {
		loginStart, err := UnmarshalServerBoundLoginStart(tc.packet, tc.version)
		if err != nil {
			t.Error(err)
		}

		if loginStart.Name != tc.unmarshalledPacket.Name {
			t.Errorf("got: %v, want: %v", loginStart.Name, tc.unmarshalledPacket.Name)
		}
	}
}
