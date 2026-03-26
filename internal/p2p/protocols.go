package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	corehost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"

	"store-node/internal/protocol"
	"store-node/internal/service"
)

const RPCProtocol = "/meshchat/offline-store/rpc/1.0.0"

type FrameLimits struct {
	RPCRequest  uint32
	RPCResponse uint32
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
	s.host.SetStreamHandler(RPCProtocol, s.HandleRPC)
}

func (s *Server) HandleRPC(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer().String()
	s.logger.Info("rpc request received", "remote_peer_id", remotePeerID)

	frame, err := s.readFrame(stream, s.frameLimits.RPCRequest)
	if err != nil {
		s.resetStream(stream, "rpc request read failed", remotePeerID, err)
		return
	}

	var rpcReq protocol.RPCRequest
	if err := json.Unmarshal(frame, &rpcReq); err != nil {
		s.writeRPCError(stream, "", protocol.CodeRPCInvalidRequest, err.Error(), remotePeerID)
		return
	}
	if rpcReq.RequestID == "" {
		s.writeRPCError(stream, "", protocol.CodeRPCMissingRequestID, "request_id is required", remotePeerID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeouts.Handler)
	defer cancel()

	switch rpcReq.Method {
	case protocol.MethodOfflineStore:
		s.handleStoreRPC(ctx, stream, remotePeerID, rpcReq)
	case protocol.MethodOfflineFetch:
		s.handleFetchRPC(ctx, stream, remotePeerID, rpcReq)
	case protocol.MethodOfflineAck:
		s.handleAckRPC(ctx, stream, remotePeerID, rpcReq)
	default:
		s.writeRPCUnknownMethod(stream, rpcReq.RequestID, rpcReq.Method, remotePeerID)
	}
}

func (s *Server) handleStoreRPC(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req protocol.StoreRequest
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		resp := &protocol.StoreResponse{
			OK:           false,
			ErrorCode:    protocol.CodeInvalidPayload,
			ErrorMessage: err.Error(),
		}
		s.writeRPCFromStore(stream, rpcReq.RequestID, resp, remotePeerID)
		return
	}
	resp := s.storeSvc.Handle(ctx, remotePeerID, &req)
	s.writeRPCFromStore(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *Server) handleFetchRPC(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req protocol.FetchRequest
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		resp := &protocol.FetchResponse{
			OK:           false,
			Items:        []*protocol.StoredMessage{},
			ErrorCode:    protocol.CodeInvalidPayload,
			ErrorMessage: err.Error(),
		}
		s.writeRPCFromFetch(stream, rpcReq.RequestID, resp, remotePeerID)
		return
	}
	resp := s.fetchSvc.Handle(ctx, remotePeerID, &req)
	if resp.OK {
		if err := s.fetchSvc.MarkDelivered(ctx, req.RecipientID, resp.Items); err != nil {
			s.logger.Error("mark delivered failed", "recipient_id", req.RecipientID, "remote_peer_id", remotePeerID, "error", err)
		}
	}
	s.writeRPCFromFetch(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *Server) handleAckRPC(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req protocol.AckRequest
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		resp := &protocol.AckResponse{
			OK:           false,
			ErrorCode:    protocol.CodeInvalidPayload,
			ErrorMessage: err.Error(),
		}
		s.writeRPCFromAck(stream, rpcReq.RequestID, resp, remotePeerID)
		return
	}
	resp := s.ackSvc.Handle(ctx, remotePeerID, &req)
	s.writeRPCFromAck(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *Server) writeRPCFromStore(stream network.Stream, requestID string, inner *protocol.StoreResponse, remotePeerID string) {
	body, err := json.Marshal(inner)
	if err != nil {
		s.writeRPCError(stream, requestID, protocol.CodeInternalError, err.Error(), remotePeerID)
		return
	}
	out := protocol.RPCResponse{
		RequestID: requestID,
		OK:        inner.OK,
		Error:     rpcErrorString(inner.OK, inner.ErrorMessage),
		Body:      body,
	}
	s.writeRPC(stream, out, remotePeerID)
}

func (s *Server) writeRPCFromFetch(stream network.Stream, requestID string, inner *protocol.FetchResponse, remotePeerID string) {
	body, err := json.Marshal(inner)
	if err != nil {
		s.writeRPCError(stream, requestID, protocol.CodeInternalError, err.Error(), remotePeerID)
		return
	}
	out := protocol.RPCResponse{
		RequestID: requestID,
		OK:        inner.OK,
		Error:     rpcErrorString(inner.OK, inner.ErrorMessage),
		Body:      body,
	}
	s.writeRPC(stream, out, remotePeerID)
}

func (s *Server) writeRPCFromAck(stream network.Stream, requestID string, inner *protocol.AckResponse, remotePeerID string) {
	body, err := json.Marshal(inner)
	if err != nil {
		s.writeRPCError(stream, requestID, protocol.CodeInternalError, err.Error(), remotePeerID)
		return
	}
	out := protocol.RPCResponse{
		RequestID: requestID,
		OK:        inner.OK,
		Error:     rpcErrorString(inner.OK, inner.ErrorMessage),
		Body:      body,
	}
	s.writeRPC(stream, out, remotePeerID)
}

func rpcErrorString(ok bool, msg string) string {
	if ok {
		return ""
	}
	return msg
}

func (s *Server) writeRPCUnknownMethod(stream network.Stream, requestID string, method string, remotePeerID string) {
	msg := fmt.Sprintf("unknown method: %q", method)
	body, err := json.Marshal(protocol.RPCErrorBody{
		ErrorCode:    protocol.CodeRPCUnknownMethod,
		ErrorMessage: msg,
		Method:       method,
	})
	if err != nil {
		s.writeRPCError(stream, requestID, protocol.CodeInternalError, fmt.Sprintf("marshal rpc error body: %v", err), remotePeerID)
		return
	}
	s.writeRPC(stream, protocol.RPCResponse{
		RequestID: requestID,
		OK:        false,
		Error:     msg,
		Body:      body,
	}, remotePeerID)
}

func (s *Server) writeRPCError(stream network.Stream, requestID string, code string, errMsg string, remotePeerID string) {
	body, err := json.Marshal(protocol.RPCErrorBody{
		ErrorCode:    code,
		ErrorMessage: errMsg,
	})
	if err != nil {
		s.writeRPC(stream, protocol.RPCResponse{
			RequestID: requestID,
			OK:        false,
			Error:     "marshal rpc error body failed",
			Body:      []byte(`{"error_code":"INTERNAL_ERROR","error_message":"marshal rpc error body failed"}`),
		}, remotePeerID)
		return
	}
	s.writeRPC(stream, protocol.RPCResponse{
		RequestID: requestID,
		OK:        false,
		Error:     errMsg,
		Body:      body,
	}, remotePeerID)
}

func (s *Server) writeRPC(stream network.Stream, resp protocol.RPCResponse, remotePeerID string) {
	if err := s.writeJSON(stream, resp, s.frameLimits.RPCResponse); err != nil {
		s.resetStream(stream, "rpc response write failed", remotePeerID, err)
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

func (s *Server) closeStream(stream network.Stream) {
	_ = stream.Close()
}
