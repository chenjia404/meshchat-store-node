package pebble

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"store-node/internal/protocol"
)

func TestConcurrentStoreSameRecipientOrdered(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	const total = 100
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-same", fmt.Sprintf("msg-%03d", i), 60))
			if err != nil {
				t.Errorf("StoreMessage() error = %v", err)
			}
		}(i)
	}
	wg.Wait()

	items, hasMore, err := st.FetchMessages(ctx, "recipient-same", 0, total+1)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if hasMore || len(items) != total {
		t.Fatalf("FetchMessages() len=%d hasMore=%v", len(items), hasMore)
	}
	for i, item := range items {
		if item.StoreSeq != uint64(i+1) {
			t.Fatalf("item[%d].StoreSeq = %d, want %d", i, item.StoreSeq, i+1)
		}
	}
}

func TestConcurrentStoreDifferentRecipients(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	recipients := []string{"recipient-a", "recipient-b"}
	var wg sync.WaitGroup
	for _, recipientID := range recipients {
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(recipientID string, i int) {
				defer wg.Done()
				_, _, _, err := st.StoreMessage(ctx, testStoredMessage(recipientID, fmt.Sprintf("%s-%03d", recipientID, i), 60))
				if err != nil {
					t.Errorf("StoreMessage() error = %v", err)
				}
			}(recipientID, i)
		}
	}
	wg.Wait()

	for _, recipientID := range recipients {
		items, _, err := st.FetchMessages(ctx, recipientID, 0, 100)
		if err != nil {
			t.Fatalf("FetchMessages(%s) error = %v", recipientID, err)
		}
		if len(items) != 50 {
			t.Fatalf("FetchMessages(%s) len=%d, want 50", recipientID, len(items))
		}
		for i, item := range items {
			if item.StoreSeq != uint64(i+1) {
				t.Fatalf("recipient=%s item[%d].StoreSeq = %d, want %d", recipientID, i, item.StoreSeq, i+1)
			}
		}
	}
}

func TestConcurrentAckAndStore(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		if _, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-ack", fmt.Sprintf("initial-%02d", i), 60)); err != nil {
			t.Fatalf("StoreMessage() setup error = %v", err)
		}
	}
	if items, _, err := st.FetchMessages(ctx, "recipient-ack", 0, 20); err != nil {
		t.Fatalf("FetchMessages() setup error = %v", err)
	} else if len(items) != 20 {
		t.Fatalf("FetchMessages() setup len=%d, want 20", len(items))
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := st.AckMessages(ctx, "recipient-ack", 10); err != nil {
			t.Errorf("AckMessages() error = %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if _, _, _, err := st.StoreMessage(ctx, testStoredMessage("recipient-ack", fmt.Sprintf("new-%02d", i), 60)); err != nil {
				t.Errorf("StoreMessage() concurrent error = %v", err)
			}
		}
	}()
	wg.Wait()

	items, _, err := st.FetchMessages(ctx, "recipient-ack", 0, 100)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if len(items) != 30 {
		t.Fatalf("FetchMessages() len=%d, want 30", len(items))
	}
	for _, item := range items {
		if item.StoreSeq <= 10 {
			t.Fatalf("unexpected acked item seq=%d still present", item.StoreSeq)
		}
	}
}

func TestConcurrentStoreQuotaAtomic(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	var success atomic.Int32
	var quotaRejected atomic.Int32
	var otherErrors atomic.Int32

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, duplicate, err := st.StoreMessageWithQuota(ctx, testStoredMessage("recipient-quota", fmt.Sprintf("msg-%d", i), 60), 1, 1<<20)
			switch {
			case err == nil && !duplicate:
				success.Add(1)
			case protocol.ErrorCode(err) == protocol.CodeRecipientQuotaExceeded:
				quotaRejected.Add(1)
			default:
				otherErrors.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if success.Load() != 1 {
		t.Fatalf("success = %d, want 1", success.Load())
	}
	if quotaRejected.Load() != 1 {
		t.Fatalf("quotaRejected = %d, want 1", quotaRejected.Load())
	}
	if otherErrors.Load() != 0 {
		t.Fatalf("otherErrors = %d, want 0", otherErrors.Load())
	}

	items, _, err := st.FetchMessages(ctx, "recipient-quota", 0, 10)
	if err != nil {
		t.Fatalf("FetchMessages() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("FetchMessages() len=%d, want 1", len(items))
	}
}
