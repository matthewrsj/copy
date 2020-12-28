package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_getCellMapMocked(t *testing.T) {
	cm, err := getCellMap(true, zap.NewExample().Sugar(), nil, "", "")
	assert.Nil(t, err)
	assert.Equal(t, 4, len(cm))
}
