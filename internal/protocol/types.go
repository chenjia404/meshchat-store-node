package protocol

type CipherPayload struct {
	Algorithm      string `json:"algorithm"`
	RecipientKeyID string `json:"recipient_key_id"`
	Nonce          string `json:"nonce"`
	Ciphertext     string `json:"ciphertext"`
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type OfflineMessageEnvelope struct {
	Version        int           `json:"version"`
	MsgID          string        `json:"msg_id"`
	SenderID       string        `json:"sender_id"`
	RecipientID    string        `json:"recipient_id"`
	ConversationID string        `json:"conversation_id"`
	CreatedAt      int64         `json:"created_at"`
	TTLSec         *int64        `json:"ttl_sec,omitempty"`
	Cipher         CipherPayload `json:"cipher"`
	Signature      Signature     `json:"signature"`
}

func (m *OfflineMessageEnvelope) EffectiveTTL(defaultTTL int64) int64 {
	if m == nil || m.TTLSec == nil {
		return defaultTTL
	}
	return *m.TTLSec
}

func (m *OfflineMessageEnvelope) Clone() *OfflineMessageEnvelope {
	if m == nil {
		return nil
	}
	cp := *m
	if m.TTLSec != nil {
		ttl := *m.TTLSec
		cp.TTLSec = &ttl
	}
	return &cp
}

type StoredMessage struct {
	StoreSeq   uint64                  `json:"store_seq"`
	ReceivedAt int64                   `json:"received_at"`
	ExpireAt   int64                   `json:"expire_at"`
	Message    *OfflineMessageEnvelope `json:"message"`
}

type StoreRequest struct {
	Version int                     `json:"version"`
	Message *OfflineMessageEnvelope `json:"message"`
}

type StoreResponse struct {
	OK           bool   `json:"ok"`
	Duplicate    bool   `json:"duplicate"`
	StoreSeq     uint64 `json:"store_seq"`
	ExpireAt     int64  `json:"expire_at"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

type FetchRequest struct {
	Version     int    `json:"version"`
	RecipientID string `json:"recipient_id"`
	AfterSeq    uint64 `json:"after_seq"`
	Limit       int    `json:"limit"`
}

type FetchResponse struct {
	OK           bool             `json:"ok"`
	Items        []*StoredMessage `json:"items"`
	HasMore      bool             `json:"has_more"`
	ErrorCode    string           `json:"error_code"`
	ErrorMessage string           `json:"error_message"`
}

type AckRequest struct {
	Version     int       `json:"version"`
	RecipientID string    `json:"recipient_id"`
	DeviceID    string    `json:"device_id"`
	AckSeq      uint64    `json:"ack_seq"`
	AckedAt     int64     `json:"acked_at"`
	Signature   Signature `json:"signature"`
}

type AckResponse struct {
	OK              bool   `json:"ok"`
	DeletedUntilSeq uint64 `json:"deleted_until_seq"`
	ErrorCode       string `json:"error_code"`
	ErrorMessage    string `json:"error_message"`
}
