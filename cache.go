package main

import (
	"container/list"
	"fmt"
	"strings"
	"sync"
)

type Cache struct {
	dataList  *list.List
	keyToNode map[string]*list.Element
	mu        sync.Mutex
	maxBytes  int
	curBytes  int
}

type cacheEntry struct {
	key   string
	value []byte
}

func NewCache(maxBytes int) (*Cache, error) {
	if maxBytes <= 0 {
		return &Cache{}, fmt.Errorf("容量 %d 至少大于0!", maxBytes)
	}
	var cache Cache
	cache.dataList = list.New()
	cache.keyToNode = make(map[string]*list.Element)
	cache.maxBytes = maxBytes
	return &cache, nil
}

// 返回只读数据，不要改
func (cache *Cache) get(key string) ([]byte, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	element, ok := cache.keyToNode[key]
	if !ok {
		return nil, false
	}
	entry := element.Value.(*cacheEntry)
	cache.dataList.MoveToFront(element)
	return entry.value, true
}

func (cache *Cache) put(key string, value []byte) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	size := len(value)

	if size > cache.maxBytes {
		return fmt.Errorf("key %q 的数据长度 %d 超过上限 %d", key, size, cache.maxBytes)
	}

	//确保旧值被删除，并且保证在没有旧值时不做任何事
	cache.removeWithoutLock(key)

	for size+cache.curBytes > cache.maxBytes {
		//这里不判空是因为当空的时候cache.curByte == 0，若还超过则size > cache.maxBytes在前面已经拦截
		death := cache.dataList.Back()
		entry := death.Value.(*cacheEntry)
		cache.removeWithoutLock(entry.key)
	}

	element := cache.dataList.PushFront(&cacheEntry{key: key, value: value})
	cache.keyToNode[key] = element
	cache.curBytes += size
	return nil
}

func (cache *Cache) removeWithoutLock(key string) {
	//并发不安全，仅函数内部使用!
	death, ok := cache.keyToNode[key]
	if !ok {
		return
	}
	entry := death.Value.(*cacheEntry)
	size := len(entry.value)
	cache.curBytes -= size
	delete(cache.keyToNode, key)
	cache.dataList.Remove(death)
}

func (cache *Cache) ableToCache(resp *Response) bool {
	if resp.statusCode != 200 {
		return false
	}

	if resp.bodyFraming != FramingContentLength {
		return false
	}

	if resp.contentLength > cache.maxBytes {
		return false
	}

	header := resp.header
	if header.has("Set-Cookie") {
		return false
	}

	vals, ok := header.get("Cache-Control")
	if !ok {
		return true
	}

	for _, val := range vals {
		val = strings.ToLower(val)
		parts := strings.Split(val, ",")
		for _, part := range parts {
			part, _, _ = strings.Cut(part, "=")
			part = strings.TrimSpace(part)
			if part == "no-store" || part == "private" || part == "no-cache" {
				return false
			}
		}
	}

	return true
}
