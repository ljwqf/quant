package monitoring

import (
	"fmt"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// AlertCondition 告警条件
type AlertCondition struct {
	ID          string
	Name        string
	Description string
	Severity    AlertType
	Enabled     bool
	Check       func() (bool, string, error)
	LastTriggered time.Time
	Cooldown    time.Duration
}

// AlertRuleManager 告警规则管理器
type AlertRuleManager struct {
	conditions    map[string]*AlertCondition
	alertManager  *AlertManager
	metrics       *Metrics
	mutex         sync.RWMutex
	stopCh        chan struct{}
}

// NewAlertRuleManager 创建告警规则管理器
func NewAlertRuleManager(alertManager *AlertManager, metrics *Metrics) *AlertRuleManager {
	return &AlertRuleManager{
		conditions:   make(map[string]*AlertCondition),
		alertManager: alertManager,
		metrics:      metrics,
		stopCh:       make(chan struct{}),
	}
}

// Start 启动告警规则检查
func (arm *AlertRuleManager) Start() {
	arm.registerDefaultConditions()
	
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				arm.checkConditions()
			case <-arm.stopCh:
				return
			}
		}
	}()
}

// Stop 停止告警规则检查
func (arm *AlertRuleManager) Stop() {
	close(arm.stopCh)
}

// registerDefaultConditions 注册默认告警条件
func (arm *AlertRuleManager) registerDefaultConditions() {
	// 系统资源告警
	arm.RegisterCondition(&AlertCondition{
		ID:          "high_cpu_usage",
		Name:        "CPU使用率过高",
		Description: "CPU使用率超过80%",
		Severity:    AlertTypeWarning,
		Enabled:     true,
		Cooldown:    5 * time.Minute,
		Check: func() (bool, string, error) {
			cpu := arm.metrics.GetSystemMetrics().GetCPUPercent()
			if cpu > 80.0 {
				return true, fmt.Sprintf("CPU使用率: %.1f%%", cpu), nil
			}
			return false, "", nil
		},
	})
	
	arm.RegisterCondition(&AlertCondition{
		ID:          "high_memory_usage",
		Name:        "内存使用率过高",
		Description: "内存使用率超过85%",
		Severity:    AlertTypeWarning,
		Enabled:     true,
		Cooldown:    5 * time.Minute,
		Check: func() (bool, string, error) {
			mem := arm.metrics.GetSystemMetrics().GetMemoryPercent()
			if mem > 85.0 {
				return true, fmt.Sprintf("内存使用率: %.1f%%", mem), nil
			}
			return false, "", nil
		},
	})
	
	arm.RegisterCondition(&AlertCondition{
		ID:          "high_disk_usage",
		Name:        "磁盘使用率过高",
		Description: "磁盘使用率超过90%",
		Severity:    AlertTypeCritical,
		Enabled:     true,
		Cooldown:    10 * time.Minute,
		Check: func() (bool, string, error) {
			disk := arm.metrics.GetSystemMetrics().GetDiskPercent()
			if disk > 90.0 {
				return true, fmt.Sprintf("磁盘使用率: %.1f%%", disk), nil
			}
			return false, "", nil
		},
	})
	
	// API性能告警
	arm.RegisterCondition(&AlertCondition{
		ID:          "high_api_error_rate",
		Name:        "API错误率过高",
		Description: "API错误率超过5%",
		Severity:    AlertTypeWarning,
		Enabled:     true,
		Cooldown:    2 * time.Minute,
		Check: func() (bool, string, error) {
			stats := arm.metrics.GetAPIMetrics().GetAllStats()
			errorRate, _ := stats["error_rate"].(float64)
			if errorRate > 0.05 {
				return true, fmt.Sprintf("API错误率: %.2f%%", errorRate*100), nil
			}
			return false, "", nil
		},
	})
	
	// 交易性能告警
	arm.RegisterCondition(&AlertCondition{
		ID:          "low_fill_rate",
		Name:        "订单成交率过低",
		Description: "订单成交率低于70%",
		Severity:    AlertTypeWarning,
		Enabled:     true,
		Cooldown:    5 * time.Minute,
		Check: func() (bool, string, error) {
			stats := arm.metrics.GetTradingMetrics().GetStats()
			fillRate, _ := stats["fill_rate"].(float64)
			if fillRate < 0.7 {
				return true, fmt.Sprintf("订单成交率: %.2f%%", fillRate*100), nil
			}
			return false, "", nil
		},
	})
	
	// 策略性能告警
	arm.RegisterCondition(&AlertCondition{
		ID:          "strategy_loss",
		Name:        "策略亏损",
		Description: "策略总亏损超过阈值",
		Severity:    AlertTypeError,
		Enabled:     true,
		Cooldown:    10 * time.Minute,
		Check: func() (bool, string, error) {
			stats := arm.metrics.GetStrategyMetrics().GetAllStats()
			totalPnL, _ := stats["total_pnl"].(float64)
			if totalPnL < -1000 {
				return true, fmt.Sprintf("策略总亏损: %.2f", totalPnL), nil
			}
			return false, "", nil
		},
	})
}

// RegisterCondition 注册告警条件
func (arm *AlertRuleManager) RegisterCondition(condition *AlertCondition) {
	arm.mutex.Lock()
	defer arm.mutex.Unlock()
	
	arm.conditions[condition.ID] = condition
	logger.Info("注册告警规则",
		zap.String("id", condition.ID),
		zap.String("name", condition.Name))
}

// UnregisterCondition 注销告警条件
func (arm *AlertRuleManager) UnregisterCondition(id string) {
	arm.mutex.Lock()
	defer arm.mutex.Unlock()
	
	delete(arm.conditions, id)
}

// EnableCondition 启用告警条件
func (arm *AlertRuleManager) EnableCondition(id string) error {
	arm.mutex.Lock()
	defer arm.mutex.Unlock()
	
	condition, exists := arm.conditions[id]
	if !exists {
		return fmt.Errorf("告警条件不存在: %s", id)
	}
	
	condition.Enabled = true
	return nil
}

// DisableCondition 禁用告警条件
func (arm *AlertRuleManager) DisableCondition(id string) error {
	arm.mutex.Lock()
	defer arm.mutex.Unlock()
	
	condition, exists := arm.conditions[id]
	if !exists {
		return fmt.Errorf("告警条件不存在: %s", id)
	}
	
	condition.Enabled = false
	return nil
}

// GetConditions 获取所有告警条件
func (arm *AlertRuleManager) GetConditions() []*AlertCondition {
	arm.mutex.RLock()
	defer arm.mutex.RUnlock()
	
	conditions := make([]*AlertCondition, 0, len(arm.conditions))
	for _, cond := range arm.conditions {
		conditions = append(conditions, cond)
	}
	return conditions
}

// checkConditions 检查所有告警条件
func (arm *AlertRuleManager) checkConditions() {
	arm.mutex.RLock()
	conditions := make([]*AlertCondition, 0, len(arm.conditions))
	for _, cond := range arm.conditions {
		if cond.Enabled {
			conditions = append(conditions, cond)
		}
	}
	arm.mutex.RUnlock()
	
	for _, cond := range conditions {
		arm.checkCondition(cond)
	}
}

// checkCondition 检查单个告警条件
func (arm *AlertRuleManager) checkCondition(condition *AlertCondition) {
	arm.mutex.Lock()
	defer arm.mutex.Unlock()
	
	if !condition.Enabled {
		return
	}
	
	if time.Since(condition.LastTriggered) < condition.Cooldown {
		return
	}
	
	triggered, message, err := condition.Check()
	if err != nil {
		logger.Warn("告警条件检查失败",
			zap.String("id", condition.ID),
			zap.Error(err))
		return
	}
	
	if triggered {
		condition.LastTriggered = time.Now()
		
		logger.Info("触发告警规则",
			zap.String("id", condition.ID),
			zap.String("name", condition.Name),
			zap.String("message", message))
		
		if err := arm.alertManager.AlertWithContext(
			condition.Severity,
			condition.Name,
			message,
			map[string]string{
				"rule_id":   condition.ID,
				"rule_name": condition.Name,
			},
			map[string]interface{}{
				"cooldown": condition.Cooldown.String(),
			},
		); err != nil {
			logger.Warn("发送告警失败",
				zap.String("id", condition.ID),
				zap.Error(err))
		}
	}
}

// TriggerCondition 手动触发告警条件
func (arm *AlertRuleManager) TriggerCondition(id string) error {
	arm.mutex.Lock()
	condition, exists := arm.conditions[id]
	arm.mutex.Unlock()
	
	if !exists {
		return fmt.Errorf("告警条件不存在: %s", id)
	}
	
	triggered, message, err := condition.Check()
	if err != nil {
		return err
	}
	
	if triggered {
		arm.mutex.Lock()
		condition.LastTriggered = time.Now()
		arm.mutex.Unlock()
		
		return arm.alertManager.AlertWithContext(
			condition.Severity,
			condition.Name,
			message,
			map[string]string{
				"rule_id":   id,
				"rule_name": condition.Name,
			},
			nil,
		)
	}
	
	return fmt.Errorf("告警条件未触发")
}
