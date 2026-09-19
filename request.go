package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Info struct {
	method  string
	scheme  string
	host    string
	port    int
	path    string
	query   string
	version string
}

// 尝试支持CONNECT
func parseRequestLine(requestLine string) (Info, error) {
	if requestLine == "" {
		return Info{}, fmt.Errorf("请求行为空！")
	}
	var info Info
	var err error
	var ok bool
	parts := strings.Split(requestLine, " ")
	if len(parts) != 3 {
		return Info{}, fmt.Errorf("无效的请求行: %q", requestLine)
	}

	var restLine string
	info.method = strings.Clone(parts[0])
	info.version = strings.Clone(parts[2])
	if info.method == "CONNECT" {
		//隧道分支
		info.scheme = "https"
		restLine = parts[1]
	} else {
		//普通http分支
		info.scheme, restLine, ok = strings.Cut(parts[1], "://")
		if !ok {
			return Info{}, fmt.Errorf("无效的请求行: %q", requestLine)
		}
		if info.scheme != "http" && info.scheme != "https" {
			return Info{}, fmt.Errorf("无效的scheme: %q", info.scheme)
		}
	}

	var authority string
	idx := strings.IndexAny(restLine, "/?#")
	if idx == -1 {
		authority = restLine
		restLine = ""
	} else {
		if info.method == "CONNECT" {
			return Info{}, fmt.Errorf("使用CONNECT不得有path,query")
		}
		authority = strings.Clone(restLine[0:idx])
		restLine = strings.Clone(restLine[idx:])
	}

	if authority == "" {
		return Info{}, fmt.Errorf("无效的authority字段: %q", authority)
	}

	var hostport string
	idx = strings.LastIndexByte(authority, '@')
	if idx != -1 {
		hostport = strings.Clone(authority[idx+1:])
	} else {
		hostport = authority
	}

	var strPort string = ""
	if hostport == "" {
		return Info{}, fmt.Errorf("缺少host, port")
	}
	if hostport[0] == '[' {
		//IPv6
		idx = strings.IndexByte(hostport, ']')
		if idx == -1 {
			return Info{}, fmt.Errorf("无效的hostport: %q", hostport)
		}
		info.host = strings.Clone(hostport[1:idx])

		rest := strings.Clone(hostport[idx+1:])
		if rest == "" {
			//这里的处理在后面的统一处理
		} else if rest[0] == ':' {
			strPort = strings.Clone(rest[1:])
			info.port, err = strconv.Atoi(strPort)
			if err != nil {
				return Info{}, fmt.Errorf("无效的端口号: %q", strPort)
			}
		} else {
			return Info{}, fmt.Errorf("无效的端口号: %q", strPort)
		}
	} else {
		//IPv4
		idx = strings.LastIndexByte(hostport, ':')
		if idx != -1 {
			info.host = strings.Clone(hostport[:idx])
			if info.host == "" {
				return Info{}, fmt.Errorf("缺少host")
			}
			strPort = strings.Clone(hostport[idx+1:])
			info.port, err = strconv.Atoi(strPort)
			if err != nil {
				return Info{}, fmt.Errorf("无效的端口号: %q", strPort)
			}
		} else {
			info.host = strings.Clone(hostport)
		}
	}

	if strPort == "" {
		if info.scheme == "http" {
			info.port = 80
		} else {
			if info.method == "CONNECT" {
				return Info{}, fmt.Errorf("缺少port")
			}
			info.port = 443
		}
	}

	idx = strings.IndexByte(restLine, '#')
	if idx != -1 {
		restLine = strings.Clone(restLine[:idx])
	}

	if restLine == "" {
		info.path = "/"
		info.query = ""
	} else {
		switch restLine[0] {
		case '?':
			info.path = "/"
			info.query = strings.Clone(restLine[1:])
		case '/':
			idx = strings.IndexByte(restLine, '?')
			if idx == -1 {
				info.path = strings.Clone(restLine)
				info.query = ""
			} else {
				info.path = strings.Clone(restLine[:idx])
				info.query = strings.Clone(restLine[idx+1:])
			}
		default:
			return Info{}, fmt.Errorf("无效的path或query: %q", restLine)
		}
	}

	return info, nil
}
