package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newLogger(lFile string, lvl zapcore.Level) zap.Config {
	lcfg := zap.NewProductionConfig()
	lcfg.Encoding = "json"
	lcfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	lcfg.DisableStacktrace = false
	lcfg.DisableCaller = true
	lcfg.Sampling = nil
	lcfg.OutputPaths = []string{
		"stdout",
		lFile,
	}

	lcfg.Level.SetLevel(lvl)

	return lcfg
}
