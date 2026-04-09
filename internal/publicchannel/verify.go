package publicchannel

import (
	"encoding/base64"

	"github.com/libp2p/go-libp2p/core/peer"

	"store-node/internal/protocol"
)

func verifyPeerSignature(ownerPeerID string, payload []byte, signatureB64 string) error {
	if signatureB64 == "" {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "signature is required")
	}
	pid, err := peer.Decode(ownerPeerID)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "invalid owner_peer_id")
	}
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "owner public key not available")
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "invalid signature encoding")
	}
	ok, err := pubKey.Verify(payload, sig)
	if err != nil || !ok {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "signature verification failed")
	}
	return nil
}

// VerifyProfile 验证 Profile 签名（owner_peer_id）。
func VerifyProfile(p *ChannelProfile) error {
	if p == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "profile is required")
	}
	payload, err := CanonicalProfile(p)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, err.Error())
	}
	return verifyPeerSignature(p.OwnerPeerID, payload, p.Signature)
}

// VerifyHead 验证 ChannelHead 签名。
func VerifyHead(h *ChannelHead) error {
	if h == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "head is required")
	}
	payload, err := CanonicalHead(h)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, err.Error())
	}
	return verifyPeerSignature(h.OwnerPeerID, payload, h.Signature)
}

// VerifyMessage 验证消息签名（owner 代签，与 owner_peer_id 一致）。
func VerifyMessage(ownerPeerID string, m *ChannelMessage) error {
	if m == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "message is nil")
	}
	if m.AuthorPeerID != ownerPeerID {
		return protocol.NewAppError(protocol.CodeUnauthorized, "author_peer_id must equal owner in v1")
	}
	if m.OwnerVersion != 1 {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "owner_version must be 1 in v1")
	}
	payload, err := CanonicalMessage(m)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, err.Error())
	}
	return verifyPeerSignature(ownerPeerID, payload, m.Signature)
}
