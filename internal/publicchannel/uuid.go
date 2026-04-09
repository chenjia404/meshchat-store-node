package publicchannel

import (
	"github.com/google/uuid"

	"store-node/internal/protocol"
)

// ValidateChannelID 要求 channel_id 为 UUIDv7 字符串。
func ValidateChannelID(id string) error {
	u, err := uuid.Parse(id)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "invalid channel_id")
	}
	if u.Version() != 7 {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "channel_id must be UUIDv7")
	}
	return nil
}
