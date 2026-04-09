package publicchannel

// PushRequest owner 向存储节点提交资料、头与消息列表。
type PushRequest struct {
	Profile  *ChannelProfile   `json:"profile"`
	Head     *ChannelHead      `json:"head"`
	Messages []*ChannelMessage `json:"messages,omitempty"`
	Changes  []*ChannelChange  `json:"changes,omitempty"`
}

// PushResponse 推送结果。
type PushResponse struct {
	OK           bool   `json:"ok"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// GetProfileRequest 拉取频道资料。
type GetProfileRequest struct {
	ChannelID string `json:"channel_id"`
}

// GetProfileResponse 。
type GetProfileResponse struct {
	OK           bool            `json:"ok"`
	Profile      *ChannelProfile `json:"profile,omitempty"`
	ErrorCode    string          `json:"error_code,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

// GetHeadRequest 拉取频道头。
type GetHeadRequest struct {
	ChannelID string `json:"channel_id"`
}

// GetHeadResponse 。
type GetHeadResponse struct {
	OK           bool         `json:"ok"`
	Head         *ChannelHead `json:"head,omitempty"`
	ErrorCode    string       `json:"error_code,omitempty"`
	ErrorMessage string       `json:"error_message,omitempty"`
}

// ListMessagesRequest 消息列表（message_id DESC）。
type ListMessagesRequest struct {
	ChannelID       string `json:"channel_id"`
	Limit           int    `json:"limit"`
	BeforeMessageID *int   `json:"before_message_id,omitempty"`
}

// ListMessagesResponse 。
type ListMessagesResponse struct {
	OK           bool               `json:"ok"`
	Messages     []*ChannelMessage  `json:"messages,omitempty"`
	ErrorCode    string             `json:"error_code,omitempty"`
	ErrorMessage string             `json:"error_message,omitempty"`
}

// GetMessageRequest 单条消息。
type GetMessageRequest struct {
	ChannelID string `json:"channel_id"`
	MessageID int    `json:"message_id"`
}

// GetMessageResponse 。
type GetMessageResponse struct {
	OK           bool             `json:"ok"`
	Message      *ChannelMessage  `json:"message,omitempty"`
	ErrorCode    string           `json:"error_code,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
}

// SyncChannelRequest 增量同步（seq > after_seq）。
type SyncChannelRequest struct {
	ChannelID string `json:"channel_id"`
	AfterSeq  int    `json:"after_seq"`
	Limit     int    `json:"limit"`
}

// SyncChannelResponse 与文档 12.4 一致。
type SyncChannelResponse struct {
	OK             bool            `json:"ok"`
	ChannelID      string          `json:"channel_id,omitempty"`
	CurrentLastSeq int             `json:"current_last_seq,omitempty"`
	HasMore        bool            `json:"has_more,omitempty"`
	NextAfterSeq   int             `json:"next_after_seq,omitempty"`
	Items          []ChannelChange `json:"items,omitempty"`
	ErrorCode      string          `json:"error_code,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
}
