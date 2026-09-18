package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

type result struct {
	latency time.Duration
	status  int
	err     error
}

func main() {
	n := flag.Int("n", 200, "总请求数")
	c := flag.Int("c", 20, "并发数")
	target := flag.String("url", "http://httpbin.org/json", "目标 URL")
	proxyAddr := flag.String("proxy", "http://127.0.0.1:8080", "代理地址")
	flag.Parse()

	proxyURL, err := url.Parse(*proxyAddr)
	if err != nil {
		fmt.Println("代理地址不合法:", err)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			DisableKeepAlives:   false,
		},
		Timeout: 15 * time.Second,
	}

	results := make([]result, *n)
	var wg sync.WaitGroup
	var counter int
	var mu sync.Mutex

	start := time.Now()

	for i := 0; i < *c; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if counter >= *n {
					mu.Unlock()
					return
				}
				idx := counter
				counter++
				mu.Unlock()

				reqStart := time.Now()
				resp, err := client.Get(*target)
				latency := time.Since(reqStart)

				if err != nil {
					results[idx] = result{latency: latency, err: err}
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				results[idx] = result{latency: latency, status: resp.StatusCode}
			}
		}()
	}

	wg.Wait()
	total := time.Since(start)

	var success, failed int
	var latencies []time.Duration
	for _, r := range results {
		if r.err != nil {
			failed++
			continue
		}
		success++
		latencies = append(latencies, r.latency)
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	qps := float64(success) / total.Seconds()

	fmt.Printf("总请求: %d  并发: %d\n", *n, *c)
	fmt.Printf("成功: %d  失败: %d\n", success, failed)
	fmt.Printf("总耗时: %v\n", total)
	fmt.Printf("QPS: %.2f\n", qps)
	if len(latencies) > 0 {
		fmt.Printf("平均延迟: %v\n", latencies[len(latencies)/2])
		fmt.Printf("P50: %v\n", latencies[len(latencies)*50/100])
		fmt.Printf("P90: %v\n", latencies[len(latencies)*90/100])
		fmt.Printf("P99: %v\n", latencies[len(latencies)*99/100])
		fmt.Printf("最大延迟: %v\n", latencies[len(latencies)-1])
	}
}
