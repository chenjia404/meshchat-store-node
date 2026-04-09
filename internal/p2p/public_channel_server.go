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
	"store-node/internal/publicchannel"
)

// PublicChannelFrameLimits 公共频道 RPC 单帧上限（含批量消息 JSON）。
type PublicChannelFrameLimits struct {
	RPCRequest  uint32
	RPCResponse uint32
}

// PublicChannelServer 处理 /meshchat/public-channel/rpc/1.0.0。
type PublicChannelServer struct {
	host        corehost.Host
	logger      *slog.Logger
	frameLimits PublicChannelFrameLimits
	timeouts    Timeouts
	svc         *publicchannel.Service
}

func NewPublicChannelServer(
	host corehost.Host,
	logger *slog.Logger,
	frameLimits PublicChannelFrameLimits,
	timeouts Timeouts,
	svc *publicchannel.Service,
) *PublicChannelServer {
	return &PublicChannelServer{
		host:        host,
		logger:    logger,
		frameLimits: frameLimits,
		timeouts:    timeouts,
		svc:         svc,
	}
}

func (s *PublicChannelServer) Register() {
	s.host.SetStreamHandler(protocol.PublicChannelRPCProtocol, s.HandleRPC)
}

func (s *PublicChannelServer) HandleRPC(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer().String()
	s.logger.Info("public channel rpc", "remote_peer_id", remotePeerID)

	frame, err := s.readFrame(stream, s.frameLimits.RPCRequest)
	if err != nil {
		s.resetStream(stream, "public channel rpc read failed", remotePeerID, err)
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
	case protocol.MethodPublicChannelPush:
		s.handlePush(ctx, stream, remotePeerID, rpcReq)
	case protocol.MethodPublicChannelGetProfile:
		s.handleGetProfile(ctx, stream, remotePeerID, rpcReq)
	case protocol.MethodPublicChannelGetHead:
		s.handleGetHead(ctx, stream, remotePeerID, rpcReq)
	case protocol.MethodPublicChannelListMessages:
		s.handleListMessages(ctx, stream, remotePeerID, rpcReq)
	case protocol.MethodPublicChannelGetMessage:
		s.handleGetMessage(ctx, stream, remotePeerID, rpcReq)
	case protocol.MethodPublicChannelSyncChannel:
		s.handleSyncChannel(ctx, stream, remotePeerID, rpcReq)
	default:
		s.writeRPCUnknownMethod(stream, rpcReq.RequestID, rpcReq.Method, remotePeerID)
	}
}

func (s *PublicChannelServer) handlePush(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req publicchannel.PushRequest
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		s.writeRPCBody(stream, rpcReq.RequestID, &publicchannel.PushResponse{
			OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: err.Error(),
		}, remotePeerID)
		return
	}
	resp := s.svc.Push(ctx, remotePeerID, &req)
	s.writeRPCBody(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *PublicChannelServer) handleGetProfile(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req struct {
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		s.writeRPCBody(stream, rpcReq.RequestID, &publicchannel.GetProfileResponse{
			OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: err.Error(),
		}, remotePeerID)
		return
	}
	resp := s.svc.GetProfile(ctx, req.ChannelID)
	s.writeRPCBody(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *PublicChannelServer) handleGetHead(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req struct {
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		s.writeRPCBody(stream, rpcReq.RequestID, &publicchannel.GetHeadResponse{
			OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: err.Error(),
		}, remotePeerID)
		return
	}
	resp := s.svc.GetHead(ctx, req.ChannelID)
	s.writeRPCBody(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *PublicChannelServer) handleListMessages(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req publicchannel.ListMessagesRequest
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		s.writeRPCBody(stream, rpcReq.RequestID, &publicchannel.ListMessagesResponse{
			OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: err.Error(),
		}, remotePeerID)
		return
	}
	resp := s.svc.ListMessages(ctx, &req)
	s.writeRPCBody(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *PublicChannelServer) handleGetMessage(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req publicchannel.GetMessageRequest
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		s.writeRPCBody(stream, rpcReq.RequestID, &publicchannel.GetMessageResponse{
			OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: err.Error(),
		}, remotePeerID)
		return
	}
	resp := s.svc.GetMessage(ctx, req.ChannelID, req.MessageID)
	s.writeRPCBody(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *PublicChannelServer) handleSyncChannel(ctx context.Context, stream network.Stream, remotePeerID string, rpcReq protocol.RPCRequest) {
	var req publicchannel.SyncChannelRequest
	if err := json.Unmarshal(rpcReq.Body, &req); err != nil {
		s.writeRPCBody(stream, rpcReq.RequestID, &publicchannel.SyncChannelResponse{
			OK: false, ErrorCode: protocol.CodeInvalidPayload, ErrorMessage: err.Error(),
		}, remotePeerID)
		return
	}
	resp := s.svc.SyncChannel(ctx, &req)
	s.writeRPCBody(stream, rpcReq.RequestID, resp, remotePeerID)
}

func (s *PublicChannelServer) writeRPCBody(stream network.Stream, requestID string, inner any, remotePeerID string) {
	body, err := json.Marshal(inner)
	if err != nil {
		s.writeRPCError(stream, requestID, protocol.CodeInternalError, err.Error(), remotePeerID)
		return
	}
	var ok bool
	switch v := inner.(type) {
	case *publicchannel.PushResponse:
		ok = v.OK
	case *publicchannel.GetProfileResponse:
		ok = v.OK
	case *publicchannel.GetHeadResponse:
		ok = v.OK
	case *publicchannel.ListMessagesResponse:
		ok = v.OK
	case *publicchannel.GetMessageResponse:
		ok = v.OK
	case *publicchannel.SyncChannelResponse:
		ok = v.OK
	default:
		ok = true
	}
	out := protocol.RPCResponse{
		RequestID: requestID,
		OK:        ok,
		Error:     rpcErrorString(ok, errorStringFromInner(inner)),
		Body:      body,
	}
	s.writeRPC(stream, out, remotePeerID)
}

func errorStringFromInner(inner any) string {
	switch v := inner.(type) {
	case *publicchannel.PushResponse:
		if v.OK {
			return ""
		}
		return v.ErrorMessage
	case *publicchannel.GetProfileResponse:
		if v.OK {
			return ""
		}
		return v.ErrorMessage
	case *publicchannel.GetHeadResponse:
		if v.OK {
			return ""
		}
		return v.ErrorMessage
	case *publicchannel.ListMessagesResponse:
		if v.OK {
			return ""
		}
		return v.ErrorMessage
	case *publicchannel.GetMessageResponse:
		if v.OK {
			return ""
		}
		return v.ErrorMessage
	case *publicchannel.SyncChannelResponse:
		if v.OK {
			return ""
		}
		return v.ErrorMessage
	default:
		return ""
	}
}

func (s *PublicChannelServer) writeRPCUnknownMethod(stream network.Stream, requestID string, method string, remotePeerID string) {
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

func (s *PublicChannelServer) writeRPCError(stream network.Stream, requestID string, code string, errMsg string, remotePeerID string) {
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

func (s *PublicChannelServer) writeRPC(stream network.Stream, resp protocol.RPCResponse, remotePeerID string) {
	if err := s.writeJSON(stream, resp, s.frameLimits.RPCResponse); err != nil {
		s.resetStream(stream, "public channel rpc response write failed", remotePeerID, err)
		return
	}
	s.closeStream(stream)
}

func (s *PublicChannelServer) readFrame(stream network.Stream, maxSize uint32) ([]byte, error) {
	if err := stream.SetReadDeadline(time.Now().Add(s.timeouts.Read)); err != nil {
		return nil, err
	}
	return ReadFrame(stream, maxSize)
}

func (s *PublicChannelServer) writeJSON(stream network.Stream, payload any, maxSize uint32) error {
	if err := stream.SetWriteDeadline(time.Now().Add(s.timeouts.Write)); err != nil {
		return err
	}
	return WriteJSON(stream, payload, maxSize)
}

func (s *PublicChannelServer) resetStream(stream network.Stream, message string, remotePeerID string, err error) {
	s.logger.Warn(message, "remote_peer_id", remotePeerID, "error", err)
	_ = stream.Reset()
}

func (s *PublicChannelServer) closeStream(stream network.Stream) {
	_ = stream.Close()
}
