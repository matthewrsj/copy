package main

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func newLogger(path string, lvl logrus.Level) (*logrus.Logger, error) {
	logger := logrus.New()
	logger.SetLevel(lvl)
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, err
	}

	lf, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}

	logger.SetOutput(lf)

	return logger, nil
}
