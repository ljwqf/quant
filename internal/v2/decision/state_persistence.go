package decision

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
)

type StatePersistenceConfig struct {
	StateDir string
}

func DefaultStatePersistenceConfig() StatePersistenceConfig {
	return StatePersistenceConfig{StateDir: "runtime/v2_state"}
}

type StatePersistence struct {
	config StatePersistenceConfig
	mu     sync.Mutex
}

type PersistedState struct {
	Symbol       string                `json:"symbol"`
	State        events.StrategyState  `json:"state"`
	LastChange   time.Time             `json:"last_change"`
	KeyLevels    map[string][]float64  `json:"key_levels,omitempty"`
	LastSnapshot events.FactorSnapshot `json:"last_snapshot,omitempty"`
}

func NewStatePersistence(config StatePersistenceConfig) (*StatePersistence, error) {
	if config.StateDir == "" {
		config.StateDir = DefaultStatePersistenceConfig().StateDir
	}
	if err := os.MkdirAll(config.StateDir, 0755); err != nil {
		return nil, err
	}
	return &StatePersistence{config: config}, nil
}

func (p *StatePersistence) Save(symbol string, state events.StrategyState, lastChange time.Time, snapshot events.FactorSnapshot) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	record := PersistedState{
		Symbol:       symbol,
		State:        state,
		LastChange:   lastChange,
		LastSnapshot: snapshot,
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(p.config.StateDir, symbol+"_state.json")
	return os.WriteFile(filePath, data, 0644)
}

func (p *StatePersistence) Load(symbol string) (*PersistedState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	filePath := filepath.Join(p.config.StateDir, symbol+"_state.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var record PersistedState
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (p *StatePersistence) LoadAll() (map[string]*PersistedState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entries, err := os.ReadDir(p.config.StateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	result := make(map[string]*PersistedState)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(p.config.StateDir, entry.Name()))
		if err != nil {
			continue
		}

		var record PersistedState
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		result[record.Symbol] = &record
	}
	return result, nil
}

func (p *StatePersistence) RestoreState(symbol string, machine *LiquidityStateMachine) error {
	record, err := p.Load(symbol)
	if err != nil {
		return err
	}
	if record == nil {
		return nil
	}

	machine.SetState(symbol, record.State, record.LastChange)
	return nil
}
