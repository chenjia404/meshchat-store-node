package pebble

import (
	"context"
	"encoding/json"

	cpebble "github.com/cockroachdb/pebble"
)

func (s *Store) DeleteExpired(ctx context.Context, now int64, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	type expiredRef struct {
		expireAt    int64
		recipientID string
		storeSeq    uint64
		expKey      []byte
	}

	iter, err := s.db.NewIter(&cpebble.IterOptions{
		LowerBound: []byte("exp/"),
		UpperBound: []byte(expUpperBound(now)),
	})
	if err != nil {
		return 0, err
	}
	defer iter.Close()

	refs := make([]expiredRef, 0, limit)
	for iter.First(); iter.Valid(); iter.Next() {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		expireAt, recipientID, storeSeq, err := parseExpKey(iter.Key())
		if err != nil {
			return 0, err
		}
		if expireAt > now {
			break
		}
		refs = append(refs, expiredRef{
			expireAt:    expireAt,
			recipientID: recipientID,
			storeSeq:    storeSeq,
			expKey:      append([]byte(nil), iter.Key()...),
		})
		if len(refs) >= limit {
			break
		}
	}
	if err := iter.Error(); err != nil {
		return 0, err
	}

	var deleted int
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		unlock := s.recipientShards.Lock(ref.recipientID)

		batch := s.db.NewBatch()
		msgValue, closer, getErr := s.db.Get([]byte(msgKey(ref.recipientID, ref.storeSeq)))
		if getErr == nil {
			var stored struct {
				ExpireAt int64 `json:"expire_at"`
				Message  struct {
					MsgID string `json:"msg_id"`
				} `json:"message"`
			}
			if err := json.Unmarshal(msgValue, &stored); err != nil {
				closer.Close()
				batch.Close()
				unlock()
				return deleted, err
			}
			closer.Close()
			if err := batch.Delete([]byte(msgKey(ref.recipientID, ref.storeSeq)), nil); err != nil {
				batch.Close()
				unlock()
				return deleted, err
			}
			if stored.Message.MsgID != "" {
				if err := batch.Delete([]byte(dupKey(ref.recipientID, stored.Message.MsgID)), nil); err != nil {
					batch.Close()
					unlock()
					return deleted, err
				}
			}
		} else if getErr != cpebble.ErrNotFound {
			batch.Close()
			unlock()
			return deleted, getErr
		}
		if err := batch.Delete(ref.expKey, nil); err != nil {
			batch.Close()
			unlock()
			return deleted, err
		}
		if err := batch.Commit(cpebble.Sync); err != nil {
			batch.Close()
			unlock()
			return deleted, err
		}
		batch.Close()
		unlock()
		deleted++
	}
	return deleted, nil
}
