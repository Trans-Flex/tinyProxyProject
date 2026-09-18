package main

import (
	"fmt"
	"strconv"
	"strings"
)

func buildRequest(info Info, head HeaderCollection) (string, error) {
	if info.method == "" {
		return "", fmt.Errorf("缺少method")
	}
	if info.host == "" {
		return "", fmt.Errorf("缺少host")
	}
	if info.port <= 0 {
		return "", fmt.Errorf("无效的port")
	}
	if info.version == "" {
		return "", fmt.Errorf("缺少version")
	}
	if info.scheme == "" {
		return "", fmt.Errorf("缺少scheme")
	}
	//使用builder是因为在go直接对string使用+=运算符连接性能较差，在该频繁连接场景下使用builder + Write组合以提高性能
	var originTarget strings.Builder
	originTarget.WriteString(info.path)
	if info.path == "" {
		originTarget.WriteByte('/')
	}
	if info.query != "" {
		originTarget.WriteByte('?')
		originTarget.WriteString(info.query)
	}

	var request strings.Builder
	request.WriteString(info.method)
	request.WriteByte(' ')
	request.WriteString(originTarget.String())
	request.WriteByte(' ')
	request.WriteString(info.version)
	request.WriteString("\r\n")

	hostport := info.host
	if (info.port != 80 && info.scheme == "http") || (info.port != 443 && info.scheme == "https") {
		hostport += ":" + strconv.Itoa(info.port)
	}
	request.WriteString("Host: ")
	request.WriteString(hostport)
	request.WriteString("\r\n")

	for _, field := range head.fields {
		if field.lowerName == "proxy-connection" || field.lowerName == "connection" || field.lowerName == "host" {
			continue
		}
		request.WriteString(field.originName)
		request.WriteString(": ")
		request.WriteString(field.trimmedVal)
		request.WriteString("\r\n")
	}

	request.WriteString("Via: 1.1 proxy\r\n")
	request.WriteString("Connection: close\r\n")

	request.WriteString("\r\n")
	return request.String(), nil
}

func buildErrorResponse(statusCode int) string {
	var errorResponse strings.Builder
	errorResponse.WriteString("HTTP/1.1")

	var reason string
	switch statusCode {
	case 400:
		reason = "Bad Request"
	case 500:
		reason = "Internal Server Error"
	case 501:
		reason = "Not Implemented"
	case 502:
		reason = "Bad Gateway"
	case 503:
		reason = "Service Unavailable"
	case 504:
		reason = "Gateway Timeout"
	default:
		reason = "Unknown"
	}

	errorResponse.WriteByte(' ')
	errorResponse.WriteString(strconv.Itoa(statusCode))
	errorResponse.WriteByte(' ')
	errorResponse.WriteString(reason)
	errorResponse.WriteString("\r\n")

	errorResponse.WriteString("Content-Length: 0\r\n")
	errorResponse.WriteString("Connection: close\r\n")
	errorResponse.WriteString("\r\n")

	return errorResponse.String()
}
