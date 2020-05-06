package towercontroller

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

// IsInterrupt returns whether the error is from a CTRL-C being pressed in a prompt
func IsInterrupt(err error) bool {
	return err == promptui.ErrInterrupt
}

func prompt(message string, val promptui.ValidateFunc) (string, error) {
	p := promptui.Prompt{
		Label:    color.New(color.FgCyan).SprintFunc()(message),
		Validate: val,
		Stdin:    os.Stdin,
	}

	result, err := p.Run()
	if err != nil {
		if IsInterrupt(err) {
			// return as-is so we can react
			return "", err
		}

		// our custom error
		return "", fmt.Errorf("prompt: %v", err)
	}

	return strings.TrimSpace(result), nil
}

func promptConfirm(message string) bool {
	_, err := (&promptui.Prompt{
		Label:     color.New(color.FgCyan).SprintFunc()(message),
		IsConfirm: true,
		Default:   "Y",
	}).Run()

	return err == nil
}
