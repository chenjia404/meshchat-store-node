package protocol

import "encoding/json"

// 统一 libp2p 流协议 /meshchat/offline-store/rpc/1.0.0 下的方法名。
const (
	MethodOfflineStore = "offline.store"
	MethodOfflineFetch = "offline.fetch"
	MethodOfflineAck   = "offline.ack"
)

// RPCRequest 为单一流上的统一请求格式。
type RPCRequest struct {
	RequestID string          `json:"request_id"`
	Method    string          `json:"method"`
	Body      json.RawMessage `json:"body"`
}

// RPCResponse 为单一流上的统一响应格式。
type RPCResponse struct {
	RequestID string          `json:"request_id"`
	OK        bool            `json:"ok"`
	Error     string          `json:"error"`
	Body      json.RawMessage `json:"body"`
}

// RPCErrorBody 为 RPC 层错误（非法 JSON、缺 request_id、未知 method 等）时 body 的固定形状，
// 与业务层 StoreResponse / FetchResponse / AckResponse 一样都含 error_code / error_message，便于客户端统一解析。
type RPCErrorBody struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Method       string `json:"method,omitempty"`
}
