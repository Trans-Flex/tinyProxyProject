package main

import (
	"net"
	"time"
)

type TimeoutConn struct {
	net.Conn
	timeout time.Duration
}

func (c *TimeoutConn) Read(p []byte) (n int, err error) {
	// 只要有读操作，立刻重置读和写的 Deadline
	c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
	c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.Conn.Read(p)
}

func (c *TimeoutConn) Write(p []byte) (n int, err error) {
	// 只要有写操作，立刻重置读和写的 Deadline
	c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
	c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.Conn.Write(p)
}
