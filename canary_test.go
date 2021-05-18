package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanaryCallback(t *testing.T) {
	registry := map[string]*FixtureInfo{
		"01-01": newTestFixtureInfoForFixture("01-01"),
		"01-02": newTestFixtureInfoForFixture("01-02"),
	}

	cbf := CanaryCallback(registry)

	cr, ok := cbf().(canaryResponse)
	assert.True(t, ok, "callback returns canaryResponse")
	assert.Len(t, cr.FixturesUp, 2)
	assert.Len(t, cr.FixturesDown, 0)
}
