package pebble

import (
	"context"
	"fmt"
	"testing"
	"time"

	"store-node/internal/protocol"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st
}

func testStoredMessage(recipientID, msgID string, ttl int64) *protocol.StoredMessage {
	return &protocol.StoredMessage{
		Message: &protocol.OfflineMessageEnvelope{
			Version:        1,
			MsgID:          msgID,
			SenderID:       "sender-1",
			RecipientID:    recipientID,
			ConversationID: "sender-1:" + recipientID,
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
				Value:     "sig",
			},
		},
	}
}

func TestStoreFetchAckAndDuplicate(t *testing.T) {
	st := newTestStore(t)
	st.SetNowFunc(func() time.Time { return time.Unix(1710000000, 0).UTC() })

	ctx := context.Background()
	msg1 := testStoredMessage("recipient-1", "msg-1", 60)
	seq1, expire1, duplicate, err := st.StoreMessage(ctx, msg1)
	if err != nil {
		t.Fatalf("StoreMessage() error = %v", err)
	}
	if duplicate || seq1 != 1 || expire1 != 1710000060 {
		t.Fatalf("StoreMessage() = seq=%d expire=%d duplicate=%v", seq1, expire1, duplicate)
	}

	dupSeq, dupExpire, duplicate, err := st.StoreMessage(ctx, testStoredMessage("recipient-1", "msg-1", 60))
	if err != nil {
		t.Fatalf("StoreMessage() duplicate error = %v", err)
	}
	if !duplicate || dupSeq != 1 || dupExpire != expire1 {
		t.Fatalf("duplicate result = seq=%d expire=%d duplicate=%v", dupSeq, dupExpire, duplicate)
	}

	seq2, _, duplicate, err := st.StoreMessage(ctx, testStoredMessage("recipient-1", "msg-2", 60))
	if err != nil {
		t.Fatalf("StoreMessage() second error = %v", err)
	}
	if duplicate || seq2 != 2 {
		t.Fatalf("StoreMessage() second = seq=%d duplicate=%v", seq2, duplicate)
	}

	items, hasMore, err := st.FetchMessages(ctx, "recipient-1", 0, 10)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if hasMore || len(items) != 2 {
		t.Fatalf("FetchMessages() len=%d hasMore=%v", len(items), hasMore)
	}
	if items[0].StoreSeq != 1 || items[1].StoreSeq != 2 {
		t.Fatalf("FetchMessages() seqs = %d,%d", items[0].StoreSeq, items[1].StoreSeq)
	}

	deletedUntil, err := st.AckMessages(ctx, "recipient-1", 1)
	if err != nil {
		t.Fatalf("AckMessages() error = %v", err)
	}
	if deletedUntil != 1 {
		t.Fatalf("AckMessages() = %d, want 1", deletedUntil)
	}

	items, hasMore, err = st.FetchMessages(ctx, "recipient-1", 0, 10)
	if err != nil {
		t.Fatalf("FetchMessages() after ack error = %v", err)
	}
	if hasMore || len(items) != 1 || items[0].StoreSeq != 2 {
		t.Fatalf("FetchMessages() after ack len=%d seq=%d", len(items), items[0].StoreSeq)
	}
}

func TestDeleteExpired(t *testing.T) {
	st := newTestStore(t)
	st.SetNowFunc(func() time.Time { return time.Unix(1710000000, 0).UTC() })

	ctx := context.Background()
	if _, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-2", "msg-exp", 1)); err != nil {
		t.Fatalf("StoreMessage() error = %v", err)
	}

	deleted, err := st.DeleteExpired(ctx, 1710000002, 100)
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteExpired() = %d, want 1", deleted)
	}

	items, _, err := st.FetchMessages(ctx, "recipient-2", 0, 10)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("FetchMessages() len=%d, want 0", len(items))
	}
}

func TestFetchFiltersExpiredBeforeGC(t *testing.T) {
	st := newTestStore(t)
	st.SetNowFunc(func() time.Time { return time.Unix(1710000000, 0).UTC() })

	ctx := context.Background()
	if _, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-exp", "msg-expired", 1)); err != nil {
		t.Fatalf("StoreMessage() expired error = %v", err)
	}
	if _, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-exp", "msg-live", 60)); err != nil {
		t.Fatalf("StoreMessage() live error = %v", err)
	}

	st.SetNowFunc(func() time.Time { return time.Unix(1710000002, 0).UTC() })

	items, hasMore, err := st.FetchMessages(ctx, "recipient-exp", 0, 10)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if hasMore {
		t.Fatalf("FetchMessages() hasMore = true, want false")
	}
	if len(items) != 1 || items[0].StoreSeq != 2 || items[0].Message.MsgID != "msg-live" {
		t.Fatalf("FetchMessages() items = %+v", items)
	}

	deletedUntil, err := st.AckMessages(ctx, "recipient-exp", 2)
	if err != nil {
		t.Fatalf("AckMessages() error = %v", err)
	}
	if deletedUntil != 2 {
		t.Fatalf("AckMessages() deletedUntil = %d, want 2", deletedUntil)
	}
}

func TestStoreMessageCleansStaleDuplicateAfterGC(t *testing.T) {
	st := newTestStore(t)
	st.SetNowFunc(func() time.Time { return time.Unix(1710000000, 0).UTC() })

	ctx := context.Background()
	if _, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-3", "msg-retry", 1)); err != nil {
		t.Fatalf("StoreMessage() initial error = %v", err)
	}

	dup := dupKey("recipient-3", "msg-retry")
	if err := st.db.Set([]byte(dup), []byte("1"), nil); err != nil {
		t.Fatalf("db.Set() dup error = %v", err)
	}
	if err := st.db.Delete([]byte(msgKey("recipient-3", 1)), nil); err != nil {
		t.Fatalf("db.Delete() msg error = %v", err)
	}

	seq, expireAt, duplicate, err := st.StoreMessage(ctx, testStoredMessage("recipient-3", "msg-retry", 60))
	if err != nil {
		t.Fatalf("StoreMessage() retry error = %v", err)
	}
	if duplicate {
		t.Fatalf("StoreMessage() duplicate = true, want false")
	}
	if seq != 2 {
		t.Fatalf("StoreMessage() seq = %d, want 2", seq)
	}
	if expireAt == 0 {
		t.Fatalf("StoreMessage() expireAt = 0, want non-zero")
	}

	items, _, err := st.FetchMessages(ctx, "recipient-3", 0, 10)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if len(items) != 1 || items[0].StoreSeq != 2 || items[0].Message.MsgID != "msg-retry" {
		t.Fatalf("FetchMessages() items = %+v", items)
	}
}

func TestAckMessagesRejectsUndeliveredFrontier(t *testing.T) {
	st := newTestStore(t)
	st.SetNowFunc(func() time.Time { return time.Unix(1710000000, 0).UTC() })

	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		if _, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-4", fmt.Sprintf("msg-frontier-%d", i), 60)); err != nil {
			t.Fatalf("StoreMessage() error = %v", err)
		}
	}

	items, _, err := st.FetchMessages(ctx, "recipient-4", 0, 2)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if len(items) != 2 || items[1].StoreSeq != 2 {
		t.Fatalf("FetchMessages() items = %+v", items)
	}

	deletedUntil, err := st.AckMessages(ctx, "recipient-4", 3)
	if err == nil {
		t.Fatalf("AckMessages() error = nil, want invalid payload")
	}
	if protocol.ErrorCode(err) != protocol.CodeInvalidPayload {
		t.Fatalf("AckMessages() error code = %s, want %s", protocol.ErrorCode(err), protocol.CodeInvalidPayload)
	}
	if deletedUntil != 0 {
		t.Fatalf("AckMessages() deletedUntil = %d, want 0", deletedUntil)
	}

	deletedUntil, err = st.AckMessages(ctx, "recipient-4", 2)
	if err != nil {
		t.Fatalf("AckMessages() error = %v", err)
	}
	if deletedUntil != 2 {
		t.Fatalf("AckMessages() deletedUntil = %d, want 2", deletedUntil)
	}
}
