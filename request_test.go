package main

import "testing"

func TestParseRequestLine(t *testing.T) {
	tests := []struct {
		name        string
		requestLine string
		wantInfo    Info
		wantErr     bool
	}{
		// 合法
		{
			name:        "标准请求",
			requestLine: "GET http://example.com/path HTTP/1.1",
			wantInfo:    Info{method: "GET", scheme: "http", host: "example.com", port: 80, path: "/path", query: "", version: "HTTP/1.1"},
		},
		{
			name:        "带端口",
			requestLine: "GET http://example.com:8080/a HTTP/1.1",
			wantInfo:    Info{method: "GET", scheme: "http", host: "example.com", port: 8080, path: "/a", version: "HTTP/1.1"},
		},
		{
			name:        "无路径",
			requestLine: "GET http://example.com HTTP/1.1",
			wantInfo:    Info{method: "GET", scheme: "http", host: "example.com", port: 80, path: "/", version: "HTTP/1.1"},
		},
		{
			name:        "带 query",
			requestLine: "GET http://example.com/a?b=1&c=2 HTTP/1.1",
			wantInfo:    Info{method: "GET", scheme: "http", host: "example.com", port: 80, path: "/a", query: "b=1&c=2", version: "HTTP/1.1"},
		},
		{
			name:        "https 默认端口",
			requestLine: "GET https://example.com/ HTTP/1.1",
			wantInfo:    Info{method: "GET", scheme: "https", host: "example.com", port: 443, path: "/", version: "HTTP/1.1"},
		},
		{
			name:        "IPv6",
			requestLine: "GET http://[::1]:8080/a HTTP/1.1",
			wantInfo:    Info{method: "GET", scheme: "http", host: "::1", port: 8080, path: "/a", version: "HTTP/1.1"},
		},
		{
			name:        "带 fragment",
			requestLine: "GET http://example.com/a#frag HTTP/1.1",
			wantInfo:    Info{method: "GET", scheme: "http", host: "example.com", port: 80, path: "/a", version: "HTTP/1.1"},
		},
		{
			name:        "CONNECT 标准",
			requestLine: "CONNECT example.com:443 HTTP/1.1",
			wantInfo:    Info{method: "CONNECT", scheme: "https", host: "example.com", port: 443, path: "/", version: "HTTP/1.1"},
		},
		{
			name:        "CONNECT IPv6",
			requestLine: "CONNECT [::1]:443 HTTP/1.1",
			wantInfo:    Info{method: "CONNECT", scheme: "https", host: "::1", port: 443, path: "/", version: "HTTP/1.1"},
		},
		{
			name:        "CONNECT 非默认端口",
			requestLine: "CONNECT example.com:8443 HTTP/1.1",
			wantInfo:    Info{method: "CONNECT", scheme: "https", host: "example.com", port: 8443, path: "/", version: "HTTP/1.1"},
		},

		// 非法
		{
			name:        "空的 requestLine",
			requestLine: "",
			wantErr:     true,
		},
		{
			name:        "段数不足",
			requestLine: "GET http://example.com/path",
			wantErr:     true,
		},
		{
			name:        "段数过多",
			requestLine: "GET http://example.com/path HTTP/1.1 extra",
			wantErr:     true,
		},
		{
			name:        "缺少 scheme",
			requestLine: "GET example.com/path HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "无效的 scheme",
			requestLine: "GET nttq://example.com/path HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "authority 为空",
			requestLine: "GET http:///path HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "端口非数字",
			requestLine: "GET http://example.com:abc/ HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "host 为空",
			requestLine: "GET http://:8080/ HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "IPv6 缺右括号",
			requestLine: "GET http://[::1/a HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "IPv6 后接非法字符",
			requestLine: "GET http://[::1]abc/ HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "CONNECT 缺少端口",
			requestLine: "CONNECT example.com HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "CONNECT IPv6 缺少端口",
			requestLine: "CONNECT [::1] HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "CONNECT 带 path",
			requestLine: "CONNECT example.com:443/path HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "CONNECT 带 query",
			requestLine: "CONNECT example.com:443?x=1 HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "CONNECT 带 fragment",
			requestLine: "CONNECT example.com:443#frag HTTP/1.1",
			wantErr:     true,
		},
		{
			name:        "CONNECT 端口非数字",
			requestLine: "CONNECT example.com:abc HTTP/1.1",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRequestLine(tt.requestLine)

			if tt.wantErr {
				if err == nil {
					t.Errorf("期望错误，实际 nil，返回: %+v", got)
				}
				return
			}
			if err != nil {
				t.Errorf("期望 nil，实际错误: %v", err)
				return
			}
			if got != tt.wantInfo {
				t.Errorf("Info 不匹配\n实际: %+v\n期望: %+v", got, tt.wantInfo)
			}
		})
	}
}
