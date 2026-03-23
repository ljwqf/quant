package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ljwqf/quant/pkg/persistence"
	"github.com/ljwqf/quant/pkg/types"
)

const executionStateKey = "execution_state"

type PersistedStrategyPosition struct {
	Strategy   string          `json:"strategy"`
	Symbol     string          `json:"symbol"`
	Side       types.OrderSide `json:"side"`
	Size       float64         `json:"size"`
	EntryPrice float64         `json:"entry_price"`
	MarkPrice  float64         `json:"mark_price"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type EngineStateSnapshot struct {
	SavedAt           time.Time                              `json:"saved_at"`
	StrategyPositions map[string][]PersistedStrategyPosition `json:"strategy_positions"`
	PendingOrders     []*types.Order                         `json:"pending_orders"`
	RiskPositions     []*types.Position                      `json:"risk_positions"`
	Metrics           map[string]interface{}                 `json:"metrics"`
}

type StateStore interface {
	Save(snapshot *EngineStateSnapshot) error
	Load(snapshot *EngineStateSnapshot) error
	Exists() bool
}

type FileStateStore struct {
	storage *persistence.FileStorage
	path    string
}

func NewFileStateStore(dir string) (*FileStateStore, error) {
	storage, err := persistence.NewFileStorage(dir)
	if err != nil {
		return nil, fmt.Errorf("create execution state storage: %w", err)
	}

	return &FileStateStore{
		storage: storage,
		path:    filepath.Join(dir, executionStateKey+".json"),
	}, nil
}

func (s *FileStateStore) Save(snapshot *EngineStateSnapshot) error {
	return s.storage.SaveSimple(executionStateKey, snapshot)
}

func (s *FileStateStore) Load(snapshot *EngineStateSnapshot) error {
	return s.storage.Load(executionStateKey, snapshot)
}

func (s *FileStateStore) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

func cloneMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
