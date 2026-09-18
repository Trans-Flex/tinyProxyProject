package main

import "testing"

func TestCheckName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// 合法输入
		{"normal host", "Host", false},
		{"lowercase", "host", false},
		{"uppercase", "HOST", false},
		{"with hyphen", "Content-Length", false},
		{"with digits", "X-Forwarded-For-2", false},
		{"single char", "A", false},
		{"special token chars", "!#$%&'*+-.^_`|~", false},
		{"mixed", "X-Custom_Header", false},

		// 非法输入
		{"empty", "", true},
		{"with space", "Host Name", true},
		{"with tab", "Host\tName", true},
		{"with colon", "Host:", true},
		{"with slash", "Host/Name", true},
		{"with paren", "Host()", true},
		{"with at", "Host@Name", true},
		{"with comma", "Host,Name", true},
		{"with non-ascii", "Host名", true},
		{"with control char", "Host\x00", true},
		{"with newline", "Host\n", true},
		{"with cr", "Host\r", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkName(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("checkName(%q) = nil, 期望错误", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("checkName(%q) = %v, 期望 nil", tt.input, err)
			}
		})
	}
}

func TestDecideFraming(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		header      HeaderCollection
		statusCode  int
		wantFraming BodyFraming
		wantLength  int
		wantErr     bool
	}{
		// 无 body
		{
			name:        "HEAD 无 body",
			method:      "HEAD",
			header:      makeHeader("Content-Length", "1024"),
			statusCode:  200,
			wantFraming: FramingNone,
			wantLength:  -1,
		},
		{
			name:        "100 无 body",
			method:      "GET",
			header:      makeHeader(),
			statusCode:  100,
			wantFraming: FramingNone,
			wantLength:  -1,
		},
		{
			name:        "204 无 body",
			method:      "GET",
			header:      makeHeader(),
			statusCode:  204,
			wantFraming: FramingNone,
			wantLength:  -1,
		},
		{
			name:        "304 无 body",
			method:      "GET",
			header:      makeHeader(),
			statusCode:  304,
			wantFraming: FramingNone,
			wantLength:  -1,
		},

		// chunked
		{
			name:        "chunked",
			method:      "GET",
			header:      makeHeader("Transfer-Encoding", "chunked"),
			statusCode:  200,
			wantFraming: FramingChunked,
			wantLength:  -1,
		},
		{
			name:        "gzip, chunked",
			method:      "GET",
			header:      makeHeader("Transfer-Encoding", "gzip, chunked"),
			statusCode:  200,
			wantFraming: FramingChunked,
			wantLength:  -1,
		},
		{
			name:        "chunked 带参数",
			method:      "GET",
			header:      makeHeader("Transfer-Encoding", "chunked; q=1"),
			statusCode:  200,
			wantFraming: FramingChunked,
			wantLength:  -1,
		},

		// Content-Length
		{
			name:        "Content-Length 1024",
			method:      "GET",
			header:      makeHeader("Content-Length", "1024"),
			statusCode:  200,
			wantFraming: FramingContentLength,
			wantLength:  1024,
		},
		{
			name:        "Content-Length 0",
			method:      "GET",
			header:      makeHeader("Content-Length", "0"),
			statusCode:  200,
			wantFraming: FramingContentLength,
			wantLength:  0,
		},
		{
			name:        "Content-Length 非数字",
			method:      "GET",
			header:      makeHeader("Content-Length", "abc"),
			statusCode:  200,
			wantFraming: FramingUntilEOF,
			wantLength:  -1,
			wantErr:     true,
		},
		{
			name:        "Content-Length 负数",
			method:      "GET",
			header:      makeHeader("Content-Length", "-1"),
			statusCode:  200,
			wantFraming: FramingUntilEOF,
			wantLength:  -1,
			wantErr:     true,
		},
		{
			name:        "Content-Length 前导零",
			method:      "GET",
			header:      makeHeader("Content-Length", "007"),
			statusCode:  200,
			wantFraming: FramingUntilEOF,
			wantLength:  -1,
			wantErr:     true,
		},

		// until EOF
		{
			name:        "都没有",
			method:      "GET",
			header:      makeHeader(),
			statusCode:  200,
			wantFraming: FramingUntilEOF,
			wantLength:  -1,
		},

		// 优先级：chunked 优先于 Content-Length
		{
			name:        "chunked 和 Content-Length 同时存在",
			method:      "GET",
			header:      makeHeader("Transfer-Encoding", "chunked", "Content-Length", "1024"),
			statusCode:  200,
			wantFraming: FramingChunked,
			wantLength:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			framing, length, err := decideFraming(tt.method, tt.header, tt.statusCode)

			if tt.wantErr {
				if err == nil {
					t.Errorf("期望错误，实际 nil，framing=%v length=%d", framing, length)
				}
				return
			}
			if err != nil {
				t.Errorf("期望 nil，实际错误: %v", err)
				return
			}
			if framing != tt.wantFraming {
				t.Errorf("framing = %v, 期望 %v", framing, tt.wantFraming)
			}
			if length != tt.wantLength {
				t.Errorf("contentLength = %d, 期望 %d", length, tt.wantLength)
			}
		})
	}
}

func makeHeader(pairs ...string) HeaderCollection {
	var h HeaderCollection
	for i := 0; i+1 < len(pairs); i += 2 {
		h.add(pairs[i], pairs[i+1])
	}
	return h
}
