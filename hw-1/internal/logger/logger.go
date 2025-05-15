package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New() *zap.Logger {
	encoderCfg := zap.NewDevelopmentEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderCfg)

	core := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stderr),
		zapcore.DebugLevel,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	zap.ReplaceGlobals(logger)
	log.SetOutput(zap.NewStdLog(logger).Writer())

	return logger
}
