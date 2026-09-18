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

			cacheResp, ok := cache.get(keyBuilder.String())
			if ok {
				c.Write(cacheResp)
				fmt.Println("cache hit:", keyBuilder.String())
				return
			}
			fmt.Println("cache miss:", keyBuilder.String())

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
				response := buildErrorResponse(502)
				io.WriteString(c, response)
				return
			}
			defer serverConn.Close()
			//向服务器发送请求
			_, err = io.WriteString(serverConn, request)
			if err != nil {
				response := buildErrorResponse(502)
				io.WriteString(c, response)
				return
			}
			//回响
			serverReader := bufio.NewReader(serverConn)
			resp, err := parseResponse(serverReader, info.method)
			if err != nil {
				io.WriteString(c, buildErrorResponse(502))
				return
			}
			c.Write(resp.rawHeader)

			ableToCache := cache.ableToCache(&resp)

			var bodyBuf bytes.Buffer
			var writer io.Writer = c
			if ableToCache {
				writer = io.MultiWriter(c, &bodyBuf)
			}
			err = copyBody(serverReader, writer, resp)
			if err != nil {
				fmt.Printf("读取body发生错误: %v\n", err)
				return
			}

			if ableToCache {
				combined := make([]byte, 0, len(resp.rawHeader)+bodyBuf.Len())
				combined = append(combined, resp.rawHeader...)
				combined = append(combined, bodyBuf.Bytes()...)
				_ = cache.put(keyBuilder.String(), combined)
				fmt.Println("cache put:", keyBuilder.String(), len(combined))
			}

		}(conn)
	}

	wg.Wait()
}
