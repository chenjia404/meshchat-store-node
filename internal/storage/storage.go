package storage

import (
	"context"

	"store-node/internal/protocol"
)

type Storage interface {
	StoreMessage(ctx context.Context, msg *protocol.StoredMessage) (storeSeq uint64, expireAt int64, duplicate bool, err error)
	FetchMessages(ctx context.Context, recipientID string, afterSeq uint64, limit int) ([]*protocol.StoredMessage, bool, error)
	AckMessages(ctx context.Context, recipientID string, ackSeq uint64) (deletedUntil uint64, err error)
	DeleteExpired(ctx context.Context, now int64, limit int) (deleted int, err error)
}
