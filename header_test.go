package main

import "testing"

func TestHeaderCollection(t *testing.T) {
	// add + has + first + get
	t.Run("add 后能查到", func(t *testing.T) {
		var h HeaderCollection
		h.add("Host", "example.com")

		if !h.has("Host") {
			t.Errorf("has(Host) = false, 期望 true")
		}
		if !h.has("host") {
			t.Errorf("has(host) = false, 大小写不敏感应命中")
		}
		if h.has("Accept") {
			t.Errorf("has(Accept) = true, 期望 false")
		}
		if got := h.first("Host"); got != "example.com" {
			t.Errorf("first(Host) = %q, 期望 %q", got, "example.com")
		}
	})

	t.Run("重复头 get 返回所有值", func(t *testing.T) {
		var h HeaderCollection
		h.add("Via", "1.1 proxy-a")
		h.add("Via", "1.1 proxy-b")

		vals, ok := h.get("Via")
		if !ok {
			t.Fatalf("get(Via) ok = false, 期望 true")
		}
		if len(vals) != 2 {
			t.Fatalf("get(Via) 返回 %d 个值, 期望 2", len(vals))
		}
		if vals[0] != "1.1 proxy-a" || vals[1] != "1.1 proxy-b" {
			t.Errorf("get(Via) = %v, 期望 [1.1 proxy-a, 1.1 proxy-b]", vals)
		}
	})

	t.Run("first 返回第一个值", func(t *testing.T) {
		var h HeaderCollection
		h.add("X-Test", "a")
		h.add("X-Test", "b")

		if got := h.first("X-Test"); got != "a" {
			t.Errorf("first(X-Test) = %q, 期望 %q", got, "a")
		}
	})

	t.Run("不存在的 key", func(t *testing.T) {
		var h HeaderCollection
		if h.has("Nothing") {
			t.Errorf("has(Nothing) = true, 期望 false")
		}
		if got := h.first("Nothing"); got != "" {
			t.Errorf("first(Nothing) = %q, 期望空", got)
		}
		vals, ok := h.get("Nothing")
		if ok {
			t.Errorf("get(Nothing) ok = true, 期望 false")
		}
		if len(vals) != 0 {
			t.Errorf("get(Nothing) 返回 %d 个值, 期望 0", len(vals))
		}
	})

	t.Run("值去首尾空白", func(t *testing.T) {
		var h HeaderCollection
		h.add("Host", "  example.com  ")

		if got := h.first("Host"); got != "example.com" {
			t.Errorf("first(Host) = %q, 期望 %q", got, "example.com")
		}
	})

	t.Run("delete 后查不到", func(t *testing.T) {
		var h HeaderCollection
		h.add("Host", "example.com")
		h.add("Accept", "*/*")
		h.delete("Host")

		if h.has("Host") {
			t.Errorf("delete 后 has(Host) = true, 期望 false")
		}
		if !h.has("Accept") {
			t.Errorf("delete(Host) 不应影响 Accept")
		}
	})

	t.Run("delete 不存在的 key 不 panic", func(t *testing.T) {
		var h HeaderCollection
		h.delete("Nothing") // 不应 panic
	})

	t.Run("set 替换为单条", func(t *testing.T) {
		var h HeaderCollection
		h.add("Host", "old1")
		h.add("Host", "old2")
		h.set("Host", "new")

		vals, _ := h.get("Host")
		if len(vals) != 1 {
			t.Fatalf("set 后 get(Host) 返回 %d 个值, 期望 1", len(vals))
		}
		if vals[0] != "new" {
			t.Errorf("set 后 get(Host) = %v, 期望 [new]", vals)
		}
	})

	t.Run("set 新 key", func(t *testing.T) {
		var h HeaderCollection
		h.set("X-New", "value")

		if got := h.first("X-New"); got != "value" {
			t.Errorf("set 新 key 后 first = %q, 期望 %q", got, "value")
		}
	})

	t.Run("大小写不敏感查找", func(t *testing.T) {
		var h HeaderCollection
		h.add("Content-Type", "text/html")

		for _, name := range []string{"Content-Type", "content-type", "CONTENT-TYPE", "CoNtEnT-TyPe"} {
			if !h.has(name) {
				t.Errorf("has(%q) = false, 期望 true", name)
			}
		}
	})
}
