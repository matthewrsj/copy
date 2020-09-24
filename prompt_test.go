package towercontroller

import (
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/manifoldco/promptui"
	"github.com/stretchr/testify/assert"
	"stash.teslamotors.com/rr/cdcontroller"
)

func Test_prompt(t *testing.T) {
	p := monkey.PatchInstanceMethod(reflect.TypeOf(&promptui.Prompt{}), "Run", func(*promptui.Prompt) (string, error) {
		return "input", nil
	})
	defer p.Unpatch()

	s, err := prompt("", cdcontroller.IsValidFixtureBarcode)
	assert.Nil(t, err)
	assert.Equal(t, "input", s)
}

func Test_promptError(t *testing.T) {
	p := monkey.PatchInstanceMethod(reflect.TypeOf(&promptui.Prompt{}), "Run", func(*promptui.Prompt) (string, error) {
		return "", assert.AnError
	})
	defer p.Unpatch()

	_, err := prompt("", cdcontroller.IsValidFixtureBarcode)
	assert.NotNil(t, err)
}

func Test_promptErrorInterrupt(t *testing.T) {
	p := monkey.PatchInstanceMethod(reflect.TypeOf(&promptui.Prompt{}), "Run", func(*promptui.Prompt) (string, error) {
		return "", promptui.ErrInterrupt
	})
	defer p.Unpatch()

	_, err := prompt("", cdcontroller.IsValidFixtureBarcode)
	assert.NotNil(t, err)
	assert.True(t, IsInterrupt(err))
}

func Test_promptConfirm(t *testing.T) {
	pc := monkey.PatchInstanceMethod(reflect.TypeOf(&promptui.Prompt{}), "Run", func(*promptui.Prompt) (string, error) {
		return "", nil
	})
	defer pc.Unpatch()

	assert.True(t, promptConfirm(""))
}
