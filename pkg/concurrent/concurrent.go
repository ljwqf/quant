package concurrent

import (
	"sync"
	"time"
)

// SymbolLock 标的锁，用于防止多个策略同时交易同一标的
type SymbolLock struct {
	symbolLocks map[string]*symbolLockEntry
	globalMutex sync.RWMutex
}

type symbolLockEntry struct {
	mutex       sync.Mutex
	holder      string
	acquiredAt  time.Time
	lockCount   int
}

var (
	instance *SymbolLock
	once     sync.Once
)

// GetSymbolLock 获取标的锁单例
func GetSymbolLock() *SymbolLock {
	once.Do(func() {
		instance = &SymbolLock{
			symbolLocks: make(map[string]*symbolLockEntry),
		}
	})
	return instance
}

// TryLock 尝试获取标的锁，非阻塞
func (sl *SymbolLock) TryLock(symbol string, holder string) bool {
	sl.globalMutex.Lock()
	entry, exists := sl.symbolLocks[symbol]
	if !exists {
		entry = &symbolLockEntry{}
		sl.symbolLocks[symbol] = entry
	}
	sl.globalMutex.Unlock()

	if entry.mutex.TryLock() {
		entry.holder = holder
		entry.acquiredAt = time.Now()
		entry.lockCount++
		return true
	}
	return false
}

// Lock 获取标的锁，阻塞
func (sl *SymbolLock) Lock(symbol string, holder string) {
	sl.globalMutex.Lock()
	entry, exists := sl.symbolLocks[symbol]
	if !exists {
		entry = &symbolLockEntry{}
		sl.symbolLocks[symbol] = entry
	}
	sl.globalMutex.Unlock()

	entry.mutex.Lock()
	entry.holder = holder
	entry.acquiredAt = time.Now()
	entry.lockCount++
}

// Unlock 释放标的锁
func (sl *SymbolLock) Unlock(symbol string) {
	sl.globalMutex.RLock()
	entry, exists := sl.symbolLocks[symbol]
	sl.globalMutex.RUnlock()

	if exists {
		entry.mutex.Unlock()
	}
}

// IsLocked 检查标的是否被锁定
func (sl *SymbolLock) IsLocked(symbol string) bool {
	sl.globalMutex.RLock()
	entry, exists := sl.symbolLocks[symbol]
	sl.globalMutex.RUnlock()

	if !exists {
		return false
	}

	if entry.mutex.TryLock() {
		entry.mutex.Unlock()
		return false
	}
	return true
}

// GetLockHolder 获取锁的持有者
func (sl *SymbolLock) GetLockHolder(symbol string) (string, bool) {
	sl.globalMutex.RLock()
	entry, exists := sl.symbolLocks[symbol]
	sl.globalMutex.RUnlock()

	if !exists {
		return "", false
	}
	return entry.holder, entry.holder != ""
}

// GetLockInfo 获取锁的信息
type LockInfo struct {
	Symbol     string
	Holder     string
	AcquiredAt time.Time
	LockCount  int
	IsLocked   bool
}

// GetAllLockInfo 获取所有锁的信息
func (sl *SymbolLock) GetAllLockInfo() []LockInfo {
	sl.globalMutex.RLock()
	defer sl.globalMutex.RUnlock()

	result := make([]LockInfo, 0, len(sl.symbolLocks))
	for symbol, entry := range sl.symbolLocks {
		isLocked := false
		if entry.mutex.TryLock() {
			entry.mutex.Unlock()
		} else {
			isLocked = true
		}
		result = append(result, LockInfo{
			Symbol:     symbol,
			Holder:     entry.holder,
			AcquiredAt: entry.acquiredAt,
			LockCount:  entry.lockCount,
			IsLocked:   isLocked,
		})
	}
	return result
}
