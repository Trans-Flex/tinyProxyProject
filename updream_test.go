package main

import "testing"

func TestHostport(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "普通 host 默认端口",
			info: Info{host: "example.com", port: 80},
			want: "example.com:80",
		},
		{
			name: "普通 host 非默认端口",
			info: Info{host: "example.com", port: 8080},
			want: "example.com:8080",
		},
		{
			name: "IPv6 带端口",
			info: Info{host: "::1", port: 8080},
			want: "[::1]:8080",
		},
		{
			name: "IPv6 默认端口",
			info: Info{host: "::1", port: 443},
			want: "[::1]:443",
		},
		{
			name: "完整 IPv6 地址",
			info: Info{host: "2001:db8::1", port: 80},
			want: "[2001:db8::1]:80",
		},
		{
			name: "IPv4",
			info: Info{host: "127.0.0.1", port: 9000},
			want: "127.0.0.1:9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hostport(tt.info)
			if got != tt.want {
				t.Errorf("hostport(%+v) = %q, 期望 %q", tt.info, got, tt.want)
			}
		})
	}
}
