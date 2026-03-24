package service

import (
	"context"
	"log/slog"

	"store-node/internal/metrics"
	"store-node/internal/protocol"
	"store-node/internal/storage"
)

type FetchService struct {
	storage       storage.Storage
	metrics       *metrics.Metrics
	logger        *slog.Logger
	fetchLimitMax int
}

func NewFetchService(st storage.Storage, metrics *metrics.Metrics, logger *slog.Logger, fetchLimitMax int) *FetchService {
	return &FetchService{
		storage:       st,
		metrics:       metrics,
		logger:        logger,
		fetchLimitMax: fetchLimitMax,
	}
}

func (s *FetchService) Handle(ctx context.Context, remotePeerID string, req *protocol.FetchRequest) *protocol.FetchResponse {
	s.metrics.FetchRequestsTotal.Add(1)
	if err := validateFetchRequest(req, s.fetchLimitMax); err != nil {
		return s.errorResponse(err)
	}
	if remotePeerID != req.RecipientID {
		s.logger.Warn("fetch unauthorized", "recipient_id", req.RecipientID, "remote_peer_id", remotePeerID, "after_seq", req.AfterSeq, "limit", req.Limit, "error_code", protocol.CodeUnauthorized)
		return s.errorResponse(protocol.NewAppError(protocol.CodeUnauthorized, "remote peer is not recipient"))
	}

	items, hasMore, err := s.storage.FetchMessages(ctx, req.RecipientID, req.AfterSeq, req.Limit)
	if err != nil {
		return s.errorResponse(protocol.NewAppError(protocol.CodeInternalError, err.Error()))
	}
	s.metrics.FetchMessagesReturnedTotal.Add(uint64(len(items)))
	s.logger.Info("fetch completed", "recipient_id", req.RecipientID, "remote_peer_id", remotePeerID, "after_seq", req.AfterSeq, "limit", req.Limit, "returned", len(items), "has_more", hasMore)
	return &protocol.FetchResponse{
		OK:      true,
		Items:   items,
		HasMore: hasMore,
	}
}

func (s *FetchService) errorResponse(err error) *protocol.FetchResponse {
	return &protocol.FetchResponse{
		OK:           false,
		Items:        []*protocol.StoredMessage{},
		ErrorCode:    protocol.ErrorCode(err),
		ErrorMessage: protocol.ErrorMessage(err),
	}
}
