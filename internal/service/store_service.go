package service

import (
	"context"
	"log/slog"

	"store-node/internal/auth"
	"store-node/internal/metrics"
	"store-node/internal/protocol"
	"store-node/internal/ratelimit"
	"store-node/internal/storage"
)

type QuotaStorage interface {
	storage.Storage
	StoreMessageWithQuota(ctx context.Context, msg *protocol.StoredMessage, maxMessages int, maxBytes int64) (storeSeq uint64, expireAt int64, duplicate bool, err error)
}

type StoreService struct {
	storage                 QuotaStorage
	verifier                auth.Verifier
	remoteLimiter           *ratelimit.Limiter
	senderLimiter           *ratelimit.Limiter
	metrics                 *metrics.Metrics
	logger                  *slog.Logger
	defaultTTLSec           int64
	maxTTLSec               int64
	maxMessageSize          int
	maxMessagesPerRecipient int
	maxBytesPerRecipient    int64
}

func NewStoreService(
	st QuotaStorage,
	verifier auth.Verifier,
	remoteLimiter *ratelimit.Limiter,
	senderLimiter *ratelimit.Limiter,
	metrics *metrics.Metrics,
	logger *slog.Logger,
	defaultTTLSec int64,
	maxTTLSec int64,
	maxMessageSize int,
	maxMessagesPerRecipient int,
	maxBytesPerRecipient int64,
) *StoreService {
	return &StoreService{
		storage:                 st,
		verifier:                verifier,
		remoteLimiter:           remoteLimiter,
		senderLimiter:           senderLimiter,
		metrics:                 metrics,
		logger:                  logger,
		defaultTTLSec:           defaultTTLSec,
		maxTTLSec:               maxTTLSec,
		maxMessageSize:          maxMessageSize,
		maxMessagesPerRecipient: maxMessagesPerRecipient,
		maxBytesPerRecipient:    maxBytesPerRecipient,
	}
}

func (s *StoreService) Handle(ctx context.Context, remotePeerID string, req *protocol.StoreRequest) *protocol.StoreResponse {
	s.metrics.StoreRequestsTotal.Add(1)
	if err := validateStoreRequest(req); err != nil {
		return s.errorResponse(err)
	}

	msg := req.Message.Clone()
	if msg.TTLSec == nil {
		ttl := s.defaultTTLSec
		msg.TTLSec = &ttl
	}
	ttl := msg.EffectiveTTL(s.defaultTTLSec)
	if ttl <= 0 {
		return s.errorResponse(protocol.NewAppError(protocol.CodeInvalidPayload, "ttl_sec must be positive"))
	}
	if ttl > s.maxTTLSec {
		return s.errorResponse(protocol.NewAppError(protocol.CodeTTLTooLarge, "ttl_sec exceeds max_ttl_sec"))
	}

	if !s.remoteLimiter.Allow(remotePeerID) {
		s.metrics.RateLimitedTotal.Add(1)
		s.logger.Warn("store remote rate limited", "sender_id", msg.SenderID, "recipient_id", msg.RecipientID, "remote_peer_id", remotePeerID, "error_code", protocol.CodeRateLimited)
		return s.errorResponse(protocol.NewAppError(protocol.CodeRateLimited, "remote peer rate limit exceeded"))
	}

	if err := s.verifier.VerifyMessageEnvelope(msg); err != nil {
		s.metrics.InvalidSignatureTotal.Add(1)
		s.logger.Warn("store signature verify failed", "sender_id", msg.SenderID, "recipient_id", msg.RecipientID, "msg_id", msg.MsgID, "remote_peer_id", remotePeerID, "error_code", protocol.ErrorCode(err))
		return s.errorResponse(err)
	}

	if !s.senderLimiter.Allow(msg.SenderID) {
		s.metrics.RateLimitedTotal.Add(1)
		s.logger.Warn("store sender rate limited", "sender_id", msg.SenderID, "recipient_id", msg.RecipientID, "remote_peer_id", remotePeerID, "error_code", protocol.CodeRateLimited)
		return s.errorResponse(protocol.NewAppError(protocol.CodeRateLimited, "sender rate limit exceeded"))
	}

	size, err := messageSize(msg)
	if err != nil {
		return s.errorResponse(protocol.NewAppError(protocol.CodeInternalError, err.Error()))
	}
	if size > s.maxMessageSize {
		s.logger.Warn("store message too large", "sender_id", msg.SenderID, "recipient_id", msg.RecipientID, "msg_id", msg.MsgID, "remote_peer_id", remotePeerID, "error_code", protocol.CodeMessageTooLarge)
		return s.errorResponse(protocol.NewAppError(protocol.CodeMessageTooLarge, "message exceeds max_message_size"))
	}

	storeSeq, expireAt, duplicate, err := s.storage.StoreMessageWithQuota(ctx, &protocol.StoredMessage{Message: msg}, s.maxMessagesPerRecipient, s.maxBytesPerRecipient)
	if err != nil {
		code := protocol.ErrorCode(err)
		if code == protocol.CodeRecipientQuotaExceeded {
			s.logger.Warn("store recipient quota exceeded", "recipient_id", msg.RecipientID, "msg_id", msg.MsgID, "remote_peer_id", remotePeerID, "error_code", code)
			return s.errorResponse(err)
		}
		if code == protocol.CodeRecipientBytesExceeded {
			s.logger.Warn("store recipient bytes exceeded", "recipient_id", msg.RecipientID, "msg_id", msg.MsgID, "remote_peer_id", remotePeerID, "error_code", code)
			return s.errorResponse(err)
		}
		return s.errorResponse(protocol.NewAppError(protocol.CodeInternalError, err.Error()))
	}

	if duplicate {
		s.metrics.StoreDuplicateTotal.Add(1)
		s.logger.Info("store duplicate", "sender_id", msg.SenderID, "recipient_id", msg.RecipientID, "msg_id", msg.MsgID, "store_seq", storeSeq, "remote_peer_id", remotePeerID)
		return &protocol.StoreResponse{
			OK:        true,
			Duplicate: true,
			StoreSeq:  storeSeq,
			ExpireAt:  expireAt,
		}
	}

	s.metrics.StoreSuccessTotal.Add(1)
	s.logger.Info("store persisted", "sender_id", msg.SenderID, "recipient_id", msg.RecipientID, "msg_id", msg.MsgID, "store_seq", storeSeq, "expire_at", expireAt, "remote_peer_id", remotePeerID)
	return &protocol.StoreResponse{
		OK:        true,
		Duplicate: false,
		StoreSeq:  storeSeq,
		ExpireAt:  expireAt,
	}
}

func (s *StoreService) errorResponse(err error) *protocol.StoreResponse {
	return &protocol.StoreResponse{
		OK:           false,
		ErrorCode:    protocol.ErrorCode(err),
		ErrorMessage: protocol.ErrorMessage(err),
	}
}
