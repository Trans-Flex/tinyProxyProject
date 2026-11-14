package main

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCopyBody_Chunked(t *testing.T) {
	// 模拟一段完整的 HTTP Chunked 响应体
	// 包含：2个正常数据块 + 0结束块 + 1个Trailer
	inputData := "5\r\nHello\r\n6\r\n World\r\n0\r\nX-Checksum: 123\r\n\r\n"

	reader := bufio.NewReader(strings.NewReader(inputData))
	var writer bytes.Buffer
	resp := Response{bodyFraming: FramingChunked}

	err := copyBody(reader, &writer, resp)
	if err != nil {
		t.Fatalf("Chunked 解析失败: %v", err)
	}

	// 验证：输出的数据是否与输入完全一模一样（原封不动转发）
	if writer.String() != inputData {
		t.Errorf("转发数据不一致!\n期望: %q\n实际: %q", inputData, writer.String())
	}

	// 验证：reader 是否完全空了（决定了能否 Put 回连接池）
	_, err = reader.ReadByte()
	if err != io.EOF {
		t.Errorf("reader 未清空! 残留错误: %v", err)
	}
}

func TestCopyBody_Chunked_InvalidHex(t *testing.T) {
	// 模拟一个非法的长度块
	inputData := "Z\r\nHello\r\n"
	reader := bufio.NewReader(strings.NewReader(inputData))
	var writer bytes.Buffer
	resp := Response{bodyFraming: FramingChunked}

	err := copyBody(reader, &writer, resp)
	if err == nil {
		t.Errorf("期望出现非法的十六进制解析错误，但返回了 nil")
	}
}

func TestCopyBody_ContentLength(t *testing.T) {
	inputData := "HelloWorld"
	reader := bufio.NewReader(strings.NewReader(inputData))
	var writer bytes.Buffer

	resp := Response{
		bodyFraming:   FramingContentLength,
		contentLength: 5,
	}

	err := copyBody(reader, &writer, resp)
	if err != nil {
		t.Fatalf("ContentLength 解析失败: %v", err)
	}

	if writer.String() != "Hello" {
		t.Errorf("期望读到 Hello, 实际读到 %q", writer.String())
	}

	// 验证剩下的 World 还在 reader 里
	rest, _ := reader.ReadString('\n')
	if rest != "World" {
		t.Errorf("期望残留 World, 实际残留 %q", rest)
	}
}
