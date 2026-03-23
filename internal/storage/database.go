package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

// Database 数据库管理器
type Database struct {
	db *sql.DB
}

// NewDatabase 创建数据库管理器
func NewDatabase(cfg *config.DatabaseConfig) *Database {
	if !cfg.Enable {
		logger.Info("数据库功能未启用")
		return nil
	}

	if cfg.Type != "sqlite" {
		logger.Error("不支持的数据库类型", zap.String("type", cfg.Type))
		return nil
	}

	dbPath := cfg.Path
	if dbPath == "" {
		dbPath = "./data/quant.db"
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("创建数据库目录失败", zap.Error(err))
		return nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		logger.Error("打开数据库失败", zap.Error(err))
		return nil
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		logger.Error("数据库连接失败", zap.Error(err))
		return nil
	}

	d := &Database{db: db}
	logger.Info("数据库初始化成功", zap.String("path", dbPath))
	return d
}

// DB 获取原始数据库连接
func (d *Database) DB() *sql.DB {
	return d.db
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	if d.db != nil {
		logger.Info("关闭数据库连接")
		return d.db.Close()
	}
	return nil
}

// Migrate 执行数据库迁移
func (d *Database) Migrate() error {
	logger.Info("开始执行数据库迁移")

	for _, migration := range migrations {
		if err := d.executeMigration(migration); err != nil {
			return err
		}
	}

	logger.Info("数据库迁移完成")
	return nil
}

func (d *Database) executeMigration(m migration) error {
	logger.Debug("执行迁移", zap.String("version", m.version), zap.String("name", m.name))

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	if _, err := tx.Exec(m.up); err != nil {
		tx.Rollback()
		return fmt.Errorf("执行迁移 %s 失败: %w", m.version, err)
	}

	if _, err := tx.Exec("INSERT OR IGNORE INTO schema_migrations (version, name, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)", m.version, m.name); err != nil {
		tx.Rollback()
		return fmt.Errorf("记录迁移版本失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}
