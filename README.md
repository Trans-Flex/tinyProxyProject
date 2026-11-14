# tinyProxyProject

用 Go 实现的 HTTP/1.1 正向代理，支持双端长连接、TCP 连接池、LRU 缓存、Chunked 流式解析、请求/响应解析及并发控制。

## 功能

- **HTTP/1.1 双端 Keep-Alive**：支持客户端与代理、代理与上游服务器之间的长连接复用。
- **动态 TCP 连接池 (`ConnPool`)**：基于 `sync.Mutex` 和带缓冲 `chan` 自研连接池。支持依赖注入 (`DialFunc`)、1ms 极速假死探测、以及基于时间戳的惰性清理 (Lazy Cleanup)。
- **多目标池管理**：使用 `sync.Map` 为不同 Host 维护专属连接池，防止目标服务器连接串包。
- **Chunked 流式状态机**：纯手写 `FramingChunked` 解析状态机，精准解析十六进制块长度与尾部 Trailer，确保长连接复用下零脏数据残留。
- **HTTPS CONNECT 隧道**：盲转发 TLS 密文，支持双向心跳机制防止恶意挂机。
- **LRU 缓存与防击穿**：基于完整 URL 的 LRU 缓存，引入 `singleflight` 合并同 Key 回源，解决缓存击穿。
- **协议校验**：完备的请求行/头字段/状态行校验，支持 IPv6，以及 hop-by-hop 头处理。
- **测试覆盖**：80+ 单元测试用例，覆盖 Chunked 边界、异常 RequestLine、缓存淘汰等，全量通过 `go test -race` 校验。

## 架构
```text
Client ──HTTP(Keep-Alive)────▶ Proxy ──HTTP(Keep-Alive)────▶ Server
                    │
                    │ LRU Cache + singleflight
                    │ sync.Map[Host] -> ConnPool
                    │
Client ──CONNECT────────────▶ Proxy ──TCP──────────────────▶ Server
                    │
                    │ 双向 io.Copy(盲转发 TLS 密文)
```
HTTP请求流程:

1. 客户端循环 (for) 监听并解析 Request 和 Header，判断 clientClose。
2. 构建转发请求，查 LRU 缓存，命中则直接返回。
3. 未命中走 singleflight 合并回源。从 sync.Map 获取目标 Host 的连接池。
4. ConnPool.Get() 获取连接（包含假死检测与惰性超时清理）。
5. 连目标服务器，转发请求，读取 Response 头并判断 body framing。
6. 边转发 body 边收集（对于 Content-Length 或 Chunked 采用流式透传）。
7. 完美转发结束后，若无报错且支持复用，则 ConnPool.Put() 回收连接。

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

## 引入长连接
| 场景 | 总耗时 | QPS | P50 延迟 | P99 延迟 |
|---|---|---|---|---|
| 短连接 | 6.28s | 79.56 | 536ms | 1.18s |
| Keep-Alive (ConnPool) | 6.40s | 78.06 | 333ms | 2.16s |

**数据洞察**：在公网无缓存场景下，长连接耗时与短连接相近，甚至 P99 偏高。原因有二：
1. **目标服务器限制**：`httpbin.org` (基于 Cloudflare) 频繁返回 `Connection: close`，代理触发防御机制被迫销毁连接，实际上退化为短连接。
2. **网络拥塞排队**：公网跨国 RTT 波动大，长连接池中某个请求遭遇网络抖动（如排队）时，会阻塞复用该连接的后续请求，导致长尾延迟（P99）升高。而短连接因独立建连，反而规避了排队效应。

## 缓存命中场景
| 场景 | 总耗时 | QPS | P50 延迟 | P99 延迟 |
|---|---|---|---|---|
| 短连接 + LRU缓存 | 0.91s | 544.43 | 3.9ms | 706ms |
| **Keep-Alive + LRU缓存** | **0.67s** | **738.50** | **888μs** | **665ms** |

**数据洞察**：在启用 LRU 缓存后，Keep-Alive 模式的 QPS 提升至 **738**，P99 降低至 **665ms**。这证明了在命中内存缓存的高并发场景下，连接池带来的零握手开销与快速响应能力全面碾压短连接。

## 技术要点

- 流式转发：用 `io.Copy` 和 `io.LimitReader`，可缓存时传 `io.MultiWriter(conn, &buf)`，函数本身不用改。
- 响应捕获：`parseResponse` 保留原始头字节 `rawHeader`，与 body 拼接后作为完整响应存入缓存。
- 接口抽象：`copyBody` 接收 `io.Writer`（`ialUpstream + forward`），边读边写，大响应不占内存。
- LRU 实现：`container/list` 双向链表 + `map`，O(1) 查找和淘汰。
- CONNECT 隧道：识别 CONNECT 后不再解析 HTTP，双向 `io.Copy` 透明转发；客户端方向复用 `bufio.Reader` 避免丢预读字节。
- 心跳超时 (TimeoutConn)：通过组合（Embedding）原生 `net.Conn`，在自定义的 `Read/Write` 方法中实时刷新 Deadline，实现优雅的空闲熔断。
- 连接池假死探测：从 `Pool` 中取出的连接，通过 `SetReadDeadline(1ms)` 试探，若报 `timeout` 则连接健康，否则判定为被服务端静默断开的僵尸连接并剔除。
- 连接池惰性清理：出池时校验 `lastUsed` 时间戳，若超时直接销毁，避免后台定时器锁竞争。
- Chunked 状态机：`io.CopyN` 精准按长度转发，先写 `\r\n` 换行，读到 0 后继续循环读取 Trailer 直到空行，完美保障 TCP 缓冲区无数据残留。

## 已知限制
- 仅缓存带有 Content-Length 的响应（Chunked 流式响应为保证低内存占用，不进行内存缓存）。
- 未对 method 做合法性校验，依赖服务器拒绝

## 未来计划
- 引入分片或 LRU 机制限制 hostPoolMap 的最大池子数量，防止恶意伪造 Host 导致内存爆炸。
- 增加 Prometheus 监控指标（连接池活跃数、命中率、QPS）。

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
├── connpool.go        # 自研 TCP 连接池（假死检测、懒清理）
├── upstream.go        # hostport + dialUpstream + forward
├── timeout_conn.go    # TimeoutConn：实现连接的空闲超时包装
├── timeout_conn_test.go # TimeoutConn 的心跳保活与超时单元测试
└── bench/
    └── main.go        # 压测工具
```