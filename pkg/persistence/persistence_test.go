package persistence

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileStorage(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_persistence_"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	storage, err := NewFileStorage(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, storage)
	assert.DirExists(t, tempDir)
}

func TestSaveAndLoad(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_save_load_"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	storage, err := NewFileStorage(tempDir)
	require.NoError(t, err)

	type TestData struct {
		Name  string
		Value int
	}

	data := TestData{
		Name:  "test",
		Value: 42,
	}

	err = storage.SaveSimple("test_key", &data)
	require.NoError(t, err)

	loadedData := TestData{}
	err = storage.Load("test_key", &loadedData)
	require.NoError(t, err)
	assert.Equal(t, data.Name, loadedData.Name)
	assert.Equal(t, data.Value, loadedData.Value)
}

func TestExists(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_exists_"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	storage, err := NewFileStorage(tempDir)
	require.NoError(t, err)

	assert.False(t, storage.Exists("nonexistent_key"))

	data := map[string]interface{}{"test": "value"}
	err = storage.SaveSimple("existing_key", &data)
	require.NoError(t, err)

	assert.True(t, storage.Exists("existing_key"))
}

func TestDelete(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_delete_"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	storage, err := NewFileStorage(tempDir)
	require.NoError(t, err)

	data := map[string]interface{}{"test": "value"}
	err = storage.SaveSimple("key_to_delete", &data)
	require.NoError(t, err)

	assert.True(t, storage.Exists("key_to_delete"))

	err = storage.Delete("key_to_delete")
	require.NoError(t, err)

	assert.False(t, storage.Exists("key_to_delete"))
}

func TestTransactionRecord(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_transaction_"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	storage, err := NewFileStorage(tempDir)
	require.NoError(t, err)

	now := time.Now()
	record := &TransactionRecord{
		ID:          "tx123",
		Strategy:    "TestStrategy",
		Symbol:      "BTC-USDT",
		Side:        "buy",
		EntryPrice:  50000.0,
		ExitPrice:   51000.0,
		Quantity:    0.01,
		PNL:         100.0,
		PNLPercent:  0.02,
		EntryTime:   now,
		ExitTime:    now.Add(1 * time.Hour),
		HoldingTime: "1h",
		Status:      "completed",
		Metadata: map[string]interface{}{
			"note": "test transaction",
		},
	}

	err = storage.SaveTransaction(record)
	require.NoError(t, err)

	loadedRecord := &TransactionRecord{}
	err = storage.LoadLatest("transaction_tx123", loadedRecord)
	require.NoError(t, err)

	assert.Equal(t, record.ID, loadedRecord.ID)
	assert.Equal(t, record.Strategy, loadedRecord.Strategy)
	assert.Equal(t, record.PNL, loadedRecord.PNL)
}

func TestStrategyState(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_strategy_state_"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	storage, err := NewFileStorage(tempDir)
	require.NoError(t, err)

	state := &StrategyState{
		Name: "TestStrategy",
		Params: map[string]interface{}{
			"param1": "value1",
		},
		Metrics: map[string]interface{}{
			"win_rate": 0.6,
		},
		Position: &PositionState{
			Symbol:     "BTC-USDT",
			Side:       "long",
			EntryPrice: 50000.0,
			Size:       0.01,
			OpenTime:   time.Now(),
			TakeProfit: 51000.0,
			StopLoss:   49000.0,
		},
		LastUpdate: time.Now(),
	}

	err = storage.SaveSimple("strategy_TestStrategy", state)
	require.NoError(t, err)

	loadedState := &StrategyState{}
	err = storage.Load("strategy_TestStrategy", loadedState)
	require.NoError(t, err)

	assert.Equal(t, state.Name, loadedState.Name)
	assert.Equal(t, state.Metrics["win_rate"], loadedState.Metrics["win_rate"])
}
