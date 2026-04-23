package internal

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	log *zap.Logger
}

func NewLogger() *Logger {
	return NewLoggerWithConfig(LoggingConfig{})
}

func NewLoggerWithConfig(cfg LoggingConfig, fields ...zap.Field) *Logger {
	cfg = cfg.WithDefaults()

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "msg"
	encoderConfig.CallerKey = "caller"

	syncers := []zapcore.WriteSyncer{zapcore.AddSync(os.Stdout)}
	if cfg.FileEnabled {
		if err := os.MkdirAll(cfg.FileDir, 0o755); err != nil {
			panic(err)
		}
		syncers = append(syncers, &dailyFileSyncer{logDir: cfg.FileDir})
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(syncers...),
		zap.NewAtomicLevelAt(parseLogLevel(cfg.Level)),
	)

	logger := zap.New(core, zap.AddCaller())
	if len(fields) > 0 {
		logger = logger.With(fields...)
	}

	return &Logger{log: logger}
}

func (l *Logger) GetLogger() *zap.Logger {
	return l.log
}

func parseLogLevel(value string) zapcore.Level {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(strings.TrimSpace(value)))); err != nil {
		return zapcore.InfoLevel
	}

	return level
}

type dailyFileSyncer struct {
	logDir      string
	mu          sync.Mutex
	file        *os.File
	currentDate string
}

func (d *dailyFileSyncer) Write(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	if d.file == nil || d.currentDate != today {
		if d.file != nil {
			_ = d.file.Close()
		}

		path := filepath.Join(d.logDir, today+".log")
		d.file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			d.file = nil
			d.currentDate = ""
			return 0, err
		}
		d.currentDate = today
	}

	return d.file.Write(p)
}

func (d *dailyFileSyncer) Sync() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file == nil {
		return nil
	}
	return d.file.Sync()
}
