package logging

import (
	"log"
	"os"

	"github.com/jaskaranSM/transfer-service/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

const EnvLocal = "local"
const logFile string = "log.txt"

func getLoggerObject() *zap.Logger {
	err := os.Remove(logFile)
	if err != nil {
		log.Println("Cannot remove logfile", zap.Error(err),
			zap.String("logFile", logFile),
		)
	}
	var level zapcore.Level = 0
	if config.Get().LogLevel == "debug" {
		level = zapcore.DebugLevel
	} else if config.Get().LogLevel == "error" {
		level = zapcore.ErrorLevel
	} else {
		level = zapcore.InfoLevel
	}

	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder

	handleSync, _, err := zap.Open(logFile)
	if err != nil {
		log.Println("Cannot open log file for zap :", zap.Error(err),
			zap.String("logFile", logFile),
		)
	}

	logger := zap.New(
		zapcore.NewTee(zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), os.Stdout, level),
			zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), handleSync, level),
		),
		zap.AddCaller(),
	)

	defer func(logger *zap.Logger) {
		err = logger.Sync()
		if err != nil {
			logger.Info("Error while syncing logger", zap.Error(err))
		}
	}(logger) // flushes buffer, if any

	return logger
}

func GetLogger() *zap.Logger {
	if Logger == nil {
		Logger = getLoggerObject()
	}

	return Logger
}
