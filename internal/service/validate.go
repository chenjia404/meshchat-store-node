package service

import (
	"encoding/json"

	"store-node/internal/protocol"
)

func validateStoreRequest(req *protocol.StoreRequest) error {
	if req == nil || req.Message == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "message is required")
	}
	msg := req.Message
	switch {
	case req.Version != 1:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "store request version must be 1")
	case msg.Version != 1:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "message version must be 1")
	case msg.MsgID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "msg_id is required")
	case msg.SenderID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "sender_id is required")
	case msg.RecipientID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "recipient_id is required")
	case msg.ConversationID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "conversation_id is required")
	case msg.CreatedAt <= 0:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "created_at is required")
	case msg.Cipher.Algorithm == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "cipher.algorithm is required")
	case msg.Cipher.RecipientKeyID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "cipher.recipient_key_id is required")
	case msg.Cipher.Nonce == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "cipher.nonce is required")
	case msg.Cipher.Ciphertext == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "cipher.ciphertext is required")
	case msg.Signature.Algorithm == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "signature.algorithm is required")
	case msg.Signature.Value == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "signature.value is required")
	default:
		return nil
	}
}

func validateFetchRequest(req *protocol.FetchRequest, fetchLimitMax int) error {
	if req == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "fetch request is required")
	}
	switch {
	case req.Version != 1:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "fetch request version must be 1")
	case req.RecipientID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "recipient_id is required")
	case req.Limit <= 0:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "limit must be positive")
	case req.Limit > fetchLimitMax:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "limit exceeds fetch_limit_max")
	default:
		return nil
	}
}

func validateAckRequest(req *protocol.AckRequest) error {
	if req == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "ack request is required")
	}
	switch {
	case req.Version != 1:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "ack request version must be 1")
	case req.RecipientID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "recipient_id is required")
	case req.DeviceID == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "device_id is required")
	case req.DeviceID != req.RecipientID:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "v1 device_id must equal recipient_id")
	case req.AckedAt <= 0:
		return protocol.NewAppError(protocol.CodeInvalidPayload, "acked_at is required")
	case req.Signature.Algorithm == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "signature.algorithm is required")
	case req.Signature.Value == "":
		return protocol.NewAppError(protocol.CodeInvalidPayload, "signature.value is required")
	default:
		return nil
	}
}

func messageSize(msg *protocol.OfflineMessageEnvelope) (int, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return 0, err
	}
	return len(payload), nil
}
