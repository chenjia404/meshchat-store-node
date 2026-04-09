package publicchannel

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "public_channels.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := NewStore(db)
	ctx := t.Context()
	p, err := s.GetProfile(ctx, "00000000-0000-7000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Fatal("expected nil profile for missing channel")
	}
}
