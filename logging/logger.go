package logging

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jaskaranSM/transfer-service/utils"
)

var Logger *zap.Logger

const EnvLocal = "local"

func getLoggerObject() *zap.Logger {
	err := os.Remove(utils.LogFile)
	if err != nil {
		log.Println("Cannot remove logfile", zap.Error(err),
			zap.String("LogFile", utils.LogFile),
		)
	}

	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder

	handleSync, _, err := zap.Open(utils.LogFile)
	if err != nil {
		log.Println("Cannot open log file for zap :", zap.Error(err),
			zap.String("LogFile", utils.LogFile),
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
