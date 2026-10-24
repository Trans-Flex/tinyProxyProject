package main

import (
	"net"
	"testing"
	"time"
)

// 测试空闲超时（静默状态）
func TestTimeoutConn_Idle(t *testing.T) {
	// 创建一对内存连接
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// 包装客户端，设置 100ms 超时
	tc := &TimeoutConn{Conn: client, timeout: 100 * time.Millisecond}

	buf := make([]byte, 10)
	start := time.Now()

	// 尝试读取（因为 server 没写数据，会一直阻塞）
	_, err := tc.Read(buf)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("期望读取超时，实际读取成功了")
	}

	// 验证是否真的等了大概 100ms
	if elapsed < 100*time.Millisecond {
		t.Fatalf("超时时间太短，期望至少 100ms，实际: %v", elapsed)
	}
	t.Logf("空闲超时符合预期，耗时: %v, 错误: %v", elapsed, err)
}

// 测试活跃保活（有数据流动时，不断刷新超时时间）
func TestTimeoutConn_Active(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// 设置 100ms 超时
	tc := &TimeoutConn{Conn: client, timeout: 100 * time.Millisecond}

	// 启动一个 goroutine，每隔 50ms 写入一次数据（小于超时时间）
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(50 * time.Millisecond)
			server.Write([]byte("ping"))
		}
		// 写完之后静默，让下一次 Read 触发超时
	}()

	buf := make([]byte, 10)
	start := time.Now()

	// 连续读取 5 次数据，总耗时约 250ms（超过了 100ms 的超时）
	for i := 0; i < 5; i++ {
		_, err := tc.Read(buf)
		if err != nil {
			t.Fatalf("第 %d 次读取发生了意外错误（说明保活失败，被误杀了）: %v", i+1, err)
		}
	}

	elapsed := time.Since(start)
	t.Logf("活跃读取成功，没有被错误断开，总计耗时: %v", elapsed)

	// 此时发送端已经停止写入，再读一次应该触发超时
	_, err := tc.Read(buf)
	if err == nil {
		t.Fatal("发送端静默后期望触发超时，实际读取成功了")
	}
	t.Logf("静默后正常触发了超时错误: %v", err)
}
