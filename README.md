# tinyProxyProject

用 Go 实现的 HTTP/1.1 正向代理，支持并发、LRU 缓存、请求/响应解析、标准错误响应。

## 功能

- HTTP/1.1 GET 请求转发
- 每连接一 goroutine 的并发模型
- 请求行解析：method、scheme、host、port、path、query、version
- IPv6 地址支持（`[::1]:8080`）
- 响应解析：状态行、头字段、body framing（Content-Length / chunked / until EOF）
- LRU 缓存：完整 URL 做 key，`container/list` + map，`sync.Mutex` 保护
- 可缓存判断：仅缓存 200、Content-Length、无 Set-Cookie、Cache-Control 允许的响应
- 标准错误响应：400 / 500 / 501 / 502
- hop-by-hop 头处理：删除 Proxy-Connection，重写 Host，添加 Via 和 Connection: close

## 架构
```text
Client → Proxy → Server
           ↓
        LRU Cache
```
请求流程：

1. 解析客户端请求行和头
2. 构建转发请求（absolute-form → origin-form，重写 Host，删 hop-by-hop）
3. 查 LRU 缓存，命中直接返回
4. 未命中则连目标服务器，转发请求
5. 解析响应头，判断 body framing
6. 边转发 body 边收集（可缓存时）
7. 转发成功后写入缓存

## 快速开始

```bash
go run .
```
另开终端:
```bash
curl.exe -x http://127.0.0.1:8080 http://example.com/
```

## 压测
环境：本机回环，目标 http://httpbin.org/json，500 请求，50 并发。
| 场景 | QPS | P50 | P90 | P99 |
|---|---|---|---|---|
| 无缓存 | 79.56 | 536ms | 708ms | 1.18s |
| 有缓存（冷启动） | 544.43 | 3.9ms | 601ms | 707ms |
| 有缓存（预热后） | 11846.01 | 3.9ms | 4.9ms | 7.4ms |

预热后 QPS 提升约 149 倍，P99 从 1.18s 降到 7.4ms。冷启动时 P90 偏高，原因是并发 50 下多个请求同时 miss 同一 key，触发 cache stampede。

## 已知限制
- 仅支持 GET 请求体转发
- 不支持 HTTPS CONNECT 隧道
- 只缓存 Content-Length 响应，chunked 和 until-EOF 不缓存
- 并发同 key 存在 cache stampede
- 不支持 keep-alive，每个请求使用 Connection: close

## 未来计划
- singleflight 合并同 key 回源
- POST 请求体转发
- HTTPS CONNECT 隧道
- chunked 响应缓存

## 项目结构
```text
.
├── main.go        # 入口：监听、accept、goroutine
├── request.go     # Info 结构体 + parseRequestLine
├── header.go      # HeaderCollection + parseHeaders + checkName
├── response.go    # Response + parseResponse + decideFraming
├── body.go        # copyBody：按 framing 读并转发
├── build.go       # buildRequest + buildErrorResponse
├── cache.go       # LRU Cache
└── bench/
    └── main.go    # 压测工具
```

## 技术要点

- 流式转发：用 io.Copy 和 io.LimitReader 边读边写，大响应不占内存
- LRU 实现：container/list 双向链表 + map，O(1) 查找和淘汰
- TeeReader 收集：可缓存响应边转发边收集，转发成功后才写缓存，保证原子性
- io.Writer 抽象：copyBody 接收 io.Writer，可缓存时传 io.MultiWriter(conn, &buf)，函数本身不用改