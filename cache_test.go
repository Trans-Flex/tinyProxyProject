package main

import (
	"bytes"
	"testing"
)

func TestCache(t *testing.T) {
	t.Run("put 后 get 命中", func(t *testing.T) {
		c, err := NewCache(1024)
		if err != nil {
			t.Fatalf("NewCache 失败: %v", err)
		}
		c.put("k1", []byte("hello"))

		got, ok := c.get("k1")
		if !ok {
			t.Fatalf("get(k1) ok = false, 期望 true")
		}
		if !bytes.Equal(got, []byte("hello")) {
			t.Errorf("get(k1) = %q, 期望 %q", got, "hello")
		}
	})

	t.Run("get 未命中", func(t *testing.T) {
		c, _ := NewCache(1024)
		got, ok := c.get("nothing")
		if ok {
			t.Errorf("get(nothing) ok = true, 期望 false")
		}
		if got != nil {
			t.Errorf("get(nothing) = %v, 期望 nil", got)
		}
	})

	t.Run("put 超出 maxBytes 返回错误", func(t *testing.T) {
		c, _ := NewCache(10)
		err := c.put("big", make([]byte, 11))
		if err == nil {
			t.Errorf("put 超大值应返回错误，实际 nil")
		}
		if _, ok := c.get("big"); ok {
			t.Errorf("超大值不应被存入缓存")
		}
	})

	t.Run("put 重复 key 替换旧值", func(t *testing.T) {
		c, _ := NewCache(1024)
		c.put("k", []byte("old"))
		c.put("k", []byte("new"))

		got, ok := c.get("k")
		if !ok {
			t.Fatalf("get(k) ok = false, 期望 true")
		}
		if !bytes.Equal(got, []byte("new")) {
			t.Errorf("get(k) = %q, 期望 %q", got, "new")
		}
	})

	t.Run("curBytes 统计正确", func(t *testing.T) {
		c, _ := NewCache(1024)
		c.put("a", make([]byte, 100))
		c.put("b", make([]byte, 200))
		if c.curBytes != 300 {
			t.Errorf("curBytes = %d, 期望 300", c.curBytes)
		}
		c.put("a", make([]byte, 50)) // 替换 a
		if c.curBytes != 250 {
			t.Errorf("替换后 curBytes = %d, 期望 250", c.curBytes)
		}
	})

	t.Run("淘汰最久未用", func(t *testing.T) {
		c, _ := NewCache(300)
		c.put("a", make([]byte, 100))
		c.put("b", make([]byte, 100))
		c.put("c", make([]byte, 100))
		// 此时 a 最久未用
		c.put("d", make([]byte, 100)) // 触发淘汰 a

		if _, ok := c.get("a"); ok {
			t.Errorf("a 应被淘汰")
		}
		if _, ok := c.get("b"); !ok {
			t.Errorf("b 不应被淘汰")
		}
		if _, ok := c.get("c"); !ok {
			t.Errorf("c 不应被淘汰")
		}
		if _, ok := c.get("d"); !ok {
			t.Errorf("d 应存在")
		}
	})

	t.Run("get 更新使用顺序", func(t *testing.T) {
		c, _ := NewCache(300)
		c.put("a", make([]byte, 100))
		c.put("b", make([]byte, 100))
		c.put("c", make([]byte, 100))
		// 访问 a，让 a 变成最近使用
		c.get("a")
		// 此时 b 最久未用
		c.put("d", make([]byte, 100)) // 淘汰 b

		if _, ok := c.get("a"); !ok {
			t.Errorf("a 被访问过，不应被淘汰")
		}
		if _, ok := c.get("b"); ok {
			t.Errorf("b 应被淘汰")
		}
	})

	t.Run("NewCache 拒绝非正数", func(t *testing.T) {
		if _, err := NewCache(0); err == nil {
			t.Errorf("NewCache(0) 应返回错误")
		}
		if _, err := NewCache(-1); err == nil {
			t.Errorf("NewCache(-1) 应返回错误")
		}
	})

	t.Run("removeWithoutLock 后查不到", func(t *testing.T) {
		c, _ := NewCache(1024)
		c.put("k", []byte("v"))
		c.removeWithoutLock("k")
		if _, ok := c.get("k"); ok {
			t.Errorf("remove 后 get(k) ok = true, 期望 false")
		}
	})

	t.Run("淘汰后 curBytes 正确", func(t *testing.T) {
		c, _ := NewCache(200)
		c.put("a", make([]byte, 100))
		c.put("b", make([]byte, 100))
		c.put("c", make([]byte, 100)) // 淘汰 a，curBytes 应为 200

		if c.curBytes != 200 {
			t.Errorf("淘汰后 curBytes = %d, 期望 200", c.curBytes)
		}
	})
}

func TestCacheKey(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "标准 http",
			info: Info{scheme: "http", host: "example.com", port: 80, path: "/path"},
			want: "http://example.com:80/path",
		},
		{
			name: "带 query",
			info: Info{scheme: "http", host: "example.com", port: 80, path: "/a", query: "b=1&c=2"},
			want: "http://example.com:80/a?b=1&c=2",
		},
		{
			name: "空 query 不加问号",
			info: Info{scheme: "http", host: "example.com", port: 80, path: "/a", query: ""},
			want: "http://example.com:80/a",
		},
		{
			name: "https 默认端口",
			info: Info{scheme: "https", host: "example.com", port: 443, path: "/"},
			want: "https://example.com:443/",
		},
		{
			name: "非默认端口",
			info: Info{scheme: "http", host: "example.com", port: 8080, path: "/a"},
			want: "http://example.com:8080/a",
		},
		{
			name: "IPv6 加方括号",
			info: Info{scheme: "http", host: "::1", port: 8080, path: "/a"},
			want: "http://[::1]:8080/a",
		},
		{
			name: "默认端口也带进 key",
			info: Info{scheme: "http", host: "example.com", port: 80, path: "/"},
			want: "http://example.com:80/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cacheKey(tt.info)
			if got != tt.want {
				t.Errorf("cacheKey(%+v) = %q, 期望 %q", tt.info, got, tt.want)
			}
		})
	}
}
