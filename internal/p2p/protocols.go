package p2p

import (
	"context"
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

	var req protocol.StoreRequest
	if err := s.readJSON(stream, &req, s.frameLimits.StoreRequest); err != nil {
		s.resetStream(stream, "store request read failed", remotePeerID, err)
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

	var req protocol.FetchRequest
	if err := s.readJSON(stream, &req, s.frameLimits.FetchRequest); err != nil {
		s.resetStream(stream, "fetch request read failed", remotePeerID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeouts.Handler)
	defer cancel()

	resp := s.fetchSvc.Handle(ctx, remotePeerID, &req)
	if err := s.writeJSON(stream, resp, s.frameLimits.FetchResponse); err != nil {
		s.resetStream(stream, "fetch response write failed", remotePeerID, err)
		return
	}
	s.closeStream(stream)
}

func (s *Server) HandleAck(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer().String()
	s.logger.Info("ack request received", "remote_peer_id", remotePeerID)

	var req protocol.AckRequest
	if err := s.readJSON(stream, &req, s.frameLimits.AckRequest); err != nil {
		s.resetStream(stream, "ack request read failed", remotePeerID, err)
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

func (s *Server) readJSON(stream network.Stream, payload any, maxSize uint32) error {
	if err := stream.SetReadDeadline(time.Now().Add(s.timeouts.Read)); err != nil {
		return err
	}
	return ReadJSON(stream, payload, maxSize)
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

func (s *Server) closeStream(stream network.Stream) {
	_ = stream.Close()
}
