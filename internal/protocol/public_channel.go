package protocol

// PublicChannelRPCProtocol 与 mesh-proxy 文档 13.3 统一流协议。
const PublicChannelRPCProtocol = "/meshchat/public-channel/rpc/1.0.0"

// 公共频道 RPC 方法名（与文档 13.5 对齐的简化命名）。
const (
	MethodPublicChannelPush         = "public_channel.push"
	MethodPublicChannelGetProfile   = "public_channel.get_profile"
	MethodPublicChannelGetHead      = "public_channel.get_head"
	MethodPublicChannelListMessages = "public_channel.list_messages"
	MethodPublicChannelGetMessage   = "public_channel.get_message"
	MethodPublicChannelSyncChannel  = "public_channel.sync_channel"
)
