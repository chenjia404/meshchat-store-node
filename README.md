# store-node

基于 Go 1.26.1、libp2p stream、Pebble 和 length-prefixed JSON 的离线私聊消息 store 节点。

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

## V1 ACK 约定

V1 没有多设备概念，因此 `AckRequest.device_id` 固定等于账号 ID。

在当前实现里，服务端要求：

```text
device_id == recipient_id
```
