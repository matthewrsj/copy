package towercontroller

import (
	"reflect"
	"strings"
	"testing"

	"bou.ke/monkey"
	"github.com/manifoldco/promptui"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cellapi"
)

func patchPrompt(msg string, val promptui.ValidateFunc) (string, error) {
	if strings.Contains(msg, "fixture") {
		return "SWIFT-01-A-01", nil
	}

	return "11223344A", nil
}

func TestScanBarcodes(t *testing.T) {
	p := monkey.Patch(prompt, patchPrompt)
	defer p.Unpatch()

	pc := monkey.Patch(promptConfirm, func(string) bool { return true })
	defer pc.Unpatch()

	gnps := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"GetNextProcessStep",
		func(*cellapi.Client, string) (string, error) {
			return "FORM_CYCLE", nil
		},
	)
	defer gnps.Unpatch()

	bcs, err := ScanBarcodes(cellapi.NewClient("baseurl"), false, zap.NewExample().Sugar())
	assert.Nil(t, err)
	assert.False(t, bcs.InProgress)
	assert.Equal(t, "FORM_CYCLE", bcs.ProcessStepName)
	assert.Equal(t, "SWIFT-01-A-01", bcs.Fixture.Raw)
	assert.Equal(t, "11223344A", bcs.Tray.Raw)
}
