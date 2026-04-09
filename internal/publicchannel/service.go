package publicchannel

import (
	"context"
	"log/slog"

	"store-node/internal/protocol"
)

// Service 公共频道存储 RPC 业务层。
type Service struct {
	store  *Store
	logger *slog.Logger
}

func NewService(store *Store, logger *slog.Logger) *Service {
	return &Service{store: store, logger: logger}
}

func (s *Service) Push(ctx context.Context, remotePeerID string, req *PushRequest) *PushResponse {
	if req == nil || req.Profile == nil || req.Head == nil {
		return &PushResponse{OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: "profile and head are required"}
	}
	var extra []*ChannelChange
	if len(req.Changes) > 0 {
		extra = req.Changes
	}
	if err := s.store.Push(ctx, remotePeerID, req.Profile, req.Head, req.Messages, extra); err != nil {
		return &PushResponse{OK: false, ErrorCode: protocol.ErrorCode(err), ErrorMessage: protocol.ErrorMessage(err)}
	}
	return &PushResponse{OK: true}
}

func (s *Service) GetProfile(ctx context.Context, channelID string) *GetProfileResponse {
	if channelID == "" {
		return &GetProfileResponse{OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: "channel_id is required"}
	}
	p, err := s.store.GetProfile(ctx, channelID)
	if err != nil {
		return &GetProfileResponse{OK: false, ErrorCode: protocol.CodeInternalError, ErrorMessage: err.Error()}
	}
	if p == nil {
		return &GetProfileResponse{OK: false, ErrorCode: protocol.CodePublicChannelNotFound, ErrorMessage: "channel not found"}
	}
	return &GetProfileResponse{OK: true, Profile: p}
}

func (s *Service) GetHead(ctx context.Context, channelID string) *GetHeadResponse {
	if channelID == "" {
		return &GetHeadResponse{OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: "channel_id is required"}
	}
	h, err := s.store.GetHead(ctx, channelID)
	if err != nil {
		return &GetHeadResponse{OK: false, ErrorCode: protocol.CodeInternalError, ErrorMessage: err.Error()}
	}
	if h == nil {
		return &GetHeadResponse{OK: false, ErrorCode: protocol.CodePublicChannelNotFound, ErrorMessage: "channel not found"}
	}
	return &GetHeadResponse{OK: true, Head: h}
}

func (s *Service) ListMessages(ctx context.Context, req *ListMessagesRequest) *ListMessagesResponse {
	if req == nil || req.ChannelID == "" {
		return &ListMessagesResponse{OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: "channel_id is required"}
	}
	ms, err := s.store.ListMessages(ctx, req.ChannelID, req.BeforeMessageID, req.Limit)
	if err != nil {
		return &ListMessagesResponse{OK: false, ErrorCode: protocol.CodeInternalError, ErrorMessage: err.Error()}
	}
	if ms == nil {
		return &ListMessagesResponse{OK: false, ErrorCode: protocol.CodePublicChannelNotFound, ErrorMessage: "channel not found"}
	}
	return &ListMessagesResponse{OK: true, Messages: ms}
}

func (s *Service) GetMessage(ctx context.Context, channelID string, messageID int) *GetMessageResponse {
	if channelID == "" {
		return &GetMessageResponse{OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: "channel_id is required"}
	}
	m, err := s.store.GetMessage(ctx, channelID, messageID)
	if err != nil {
		return &GetMessageResponse{OK: false, ErrorCode: protocol.CodeInternalError, ErrorMessage: err.Error()}
	}
	if m == nil {
		return &GetMessageResponse{OK: false, ErrorCode: protocol.CodePublicChannelNotFound, ErrorMessage: "message not found"}
	}
	return &GetMessageResponse{OK: true, Message: m}
}

func (s *Service) SyncChannel(ctx context.Context, req *SyncChannelRequest) *SyncChannelResponse {
	if req == nil || req.ChannelID == "" {
		return &SyncChannelResponse{OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: "channel_id is required"}
	}
	res, err := s.store.SyncChanges(ctx, req.ChannelID, req.AfterSeq, req.Limit)
	if err != nil {
		return &SyncChannelResponse{OK: false, ErrorCode: protocol.CodeInternalError, ErrorMessage: err.Error()}
	}
	if res == nil {
		return &SyncChannelResponse{OK: false, ErrorCode: protocol.CodePublicChannelNotFound, ErrorMessage: "channel not found"}
	}
	return &SyncChannelResponse{
		OK:             true,
		ChannelID:      res.ChannelID,
		CurrentLastSeq: res.CurrentLastSeq,
		HasMore:        res.HasMore,
		NextAfterSeq:   res.NextAfterSeq,
		Items:          res.Items,
	}
}
