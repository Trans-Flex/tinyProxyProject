package main

import (
	"strings"
	"testing"
)

func TestBuildErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantReason string
	}{
		{"400 Bad Request", 400, "400 Bad Request"},
		{"500 Internal Server Error", 500, "500 Internal Server Error"},
		{"501 Not Implemented", 501, "501 Not Implemented"},
		{"502 Bad Gateway", 502, "502 Bad Gateway"},
		{"503 Service Unavailable", 503, "503 Service Unavailable"},
		{"504 Gateway Timeout", 504, "504 Gateway Timeout"},
		{"未知状态码 fallback", 999, "999 Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := buildErrorResponse(tt.statusCode)

			wantContains := []string{
				"HTTP/1.1",
				tt.wantReason,
				"Content-Length: 0",
				"Connection: close",
			}
			for _, want := range wantContains {
				if !strings.Contains(resp, want) {
					t.Errorf("响应缺少 %q\n实际: %q", want, resp)
				}
			}
			if !strings.HasSuffix(resp, "\r\n\r\n") {
				t.Errorf("响应缺少结尾空行\n实际: %q", resp)
			}
		})
	}
}

func TestBuildRequest(t *testing.T) {
	// 辅助：构造 Info
	mkInfo := func(method, scheme, host string, port int, path, query, version string) Info {
		return Info{
			method:  method,
			scheme:  scheme,
			host:    host,
			port:    port,
			path:    path,
			query:   query,
			version: version,
		}
	}

	t.Run("标准 GET 请求", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "/path", "", "HTTP/1.1")
		head := makeHeader("Host", "example.com", "User-Agent", "curl/8.0", "Accept", "*/*")

		got, err := buildRequest(info, head)
		if err != nil {
			t.Fatalf("buildRequest 出错: %v", err)
		}

		wantContains := []string{
			"GET /path HTTP/1.1\r\n",
			"Host: example.com\r\n",
			"User-Agent: curl/8.0\r\n",
			"Accept: */*\r\n",
			"Via: 1.1 proxy\r\n",
			"Connection: close\r\n",
		}
		for _, want := range wantContains {
			if !strings.Contains(got, want) {
				t.Errorf("请求缺少 %q\n实际:\n%s", want, got)
			}
		}
	})

	t.Run("带 query", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "/a", "b=1&c=2", "HTTP/1.1")
		head := makeHeader("Host", "example.com")

		got, _ := buildRequest(info, head)
		if !strings.Contains(got, "GET /a?b=1&c=2 HTTP/1.1\r\n") {
			t.Errorf("请求行未包含 query\n实际:\n%s", got)
		}
	})

	t.Run("path 为空默认 /", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "", "", "HTTP/1.1")
		head := makeHeader()

		got, _ := buildRequest(info, head)
		if !strings.Contains(got, "GET / HTTP/1.1\r\n") {
			t.Errorf("空 path 应默认为 /\n实际:\n%s", got)
		}
	})

	t.Run("非默认端口 Host 带端口", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 8080, "/", "", "HTTP/1.1")
		head := makeHeader()

		got, _ := buildRequest(info, head)
		if !strings.Contains(got, "Host: example.com:8080\r\n") {
			t.Errorf("非默认端口 Host 应带端口\n实际:\n%s", got)
		}
	})

	t.Run("默认端口 Host 不带端口", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "/", "", "HTTP/1.1")
		head := makeHeader()

		got, _ := buildRequest(info, head)
		if !strings.Contains(got, "Host: example.com\r\n") {
			t.Errorf("默认端口 Host 不应带端口\n实际:\n%s", got)
		}
		if strings.Contains(got, "Host: example.com:80\r\n") {
			t.Errorf("默认端口不应显式写出 :80\n实际:\n%s", got)
		}
	})

	t.Run("Proxy-Connection 被删除", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "/", "", "HTTP/1.1")
		head := makeHeader("Proxy-Connection", "keep-alive", "User-Agent", "curl/8.0")

		got, _ := buildRequest(info, head)
		if strings.Contains(got, "Proxy-Connection") {
			t.Errorf("Proxy-Connection 应被删除\n实际:\n%s", got)
		}
		if !strings.Contains(got, "User-Agent: curl/8.0\r\n") {
			t.Errorf("其他头应保留\n实际:\n%s", got)
		}
	})

	t.Run("原始 Host 被重写", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "/", "", "HTTP/1.1")
		head := makeHeader("Host", "evil.com") // 客户端发的 Host 与 URI 不符

		got, _ := buildRequest(info, head)
		if strings.Contains(got, "evil.com") {
			t.Errorf("原始 Host 应被重写，不应出现 evil.com\n实际:\n%s", got)
		}
		if !strings.Contains(got, "Host: example.com\r\n") {
			t.Errorf("Host 应重写为 example.com\n实际:\n%s", got)
		}
	})

	t.Run("Connection 头被删除", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "/", "", "HTTP/1.1")
		head := makeHeader("Connection", "keep-alive")

		got, _ := buildRequest(info, head)
		if strings.Contains(got, "Connection: keep-alive") {
			t.Errorf("原始 Connection 应被删除\n实际:\n%s", got)
		}
		if !strings.Contains(got, "Connection: close\r\n") {
			t.Errorf("应添加 Connection: close\n实际:\n%s", got)
		}
	})

	t.Run("missing host 返回错误", func(t *testing.T) {
		info := mkInfo("GET", "http", "", 80, "/", "", "HTTP/1.1")
		head := makeHeader()

		_, err := buildRequest(info, head)
		if err == nil {
			t.Errorf("host 为空应返回错误")
		}
	})

	t.Run("missing method 返回错误", func(t *testing.T) {
		info := mkInfo("", "http", "example.com", 80, "/", "", "HTTP/1.1")
		head := makeHeader()

		_, err := buildRequest(info, head)
		if err == nil {
			t.Errorf("method 为空应返回错误")
		}
	})

	t.Run("missing version 返回错误", func(t *testing.T) {
		info := mkInfo("GET", "http", "example.com", 80, "/", "", "")
		head := makeHeader()

		_, err := buildRequest(info, head)
		if err == nil {
			t.Errorf("version 为空应返回错误")
		}
	})
}
