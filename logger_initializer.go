package kgsotel

import (
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/log/global"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func initLogger(serviceName string) *zap.Logger {
	// Create a new logger
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewConsoleEncoder(getConsoleConfig()), zapcore.AddSync(os.Stdout), zapcore.DebugLevel),
		otelzap.NewCore(serviceName, otelzap.WithLoggerProvider(global.GetLoggerProvider())),
	)
	logger := zap.New(core)

	// Replace the global logger
	zap.ReplaceGlobals(logger)

	return logger
}

func getConsoleConfig() zapcore.EncoderConfig {
	// Custom encoder configuration
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    customLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   customCallerEncoder,
	}
	return encoderConfig
}

// Custom log level encoder
func customLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var levelColor string
	switch l {
	case zapcore.DebugLevel:
		levelColor = "\x1b[36m" // Cyan
	case zapcore.InfoLevel:
		levelColor = "\x1b[32m" // Green
	case zapcore.WarnLevel:
		levelColor = "\x1b[33m" // Yellow
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		levelColor = "\x1b[31m" // Red
	default:
		levelColor = "\x1b[0m" // Default
	}
	enc.AppendString(fmt.Sprintf("%s%s\x1b[0m", levelColor, l.CapitalString()))
}

// Custom log time encoder
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	timeColor := "\x1b[36m" // Cyan for timestamp
	timeStr := t.Format("2006-01-02 15:04:05.000")
	enc.AppendString(fmt.Sprintf("%s%s\x1b[0m", timeColor, timeStr))
}

// Custom log caller encoder
func customCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	callerColor := "\x1b[35m" // Magenta for caller
	callerStr := caller.TrimmedPath()
	enc.AppendString(fmt.Sprintf("%s%s\x1b[0m", callerColor, callerStr))
}
