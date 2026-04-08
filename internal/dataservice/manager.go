package dataservice

import (
	"sync"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// SourceManager 数据源管理器
type SourceManager struct {
	sources map[string]DataSource
	mutex   sync.RWMutex
}

// NewSourceManager 创建数据源管理器
func NewSourceManager() *SourceManager {
	return &SourceManager{
		sources: make(map[string]DataSource),
	}
}

// RegisterSource 注册数据源
func (m *SourceManager) RegisterSource(source DataSource) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	name := source.Name()
	if _, exists := m.sources[name]; exists {
		return ErrSourceAlreadyExists
	}

	m.sources[name] = source
	logger.Info("数据源已注册", zap.String("name", name), zap.String("type", string(source.Type())))
	return nil
}

// UnregisterSource 注销数据源
func (m *SourceManager) UnregisterSource(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	source, exists := m.sources[name]
	if !exists {
		return ErrSourceNotFound
	}

	if err := source.Close(); err != nil {
		logger.Warn("关闭数据源失败", zap.String("name", name), zap.Error(err))
	}

	delete(m.sources, name)
	logger.Info("数据源已注销", zap.String("name", name))
	return nil
}

// GetSource 获取数据源
func (m *SourceManager) GetSource(name string) (DataSource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	source, exists := m.sources[name]
	if !exists {
		return nil, ErrSourceNotFound
	}
	return source, nil
}

// GetSourcesByType 根据类型获取数据源
func (m *SourceManager) GetSourcesByType(sourceType DataSourceType) []DataSource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var sources []DataSource
	for _, source := range m.sources {
		if source.Type() == sourceType {
			sources = append(sources, source)
		}
	}
	return sources
}

// GetAllSources 获取所有数据源
func (m *SourceManager) GetAllSources() []DataSource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var sources []DataSource
	for _, source := range m.sources {
		sources = append(sources, source)
	}
	return sources
}

// GetHealthySources 获取健康的数据源
func (m *SourceManager) GetHealthySources() []DataSource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var sources []DataSource
	for _, source := range m.sources {
		if source.IsHealthy() {
			sources = append(sources, source)
		}
	}
	return sources
}

// CloseAll 关闭所有数据源
func (m *SourceManager) CloseAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for name, source := range m.sources {
		if err := source.Close(); err != nil {
			logger.Warn("关闭数据源失败", zap.String("name", name), zap.Error(err))
		}
		delete(m.sources, name)
	}
	logger.Info("所有数据源已关闭")
}
