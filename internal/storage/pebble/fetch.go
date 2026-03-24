package pebble

import (
	"context"
	"encoding/json"

	cpebble "github.com/cockroachdb/pebble"

	"store-node/internal/protocol"
)

func (s *Store) FetchMessages(ctx context.Context, recipientID string, afterSeq uint64, limit int) ([]*protocol.StoredMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if limit <= 0 {
		return []*protocol.StoredMessage{}, false, nil
	}
	now := s.nowFunc().UTC().Unix()
	iter, err := s.db.NewIter(&cpebble.IterOptions{
		LowerBound: []byte(msgKey(recipientID, afterSeq+1)),
		UpperBound: prefixUpperBound(msgPrefix(recipientID)),
	})
	if err != nil {
		return nil, false, err
	}
	defer iter.Close()

	items := make([]*protocol.StoredMessage, 0, limit)
	hasMore := false
	for iter.First(); iter.Valid(); iter.Next() {
		var stored protocol.StoredMessage
		if err := json.Unmarshal(iter.Value(), &stored); err != nil {
			return nil, false, err
		}
		if stored.ExpireAt <= now {
			continue
		}
		if len(items) >= limit {
			hasMore = true
			break
		}
		items = append(items, &stored)
	}
	if err := iter.Error(); err != nil {
		return nil, false, err
	}
	return items, hasMore, nil
}

func (s *Store) MarkDelivered(ctx context.Context, recipientID string, items []*protocol.StoredMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	now := s.nowFunc().UTC().Unix()
	unlock := s.recipientShards.Lock(recipientID)
	defer unlock()

	current, err := s.frontier(deliveredKey(recipientID))
	if err != nil {
		return err
	}
	delivered := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if item != nil {
			delivered[item.StoreSeq] = struct{}{}
		}
	}

	iter, err := s.db.NewIter(&cpebble.IterOptions{
		LowerBound: []byte(msgKey(recipientID, current+1)),
		UpperBound: prefixUpperBound(msgPrefix(recipientID)),
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	next := current + 1
	newFrontier := current
	for iter.First(); iter.Valid(); iter.Next() {
		var stored protocol.StoredMessage
		if err := json.Unmarshal(iter.Value(), &stored); err != nil {
			return err
		}
		if stored.StoreSeq != next {
			break
		}
		if stored.ExpireAt <= now {
			newFrontier = stored.StoreSeq
			next++
			continue
		}
		if _, ok := delivered[stored.StoreSeq]; !ok {
			break
		}
		newFrontier = stored.StoreSeq
		next++
	}
	if err := iter.Error(); err != nil {
		return err
	}
	if newFrontier == current {
		return nil
	}
	batch := s.db.NewBatch()
	defer batch.Close()
	if err := s.setFrontier(batch, deliveredKey(recipientID), newFrontier); err != nil {
		return err
	}
	return batch.Commit(cpebble.Sync)
}
