package p2p

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	corehost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"store-node/internal/auth"
	"store-node/internal/metrics"
	"store-node/internal/protocol"
	"store-node/internal/ratelimit"
	"store-node/internal/service"
	pebblestore "store-node/internal/storage/pebble"
)

func testServerFrameLimits() FrameLimits {
	return FrameLimits{
		RPCRequest:  1 << 20,
		RPCResponse: 1 << 20,
	}
}

var testRPCSeq atomic.Uint64

func nextRPCRequestID() string {
	return fmt.Sprintf("req-%d", testRPCSeq.Add(1))
}

func testServerTimeouts() Timeouts {
	return Timeouts{
		Read:    5 * time.Second,
		Write:   5 * time.Second,
		Handler: 5 * time.Second,
	}
}

type protocolTestEnv struct {
	store     *pebblestore.Store
	server    corehost.Host
	sender    corehost.Host
	recipient corehost.Host
	other     corehost.Host
	senderKey crypto.PrivKey
	recvKey   crypto.PrivKey
}

func newProtocolTestEnv(t *testing.T) *protocolTestEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, err := pebblestore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	serverHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(server) error = %v", err)
	}
	senderKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key(sender) error = %v", err)
	}
	senderHost, err := libp2p.New(libp2p.Identity(senderKey), libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(sender) error = %v", err)
	}
	recvKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key(recipient) error = %v", err)
	}
	recipientHost, err := libp2p.New(libp2p.Identity(recvKey), libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(recipient) error = %v", err)
	}
	otherHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(other) error = %v", err)
	}

	verifier := auth.NewVerifier(60)
	metricsObj := metrics.New()
	storeSvc := service.NewStoreService(store, verifier, ratelimit.New(100), ratelimit.New(100), metricsObj, logger, 60, 3600, 1<<20, 1000, 1<<24)
	fetchSvc := service.NewFetchService(store, metricsObj, logger, 100)
	ackSvc := service.NewAckService(store, verifier, metricsObj, logger)
	server := NewServer(serverHost, logger, testServerFrameLimits(), testServerTimeouts(), storeSvc, fetchSvc, ackSvc)
	server.Register()

	info := peer.AddrInfo{ID: serverHost.ID(), Addrs: serverHost.Addrs()}
	for _, host := range []corehost.Host{senderHost, recipientHost, otherHost} {
		if err := host.Connect(context.Background(), info); err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
	}

	t.Cleanup(func() {
		_ = otherHost.Close()
		_ = recipientHost.Close()
		_ = senderHost.Close()
		_ = serverHost.Close()
		_ = store.Close()
	})

	return &protocolTestEnv{
		store:     store,
		server:    serverHost,
		sender:    senderHost,
		recipient: recipientHost,
		other:     otherHost,
		senderKey: senderKey,
		recvKey:   recvKey,
	}
}

func newProtocolTestEnvWithStoreRateLimit(t *testing.T, limit int) *protocolTestEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, err := pebblestore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	serverHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(server) error = %v", err)
	}
	senderKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key(sender) error = %v", err)
	}
	senderHost, err := libp2p.New(libp2p.Identity(senderKey), libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(sender) error = %v", err)
	}
	recvKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatalf("GenerateEd25519Key(recipient) error = %v", err)
	}
	recipientHost, err := libp2p.New(libp2p.Identity(recvKey), libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(recipient) error = %v", err)
	}
	otherHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New(other) error = %v", err)
	}

	verifier := auth.NewVerifier(60)
	metricsObj := metrics.New()
	storeSvc := service.NewStoreService(store, verifier, ratelimit.New(limit), ratelimit.New(limit), metricsObj, logger, 60, 3600, 1<<20, 1000, 1<<24)
	fetchSvc := service.NewFetchService(store, metricsObj, logger, 100)
	ackSvc := service.NewAckService(store, verifier, metricsObj, logger)
	server := NewServer(serverHost, logger, testServerFrameLimits(), testServerTimeouts(), storeSvc, fetchSvc, ackSvc)
	server.Register()

	info := peer.AddrInfo{ID: serverHost.ID(), Addrs: serverHost.Addrs()}
	for _, host := range []corehost.Host{senderHost, recipientHost, otherHost} {
		if err := host.Connect(context.Background(), info); err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
	}

	t.Cleanup(func() {
		_ = otherHost.Close()
		_ = recipientHost.Close()
		_ = senderHost.Close()
		_ = serverHost.Close()
		_ = store.Close()
	})

	return &protocolTestEnv{
		store:     store,
		server:    serverHost,
		sender:    senderHost,
		recipient: recipientHost,
		other:     otherHost,
		senderKey: senderKey,
		recvKey:   recvKey,
	}
}

func TestStoreFetchAckProtocols(t *testing.T) {
	env := newProtocolTestEnv(t)
	req := signedStoreRequest(t, env.senderKey, env.sender.ID().String(), env.recipient.ID().String(), "msg-1")

	storeResp := sendStoreRequest(t, env.sender, env.server.ID(), req)
	if !storeResp.OK || storeResp.StoreSeq != 1 || storeResp.Duplicate {
		t.Fatalf("store response = %+v", storeResp)
	}

	fetchResp := sendFetchRequest(t, env.recipient, env.server.ID(), &protocol.FetchRequest{
		Version:     1,
		RecipientID: env.recipient.ID().String(),
		AfterSeq:    0,
		Limit:       10,
	})
	if !fetchResp.OK || len(fetchResp.Items) != 1 || fetchResp.Items[0].StoreSeq != 1 {
		t.Fatalf("fetch response = %+v", fetchResp)
	}

	ackReq := signedAckRequest(t, env.recvKey, env.recipient.ID().String(), 1)
	ackResp := sendAckRequest(t, env.recipient, env.server.ID(), ackReq)
	if !ackResp.OK || ackResp.DeletedUntilSeq != 1 {
		t.Fatalf("ack response = %+v", ackResp)
	}

	fetchResp = sendFetchRequest(t, env.recipient, env.server.ID(), &protocol.FetchRequest{
		Version:     1,
		RecipientID: env.recipient.ID().String(),
		AfterSeq:    0,
		Limit:       10,
	})
	if !fetchResp.OK || len(fetchResp.Items) != 0 {
		t.Fatalf("fetch after ack response = %+v", fetchResp)
	}
}

func TestStoreInvalidSignature(t *testing.T) {
	env := newProtocolTestEnv(t)
	req := signedStoreRequest(t, env.senderKey, env.sender.ID().String(), env.recipient.ID().String(), "msg-2")
	req.Message.MsgID = "tampered-msg"

	resp := sendStoreRequest(t, env.sender, env.server.ID(), req)
	if resp.OK || resp.ErrorCode != protocol.CodeInvalidSignature {
		t.Fatalf("store response = %+v", resp)
	}
}

func TestInvalidSignatureDoesNotConsumeSenderRateLimit(t *testing.T) {
	env := newProtocolTestEnvWithStoreRateLimit(t, 1)

	badReq := signedStoreRequest(t, env.senderKey, env.sender.ID().String(), env.recipient.ID().String(), "msg-bad")
	badReq.Message.MsgID = "tampered-msg"
	badResp := sendStoreRequest(t, env.other, env.server.ID(), badReq)
	if badResp.OK || badResp.ErrorCode != protocol.CodeInvalidSignature {
		t.Fatalf("bad store response = %+v", badResp)
	}

	goodReq := signedStoreRequest(t, env.senderKey, env.sender.ID().String(), env.recipient.ID().String(), "msg-good")
	goodResp := sendStoreRequest(t, env.sender, env.server.ID(), goodReq)
	if !goodResp.OK {
		t.Fatalf("good store response = %+v", goodResp)
	}
}

func TestUnauthorizedFetch(t *testing.T) {
	env := newProtocolTestEnv(t)
	req := signedStoreRequest(t, env.senderKey, env.sender.ID().String(), env.recipient.ID().String(), "msg-3")
	if resp := sendStoreRequest(t, env.sender, env.server.ID(), req); !resp.OK {
		t.Fatalf("store response = %+v", resp)
	}

	resp := sendFetchRequest(t, env.other, env.server.ID(), &protocol.FetchRequest{
		Version:     1,
		RecipientID: env.recipient.ID().String(),
		AfterSeq:    0,
		Limit:       10,
	})
	if resp.OK || resp.ErrorCode != protocol.CodeUnauthorized {
		t.Fatalf("fetch response = %+v", resp)
	}
}

func TestUnauthorizedAck(t *testing.T) {
	env := newProtocolTestEnv(t)
	req := signedStoreRequest(t, env.senderKey, env.sender.ID().String(), env.recipient.ID().String(), "msg-4")
	if resp := sendStoreRequest(t, env.sender, env.server.ID(), req); !resp.OK {
		t.Fatalf("store response = %+v", resp)
	}

	ackReq := signedAckRequest(t, env.recvKey, env.recipient.ID().String(), 1)
	resp := sendAckRequest(t, env.other, env.server.ID(), ackReq)
	if resp.OK || resp.ErrorCode != protocol.CodeUnauthorized {
		t.Fatalf("ack response = %+v", resp)
	}
}

func TestAckRejectsBeyondDeliveredFrontier(t *testing.T) {
	env := newProtocolTestEnv(t)
	for i := 1; i <= 3; i++ {
		req := signedStoreRequest(t, env.senderKey, env.sender.ID().String(), env.recipient.ID().String(), fmt.Sprintf("msg-frontier-%d", i))
		if resp := sendStoreRequest(t, env.sender, env.server.ID(), req); !resp.OK {
			t.Fatalf("store response = %+v", resp)
		}
	}

	fetchResp := sendFetchRequest(t, env.recipient, env.server.ID(), &protocol.FetchRequest{
		Version:     1,
		RecipientID: env.recipient.ID().String(),
		AfterSeq:    0,
		Limit:       2,
	})
	if !fetchResp.OK || len(fetchResp.Items) != 2 {
		t.Fatalf("fetch response = %+v", fetchResp)
	}

	ackResp := sendAckRequest(t, env.recipient, env.server.ID(), signedAckRequest(t, env.recvKey, env.recipient.ID().String(), 3))
	if ackResp.OK || ackResp.ErrorCode != protocol.CodeInvalidPayload {
		t.Fatalf("ack response = %+v", ackResp)
	}
}

func TestOversizedRPCRequestResetsStream(t *testing.T) {
	env := newProtocolTestEnv(t)

	stream, err := env.sender.NewStream(context.Background(), env.server.ID(), RPCProtocol)
	if err != nil {
		t.Fatalf("NewStream(rpc) error = %v", err)
	}
	defer stream.Close()

	// 只发送 4 字节大端长度头，声明长度 = max+1；服务端 ReadFrame 读完头即可判定超长并 reset，
	// 无需发送整帧负载，避免写端因对端提前 reset 而失败、导致测试在「未验证服务端路径」时误通过。
	max := testServerFrameLimits().RPCRequest
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(max)+1)
	if _, err := stream.Write(header[:]); err != nil {
		t.Fatalf("write length header: %v", err)
	}

	_ = stream.SetReadDeadline(time.Now().Add(2 * time.Second))
	var rpcResp protocol.RPCResponse
	if err := ReadJSON(stream, &rpcResp, 1<<20); err == nil {
		t.Fatalf("ReadJSON() error = nil, want reset or EOF after server rejects oversized frame")
	}
}

func TestRPCUnknownMethodReturnsStructuredBody(t *testing.T) {
	env := newProtocolTestEnv(t)
	rpcReq := protocol.RPCRequest{
		RequestID: nextRPCRequestID(),
		Method:    "not.a.real.method",
		Body:      json.RawMessage(`{}`),
	}
	stream, err := env.sender.NewStream(context.Background(), env.server.ID(), RPCProtocol)
	if err != nil {
		t.Fatalf("NewStream(rpc) error = %v", err)
	}
	defer stream.Close()
	if err := WriteJSON(stream, &rpcReq, testServerFrameLimits().RPCRequest); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	var rpcResp protocol.RPCResponse
	if err := ReadJSON(stream, &rpcResp, testServerFrameLimits().RPCResponse); err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	if rpcResp.OK || len(rpcResp.Body) == 0 {
		t.Fatalf("rpc response = %+v", rpcResp)
	}
	var eb protocol.RPCErrorBody
	if err := json.Unmarshal(rpcResp.Body, &eb); err != nil {
		t.Fatalf("unmarshal rpc error body: %v", err)
	}
	if eb.ErrorCode != protocol.CodeRPCUnknownMethod || eb.Method != "not.a.real.method" {
		t.Fatalf("RPCErrorBody = %+v", eb)
	}
}

func TestRPCInvalidEnvelopeJSONReturnsStructuredBody(t *testing.T) {
	env := newProtocolTestEnv(t)
	stream, err := env.sender.NewStream(context.Background(), env.server.ID(), RPCProtocol)
	if err != nil {
		t.Fatalf("NewStream(rpc) error = %v", err)
	}
	defer stream.Close()
	raw := []byte(`not valid json`)
	if err := WriteFrame(stream, raw, uint32(len(raw))); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	var rpcResp protocol.RPCResponse
	if err := ReadJSON(stream, &rpcResp, testServerFrameLimits().RPCResponse); err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	if rpcResp.OK || len(rpcResp.Body) == 0 {
		t.Fatalf("rpc response = %+v", rpcResp)
	}
	var eb protocol.RPCErrorBody
	if err := json.Unmarshal(rpcResp.Body, &eb); err != nil {
		t.Fatalf("unmarshal rpc error body: %v", err)
	}
	if eb.ErrorCode != protocol.CodeRPCInvalidRequest {
		t.Fatalf("error_code = %q, want %q", eb.ErrorCode, protocol.CodeRPCInvalidRequest)
	}
}

func TestInvalidJSONFetchReturnsStructuredError(t *testing.T) {
	env := newProtocolTestEnv(t)

	stream, err := env.sender.NewStream(context.Background(), env.server.ID(), RPCProtocol)
	if err != nil {
		t.Fatalf("NewStream(rpc) error = %v", err)
	}
	defer stream.Close()

	rpcReq := protocol.RPCRequest{
		RequestID: nextRPCRequestID(),
		Method:    protocol.MethodOfflineFetch,
		Body:      json.RawMessage(`true`),
	}
	if err := WriteJSON(stream, &rpcReq, testServerFrameLimits().RPCRequest); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var rpcResp protocol.RPCResponse
	if err := ReadJSON(stream, &rpcResp, 1<<20); err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	var inner protocol.FetchResponse
	if err := json.Unmarshal(rpcResp.Body, &inner); err != nil {
		t.Fatalf("unmarshal rpc body: %v", err)
	}
	if inner.OK || inner.ErrorCode != protocol.CodeInvalidPayload {
		t.Fatalf("fetch response = %+v", inner)
	}
}

func signedStoreRequest(t *testing.T, priv crypto.PrivKey, senderID, recipientID, msgID string) *protocol.StoreRequest {
	t.Helper()
	ttl := int64(60)
	msg := &protocol.OfflineMessageEnvelope{
		Version:        1,
		MsgID:          msgID,
		SenderID:       senderID,
		RecipientID:    recipientID,
		ConversationID: senderID + ":" + recipientID,
		CreatedAt:      1710000000,
		TTLSec:         &ttl,
		Cipher: protocol.CipherPayload{
			Algorithm:      "x25519-xsalsa20-poly1305",
			RecipientKeyID: "key-1",
			Nonce:          "nonce",
			Ciphertext:     "ciphertext",
		},
		Signature: protocol.Signature{
			Algorithm: "ed25519",
		},
	}
	payload, err := auth.CanonicalMessageEnvelope(msg, 60)
	if err != nil {
		t.Fatalf("CanonicalMessageEnvelope() error = %v", err)
	}
	sig, err := priv.Sign(payload)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	msg.Signature.Value = base64.StdEncoding.EncodeToString(sig)
	return &protocol.StoreRequest{
		Version: 1,
		Message: msg,
	}
}

func signedAckRequest(t *testing.T, priv crypto.PrivKey, recipientID string, ackSeq uint64) *protocol.AckRequest {
	t.Helper()
	req := &protocol.AckRequest{
		Version:     1,
		RecipientID: recipientID,
		DeviceID:    recipientID,
		AckSeq:      ackSeq,
		AckedAt:     1710000100,
		Signature: protocol.Signature{
			Algorithm: "ed25519",
		},
	}
	payload, err := auth.CanonicalAck(req)
	if err != nil {
		t.Fatalf("CanonicalAck() error = %v", err)
	}
	sig, err := priv.Sign(payload)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	req.Signature.Value = base64.StdEncoding.EncodeToString(sig)
	return req
}

func sendStoreRequest(t *testing.T, client corehost.Host, serverID peer.ID, req *protocol.StoreRequest) *protocol.StoreResponse {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal(store body) error = %v", err)
	}
	rpcResp := sendRPC(t, client, serverID, protocol.MethodOfflineStore, body)
	var inner protocol.StoreResponse
	if err := json.Unmarshal(rpcResp.Body, &inner); err != nil {
		t.Fatalf("unmarshal store rpc body: %v", err)
	}
	return &inner
}

func sendFetchRequest(t *testing.T, client corehost.Host, serverID peer.ID, req *protocol.FetchRequest) *protocol.FetchResponse {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal(fetch body) error = %v", err)
	}
	rpcResp := sendRPC(t, client, serverID, protocol.MethodOfflineFetch, body)
	var inner protocol.FetchResponse
	if err := json.Unmarshal(rpcResp.Body, &inner); err != nil {
		t.Fatalf("unmarshal fetch rpc body: %v", err)
	}
	return &inner
}

func sendAckRequest(t *testing.T, client corehost.Host, serverID peer.ID, req *protocol.AckRequest) *protocol.AckResponse {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal(ack body) error = %v", err)
	}
	rpcResp := sendRPC(t, client, serverID, protocol.MethodOfflineAck, body)
	var inner protocol.AckResponse
	if err := json.Unmarshal(rpcResp.Body, &inner); err != nil {
		t.Fatalf("unmarshal ack rpc body: %v", err)
	}
	return &inner
}

func sendRPC(t *testing.T, client corehost.Host, serverID peer.ID, method string, body json.RawMessage) protocol.RPCResponse {
	t.Helper()
	stream, err := client.NewStream(context.Background(), serverID, RPCProtocol)
	if err != nil {
		t.Fatalf("NewStream(rpc) error = %v", err)
	}
	defer stream.Close()
	rpcReq := protocol.RPCRequest{
		RequestID: nextRPCRequestID(),
		Method:    method,
		Body:      body,
	}
	if err := WriteJSON(stream, &rpcReq, testServerFrameLimits().RPCRequest); err != nil {
		t.Fatalf("WriteJSON(rpc) error = %v", err)
	}
	var rpcResp protocol.RPCResponse
	if err := ReadJSON(stream, &rpcResp, testServerFrameLimits().RPCResponse); err != nil {
		t.Fatalf("ReadJSON(rpc) error = %v", err)
	}
	return rpcResp
}
