package main

import (
	"fmt"
	"strings"
)

type HeaderField struct {
	originName string
	lowerName  string
	originVal  string
	trimmedVal string
}

type HeaderCollection struct {
	fields []HeaderField
	index  map[string][]int
}

func (h *HeaderCollection) has(name string) bool {
	search := strings.ToLower(name)
	_, ok := h.index[search]
	return ok
}

func (h *HeaderCollection) first(name string) string {
	search := strings.ToLower(name)
	vals, ok := h.index[search]
	if !ok {
		return ""
	}
	return h.fields[vals[0]].trimmedVal
}

func (h *HeaderCollection) get(name string) ([]string, bool) {
	search := strings.ToLower(name)

	var ans []string
	vals, ok := h.index[search]
	if !ok {
		return ans, false
	}
	for _, idx := range vals {
		ans = append(ans, h.fields[idx].trimmedVal)
	}

	return ans, true
}

func (h *HeaderCollection) delete(name string) {
	search := strings.ToLower(name)
	if _, ok := h.index[search]; !ok {
		return
	}
	var newFields []HeaderField
	newIndex := make(map[string][]int)
	idx := 0
	for i := 0; i < len(h.fields); i++ {
		if search == h.fields[i].lowerName {
			continue
		}
		newFields = append(newFields, h.fields[i])
		newIndex[h.fields[i].lowerName] = append(newIndex[h.fields[i].lowerName], idx)
		idx++
	}
	h.fields = newFields
	h.index = newIndex
}

func (h *HeaderCollection) add(name string, value string) {
	if h.index == nil {
		h.index = make(map[string][]int)
	}
	lowerName := strings.ToLower(name)
	trimmedVal := strings.TrimSpace(value)
	var field HeaderField
	field.originName = name
	field.lowerName = lowerName
	field.originVal = value
	field.trimmedVal = trimmedVal

	idx := len(h.fields)
	h.fields = append(h.fields, field)
	h.index[lowerName] = append(h.index[lowerName], idx)
}

func (h *HeaderCollection) set(name string, value string) {
	h.delete(name)
	h.add(name, value)
}

func parseHeaders(header string) (HeaderCollection, error) {
	var head HeaderCollection
	head.index = make(map[string][]int)

	header = strings.ReplaceAll(header, "\r\n", "\n")
	parts := strings.Split(header, "\n")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" {
			break
		}
		idx := strings.IndexByte(parts[i], ':')
		if idx == -1 {
			return HeaderCollection{}, fmt.Errorf("无效的header字段: %q", parts[i])
		}
		name := strings.Clone(parts[i][:idx])
		err := checkName(name)
		if err != nil {
			return HeaderCollection{}, fmt.Errorf("解析头字段 %q 失败: %w", header, err)
		}
		value := strings.Clone(parts[i][idx+1:])

		head.add(name, value)
	}

	return head, nil
}
