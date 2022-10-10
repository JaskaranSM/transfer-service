package logging

import (
	"log"
	"os"

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
		zapcore.NewTee(zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), os.Stdout, zap.InfoLevel),
			zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), handleSync, zap.InfoLevel),
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
