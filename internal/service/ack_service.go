package service

import (
	"context"
	"log/slog"

	"store-node/internal/auth"
	"store-node/internal/metrics"
	"store-node/internal/protocol"
	"store-node/internal/storage"
)

type AckService struct {
	storage  storage.Storage
	verifier auth.Verifier
	metrics  *metrics.Metrics
	logger   *slog.Logger
}

func NewAckService(st storage.Storage, verifier auth.Verifier, metrics *metrics.Metrics, logger *slog.Logger) *AckService {
	return &AckService{
		storage:  st,
		verifier: verifier,
		metrics:  metrics,
		logger:   logger,
	}
}

func (s *AckService) Handle(ctx context.Context, remotePeerID string, req *protocol.AckRequest) *protocol.AckResponse {
	s.metrics.AckRequestsTotal.Add(1)
	if err := validateAckRequest(req); err != nil {
		return s.errorResponse(err)
	}
	if err := s.verifier.VerifyAck(req); err != nil {
		s.metrics.InvalidSignatureTotal.Add(1)
		s.logger.Warn("ack signature verify failed", "recipient_id", req.RecipientID, "ack_seq", req.AckSeq, "remote_peer_id", remotePeerID, "error_code", protocol.ErrorCode(err))
		return s.errorResponse(err)
	}
	if remotePeerID != req.RecipientID {
		s.logger.Warn("ack unauthorized", "recipient_id", req.RecipientID, "ack_seq", req.AckSeq, "remote_peer_id", remotePeerID, "error_code", protocol.CodeUnauthorized)
		return s.errorResponse(protocol.NewAppError(protocol.CodeUnauthorized, "remote peer is not recipient"))
	}

	deletedUntil, deletedCount, err := s.storage.AckMessages(ctx, req.RecipientID, req.AckSeq)
	if err != nil {
		s.logger.Warn("ack rejected", "recipient_id", req.RecipientID, "ack_seq", req.AckSeq, "remote_peer_id", remotePeerID, "error_code", protocol.ErrorCode(err))
		return s.errorResponse(err)
	}
	if deletedCount > 0 {
		s.metrics.AckDeletedTotal.Add(uint64(deletedCount))
	}
	s.logger.Info("ack completed", "recipient_id", req.RecipientID, "ack_seq", req.AckSeq, "deleted_until_seq", deletedUntil, "deleted_count", deletedCount, "remote_peer_id", remotePeerID)
	return &protocol.AckResponse{
		OK:              true,
		DeletedUntilSeq: deletedUntil,
	}
}

func (s *AckService) errorResponse(err error) *protocol.AckResponse {
	return &protocol.AckResponse{
		OK:           false,
		ErrorCode:    protocol.ErrorCode(err),
		ErrorMessage: protocol.ErrorMessage(err),
	}
}
