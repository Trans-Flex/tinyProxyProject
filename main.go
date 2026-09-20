package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log"
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
			log.Printf("accept 错误: %v", err)
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
				serverConn, err := dialUpstream(info)
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
			//POST分支
			if info.method == "POST" {
				contentLengthStr := headerCollection.first("Content-Length")
				if contentLengthStr == "" {
					io.WriteString(c, buildErrorResponse(501))
					return
				}
				contentLength, err := strconv.Atoi(contentLengthStr)
				if err != nil || contentLength < 0 {
					io.WriteString(c, buildErrorResponse(400))
					return
				}
				_, err = forward(info, request, clientReader, FramingContentLength, contentLength, c)
				if err != nil {
					log.Printf("发生错误: %v", err)
					io.WriteString(c, buildErrorResponse(502))
					return
				}
				return
			}
			//GET分支

			//构建key并查找LRU缓存
			//因为默认端口也已经作为数据存入info，所以不需要额外判定以提高命中率

			key := cacheKey(info)
			cacheResp, ok := cache.get(key)
			if ok {
				c.Write(cacheResp)
				return
			}

			result, err, _ := sf.Do(key, func() (any, error) {
				//再次尝试
				cacheResp, ok := cache.get(key)
				if ok {
					return cacheResp, nil
				}

				var bodyBuf bytes.Buffer
				resp, err := forward(info, request, nil, FramingNone, 0, &bodyBuf)
				if err != nil {
					return nil, err
				}

				data := bodyBuf.Bytes()
				if cache.ableToCache(&resp) {
					_ = cache.put(key, data)
				}
				return data, nil
			})

			if err != nil {
				log.Printf("发生错误: %v", err)
				io.WriteString(c, buildErrorResponse(502))
				return
			}
			c.Write(result.([]byte))

		}(conn)
	}

	wg.Wait()
}
