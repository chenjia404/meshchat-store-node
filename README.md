# store-node

基于 Go 1.26.1、libp2p stream、Pebble 和 length-prefixed JSON 的离线私聊消息 store 节点。

## libp2p 流协议（统一 RPC）

节点只注册一个协议 ID：`/meshchat/offline-store/rpc/1.0.0`。载荷为 **4 字节大端长度 + JSON**（与原先一致）。

请求：

```json
{
  "request_id": "uuid",
  "method": "offline.store",
  "body": {}
}
```

响应：

```json
{
  "request_id": "uuid",
  "ok": true,
  "error": "",
  "body": {}
}
```

- `method` 取值：`offline.store`、`offline.fetch`、`offline.ack`
- `body`：分别为原先的 `StoreRequest`、`FetchRequest`、`AckRequest` JSON（字段不变）
- 响应里的 `body`：成功时为原先的 `StoreResponse` / `FetchResponse` / `AckResponse`；业务失败时同样为上述结构（含 `error_code` / `error_message`）。**RPC 层错误**（非法外层 JSON、缺少 `request_id`、未知 `method` 等）时，`body` 固定为 `{"error_code":"...","error_message":"..."}`，未知方法时额外带 `"method":"请求里的 method"`。这样客户端始终可以按 JSON 解出 `error_code`，无需对 `body == null` 单独分支。

## 启动

1. 安装 Go 1.26.1。
2. 在项目根目录执行：

```bash
go mod tidy
go run ./cmd/stored
```

## 使用自定义配置

```bash
go run ./cmd/stored -config ./config.yaml
```

配置项结构与 `store.md` 第 19 节一致。

## 覆盖默认监听端口

可通过命令行参数覆盖默认监听端口，参数会同时作用于默认的 TCP 和 QUIC 地址：

```bash
go run ./cmd/stored -port 4101
```

## 覆盖 libp2p 广播 IP

如果节点监听在 `0.0.0.0`，但你希望 libp2p 对外广播一个固定公网或局域网 IP，可使用：

```bash
go run ./cmd/stored -announce-ip 192.168.1.10
```

该参数会基于当前 `listen_addrs` 生成对应的广播地址，例如把 `0.0.0.0` 或 `127.0.0.1` 替换成你指定的 IP。

如果没有传 `-announce-ip`，且配置里也没有 `announce_addrs`，程序会在启动时自动请求公网 IP 查询接口，并把结果作为 libp2p 的广播地址。监听地址仍然保持 `0.0.0.0`，适合 Docker 内运行、宿主机对外暴露端口的场景。

也可以在配置文件中直接设置：

```yaml
node:
  listen_addrs:
    - /ip4/0.0.0.0/tcp/4001
    - /ip4/0.0.0.0/udp/4001/quic-v1
  announce_addrs:
    - /ip4/203.0.113.10/tcp/4001
    - /ip4/203.0.113.10/udp/4001/quic-v1
```

## 节点 ID 持久化

节点私钥现在会默认保存到 `store.data_dir` 下的 `node_identity.key`，因此同一个数据目录重启后，`peer ID` 不会变化。

如果你需要自定义路径，也可以在配置中指定：

```yaml
node:
  identity_key_path: ./data/node_identity.key
```

如果是在 Docker 中运行，请把数据目录挂载成持久卷，否则容器重建后私钥文件也会一起丢失，`peer ID` 仍然会变化。

## V1 ACK 约定

V1 没有多设备概念，因此 `AckRequest.device_id` 固定等于账号 ID。

在当前实现里，服务端要求：

```text
device_id == recipient_id
```
