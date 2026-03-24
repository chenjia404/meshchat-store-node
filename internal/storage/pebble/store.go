package pebble

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	cpebble "github.com/cockroachdb/pebble"

	"store-node/internal/protocol"
)

func (s *Store) StoreMessage(ctx context.Context, msg *protocol.StoredMessage) (uint64, int64, bool, error) {
	return s.storeMessageWithQuota(ctx, msg, 0, 0, false)
}

func (s *Store) StoreMessageWithQuota(ctx context.Context, msg *protocol.StoredMessage, maxMessages int, maxBytes int64) (uint64, int64, bool, error) {
	return s.storeMessageWithQuota(ctx, msg, maxMessages, maxBytes, true)
}

func (s *Store) storeMessageWithQuota(ctx context.Context, msg *protocol.StoredMessage, maxMessages int, maxBytes int64, enforceQuota bool) (uint64, int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, false, err
	}
	if msg == nil || msg.Message == nil {
		return 0, 0, false, protocol.NewAppError(protocol.CodeInvalidPayload, "stored message is required")
	}
	recipientID := msg.Message.RecipientID
	unlock := s.recipientShards.Lock(recipientID)
	defer unlock()

	for {
		dupValue, closer, err := s.db.Get([]byte(dupKey(recipientID, msg.Message.MsgID)))
		if err == nil {
			storeSeq, parseErr := parseUint64(dupValue)
			closer.Close()
			if parseErr != nil {
				return 0, 0, false, parseErr
			}
			existing, fetchErr := s.loadMessage(recipientID, storeSeq)
			if fetchErr == nil {
				return storeSeq, existing.ExpireAt, true, nil
			}
			if fetchErr == cpebble.ErrNotFound {
				if err := s.deleteStaleDuplicate(recipientID, msg.Message.MsgID); err != nil {
					return 0, 0, false, err
				}
				continue
			}
			return 0, 0, false, fetchErr
		}
		if err != cpebble.ErrNotFound {
			return 0, 0, false, err
		}
		break
	}

	lastSeq, err := s.lastSeq(recipientID)
	if err != nil {
		return 0, 0, false, err
	}
	storeSeq := lastSeq + 1
	now := s.nowFunc().UTC().Unix()
	ttl := msg.Message.EffectiveTTL(0)
	msg.StoreSeq = storeSeq
	msg.ReceivedAt = now
	msg.ExpireAt = now + ttl

	payload, err := json.Marshal(msg)
	if err != nil {
		return 0, 0, false, err
	}

	if enforceQuota {
		count, totalBytes, err := s.recipientUsageLocked(recipientID)
		if err != nil {
			return 0, 0, false, err
		}
		if maxMessages > 0 && count >= maxMessages {
			return 0, 0, false, protocol.NewAppError(protocol.CodeRecipientQuotaExceeded, "recipient pending message quota exceeded")
		}
		if maxBytes > 0 && totalBytes+int64(len(payload)) > maxBytes {
			return 0, 0, false, protocol.NewAppError(protocol.CodeRecipientBytesExceeded, "recipient pending bytes exceeded")
		}
	}

	batch := s.db.NewBatch()
	defer batch.Close()

	if err := batch.Set([]byte(msgKey(recipientID, storeSeq)), payload, nil); err != nil {
		return 0, 0, false, err
	}
	if err := batch.Set([]byte(dupKey(recipientID, msg.Message.MsgID)), []byte(strconv.FormatUint(storeSeq, 10)), nil); err != nil {
		return 0, 0, false, err
	}
	if err := batch.Set([]byte(expKey(msg.ExpireAt, recipientID, storeSeq)), []byte{}, nil); err != nil {
		return 0, 0, false, err
	}
	if err := batch.Set([]byte(seqKey(recipientID)), []byte(strconv.FormatUint(storeSeq, 10)), nil); err != nil {
		return 0, 0, false, err
	}
	if err := batch.Commit(cpebble.Sync); err != nil {
		return 0, 0, false, err
	}
	return storeSeq, msg.ExpireAt, false, nil
}

func (s *Store) deleteStaleDuplicate(recipientID, msgID string) error {
	batch := s.db.NewBatch()
	defer batch.Close()
	if err := batch.Delete([]byte(dupKey(recipientID, msgID)), nil); err != nil {
		return err
	}
	return batch.Commit(cpebble.Sync)
}

func (s *Store) RecipientUsage(ctx context.Context, recipientID string) (int, int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	return s.recipientUsageLocked(recipientID)
}

func (s *Store) recipientUsageLocked(recipientID string) (int, int64, error) {
	iter, err := s.db.NewIter(&cpebble.IterOptions{
		LowerBound: []byte(msgPrefix(recipientID)),
		UpperBound: prefixUpperBound(msgPrefix(recipientID)),
	})
	if err != nil {
		return 0, 0, err
	}
	defer iter.Close()

	var count int
	var totalBytes int64
	for iter.First(); iter.Valid(); iter.Next() {
		count++
		totalBytes += int64(len(iter.Value()))
	}
	if err := iter.Error(); err != nil {
		return 0, 0, err
	}
	return count, totalBytes, nil
}

func (s *Store) lastSeq(recipientID string) (uint64, error) {
	val, closer, err := s.db.Get([]byte(seqKey(recipientID)))
	if err == cpebble.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer closer.Close()
	return parseUint64(val)
}

func (s *Store) frontier(key string) (uint64, error) {
	val, closer, err := s.db.Get([]byte(key))
	if err == cpebble.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer closer.Close()
	return parseUint64(val)
}

func (s *Store) setFrontier(batch *cpebble.Batch, key string, seq uint64) error {
	return batch.Set([]byte(key), []byte(strconv.FormatUint(seq, 10)), nil)
}

func (s *Store) loadMessage(recipientID string, storeSeq uint64) (*protocol.StoredMessage, error) {
	val, closer, err := s.db.Get([]byte(msgKey(recipientID, storeSeq)))
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	var stored protocol.StoredMessage
	if err := json.Unmarshal(val, &stored); err != nil {
		return nil, fmt.Errorf("unmarshal stored message: %w", err)
	}
	return &stored, nil
}
