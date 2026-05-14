package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMissedSignalLoggerRecordsLargeMove(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missed")
	logger, err := NewMissedSignalLogger(MissedSignalConfig{LargeMoveThresholdBps: 200, LogDir: dir})
	require.NoError(t, err)

	snapshot := events.FactorSnapshot{Symbol: "BTC-USDT", Timestamp: time.Now(), MidPrice: 100, SpreadStable: true}
	logger.Check(300, "BTC-USDT", events.StrategyStateIdle, snapshot, "")

	require.NoError(t, logger.Flush())

	data, err := os.ReadFile(filepath.Join(dir, "missed_signals.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "BTC-USDT")
	assert.Contains(t, string(data), "300")

	require.NoError(t, logger.Close())
}

func TestMissedSignalLoggerIgnoresSmallMove(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missed")
	logger, err := NewMissedSignalLogger(MissedSignalConfig{LargeMoveThresholdBps: 200, LogDir: dir})
	require.NoError(t, err)

	snapshot := events.FactorSnapshot{Symbol: "BTC-USDT", Timestamp: time.Now()}
	logger.Check(50, "BTC-USDT", events.StrategyStateIdle, snapshot, "")

	require.NoError(t, logger.Flush())

	data, err := os.ReadFile(filepath.Join(dir, "missed_signals.json"))
	require.NoError(t, err)
	assert.Empty(t, string(data))

	require.NoError(t, logger.Close())
}

func TestSnapshotArchiverWritesArchive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "snapshots")
	archiver, err := NewSnapshotArchiver(SnapshotArchiverConfig{ArchiveDir: dir})
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, archiver.Archive(SnapshotArchive{
		Timestamp:   now,
		Symbol:      "BTC-USDT",
		EventType:   "state_transition",
		StateBefore: events.StrategyStateIdle,
		StateAfter:  events.StrategyStateSweepingHunting,
		Snapshot:    events.FactorSnapshot{Symbol: "BTC-USDT", Timestamp: now, MidPrice: 100},
	}))

	dateDir := filepath.Join(dir, now.Format("2006-01-02"))
	entries, err := os.ReadDir(dateDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	data, err := os.ReadFile(filepath.Join(dateDir, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(data), "Sweeping_Hunting")
	assert.Contains(t, string(data), "Idle")
}
