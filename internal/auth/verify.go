package auth

import (
	"encoding/base64"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	"store-node/internal/protocol"
)

type Verifier interface {
	VerifyMessageEnvelope(msg *protocol.OfflineMessageEnvelope) error
	VerifyAck(req *protocol.AckRequest) error
}

type PeerVerifier struct {
	DefaultTTLSec int64
}

func NewVerifier(defaultTTLSec int64) *PeerVerifier {
	return &PeerVerifier{DefaultTTLSec: defaultTTLSec}
}

func (v *PeerVerifier) VerifyMessageEnvelope(msg *protocol.OfflineMessageEnvelope) error {
	if msg == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "message is required")
	}
	if msg.Signature.Algorithm != "ed25519" {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "unsupported signature algorithm")
	}
	pid, err := peer.Decode(msg.SenderID)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "invalid sender_id")
	}
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "sender public key not available")
	}
	sig, err := base64.StdEncoding.DecodeString(msg.Signature.Value)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "invalid signature encoding")
	}
	payload, err := CanonicalMessageEnvelope(msg, v.DefaultTTLSec)
	if err != nil {
		return fmt.Errorf("canonical message: %w", err)
	}
	ok, err := pubKey.Verify(payload, sig)
	if err != nil || !ok {
		return protocol.NewAppError(protocol.CodeInvalidSignature, "message signature verification failed")
	}
	return nil
}

func (v *PeerVerifier) VerifyAck(req *protocol.AckRequest) error {
	if req == nil {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "ack is required")
	}
	if req.Signature.Algorithm != "ed25519" {
		return protocol.NewAppError(protocol.CodeInvalidAckSignature, "unsupported signature algorithm")
	}
	pid, err := peer.Decode(req.RecipientID)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidAckSignature, "invalid recipient_id")
	}
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidAckSignature, "recipient public key not available")
	}
	sig, err := base64.StdEncoding.DecodeString(req.Signature.Value)
	if err != nil {
		return protocol.NewAppError(protocol.CodeInvalidAckSignature, "invalid signature encoding")
	}
	payload, err := CanonicalAck(req)
	if err != nil {
		return fmt.Errorf("canonical ack: %w", err)
	}
	ok, err := pubKey.Verify(payload, sig)
	if err != nil || !ok {
		return protocol.NewAppError(protocol.CodeInvalidAckSignature, "ack signature verification failed")
	}
	return nil
}
