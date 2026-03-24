package auth

import (
	"encoding/json"

	"store-node/internal/protocol"
)

func CanonicalMessageEnvelope(msg *protocol.OfflineMessageEnvelope, defaultTTL int64) ([]byte, error) {
	ttl := msg.EffectiveTTL(defaultTTL)
	return json.Marshal([]any{
		msg.Version,
		msg.MsgID,
		msg.SenderID,
		msg.RecipientID,
		msg.ConversationID,
		msg.CreatedAt,
		ttl,
		msg.Cipher.Algorithm,
		msg.Cipher.RecipientKeyID,
		msg.Cipher.Nonce,
		msg.Cipher.Ciphertext,
	})
}

func CanonicalAck(req *protocol.AckRequest) ([]byte, error) {
	return json.Marshal([]any{
		req.Version,
		req.RecipientID,
		req.DeviceID,
		req.AckSeq,
		req.AckedAt,
	})
}
