package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	var wg sync.WaitGroup
	var cache *Cache
	cache, err = NewCache(16384)
	if err != nil {
		panic(err)
	}

	//引入singleflight以解决缓存击穿问题
	sf := &singleflight.Group{}

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				// 监听器被关，退出
				break
			}
			// 其他错误，记录日志，继续
			fmt.Println("accept 错误:", err)
			continue
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer c.Close()
			first := true
			var info Info
			var headBuilder strings.Builder
			clientReader := bufio.NewReader(c)
			for {
				line, err := clientReader.ReadString('\n')
				if line == "\r\n" || line == "\n" {
					break
				} else if err != nil {
					return
				}

				if first {
					line = strings.TrimRight(line, "\r\n")
					info, err = parseRequestLine(line)
					if err != nil {
						response := buildErrorResponse(400)
						io.WriteString(c, response)
						return
					}
					first = false
				} else {
					headBuilder.WriteString(line)
				}
			}

			if info.method == "CONNECT" {
				//隧道分支

				//与服务器连接
				var hostport string
				idx := strings.IndexByte(info.host, ':')
				if idx != -1 {
					hostport = "[" + info.host + "]"
				} else {
					hostport = info.host
				}
				hostport += ":" + strconv.Itoa(info.port)
				serverConn, err := net.Dial("tcp", hostport)
				if err != nil {
					io.WriteString(c, buildErrorResponse(502))
					return
				}
				defer serverConn.Close()

				io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")

				var waitgroup sync.WaitGroup
				waitgroup.Add(2)
				go func() {
					defer waitgroup.Done()
					io.Copy(serverConn, clientReader) // 客户端 → 服务器
					serverConn.Close()
					c.Close()
				}()
				go func() {
					defer waitgroup.Done()
					io.Copy(c, serverConn) // 服务器 → 客户端
					serverConn.Close()
					c.Close()
				}()
				waitgroup.Wait()
				return
			}

			//解析头字段并存储
			headerCollection, err := parseHeaders(headBuilder.String())
			if err != nil {
				response := buildErrorResponse(400)
				io.WriteString(c, response)
				return
			}
			//构建请求
			request, err := buildRequest(info, headerCollection)
			if err != nil {
				response := buildErrorResponse(500)
				io.WriteString(c, response)
				return
			}
			//构建key并查找LRU缓存
			//因为默认端口也已经作为数据存入info，所以不需要额外判定以提高命中率
			var keyBuilder strings.Builder
			keyBuilder.WriteString(info.scheme + "://")
			if strings.Contains(info.host, ":") {
				keyBuilder.WriteString("[" + info.host + "]")
			} else {
				keyBuilder.WriteString(info.host)
			}
			keyBuilder.WriteString(":" + strconv.Itoa(info.port))
			keyBuilder.WriteString(info.path)
			if info.query != "" {
				keyBuilder.WriteString("?" + info.query)
			}

			key := keyBuilder.String()
			cacheResp, ok := cache.get(key)
			if ok {
				c.Write(cacheResp)
				fmt.Println("cache hit:", key)
				return
			}
			fmt.Println("cache miss:", key)

			result, err, _ := sf.Do(key, func() (any, error) {
				//再次尝试
				cacheResp, ok := cache.get(key)
				if ok {
					fmt.Println("cache hit:", key)
					return cacheResp, nil
				}

				//与服务器连接
				var hostport string
				idx := strings.IndexByte(info.host, ':')
				if idx != -1 {
					hostport = "[" + info.host + "]"
				} else {
					hostport = info.host
				}
				hostport += ":" + strconv.Itoa(info.port)
				serverConn, err := net.Dial("tcp", hostport)
				if err != nil {
					return nil, fmt.Errorf("连接 %s 失败: %w", hostport, err)
				}
				defer serverConn.Close()
				//向服务器发送请求
				_, err = io.WriteString(serverConn, request)
				if err != nil {
					return nil, fmt.Errorf("向服务器发送请求失败: %w", err)
				}
				//回响
				serverReader := bufio.NewReader(serverConn)
				resp, err := parseResponse(serverReader, info.method)
				if err != nil {
					return nil, fmt.Errorf("解析响应失败: %w", err)
				}

				ableToCache := cache.ableToCache(&resp)

				var bodyBuf bytes.Buffer
				err = copyBody(serverReader, &bodyBuf, resp)
				if err != nil {
					return nil, fmt.Errorf("读取body发生错误: %w", err)
				}

				combined := make([]byte, 0, len(resp.rawHeader)+bodyBuf.Len())
				combined = append(combined, resp.rawHeader...)
				combined = append(combined, bodyBuf.Bytes()...)
				if ableToCache {
					_ = cache.put(key, combined)
					fmt.Println("cache put:", key, len(combined))
				}
				return combined, nil
			})

			if err != nil {
				io.WriteString(c, buildErrorResponse(502))
				return
			}
			c.Write(result.([]byte))

		}(conn)
	}

	wg.Wait()
}
