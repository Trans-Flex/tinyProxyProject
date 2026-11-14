package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type idleConn struct {
	conn     net.Conn
	lastUsed time.Time // 记录放回池子的时间
}

type ConnPool struct {
	idleConns  chan *idleConn
	totalConns int32
	maxActive  int32
	mu         sync.Mutex

	idleTimeout time.Duration

	dialFunc func() (net.Conn, error)
}

func NewConnPool(maxIdle int, idleTimeout time.Duration, dialFunc func() (net.Conn, error)) *ConnPool {
	return &ConnPool{
		idleConns:   make(chan *idleConn, maxIdle),
		totalConns:  0,
		maxActive:   int32(2 * maxIdle),
		idleTimeout: idleTimeout,
		dialFunc:    dialFunc,
	}
}

func (p *ConnPool) Get() (net.Conn, error) {
	for {
		select {
		case idle := <-p.idleConns:
			if p.check(idle) {
				return idle.conn, nil
			}
			p.destroyConn(idle.conn)
			continue
		default:
			p.mu.Lock()
			if p.totalConns < p.maxActive {
				p.totalConns++
				p.mu.Unlock()

				conn, err := p.dialFunc()
				if err != nil {
					p.mu.Lock()
					p.totalConns--
					p.mu.Unlock()
					return nil, err
				}
				return conn, nil
			}
			p.mu.Unlock()
		}

		select {
		case idle := <-p.idleConns:
			if p.check(idle) {
				return idle.conn, nil
			}
			p.destroyConn(idle.conn)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (p *ConnPool) Put(conn net.Conn) {
	if conn == nil {
		return
	}
	select {
	case p.idleConns <- &idleConn{conn: conn, lastUsed: time.Now()}:
		// 塞进去了，安全归还
		fmt.Println("连接归还成功")
	default:
		// 通道满了，说明空闲连接太多，直接销毁
		conn.Close()
		p.mu.Lock()
		p.totalConns--
		p.mu.Unlock()
		fmt.Println("池子满了，多余连接已销毁")
	}
}

func (p *ConnPool) isAlive(conn net.Conn) bool {
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	var buf [1]byte
	_, err := conn.Read(buf[:])
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			conn.SetReadDeadline(time.Time{})
			return true
		}
	}
	return false
}

func (p *ConnPool) destroyConn(conn net.Conn) {
	conn.Close()
	p.mu.Lock()
	p.totalConns--
	p.mu.Unlock()
}

func (p *ConnPool) check(idle *idleConn) bool {
	if time.Since(idle.lastUsed) > p.idleTimeout {
		return false
	}
	if !p.isAlive(idle.conn) {
		return false
	}
	return true
}

func (p *ConnPool) Discard(conn net.Conn) {
	if conn == nil {
		return
	}
	_ = conn.Close()
	p.mu.Lock()
	p.totalConns--
	p.mu.Unlock()
}
