package p2p

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	corehost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"

	"store-node/internal/protocol"
	"store-node/internal/service"
)

const (
	StoreProtocol = "/chat/offline/store/1.0.0"
	FetchProtocol = "/chat/offline/fetch/1.0.0"
	AckProtocol   = "/chat/offline/ack/1.0.0"
)

type ProtocolServer interface {
	HandleStore(stream network.Stream)
	HandleFetch(stream network.Stream)
	HandleAck(stream network.Stream)
}

type FrameLimits struct {
	StoreRequest  uint32
	StoreResponse uint32
	FetchRequest  uint32
	FetchResponse uint32
	AckRequest    uint32
	AckResponse   uint32
}

type Timeouts struct {
	Read    time.Duration
	Write   time.Duration
	Handler time.Duration
}

type Server struct {
	host        corehost.Host
	logger      *slog.Logger
	frameLimits FrameLimits
	timeouts    Timeouts
	storeSvc    *service.StoreService
	fetchSvc    *service.FetchService
	ackSvc      *service.AckService
}

func NewServer(
	host corehost.Host,
	logger *slog.Logger,
	frameLimits FrameLimits,
	timeouts Timeouts,
	storeSvc *service.StoreService,
	fetchSvc *service.FetchService,
	ackSvc *service.AckService,
) *Server {
	return &Server{
		host:        host,
		logger:      logger,
		frameLimits: frameLimits,
		timeouts:    timeouts,
		storeSvc:    storeSvc,
		fetchSvc:    fetchSvc,
		ackSvc:      ackSvc,
	}
}

func (s *Server) Register() {
	s.host.SetStreamHandler(StoreProtocol, s.HandleStore)
	s.host.SetStreamHandler(FetchProtocol, s.HandleFetch)
	s.host.SetStreamHandler(AckProtocol, s.HandleAck)
}

func (s *Server) HandleStore(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer().String()
	s.logger.Info("store request received", "remote_peer_id", remotePeerID)

	frame, err := s.readFrame(stream, s.frameLimits.StoreRequest)
	if err != nil {
		s.resetStream(stream, "store request read failed", remotePeerID, err)
		return
	}
	var req protocol.StoreRequest
	if err := json.Unmarshal(frame, &req); err != nil {
		s.writeInvalidStoreResponse(stream, remotePeerID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeouts.Handler)
	defer cancel()

	resp := s.storeSvc.Handle(ctx, remotePeerID, &req)
	if err := s.writeJSON(stream, resp, s.frameLimits.StoreResponse); err != nil {
		s.resetStream(stream, "store response write failed", remotePeerID, err)
		return
	}
	s.closeStream(stream)
}

func (s *Server) HandleFetch(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer().String()
	s.logger.Info("fetch request received", "remote_peer_id", remotePeerID)

	frame, err := s.readFrame(stream, s.frameLimits.FetchRequest)
	if err != nil {
		s.resetStream(stream, "fetch request read failed", remotePeerID, err)
		return
	}
	var req protocol.FetchRequest
	if err := json.Unmarshal(frame, &req); err != nil {
		s.writeInvalidFetchResponse(stream, remotePeerID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeouts.Handler)
	defer cancel()

	resp := s.fetchSvc.Handle(ctx, remotePeerID, &req)
	if err := s.writeJSON(stream, resp, s.frameLimits.FetchResponse); err != nil {
		s.resetStream(stream, "fetch response write failed", remotePeerID, err)
		return
	}
	if resp.OK {
		if err := s.fetchSvc.MarkDelivered(ctx, req.RecipientID, resp.Items); err != nil {
			s.logger.Error("mark delivered failed", "recipient_id", req.RecipientID, "remote_peer_id", remotePeerID, "error", err)
		}
	}
	s.closeStream(stream)
}

func (s *Server) HandleAck(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer().String()
	s.logger.Info("ack request received", "remote_peer_id", remotePeerID)

	frame, err := s.readFrame(stream, s.frameLimits.AckRequest)
	if err != nil {
		s.resetStream(stream, "ack request read failed", remotePeerID, err)
		return
	}
	var req protocol.AckRequest
	if err := json.Unmarshal(frame, &req); err != nil {
		s.writeInvalidAckResponse(stream, remotePeerID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeouts.Handler)
	defer cancel()

	resp := s.ackSvc.Handle(ctx, remotePeerID, &req)
	if err := s.writeJSON(stream, resp, s.frameLimits.AckResponse); err != nil {
		s.resetStream(stream, "ack response write failed", remotePeerID, err)
		return
	}
	s.closeStream(stream)
}

func (s *Server) readFrame(stream network.Stream, maxSize uint32) ([]byte, error) {
	if err := stream.SetReadDeadline(time.Now().Add(s.timeouts.Read)); err != nil {
		return nil, err
	}
	return ReadFrame(stream, maxSize)
}

func (s *Server) writeJSON(stream network.Stream, payload any, maxSize uint32) error {
	if err := stream.SetWriteDeadline(time.Now().Add(s.timeouts.Write)); err != nil {
		return err
	}
	return WriteJSON(stream, payload, maxSize)
}

func (s *Server) resetStream(stream network.Stream, message string, remotePeerID string, err error) {
	s.logger.Warn(message, "remote_peer_id", remotePeerID, "error", err)
	_ = stream.Reset()
}

func (s *Server) writeInvalidStoreResponse(stream network.Stream, remotePeerID string, err error) {
	resp := &protocol.StoreResponse{
		OK:           false,
		ErrorCode:    protocol.CodeInvalidPayload,
		ErrorMessage: err.Error(),
	}
	if writeErr := s.writeJSON(stream, resp, s.frameLimits.StoreResponse); writeErr != nil {
		s.resetStream(stream, "store invalid payload response write failed", remotePeerID, writeErr)
		return
	}
	s.closeStream(stream)
}

func (s *Server) writeInvalidFetchResponse(stream network.Stream, remotePeerID string, err error) {
	resp := &protocol.FetchResponse{
		OK:           false,
		Items:        []*protocol.StoredMessage{},
		ErrorCode:    protocol.CodeInvalidPayload,
		ErrorMessage: err.Error(),
	}
	if writeErr := s.writeJSON(stream, resp, s.frameLimits.FetchResponse); writeErr != nil {
		s.resetStream(stream, "fetch invalid payload response write failed", remotePeerID, writeErr)
		return
	}
	s.closeStream(stream)
}

func (s *Server) writeInvalidAckResponse(stream network.Stream, remotePeerID string, err error) {
	resp := &protocol.AckResponse{
		OK:           false,
		ErrorCode:    protocol.CodeInvalidPayload,
		ErrorMessage: err.Error(),
	}
	if writeErr := s.writeJSON(stream, resp, s.frameLimits.AckResponse); writeErr != nil {
		s.resetStream(stream, "ack invalid payload response write failed", remotePeerID, writeErr)
		return
	}
	s.closeStream(stream)
}

func (s *Server) closeStream(stream network.Stream) {
	_ = stream.Close()
}
