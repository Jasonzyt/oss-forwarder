package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	// OSS基础URL
	ossBaseURL = "https://jasonzyt-gallery.oss-cn-hongkong-internal.aliyuncs.com"
	// 服务器监听端口
	serverPort = ":8000"
	// 请求超时时间
	requestTimeout = 30 * time.Second
)

// OSSProxy 结构体用于封装OSS代理功能
type OSSProxy struct {
	client *http.Client
}

// NewOSSProxy 创建新的OSS代理实例
func NewOSSProxy() *OSSProxy {
	return &OSSProxy{
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// ServeHTTP 实现http.Handler接口
func (p *OSSProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 只处理GET请求
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取请求路径
	path := r.URL.Path

	// 构建OSS URL
	ossURL := ossBaseURL + path

	log.Printf("代理请求: %s -> %s", r.URL.Path, ossURL)

	// 创建对OSS的请求
	ossReq, err := http.NewRequest(http.MethodGet, ossURL, nil)
	if err != nil {
		log.Printf("创建OSS请求失败: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 复制原始请求的一些头部（如果需要）
	copyHeaders := []string{
		"User-Agent",
		"Accept",
		"Accept-Encoding",
		"Accept-Language",
	}

	for _, header := range copyHeaders {
		if value := r.Header.Get(header); value != "" {
			ossReq.Header.Set(header, value)
		}
	}

	// 发送请求到OSS
	resp, err := p.client.Do(ossReq)
	if err != nil {
		log.Printf("OSS请求失败: %v", err)
		http.Error(w, "Failed to fetch from OSS", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 设置状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	bytesWritten, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("复制响应体失败: %v", err)
		return
	}

	log.Printf("成功代理请求 %s，返回 %d 字节，状态码: %d", path, bytesWritten, resp.StatusCode)
}

// healthCheck 健康检查处理器
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "ok", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
}

func main() {
	// 创建OSS代理实例
	proxy := NewOSSProxy()

	// 设置路由
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", healthCheck)

	// 所有其他路径都转发到OSS
	mux.Handle("/", proxy)

	// 创建服务器
	server := &http.Server{
		Addr:         serverPort,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("OSS代理服务器启动，监听端口 %s", serverPort)
	log.Printf("OSS目标地址: %s", ossBaseURL)
	log.Printf("健康检查地址: http://localhost%s/health", serverPort)

	// 启动服务器
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
