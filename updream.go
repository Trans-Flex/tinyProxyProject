package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

func hostport(info Info) string {
	var hostport string
	idx := strings.IndexByte(info.host, ':')
	if idx != -1 {
		hostport = "[" + info.host + "]"
	} else {
		hostport = info.host
	}
	hostport += ":" + strconv.Itoa(info.port)
	return hostport
}

func dialUpstream(info Info) (net.Conn, error) {
	hostport := hostport(info)
	serverConn, err := net.Dial("tcp", hostport)
	return serverConn, err
}

func forward(info Info, request string, bodyReader *bufio.Reader, bodyFraming BodyFraming, bodyLength int, writer io.Writer) (Response, error) {
	//与服务器连接
	serverConn, err := dialUpstream(info)
	if err != nil {
		return Response{}, fmt.Errorf("连接 %s 失败: %w", hostport(info), err)
	}
	defer serverConn.Close()
	//向服务器发送请求行和头
	_, err = io.WriteString(serverConn, request)
	if err != nil {
		return Response{}, fmt.Errorf("向服务器发送请求失败: %w", err)
	}
	//转发请求体
	if bodyFraming != FramingNone {
		if bodyFraming != FramingContentLength {
			return Response{}, fmt.Errorf("暂不支持的请求体 framing: %v", bodyFraming)
		}
		if bodyReader == nil {
			return Response{}, fmt.Errorf("请求体 framing 为 ContentLength 但 bodyReader 为 nil")
		}
		if bodyLength > 0 {
			_, err := io.CopyN(serverConn, bodyReader, int64(bodyLength))
			if err != nil {
				return Response{}, fmt.Errorf("转发请求体失败: %w", err)
			}
		}
	}
	//回响
	serverReader := bufio.NewReader(serverConn)
	resp, err := parseResponse(serverReader, info.method)
	if err != nil {
		return Response{}, fmt.Errorf("解析响应失败: %w", err)
	}

	_, err = io.Copy(writer, bytes.NewReader(resp.rawHeader))
	if err != nil {
		return Response{}, fmt.Errorf("写入原始头字段失败: %w", err)
	}
	err = copyBody(serverReader, writer, resp)
	if err != nil {
		return Response{}, fmt.Errorf("读取响应体失败: %w", err)
	}

	return resp, nil
}
