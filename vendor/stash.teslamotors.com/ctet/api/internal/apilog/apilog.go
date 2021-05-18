// Package apilog provides log resources for the api packages
package apilog

import (
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log logs the message if the logger is not nil
func Log(logger *zap.Logger, level zapcore.Level, msg string, fields ...zap.Field) {
	if logger == nil {
		return
	}

	var lFunc func(string, ...zap.Field)

	switch level {
	case zap.DebugLevel:
		lFunc = logger.Debug
	case zap.InfoLevel:
		lFunc = logger.Info
	case zap.WarnLevel:
		lFunc = logger.Warn
	case zap.ErrorLevel:
		lFunc = logger.Error
	case zap.FatalLevel:
		lFunc = logger.Fatal
	default:
		lFunc = logger.Info
	}

	lFunc(msg, fields...)
}

// HTTPError writes an error to the ResponseWriter and logs the error via zap as well
func HTTPError(logger *zap.Logger, w http.ResponseWriter, status int, msg string, fields ...zap.Field) {
	http.Error(w, msg, status)
	Log(logger, zap.ErrorLevel, msg, fields...)
}
