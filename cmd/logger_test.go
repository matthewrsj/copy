package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func Test_newLogger(t *testing.T) {
	lcfg := newLogger("test", zapcore.InfoLevel)
	assert.Equal(t, zapcore.InfoLevel, lcfg.Level.Level())
}
