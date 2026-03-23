package logger

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	DefaultLogSampleRate = 0.1
	DefaultMaxSize       = 100
	DefaultMaxBackups    = 10
	DefaultMaxAge        = 30
	DefaultCompress      = true
)

type LogConfig struct {
	Level      string
	OutputPath string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

var log *zap.Logger

type LogSampler struct {
	sampleRate float64
	count      int64
	mutex      sync.Mutex
}

var (
	logSamplerInstance *LogSampler
	logSamplerOnce     sync.Once
)

func GetLogSampler() *LogSampler {
	logSamplerOnce.Do(func() {
		logSamplerInstance = &LogSampler{
			sampleRate: DefaultLogSampleRate,
		}
	})
	return logSamplerInstance
}

func (ls *LogSampler) ShouldSample() bool {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	ls.count++
	if ls.sampleRate >= 1.0 {
		return true
	}
	return ls.count%int64(1.0/ls.sampleRate) == 0
}

func (ls *LogSampler) SetSampleRate(rate float64) {
	if rate < 0 || rate > 1.0 {
		rate = DefaultLogSampleRate
	}
	ls.mutex.Lock()
	defer ls.mutex.Unlock()
	ls.sampleRate = rate
}

func Init(logLevel string, outputPath string) error {
	return InitWithConfig(&LogConfig{
		Level:      logLevel,
		OutputPath: outputPath,
		MaxSize:    DefaultMaxSize,
		MaxBackups: DefaultMaxBackups,
		MaxAge:     DefaultMaxAge,
		Compress:   DefaultCompress,
	})
}

func InitWithConfig(cfg *LogConfig) error {
	if cfg.OutputPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0755); err != nil {
			return err
		}
	}

	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var core zapcore.Core
	if cfg.OutputPath != "" {
		maxSize := cfg.MaxSize
		if maxSize <= 0 {
			maxSize = DefaultMaxSize
		}
		maxBackups := cfg.MaxBackups
		if maxBackups <= 0 {
			maxBackups = DefaultMaxBackups
		}
		maxAge := cfg.MaxAge
		if maxAge <= 0 {
			maxAge = DefaultMaxAge
		}

		lumberJackLogger := &lumberjack.Logger{
			Filename:   cfg.OutputPath,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   cfg.Compress,
			LocalTime:  true,
		}

		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(lumberJackLogger),
			level,
		)

		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)

		core = zapcore.NewTee(fileCore, consoleCore)
	} else {
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)
	}

	log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return nil
}

// GetLogger 获取日志记录器
func GetLogger() *zap.Logger {
	if log == nil {
		// 默认初始化
		_ = Init("info", "")
	}
	return log
}

// Debug 记录调试级别日志
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Info 记录信息级别日志
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Warn 记录警告级别日志
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Error 记录错误级别日志
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Fatal 记录致命级别日志并退出
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// With 添加上下文字段
func With(fields ...zap.Field) *zap.Logger {
	return GetLogger().With(fields...)
}

// WithContext 添加上下文信息
func WithContext(key string, value interface{}) *zap.Logger {
	return GetLogger().With(zap.Any(key, value))
}

// WithError 添加错误信息
func WithError(err error) *zap.Logger {
	return GetLogger().With(zap.Error(err))
}

// WithTime 添加时间信息
func WithTime(t time.Time) *zap.Logger {
	return GetLogger().With(zap.Time("time", t))
}

// WithString 添加字符串字段
func WithString(key string, value string) *zap.Logger {
	return GetLogger().With(zap.String(key, value))
}

// WithInt 添加整数字段
func WithInt(key string, value int) *zap.Logger {
	return GetLogger().With(zap.Int(key, value))
}

// WithFloat 添加浮点数字段
func WithFloat(key string, value float64) *zap.Logger {
	return GetLogger().With(zap.Float64(key, value))
}

// WithBool 添加布尔字段
func WithBool(key string, value bool) *zap.Logger {
	return GetLogger().With(zap.Bool(key, value))
}

// InfoWithSampling 带采样的Info日志
func InfoWithSampling(msg string, fields ...zap.Field) {
	if GetLogSampler().ShouldSample() {
		GetLogger().Info(msg, fields...)
	}
}

// DebugWithSampling 带采样的Debug日志
func DebugWithSampling(msg string, fields ...zap.Field) {
	if GetLogSampler().ShouldSample() {
		GetLogger().Debug(msg, fields...)
	}
}
