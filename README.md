# tinyProxyProject

用 Go 实现的 HTTP/1.1 正向代理，支持并发、LRU 缓存、请求/响应解析、标准错误响应。

## 功能

- HTTP/1.1 GET 与 POST 请求转发
- HTTPS CONNECT 隧道，盲转发 TLS 密文，不解密
- CONNECT 隧道空闲超时（基于双向心跳机制，防止恶意挂机耗尽系统 fd）
- 每连接一 goroutine 的并发模型
- 请求行解析：method、scheme、host、port、path、query、version
- IPv6 地址支持（`[::1]:8080`）
- 响应解析：状态行、头字段、body framing（Content-Length / chunked / until EOF）
- LRU 缓存：完整 URL 做 key，`container/list` + map，`sync.Mutex` 保护
- 可缓存判断：仅缓存 200、Content-Length、无 Set-Cookie、Cache-Control 允许的响应
- singleflight 合并同 key 回源，解决缓存击穿
- 标准错误响应：400 / 500 / 501 / 502
- hop-by-hop 头处理：删除 Proxy-Connection，重写 Host，添加 Via 和 Connection: close
- 完备的测试体系：80+ 单元测试用例，全量通过 `go test -race` 校验

## 架构
```text
Client ──HTTP────▶ Proxy ──HTTP────▶ Server
                    │
                    │ LRU Cache + singleflight
                    │
Client ──CONNECT──▶ Proxy ──TCP─────▶ Server
                    │
                    │ 双向 io.Copy(盲转发 TLS 密文)
```
HTTP请求流程:

1. 解析客户端请求行和头
2. 构建转发请求（absolute-form → origin-form，重写 Host，删 hop-by-hop）
3. 查 LRU 缓存，命中直接返回
4. 未命中则走 singleflight 合并同 key 回源
5. 连目标服务器，转发请求
6. 解析响应头，判断 body framing
7. 边转发 body 边收集（可缓存时）
8. 转发成功后写入缓存

CONNECT隧道流程:

1. 解析 CONNECT 请求行，拿到 host:port
2. 读完头字段并丢弃
3. 连目标服务器，失败返回 502
4. 回 200 Connection Established
5. 双向 ```io.Copy``` 透明转发，不解析任何内容
6. 空闲超时控制：将 TCP 连接包装进自定义的 ```TimeoutConn```，无论读写只要有数据流动，就自动重置双向 Deadline。若静默超时，底层 ```Read/Write``` 抛出错误，自动断开连接释放资源。

## 快速开始

```bash
go run .
```
另开终端:
```bash
# HTTP 请求
curl.exe -x http://127.0.0.1:8080 http://example.com/

# HTTPS 请求（走 CONNECT 隧道）
curl.exe -x http://127.0.0.1:8080 https://example.com/
```

## 压测
环境：本机回环，目标 http://httpbin.org/json，500 请求，50 并发。
| 场景 | QPS | P50 | P90 | P99 |
|---|---|---|---|---|
| 无缓存 | 79.56 | 536ms | 708ms | 1.18s |
| 有缓存（冷启动） | 544.43 | 3.9ms | 601ms | 707ms |
| 有缓存（预热后） | 11846.01 | 3.9ms | 4.9ms | 7.4ms |

预热后 QPS 提升约 149 倍，P99 从 1.18s 降到 7.4ms。冷启动时 P90 偏高，原因是并发 50 下多个请求同时 miss 同一 key，触发 cache stampede。

## singleflight优化缓存击穿
50 请求，50 并发，冷启动，目标 http://httpbin.org/json
| 场景 | 上游回源次数 | QPS | P50 | P99 | 总耗时 |
|---|---|---|---|---|---|
| 无 singleflight | 50 | 4.06 | 862ms | 12.32s | 12.32s |
| 有 singleflight | 1 | 17.22 | 2.90s | 2.90s | 2.90s |

50 并发冷启动下，singleflight 将上游回源次数从 50 降到 1，QPS 提升 4.24 倍，P99 从 12.32s 降到 2.90s。代价是中位数延迟从 862ms 变为 2.90s，因为所有等待者共享同一次回源的延迟。这是“保护上游、降低尾部延迟”和“中位数延迟”之间的取舍。

## 技术要点

- 流式转发：用 ```io.Copy``` 和 ```io.LimitReader```，可缓存时传 ```io.MultiWriter(conn, &buf)```，函数本身不用改。
- 响应捕获：```parseResponse``` 保留原始头字节 ```rawHeader```，与 body 拼接后作为完整响应存入缓存。
- 接口抽象：```copyBody``` 接收 ```io.Writer```（```ialUpstream + forward```），边读边写，大响应不占内存。
- LRU 实现：```container/list``` 双向链表 + map，O(1) 查找和淘汰。
- CONNECT 隧道：识别 CONNECT 后不再解析 HTTP，双向 ```io.Copy``` 透明转发；客户端方向复用 ```bufio.Reader``` 避免丢预读字节。
- 心跳超时 (TimeoutConn)：通过组合（Embedding）原生 ```net.Conn```，在自定义的 ```Read/Write``` 方法中实时刷新 Deadline，实现优雅的空闲熔断。

## 已知限制
- 只缓存 Content-Length 响应，chunked 和 until-EOF 不缓存
- 不支持 keep-alive，每个请求使用 Connection: close
- 未对 method 做合法性校验，依赖服务器拒绝

## 未来计划
- chunked 响应缓存
- keep-alive 连接复用

## 项目结构
```text
.
├── main.go            # 入口：监听、accept、goroutine、CONNECT 隧道、请求分发
├── request.go         # Info 结构体 + parseRequestLine（含 CONNECT 解析）
├── header.go          # HeaderField + HeaderCollection + parseHeaders + checkName
├── response.go        # Response + parseResponse + decideFraming
├── body.go            # copyBody：按 framing 读响应体并转发
├── build.go           # buildRequest + buildErrorResponse
├── cache.go           # LRU Cache + cacheKey + ableToCache
├── upstream.go        # hostport + dialUpstream + forward
├── timeout_conn.go    # TimeoutConn：实现连接的空闲超时包装
├── timeout_conn_test.go # TimeoutConn 的心跳保活与超时单元测试
└── bench/
    └── main.go        # 压测工具
```