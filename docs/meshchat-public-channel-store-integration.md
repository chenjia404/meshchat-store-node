# meshchat 接入 store-node 公开频道存储（AI 实现说明）

本文档供在 **meshchat** 侧实现「去中心化公开频道」与 **store-node** 的 **推送（owner）** 与 **拉取（任意节点）** 时使用。实现须与本文及引用规范 **逐字段一致**，否则验签或入库会失败。

---

## 1. 角色与目标

| 角色 | 行为 |
|------|------|
| **频道 owner** | 使用 **与 `owner_peer_id` 相同的 libp2p 身份** 连接 store-node，调用 `public_channel.push` 提交已签名的资料、头、消息（及可选变更索引）。 |
| **订阅者 / 读者** | 任意 libp2p 节点调用 `get_*` / `sync_channel` / `list_messages` 拉取数据，**无需**与 `owner_peer_id` 一致。 |

**store-node** 只做：验签、单调性检查、SQLite 持久化；不负责代你生成 `channel_id`、分配 `seq` 或替你签名。这些均在 **meshchat / mesh-proxy 业务层** 按 v1 协议完成后再推送。

---

## 2. 规范性引用（必读）

业务语义、字段含义、`seq` / `message_id` / `version` 规则、消息类型等以官方说明为准：

- 仓库：**[meshproxy 文档：去中心化公开频道协议与本地存储设计（最终版 v1）](https://github.com/chenjia404/meshproxy/blob/master/docs/%E5%8E%BB%E4%B8%AD%E5%BF%83%E5%8C%96%E5%85%AC%E5%BC%80%E9%A2%91%E9%81%93%E5%8D%8F%E8%AE%AE%E4%B8%8E%E6%9C%AC%E5%9C%B0%E5%AD%98%E5%82%A8%E8%AE%BE%E8%AE%A1%EF%BC%88%E6%9C%80%E7%BB%88%E7%89%88%20v1%EF%BC%89.md)**

本文仅描述 **与 store-node 对接的传输层、RPC 名、JSON 形状及验签字节构造**（与 `store-node` 源码一致）。

---

## 3. 传输：libp2p Stream

- **协议 ID（必须完全一致）**  
  ` /meshchat/public-channel/rpc/1.0.0 `

- **连接对象**  
  使用 libp2p 连接至 store-node 的 `Host`（多地址中含 `/p2p/<store_peer_id>`）。

- **请求/响应模式**  
  每个 RPC：**客户端发送一帧请求 → 服务端返回一帧响应后关闭 stream**（与现有 offline-store RPC 相同的一请求一应答模型）。可每次调用新开 stream。

---

## 4. 帧格式（与 offline-store 相同）

1. **前 4 字节**：载荷长度，**大端 `uint32`**（不含这 4 字节自身）。
2. **后续 N 字节**：UTF-8 JSON 文本。

**单帧大小上限**：store-node 对公共频道 RPC 限制为 **8 MiB**（含请求与响应）。超长会被拒绝。

---

## 5. 外层 RPC 信封

所有方法共用同一外层结构（字段名固定）。

### 5.1 请求 `RPCRequest`

```json
{
  "request_id": "uuid 或任意唯一字符串",
  "method": "见第 7 节方法名",
  "body": { }
}
```

- `body` 为**方法专属** JSON（见第 7 节），整段作为 JSON 对象序列化进 `body` 字段。

### 5.2 响应 `RPCResponse`

```json
{
  "request_id": "与请求一致",
  "ok": true,
  "error": "",
  "body": { }
}
```

- 业务成功时 `ok` 为 `true`，`error` 多为空字符串；失败时 `ok` 为 `false`，`error` 可含可读说明。
- **具体错误码与业务体**在 `body` 内（各方法内层结构中的 `error_code` / `error_message`），与离线 store 一样需**解析 `body`**。

### 5.3 RPC 层错误（如非法 JSON、缺 `request_id`、未知 `method`）

`ok` 为 `false`，`body` 可能为：

```json
{
  "error_code": "RPC_INVALID_REQUEST | RPC_MISSING_REQUEST_ID | RPC_UNKNOWN_METHOD | ...",
  "error_message": "人类可读说明",
  "method": "未知方法时可能带上请求的 method"
}
```

---

## 6. JSON 数据模型（与 store-node 一致）

时间戳均为 **Unix 秒**（整数）。

### 6.1 `ChannelImage`

字段顺序在 **canonical** 中固定（见第 8 节）：`cid`, `media_id`, `blob_id`, `sha256`, `url`, `mime`, `size`, `width`, `height`, `name`。

### 6.2 `ChannelFile`

`cid`, `media_id`, `blob_id`, `sha256`, `url`, `mime`, `size`, `name`。

### 6.3 `ChannelContent`

```json
{
  "text": "",
  "images": [ /* ChannelImage */ ],
  "files": [ /* ChannelFile */ ]
}
```

### 6.4 `ChannelProfile`

含 `signature`（Base64 编码的签名字节，见第 8 节）。  
`owner_version` 在 v1 中为 **1**。  
`channel_id` 必须为 **UUIDv7** 字符串。

### 6.5 `ChannelHead`

含 `signature`。与 `ChannelProfile` 的 `channel_id`、`owner_peer_id` 必须一致。

### 6.6 `ChannelMessage`

- v1：`owner_version` 必须为 **1**；`author_peer_id` 必须等于 **`owner_peer_id`**（仅 owner 写）。
- `message_type`：`text` / `image` / `file` / `system` / `deleted` 等（与 v1 文档一致）。
- 含 `signature`。

### 6.7 `ChannelChange`（同步增量项）

用于 `sync_channel` 返回及可选 `push.changes`。

```json
{
  "channel_id": "0195f3f0-8d4a-7c12-b2c1-9db1f0a9e123",
  "seq": 121,
  "change_type": "message",
  "message_id": 201,
  "version": 1,
  "is_deleted": false,
  "profile_version": null,
  "created_at": 1710010000
}
```

`message_id` / `version` / `is_deleted` / `profile_version` 在类型为 `profile` 等场景下可为 JSON `null`（Go 侧为指针省略或 null）。

---

## 7. 方法列表

### 7.1 `public_channel.push`（仅 owner）

**用途**：提交或更新频道状态（资料 + 头 + 可选消息列表 + 可选显式变更）。

**请求 `body`：**

```json
{
  "profile": { /* ChannelProfile */ },
  "head": { /* ChannelHead */ },
  "messages": [ /* ChannelMessage, 可选 */ ],
  "changes": [ /* ChannelChange, 可选 */ ]
}
```

**服务端强制规则：**

1. 建立 stream 的 **远端 PeerID 字符串** 必须等于 `profile.owner_peer_id`（及 `head.owner_peer_id`），否则 **UNAUTHORIZED**。
2. `channel_id` 必须为合法 **UUIDv7**。
3. `profile`、`head`、每条 `messages[]` 的 Ed25519 验签必须通过（见第 8 节）。
4. `head.last_seq` 不得小于库中已有值；单条消息 `version`/`seq` 不得回退。

**响应 `body`：**

```json
{
  "ok": true,
  "error_code": "",
  "error_message": ""
}
```

失败时 `ok` 为 `false`，`error_code` 如：`INVALID_PAYLOAD`、`INVALID_SIGNATURE`、`UNAUTHORIZED`、`PUBLIC_CHANNEL_STALE` 等。

**说明**：仅更新资料、无消息时，可将 `messages` 省略或置空；若仅资料变更，需符合 v1 中 `profile_version` / `last_seq` 规则。可选 `changes` 用于显式写入 `ChannelChange`；若不传，服务端在部分场景下会写入消息对应的 change，资料类变更可按 v1 在客户端一并算好再推。

---

### 7.2 `public_channel.get_profile`

**请求 `body`：**

```json
{
  "channel_id": "uuidv7-string"
}
```

**响应 `body`：** `ok`、`profile`（成功时）、`error_code`、`error_message`。  
频道不存在：`PUBLIC_CHANNEL_NOT_FOUND`。

---

### 7.3 `public_channel.get_head`

**请求 `body`：** 同 `get_profile`（仅 `channel_id`）。

**响应 `body`：** `ok`、`head`、`error_code`、`error_message`。

---

### 7.4 `public_channel.list_messages`

**请求 `body`：**

```json
{
  "channel_id": "uuidv7-string",
  "limit": 20,
  "before_message_id": 181
}
```

- `limit`：默认服务端按 **20** 处理若非法；上限 **500**。
- `before_message_id`：可选；有则返回 `message_id < before_message_id` 的若干条；无则从当前最大 `message_id` 往下取。
- 排序：**`message_id` 降序**（与 v1 阅读列表一致）。

**响应 `body`：** `ok`、`messages`（`ChannelMessage` 数组）。

---

### 7.5 `public_channel.get_message`

**请求 `body`：**

```json
{
  "channel_id": "uuidv7-string",
  "message_id": 15
}
```

**响应 `body`：** `ok`、`message`、`error_code`、`error_message`。

---

### 7.6 `public_channel.sync_channel`

**请求 `body`：**

```json
{
  "channel_id": "uuidv7-string",
  "after_seq": 0,
  "limit": 50
}
```

- 返回 **`seq > after_seq`** 的 `ChannelChange` 列表（按 `seq` 升序）。
- `limit` 默认修正；上限 **500**。

**响应 `body`（成功时）：**

```json
{
  "ok": true,
  "channel_id": "0195f3f0-8d4a-7c12-b2c1-9db1f0a9e123",
  "current_last_seq": 168,
  "has_more": true,
  "next_after_seq": 122,
  "items": [ /* ChannelChange */ ]
}
```

**游标说明（实现拉取循环时必读）：**

- `current_last_seq`：服务端记录的频道当前最大 `seq`（来自 head）。
- `has_more`：是否还有 **`seq` 大于本批最后一条** 的变更。
- `next_after_seq`：本批返回的变更中 **最后一条的 `seq`**（若无返回项则与请求的 `after_seq` 一致）。**下一页请求**应设置 `after_seq = next_after_seq`，以获取 `seq` 更大的后续变更（与「仅 seq 单调递增」一致）。

---

## 8. 验签：canonical 字节与 Ed25519（必须与 store-node 一致）

签名算法：**Ed25519**（libp2p peer 身份密钥）。  
签名值：**标准 Base64** 编码后写入 JSON 的 `signature` 字段。  
验签数据：对下列结构 **`json.Marshal`** 得到的 **UTF-8 字节**（**不是**对 map 随意序列化）。

### 8.1 `ChannelProfile`

对 `[]any` 做 **一次** `json.Marshal`：

```text
channel_id,
owner_peer_id,
owner_version,
name,
avatar_canonical,   // nil 或 []any：cid, media_id, blob_id, sha256, url, mime, size, width, height, name
bio,
profile_version,
created_at,
updated_at
```

### 8.2 `ChannelHead`

```text
channel_id,
owner_peer_id,
owner_version,
last_message_id,
profile_version,
last_seq,
updated_at
```

### 8.3 `ChannelMessage`

1. 先构造 `content` 的 canonical：对 `[]any{ text, images[], files[] }` 做 `json.Marshal` 得到字节，再 `json.Unmarshal` 到 `interface{}` 得到 `contentVal`（保证与 store-node 中「先 Marshal 再 Unmarshal 再嵌入」一致）。
2. 再对下列数组做 `json.Marshal` 作为验签负载：

```text
channel_id,
message_id,
version,
seq,
owner_version,
creator_peer_id,
author_peer_id,
created_at,
updated_at,
is_deleted,
message_type,
contentVal
```

### 8.4 签名与校验

- 使用 **owner** 的 **libp2p 私钥** 对 **上述字节** 做 Ed25519 签名，再 Base64 写入对应对象的 `signature`。
- 校验时使用 **`owner_peer_id` 解码为 PeerID → 提取公钥** 验证（与 meshchat 现有离线消息验签方式同一套路）。

---

## 9. meshchat 实现检查清单（AI 可按序自检）

1. [ ] 使用协议 ID：`/meshchat/public-channel/rpc/1.0.0`。
2. [ ] 每 RPC：4 字节大端长度 + JSON；解析响应同样读 4 字节再读 body。
3. [ ] `RPCRequest` / `RPCResponse` 外层字段齐全，`request_id` 唯一。
4. [ ] Owner 推送：`RemotePeer.String() == owner_peer_id`。
5. [ ] `channel_id` 使用 UUIDv7；`owner_version` / 消息 `owner_version` 在 v1 为 1。
6. [ ] `CanonicalProfile` / `CanonicalHead` / `CanonicalMessage` 与 **第 8 节** 完全一致后再签名。
7. [ ] 拉取：`get_profile` → `get_head` → `list_messages` / `sync_channel` 组合与 v1「首次进入 / 增量」流程一致。
8. [ ] 处理 `body.error_code`，区分 `PUBLIC_CHANNEL_NOT_FOUND`、`PUBLIC_CHANNEL_STALE`、`UNAUTHORIZED`、`INVALID_SIGNATURE` 等。

---

## 10. 与 HTTP 文档的关系

mesh-proxy 文档中 **HTTP 路径**（如 `/api/v1/public-channels/...`）描述的是 **本地 HTTP / 控制台** 语义；**store-node** 当前仅提供 **libp2p stream RPC**，无 HTTP。meshchat 应 **直接按本文** 实现 P2P 客户端；若需 HTTP 网关，需在其它服务中自行映射。

---

## 11. 参考代码位置（store-node 仓库）

| 内容 | 路径 |
|------|------|
| 协议 ID 与方法名 | `internal/protocol/public_channel.go` |
| RPC 与数据类型 | `internal/publicchannel/*.go` |
| canonical 实现 | `internal/publicchannel/canonical.go` |
| 帧读写 | `internal/p2p/codec.go` |

实现 meshchat 时，**验签字节**建议以 `canonical.go` 为黄金参考，或编写**与之一致的**跨语言测试向量（同输入 → 同 SHA256 或同十六进制字节）。

---

**文档版本**：与 store-node 当前实现同步；若服务端升级方法或帧限制，以仓库内代码为准并更新本文。
