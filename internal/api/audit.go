package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// AuditEventType 审计事件类型
type AuditEventType string

const (
	AuditOrderCreate      AuditEventType = "order.create"
	AuditOrderCancel      AuditEventType = "order.cancel"
	AuditConfigUpdate     AuditEventType = "config.update"
	AuditStrategyStart    AuditEventType = "strategy.start"
	AuditStrategyStop     AuditEventType = "strategy.stop"
	AuditStrategyParams   AuditEventType = "strategy.params"
	AuditLeverageChange   AuditEventType = "leverage.change"
	AuditTpSlChange       AuditEventType = "tpsl.change"
	AuditTrailingStop     AuditEventType = "trailing.stop"
	AuditTimedOrderCreate AuditEventType = "timedorder.create"
	AuditTimedOrderCancel AuditEventType = "timedorder.cancel"
	AuditCondOrderCreate  AuditEventType = "condorder.create"
	AuditCondOrderCancel  AuditEventType = "condorder.cancel"
	AuditRebalanceReset   AuditEventType = "rebalance.reset"
)

// AuditRecord 审计日志记录
type AuditRecord struct {
	Timestamp   string            `json:"timestamp"`
	EventType   AuditEventType    `json:"event_type"`
	Actor       string            `json:"actor"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	RemoteAddr  string            `json:"remote_addr"`
	RequestID   string            `json:"request_id,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
}

var auditRequestID uint64

func nextAuditRequestID() string {
	auditRequestID++
	return fmt.Sprintf("audit-%d", auditRequestID)
}

func (r *AuditRecord) toZapFields() []zap.Field {
	fields := make([]zap.Field, 0, len(r.Details)+5)
	fields = append(fields,
		zap.String("event_type", string(r.EventType)),
		zap.String("actor", r.Actor),
		zap.String("method", r.Method),
		zap.String("path", r.Path),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("status", r.Status),
		zap.String("description", r.Description),
	)
	for k, v := range r.Details {
		fields = append(fields, zap.String(k, v))
	}
	return fields
}

// logAudit 记录结构化审计日志
func logAudit(eventType AuditEventType, description string, actor, method, path, remoteAddr string, details map[string]string, status string) {
	record := &AuditRecord{
		Timestamp:   time.Now().Format(time.RFC3339),
		EventType:   eventType,
		Actor:       actor,
		Method:      method,
		Path:        path,
		RemoteAddr:  remoteAddr,
		Details:     details,
		Status:      status,
		Description: description,
	}

	logger.Info("AUDIT", record.toZapFields()...)

	// 同时也输出 JSON 格式便于聚合
	data, err := json.Marshal(record)
	if err == nil {
		logger.Info("AUDIT_JSON", zap.String("payload", string(data)))
	}
}

// auditMiddleware 返回一个包装函数，用于在 handler 中记录审计日志
func (s *Server) auditMiddleware(eventType AuditEventType, description string, detailFunc func(r *http.Request) map[string]string) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 调用原始 handler
			next(w, r)

			// 确定调用方身份
			actor := "anonymous"
			token := r.Header.Get("X-API-Token")
			if token != "" {
				if s.apiToken != "" && s.hasValidToken(token) {
					actor = "api_token"
				}
			}
			if isLocalRequest(r) {
				actor = "local"
			}

			details := map[string]string{}
			if detailFunc != nil {
				details = detailFunc(r)
			}

			logAudit(eventType, description, actor, r.Method, r.URL.Path, r.RemoteAddr, details, "completed")
		}
	}
}
