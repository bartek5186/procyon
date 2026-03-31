package internal

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	log *logrus.Logger
}

func NewLogger() *Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.DebugLevel)

	if err := os.MkdirAll("log", 0o755); err != nil {
		logrus.WithError(err).Fatal("failed to create log directory")
	}

	filename := filepath.Join("log", time.Now().UTC().Format("2006-01-02")+".log")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		logrus.WithError(err).Fatal("failed to open log file")
	}

	log.SetOutput(io.MultiWriter(os.Stdout, file))

	return &Logger{log: log}
}

func (l *Logger) GetLogger() *logrus.Logger {
	return l.log
}
