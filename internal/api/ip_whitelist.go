package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

// IPWhitelist 可动态更新的 IP 白名单中间件
type IPWhitelist struct {
	mu       sync.RWMutex
	allowed  []*net.IPNet
	enabled  bool
	onReject func(ip string)
}

// NewIPWhitelist 从 CIDR 列表创建白名单。空列表则禁用白名单检查。
func NewIPWhitelist(cidrs []string, onReject func(ip string)) *IPWhitelist {
	var networks []*net.IPNet
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if !strings.Contains(cidr, "/") {
			cidr = cidr + "/32"
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// 跳过无效 CIDR，记录警告（通过 onReject 回调）
			if onReject != nil {
				onReject(fmt.Sprintf("invalid CIDR: %s", cidr))
			}
			continue
		}
		networks = append(networks, ipNet)
	}

	return &IPWhitelist{
		allowed:  networks,
		enabled:  len(networks) > 0,
		onReject: onReject,
	}
}

// Middleware 返回 HTTP 中间件，仅允许白名单中的 IP 访问
func (w *IPWhitelist) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !w.enabled {
			next.ServeHTTP(rw, r)
			return
		}

		clientIP := extractClientIP(r)
		if clientIP == "" {
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		if w.isAllowed(clientIP) {
			next.ServeHTTP(rw, r)
			return
		}

		if w.onReject != nil {
			w.onReject(clientIP)
		}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusForbidden)
		rw.Write([]byte(`{"error":"IP不在白名单中"}`))
	})
}

// Update 动态更新白名单 CIDR 列表
func (w *IPWhitelist) Update(cidrs []string) {
	var networks []*net.IPNet
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if !strings.Contains(cidr, "/") {
			cidr = cidr + "/32"
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		networks = append(networks, ipNet)
	}

	w.mu.Lock()
	w.allowed = networks
	w.enabled = len(networks) > 0
	w.mu.Unlock()
}

func (w *IPWhitelist) isAllowed(ip string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, ipNet := range w.allowed {
		if ipNet.Contains(parsedIP) {
			return true
		}
	}
	return false
}

// extractClientIP 从请求中提取客户端 IP
func extractClientIP(r *http.Request) string {
	// 检查 X-Real-IP（nginx 常用）
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	// 检查 X-Forwarded-For（第一个非代理 IP）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, p := range parts {
			ip := strings.TrimSpace(p)
			if ip != "" {
				return ip
			}
		}
	}
	// 从 RemoteAddr 提取
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
