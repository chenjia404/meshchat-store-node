package pebble

import (
	"time"

	"github.com/cockroachdb/pebble"

	"store-node/internal/shard"
)

type Store struct {
	db              *pebble.DB
	nowFunc         func() time.Time
	ackDeleteBatch  int
	recipientShards *shard.Manager
}

func Open(path string) (*Store, error) {
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		return nil, err
	}
	return &Store{
		db:              db,
		nowFunc:         time.Now,
		ackDeleteBatch:  1000,
		recipientShards: shard.New(256),
	}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SetNowFunc(nowFunc func() time.Time) {
	if nowFunc != nil {
		s.nowFunc = nowFunc
	}
}
