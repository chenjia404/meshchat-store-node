package publicchannel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"store-node/internal/protocol"
)

// Store SQLite 持久化（协议 v1 表结构）。
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error {
	return s.db.Close()
}

// channelRow 内部行映射。
type channelRow struct {
	ID                      int64
	ChannelID               string
	OwnerPeerID             string
	OwnerVersion            int
	Name                    string
	AvatarJSON              string
	Bio                     string
	MessageRetentionMinutes int
	ProfileVersion          int
	LastMessageID           int
	LastSeq                 int
	CreatedAt               int64
	UpdatedAt               int64
	HeadUpdatedAt           int64
	ProfileSignature        string
	HeadSignature           string
}

func (s *Store) getChannelByChannelID(ctx context.Context, channelID string) (*channelRow, error) {
	return s.scanChannelRow(s.db.QueryRowContext(ctx, `
SELECT id, channel_id, owner_peer_id, owner_version, name, avatar_json, bio, message_retention_minutes,
       profile_version, last_message_id, last_seq, created_at, updated_at, head_updated_at,
       profile_signature, head_signature
FROM public_channels WHERE channel_id = ?`, channelID))
}

func (s *Store) getChannelByChannelIDTx(ctx context.Context, tx *sql.Tx, channelID string) (*channelRow, error) {
	return s.scanChannelRow(tx.QueryRowContext(ctx, `
SELECT id, channel_id, owner_peer_id, owner_version, name, avatar_json, bio, message_retention_minutes,
       profile_version, last_message_id, last_seq, created_at, updated_at, head_updated_at,
       profile_signature, head_signature
FROM public_channels WHERE channel_id = ?`, channelID))
}

func (s *Store) scanChannelRow(row *sql.Row) (*channelRow, error) {
	var r channelRow
	err := row.Scan(
		&r.ID, &r.ChannelID, &r.OwnerPeerID, &r.OwnerVersion, &r.Name, &r.AvatarJSON, &r.Bio,
		&r.MessageRetentionMinutes, &r.ProfileVersion, &r.LastMessageID, &r.LastSeq,
		&r.CreatedAt, &r.UpdatedAt, &r.HeadUpdatedAt, &r.ProfileSignature, &r.HeadSignature,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) upsertChannel(ctx context.Context, tx *sql.Tx, p *ChannelProfile, h *ChannelHead) error {
	avatarJSON := "{}"
	if p.Avatar != nil {
		b, err := json.Marshal(p.Avatar)
		if err != nil {
			return err
		}
		avatarJSON = string(b)
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO public_channels (
  channel_id, owner_peer_id, owner_version, name, avatar_json, bio, message_retention_minutes,
  profile_version, last_message_id, last_seq, created_at, updated_at, head_updated_at,
  profile_signature, head_signature
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(channel_id) DO UPDATE SET
  owner_peer_id = excluded.owner_peer_id,
  owner_version = excluded.owner_version,
  name = excluded.name,
  avatar_json = excluded.avatar_json,
  bio = excluded.bio,
  message_retention_minutes = excluded.message_retention_minutes,
  profile_version = excluded.profile_version,
  last_message_id = excluded.last_message_id,
  last_seq = excluded.last_seq,
  updated_at = excluded.updated_at,
  head_updated_at = excluded.head_updated_at,
  profile_signature = excluded.profile_signature,
  head_signature = excluded.head_signature
`,
		p.ChannelID, p.OwnerPeerID, p.OwnerVersion, p.Name, avatarJSON, p.Bio, p.MessageRetentionMinutes,
		p.ProfileVersion, h.LastMessageID, h.LastSeq, p.CreatedAt, p.UpdatedAt, h.UpdatedAt,
		p.Signature, h.Signature,
	)
	return err
}

type messageRow struct {
	MessageID     int
	Version       int
	Seq           int
	OwnerVersion  int
	CreatorPeerID string
	AuthorPeerID  string
	CreatedAt     int64
	UpdatedAt     int64
	IsDeleted     bool
	MessageType   string
	ContentJSON   string
	Signature     string
}

func (s *Store) getMessage(ctx context.Context, tx *sql.Tx, channelDBID int64, messageID int) (*messageRow, error) {
	var r messageRow
	var isDel int
	q := `SELECT message_id, version, seq, owner_version, creator_peer_id, author_peer_id,
		created_at, updated_at, is_deleted, message_type, content_json, signature
		FROM public_channel_messages WHERE channel_db_id = ? AND message_id = ?`
	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx, q, channelDBID, messageID).Scan(
			&r.MessageID, &r.Version, &r.Seq, &r.OwnerVersion, &r.CreatorPeerID, &r.AuthorPeerID,
			&r.CreatedAt, &r.UpdatedAt, &isDel, &r.MessageType, &r.ContentJSON, &r.Signature,
		)
	} else {
		err = s.db.QueryRowContext(ctx, q, channelDBID, messageID).Scan(
			&r.MessageID, &r.Version, &r.Seq, &r.OwnerVersion, &r.CreatorPeerID, &r.AuthorPeerID,
			&r.CreatedAt, &r.UpdatedAt, &isDel, &r.MessageType, &r.ContentJSON, &r.Signature,
		)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.IsDeleted = isDel != 0
	return &r, nil
}

func contentJSON(m *ChannelMessage) (string, error) {
	b, err := json.Marshal(m.Content)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Push 由频道 owner 提交资料、头与消息（事务）。
func (s *Store) Push(ctx context.Context, remotePeerID string, p *ChannelProfile, h *ChannelHead, messages []*ChannelMessage, extraChanges []*ChannelChange) error {
	if err := ValidateChannelID(p.ChannelID); err != nil {
		return err
	}
	if p.ChannelID != h.ChannelID {
		return protocol.NewAppError(protocol.CodeInvalidPayload, "profile and head channel_id mismatch")
	}
	if remotePeerID != p.OwnerPeerID {
		return protocol.NewAppError(protocol.CodeUnauthorized, "only owner peer may push")
	}
	if err := VerifyProfile(p); err != nil {
		return err
	}
	if err := VerifyHead(h); err != nil {
		return err
	}
	for _, m := range messages {
		if m.ChannelID != p.ChannelID {
			return protocol.NewAppError(protocol.CodeInvalidPayload, "message channel_id mismatch")
		}
		if err := VerifyMessage(p.OwnerPeerID, m); err != nil {
			return err
		}
	}

	old, err := s.getChannelByChannelID(ctx, p.ChannelID)
	if err != nil {
		return err
	}
	if old != nil {
		if h.LastSeq < old.LastSeq {
			return protocol.NewAppError(protocol.CodePublicChannelStale, "head.last_seq regressed")
		}
		if h.OwnerPeerID != old.OwnerPeerID {
			return protocol.NewAppError(protocol.CodeUnauthorized, "owner_peer_id cannot change")
		}
	}

	for _, m := range messages {
		if m.Seq > h.LastSeq || m.MessageID > h.LastMessageID {
			return protocol.NewAppError(protocol.CodeInvalidPayload, "message seq/message_id exceeds head")
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.upsertChannel(ctx, tx, p, h); err != nil {
		return err
	}
	chRow, err := s.getChannelByChannelIDTx(ctx, tx, p.ChannelID)
	if err != nil {
		return err
	}
	if chRow == nil {
		return protocol.NewAppError(protocol.CodeInternalError, "channel row missing after upsert")
	}
	channelDBID := chRow.ID

	for _, m := range messages {
		prev, err := s.getMessage(ctx, tx, channelDBID, m.MessageID)
		if err != nil {
			return err
		}
		if prev != nil {
			if m.Version < prev.Version {
				return protocol.NewAppError(protocol.CodePublicChannelStale, "message version regressed")
			}
			if m.Version == prev.Version && m.Seq < prev.Seq {
				return protocol.NewAppError(protocol.CodePublicChannelStale, "message seq regressed for same version")
			}
		}
		cj, err := contentJSON(m)
		if err != nil {
			return err
		}
		isDel := 0
		if m.IsDeleted {
			isDel = 1
		}
		_, err = tx.ExecContext(ctx, `
DELETE FROM public_channel_messages WHERE channel_db_id = ? AND message_id = ?`, channelDBID, m.MessageID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO public_channel_messages (
  channel_db_id, message_id, version, seq, owner_version, creator_peer_id, author_peer_id,
  created_at, updated_at, is_deleted, message_type, content_json, signature
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			channelDBID, m.MessageID, m.Version, m.Seq, m.OwnerVersion, m.CreatorPeerID, m.AuthorPeerID,
			m.CreatedAt, m.UpdatedAt, isDel, m.MessageType, cj, m.Signature,
		)
		if err != nil {
			return err
		}
		ver := m.Version
		del := m.IsDeleted
		_, err = tx.ExecContext(ctx, `
INSERT INTO public_channel_changes (channel_db_id, seq, change_type, message_id, version, is_deleted, profile_version, created_at)
VALUES (?, ?, 'message', ?, ?, ?, NULL, ?)
ON CONFLICT(channel_db_id, seq) DO UPDATE SET
  change_type = excluded.change_type,
  message_id = excluded.message_id,
  version = excluded.version,
  is_deleted = excluded.is_deleted,
  created_at = excluded.created_at`,
			channelDBID, m.Seq, m.MessageID, ver, boolToInt(del), m.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	oldPV := 0
	if old != nil {
		oldPV = old.ProfileVersion
	}
	if p.ProfileVersion > oldPV && len(messages) == 0 && len(extraChanges) == 0 {
		_, err = tx.ExecContext(ctx, `
INSERT INTO public_channel_changes (channel_db_id, seq, change_type, message_id, version, is_deleted, profile_version, created_at)
VALUES (?, ?, 'profile', NULL, NULL, NULL, ?, ?)
ON CONFLICT(channel_db_id, seq) DO UPDATE SET
  change_type = 'profile',
  profile_version = excluded.profile_version,
  created_at = excluded.created_at`,
			channelDBID, h.LastSeq, p.ProfileVersion, h.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	for _, ch := range extraChanges {
		if ch.ChannelID != "" && ch.ChannelID != p.ChannelID {
			return protocol.NewAppError(protocol.CodeInvalidPayload, "change channel_id mismatch")
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO public_channel_changes (channel_db_id, seq, change_type, message_id, version, is_deleted, profile_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(channel_db_id, seq) DO UPDATE SET
  change_type = excluded.change_type,
  message_id = excluded.message_id,
  version = excluded.version,
  is_deleted = excluded.is_deleted,
  profile_version = excluded.profile_version,
  created_at = excluded.created_at`,
			channelDBID, ch.Seq, ch.ChangeType, nullableInt(ch.MessageID), nullableIntPtr(ch.Version), nullableBoolPtr(ch.IsDeleted), nullableIntPtr(ch.ProfileVersion), ch.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableIntPtr(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableBoolPtr(p *bool) any {
	if p == nil {
		return nil
	}
	if *p {
		return 1
	}
	return 0
}

// GetProfile 返回频道资料 JSON 视图。
func (s *Store) GetProfile(ctx context.Context, channelID string) (*ChannelProfile, error) {
	r, err := s.getChannelByChannelID(ctx, channelID)
	if err != nil || r == nil {
		return nil, err
	}
	var av *ChannelImage
	if r.AvatarJSON != "" && r.AvatarJSON != "{}" {
		var img ChannelImage
		if err := json.Unmarshal([]byte(r.AvatarJSON), &img); err != nil {
			return nil, err
		}
		av = &img
	}
	return &ChannelProfile{
		ChannelID:               r.ChannelID,
		OwnerPeerID:             r.OwnerPeerID,
		OwnerVersion:            r.OwnerVersion,
		Name:                    r.Name,
		Avatar:                  av,
		Bio:                     r.Bio,
		MessageRetentionMinutes: r.MessageRetentionMinutes,
		ProfileVersion:          r.ProfileVersion,
		CreatedAt:               r.CreatedAt,
		UpdatedAt:               r.UpdatedAt,
		Signature:               r.ProfileSignature,
	}, nil
}

// GetHead 返回频道头。
func (s *Store) GetHead(ctx context.Context, channelID string) (*ChannelHead, error) {
	r, err := s.getChannelByChannelID(ctx, channelID)
	if err != nil || r == nil {
		return nil, err
	}
	return &ChannelHead{
		ChannelID:      r.ChannelID,
		OwnerPeerID:    r.OwnerPeerID,
		OwnerVersion:   r.OwnerVersion,
		LastMessageID:  r.LastMessageID,
		ProfileVersion: r.ProfileVersion,
		LastSeq:        r.LastSeq,
		UpdatedAt:      r.HeadUpdatedAt,
		Signature:      r.HeadSignature,
	}, nil
}

// GetMessage 单条消息。
func (s *Store) GetMessage(ctx context.Context, channelID string, messageID int) (*ChannelMessage, error) {
	r, err := s.getChannelByChannelID(ctx, channelID)
	if err != nil || r == nil {
		return nil, err
	}
	mr, err := s.getMessage(ctx, nil, r.ID, messageID)
	if err != nil || mr == nil {
		return nil, err
	}
	return rowToMessage(r.ChannelID, mr)
}

func rowToMessage(channelID string, mr *messageRow) (*ChannelMessage, error) {
	var c ChannelContent
	if mr.ContentJSON != "" {
		if err := json.Unmarshal([]byte(mr.ContentJSON), &c); err != nil {
			return nil, err
		}
	}
	return &ChannelMessage{
		ChannelID:     channelID,
		MessageID:     mr.MessageID,
		Version:       mr.Version,
		Seq:           mr.Seq,
		OwnerVersion:  mr.OwnerVersion,
		CreatorPeerID: mr.CreatorPeerID,
		AuthorPeerID:  mr.AuthorPeerID,
		CreatedAt:     mr.CreatedAt,
		UpdatedAt:     mr.UpdatedAt,
		IsDeleted:     mr.IsDeleted,
		MessageType:   mr.MessageType,
		Content:       c,
		Signature:     mr.Signature,
	}, nil
}

// ListMessages 按 message_id DESC 分页。
func (s *Store) ListMessages(ctx context.Context, channelID string, beforeMessageID *int, limit int) ([]*ChannelMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	r, err := s.getChannelByChannelID(ctx, channelID)
	if err != nil || r == nil {
		return nil, err
	}
	var rows *sql.Rows
	if beforeMessageID != nil {
		rows, err = s.db.QueryContext(ctx, `
SELECT message_id, version, seq, owner_version, creator_peer_id, author_peer_id,
  created_at, updated_at, is_deleted, message_type, content_json, signature
FROM public_channel_messages WHERE channel_db_id = ? AND message_id < ?
ORDER BY message_id DESC LIMIT ?`, r.ID, *beforeMessageID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT message_id, version, seq, owner_version, creator_peer_id, author_peer_id,
  created_at, updated_at, is_deleted, message_type, content_json, signature
FROM public_channel_messages WHERE channel_db_id = ?
ORDER BY message_id DESC LIMIT ?`, r.ID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ChannelMessage
	for rows.Next() {
		var mr messageRow
		var isDel int
		if err := rows.Scan(&mr.MessageID, &mr.Version, &mr.Seq, &mr.OwnerVersion, &mr.CreatorPeerID, &mr.AuthorPeerID,
			&mr.CreatedAt, &mr.UpdatedAt, &isDel, &mr.MessageType, &mr.ContentJSON, &mr.Signature); err != nil {
			return nil, err
		}
		mr.IsDeleted = isDel != 0
		m, err := rowToMessage(r.ChannelID, &mr)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SyncResult 增量同步结果。
type SyncResult struct {
	ChannelID      string
	CurrentLastSeq int
	HasMore        bool
	NextAfterSeq   int
	Items          []ChannelChange
}

// SyncChanges seq > afterSeq。
func (s *Store) SyncChanges(ctx context.Context, channelID string, afterSeq int, limit int) (*SyncResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	r, err := s.getChannelByChannelID(ctx, channelID)
	if err != nil || r == nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT seq, change_type, message_id, version, is_deleted, profile_version, created_at
FROM public_channel_changes WHERE channel_db_id = ? AND seq > ?
ORDER BY seq ASC LIMIT ?`, r.ID, afterSeq, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ChannelChange
	for rows.Next() {
		var ch ChannelChange
		var msgID, ver, isDel, pv sql.NullInt64
		if err := rows.Scan(&ch.Seq, &ch.ChangeType, &msgID, &ver, &isDel, &pv, &ch.CreatedAt); err != nil {
			return nil, err
		}
		ch.ChannelID = channelID
		if msgID.Valid {
			v := int(msgID.Int64)
			ch.MessageID = &v
		}
		if ver.Valid {
			v := int(ver.Int64)
			ch.Version = &v
		}
		if isDel.Valid {
			v := isDel.Int64 != 0
			ch.IsDeleted = &v
		}
		if pv.Valid {
			v := int(pv.Int64)
			ch.ProfileVersion = &v
		}
		items = append(items, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextAfter := afterSeq
	if len(items) > 0 {
		nextAfter = items[len(items)-1].Seq
	}
	return &SyncResult{
		ChannelID:      channelID,
		CurrentLastSeq: r.LastSeq,
		HasMore:        hasMore,
		NextAfterSeq:   nextAfter,
		Items:          items,
	}, nil
}
