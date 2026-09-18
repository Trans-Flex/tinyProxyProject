package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

var tokenTable [128]bool

func init() {
	for c := 'a'; c <= 'z'; c++ {
		tokenTable[c] = true
	}
	for c := 'A'; c <= 'Z'; c++ {
		tokenTable[c] = true
	}
	for c := '0'; c <= '9'; c++ {
		tokenTable[c] = true
	}
	tokenTable['!'] = true
	tokenTable['#'] = true
	tokenTable['$'] = true
	tokenTable['%'] = true
	tokenTable['&'] = true
	tokenTable['\''] = true
	tokenTable['*'] = true
	tokenTable['+'] = true
	tokenTable['-'] = true
	tokenTable['.'] = true
	tokenTable['^'] = true
	tokenTable['_'] = true
	tokenTable['`'] = true
	tokenTable['|'] = true
	tokenTable['~'] = true
}

type BodyFraming int

const (
	FramingNone BodyFraming = iota
	FramingContentLength
	FramingChunked
	FramingUntilEOF
)

type Response struct {
	version       string
	statusCode    int
	reason        string
	contentLength int
	bodyFraming   BodyFraming
	header        HeaderCollection
	rawHeader     []byte
}

func parseResponse(reader *bufio.Reader, method string) (Response, error) {
	if reader == nil {
		return Response{}, fmt.Errorf("传入的reader不能为空!")
	}

	var response Response
	var buf bytes.Buffer
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return Response{}, fmt.Errorf("读取状态行失败: %w", err)
	}

	buf.WriteString(statusLine)
	statusLine = strings.TrimRight(statusLine, "\r\n")
	version, rest, ok := strings.Cut(statusLine, " ")
	if !ok {
		return Response{}, fmt.Errorf("不合规的状态行: %q", statusLine)
	}
	response.version = version

	strStatusCode, reason, _ := strings.Cut(rest, " ")
	if len(strStatusCode) != 3 {
		return Response{}, fmt.Errorf("不合规的状态码: %q", strStatusCode)
	}
	response.statusCode, err = strconv.Atoi(strStatusCode)
	if err != nil {
		return Response{}, fmt.Errorf("不合规的状态码: %q", strStatusCode)
	}
	if response.statusCode < 100 || response.statusCode > 599 {
		return Response{}, fmt.Errorf("不合规的状态码: %q", strStatusCode)
	}
	response.reason = reason

	var header HeaderCollection
	header.index = make(map[string][]int)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return Response{}, fmt.Errorf("读取头字段失败: %w", err)
		}
		buf.WriteString(line)
		if line == "\r\n" || line == "\n" {
			break
		}
		line = strings.TrimRight(line, "\r\n")

		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return Response{}, fmt.Errorf("不合规的头字段: %q", line)
		}
		err = checkName(name)
		if err != nil {
			return Response{}, fmt.Errorf("解析头字段名 %q 失败: %w", name, err)
		}
		header.add(name, value)
	}

	response.header = header
	response.rawHeader = buf.Bytes()

	response.bodyFraming, response.contentLength, err = decideFraming(method, response.header, response.statusCode)
	if err != nil {
		return Response{}, fmt.Errorf("解析响应 framing 失败: %w", err)
	}
	return response, nil
}

func decideFraming(method string, header HeaderCollection, statusCode int) (BodyFraming, int, error) {
	if method == "HEAD" || (statusCode > 99 && statusCode < 200) || statusCode == 204 || statusCode == 304 {
		return FramingNone, -1, nil
	}

	chunkedStr := header.first("Transfer-Encoding")
	parts := strings.Split(chunkedStr, ",")
	end := len(parts) - 1
	parts[end], _, _ = strings.Cut(parts[end], ";")
	parts[end] = strings.TrimSpace(parts[end])
	parts[end] = strings.ToLower(parts[end])
	if parts[end] == "chunked" {
		return FramingChunked, -1, nil
	}

	contentLengthStr := header.first("Content-Length")
	if contentLengthStr != "" {
		if len(contentLengthStr) > 1 && contentLengthStr[0] == '0' {
			//含有前导0
			return FramingUntilEOF, -1, fmt.Errorf("不合规的Content-Length字段: %q", contentLengthStr)
		}
		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil {
			return FramingUntilEOF, -1, fmt.Errorf("不合规的Content-Length字段: %q", contentLengthStr)
		}
		if contentLength < 0 {
			return FramingUntilEOF, -1, fmt.Errorf("不合规的Content-Length字段: %q", contentLength)
		}
		return FramingContentLength, contentLength, nil
	}

	return FramingUntilEOF, -1, nil
}

func checkName(name string) error {
	if name == "" {
		return fmt.Errorf("name不可为空")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 128 || !tokenTable[c] {
			return fmt.Errorf("name包含了违规字符: %q", c)
		}
	}

	return nil
}
