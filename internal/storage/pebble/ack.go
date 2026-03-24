package pebble

import (
	"context"
	"encoding/json"

	cpebble "github.com/cockroachdb/pebble"

	"store-node/internal/protocol"
)

func (s *Store) AckMessages(ctx context.Context, recipientID string, ackSeq uint64) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	unlock := s.recipientShards.Lock(recipientID)
	defer unlock()

	ackedUntil, err := s.frontier(ackedKey(recipientID))
	if err != nil {
		return 0, err
	}
	if ackSeq <= ackedUntil {
		return ackedUntil, nil
	}

	deliveredUntil, err := s.frontier(deliveredKey(recipientID))
	if err != nil {
		return 0, err
	}
	if ackSeq > deliveredUntil {
		return ackedUntil, protocol.NewAppError(protocol.CodeInvalidPayload, "ack_seq exceeds delivered frontier")
	}

	deletedUntil := ackedUntil
	for {
		batch := s.db.NewBatch()
		iter, err := s.db.NewIter(&cpebble.IterOptions{
			LowerBound: []byte(msgPrefix(recipientID)),
			UpperBound: prefixUpperBound(msgPrefix(recipientID)),
		})
		if err != nil {
			batch.Close()
			return deletedUntil, err
		}

		var deletedInBatch int
		for iter.First(); iter.Valid(); iter.Next() {
			if err := ctx.Err(); err != nil {
				iter.Close()
				batch.Close()
				return deletedUntil, err
			}
			_, storeSeq, err := parseMsgKey(iter.Key())
			if err != nil {
				iter.Close()
				batch.Close()
				return deletedUntil, err
			}
			if storeSeq > ackSeq {
				break
			}

			var stored protocol.StoredMessage
			if err := json.Unmarshal(iter.Value(), &stored); err != nil {
				iter.Close()
				batch.Close()
				return deletedUntil, err
			}
			if err := batch.Delete(iter.Key(), nil); err != nil {
				iter.Close()
				batch.Close()
				return deletedUntil, err
			}
			if stored.Message != nil {
				if err := batch.Delete([]byte(dupKey(recipientID, stored.Message.MsgID)), nil); err != nil {
					iter.Close()
					batch.Close()
					return deletedUntil, err
				}
			}
			if err := batch.Delete([]byte(expKey(stored.ExpireAt, recipientID, storeSeq)), nil); err != nil {
				iter.Close()
				batch.Close()
				return deletedUntil, err
			}
			deletedUntil = storeSeq
			deletedInBatch++
			if deletedInBatch >= s.ackDeleteBatch {
				break
			}
		}
		if err := iter.Error(); err != nil {
			iter.Close()
			batch.Close()
			return deletedUntil, err
		}
		iter.Close()

		if deletedInBatch == 0 {
			if deletedUntil < ackSeq {
				if err := s.setFrontier(batch, ackedKey(recipientID), ackSeq); err != nil {
					batch.Close()
					return deletedUntil, err
				}
				if err := batch.Commit(cpebble.Sync); err != nil {
					batch.Close()
					return deletedUntil, err
				}
				deletedUntil = ackSeq
			}
			batch.Close()
			return deletedUntil, nil
		}
		if deletedUntil >= ackedUntil {
			if err := s.setFrontier(batch, ackedKey(recipientID), deletedUntil); err != nil {
				batch.Close()
				return deletedUntil, err
			}
		}
		if err := batch.Commit(cpebble.Sync); err != nil {
			batch.Close()
			return deletedUntil, err
		}
		batch.Close()

		if deletedInBatch < s.ackDeleteBatch || deletedUntil >= ackSeq {
			return deletedUntil, nil
		}
	}
}
