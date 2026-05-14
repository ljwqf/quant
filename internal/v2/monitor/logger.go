package monitor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

type MissedSignalConfig struct {
	LargeMoveThresholdBps float64
	LogDir                string
	FlushInterval         time.Duration
}

func DefaultMissedSignalConfig() MissedSignalConfig {
	return MissedSignalConfig{
		LargeMoveThresholdBps: 200,
		LogDir:                "logs/missed_signals",
		FlushInterval:         5 * time.Second,
	}
}

type MissedSignalLogger struct {
	config  MissedSignalConfig
	mu      sync.Mutex
	buffer  []MissedSignalRecord
	file    *os.File
	encoder *json.Encoder
}

type MissedSignalRecord struct {
	Timestamp      time.Time             `json:"timestamp"`
	Symbol         string                `json:"symbol"`
	ReturnBps      float64               `json:"return_bps"`
	CurrentState   events.StrategyState  `json:"current_state"`
	Snapshot       events.FactorSnapshot `json:"factor_snapshot"`
	Diagnosis      string                `json:"diagnosis"`
	NoSignalReason string                `json:"no_signal_reason"`
}

func NewMissedSignalLogger(config MissedSignalConfig) (*MissedSignalLogger, error) {
	if config.LargeMoveThresholdBps == 0 {
		config.LargeMoveThresholdBps = DefaultMissedSignalConfig().LargeMoveThresholdBps
	}
	if config.LogDir == "" {
		config.LogDir = DefaultMissedSignalConfig().LogDir
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = DefaultMissedSignalConfig().FlushInterval
	}

	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("missed_signal_log_dir: %w", err)
	}

	filePath := filepath.Join(config.LogDir, "missed_signals.json")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("missed_signal_log_file: %w", err)
	}

	return &MissedSignalLogger{
		config:  config,
		buffer:  make([]MissedSignalRecord, 0),
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

func (l *MissedSignalLogger) Check(returnBps float64, symbol string, state events.StrategyState, snapshot events.FactorSnapshot, noSignalReason string) {
	if math.Abs(returnBps) < l.config.LargeMoveThresholdBps {
		return
	}

	diagnosis := diagnoseMiss(state, snapshot, noSignalReason)

	record := MissedSignalRecord{
		Timestamp:      snapshot.Timestamp,
		Symbol:         symbol,
		ReturnBps:      returnBps,
		CurrentState:   state,
		Snapshot:       snapshot,
		Diagnosis:      diagnosis,
		NoSignalReason: noSignalReason,
	}

	l.mu.Lock()
	l.buffer = append(l.buffer, record)
	l.mu.Unlock()
}

func (l *MissedSignalLogger) Flush() error {
	l.mu.Lock()
	records := l.buffer
	l.buffer = make([]MissedSignalRecord, 0)
	l.mu.Unlock()

	for _, record := range records {
		if err := l.encoder.Encode(record); err != nil {
			logger.Warn("missed_signal_log_write_failed",
				zap.String("symbol", record.Symbol),
				zap.Error(err),
			)
			l.mu.Lock()
			l.buffer = append(l.buffer, record)
			l.mu.Unlock()
			return err
		}
	}
	return nil
}

func (l *MissedSignalLogger) Close() error {
	l.Flush()
	return l.file.Close()
}

func diagnoseMiss(state events.StrategyState, snapshot events.FactorSnapshot, noSignalReason string) string {
	if state == events.StrategyStateSweepingHunting {
		return "sweeping_hunting_in_progress"
	}
	if state == events.StrategyStateCooldown {
		return "cooldown_active"
	}
	if !snapshot.SpreadStable {
		return "spread_unstable"
	}
	if snapshot.TopAskWall.Collapse || snapshot.TopBidWall.Collapse {
		return "wall_collapse_without_confirmed_imbalance"
	}
	if noSignalReason != "" {
		return noSignalReason
	}
	return "threshold_too_strict_or_factor_missing"
}

type SnapshotArchiverConfig struct {
	ArchiveDir string
}

func DefaultSnapshotArchiverConfig() SnapshotArchiverConfig {
	return SnapshotArchiverConfig{
		ArchiveDir: "logs/snapshots",
	}
}

type SnapshotArchiver struct {
	config SnapshotArchiverConfig
	mu     sync.Mutex
}

type SnapshotArchive struct {
	Timestamp   time.Time             `json:"timestamp"`
	Symbol      string                `json:"symbol"`
	EventType   string                `json:"event_type"`
	StateBefore events.StrategyState  `json:"state_before"`
	StateAfter  events.StrategyState  `json:"state_after"`
	Signal      events.SignalIntent   `json:"signal,omitempty"`
	Snapshot    events.FactorSnapshot `json:"factor_snapshot"`
}

func NewSnapshotArchiver(config SnapshotArchiverConfig) (*SnapshotArchiver, error) {
	if config.ArchiveDir == "" {
		config.ArchiveDir = DefaultSnapshotArchiverConfig().ArchiveDir
	}
	if err := os.MkdirAll(config.ArchiveDir, 0755); err != nil {
		return nil, fmt.Errorf("snapshot_archive_dir: %w", err)
	}
	return &SnapshotArchiver{config: config}, nil
}

func (a *SnapshotArchiver) Archive(archive SnapshotArchive) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	dateDir := filepath.Join(a.config.ArchiveDir, archive.Timestamp.Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s_%s_%d.json",
		archive.Symbol,
		archive.EventType,
		archive.Timestamp.UnixNano(),
	)
	filePath := filepath.Join(dateDir, fileName)

	data, err := json.MarshalIndent(archive, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}
