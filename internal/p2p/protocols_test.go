package p2p

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
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
		StoreRequest:  1 << 20,
		StoreResponse: 1 << 16,
		FetchRequest:  1 << 14,
		FetchResponse: 1 << 20,
		AckRequest:    1 << 14,
		AckResponse:   1 << 16,
	}
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

func TestOversizedFetchRequestResetsStream(t *testing.T) {
	env := newProtocolTestEnv(t)

	stream, err := env.sender.NewStream(context.Background(), env.server.ID(), FetchProtocol)
	if err != nil {
		t.Fatalf("NewStream(fetch) error = %v", err)
	}
	defer stream.Close()

	if err := WriteFrame(stream, bytes.Repeat([]byte("a"), int(testServerFrameLimits().FetchRequest)+1), 1<<20); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	_ = stream.SetReadDeadline(time.Now().Add(2 * time.Second))

	var resp protocol.FetchResponse
	if err := ReadJSON(stream, &resp, 1<<20); err == nil {
		t.Fatalf("ReadJSON() error = nil, want reset")
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
	stream, err := client.NewStream(context.Background(), serverID, StoreProtocol)
	if err != nil {
		t.Fatalf("NewStream(store) error = %v", err)
	}
	defer stream.Close()
	if err := WriteJSON(stream, req, 1<<20); err != nil {
		t.Fatalf("WriteJSON(store) error = %v", err)
	}
	var resp protocol.StoreResponse
	if err := ReadJSON(stream, &resp, 1<<20); err != nil {
		t.Fatalf("ReadJSON(store) error = %v", err)
	}
	return &resp
}

func sendFetchRequest(t *testing.T, client corehost.Host, serverID peer.ID, req *protocol.FetchRequest) *protocol.FetchResponse {
	t.Helper()
	stream, err := client.NewStream(context.Background(), serverID, FetchProtocol)
	if err != nil {
		t.Fatalf("NewStream(fetch) error = %v", err)
	}
	defer stream.Close()
	if err := WriteJSON(stream, req, 1<<20); err != nil {
		t.Fatalf("WriteJSON(fetch) error = %v", err)
	}
	var resp protocol.FetchResponse
	if err := ReadJSON(stream, &resp, 1<<20); err != nil {
		t.Fatalf("ReadJSON(fetch) error = %v", err)
	}
	return &resp
}

func sendAckRequest(t *testing.T, client corehost.Host, serverID peer.ID, req *protocol.AckRequest) *protocol.AckResponse {
	t.Helper()
	stream, err := client.NewStream(context.Background(), serverID, AckProtocol)
	if err != nil {
		t.Fatalf("NewStream(ack) error = %v", err)
	}
	defer stream.Close()
	if err := WriteJSON(stream, req, 1<<20); err != nil {
		t.Fatalf("WriteJSON(ack) error = %v", err)
	}
	var resp protocol.AckResponse
	if err := ReadJSON(stream, &resp, 1<<20); err != nil {
		t.Fatalf("ReadJSON(ack) error = %v", err)
	}
	return &resp
}
