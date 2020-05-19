package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// nolint:scopelint // common in tests
func Test_newCellMask(t *testing.T) {
	testCases := []struct {
		cps []bool
		exp []uint32
	}{
		{
			cps: []bool{
				true, true, false, true, true, false, false, false, false, false, false, false, false, false, false, false,
				false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false,
				false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false,
				false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true,
			},
			exp: []uint32{0x1b, 0x80000000},
		},
		{
			cps: []bool{},
			exp: []uint32{},
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			act := newCellMask(tc.cps)
			if !assert.Equal(t, len(tc.exp), len(act)) {
				return
			}

			for i, a := range act {
				assert.Equal(t, tc.exp[i], a)
			}
		})
	}
}
