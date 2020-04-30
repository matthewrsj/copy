package towercontroller

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

func promptDeadline(message string, td time.Time) (string, error) {
	ctx, cancel := context.WithDeadline(context.Background(), td)
	defer cancel()

	resultCh := make(chan string)

	go func() {
		resultCh <- prompt(message)
	}()

	select {
	case <-ctx.Done():
		return "", errors.New("prompt timeout exceeded")
	case r := <-resultCh:
		return r, nil
	}
}

func prompt(message string) string {
	cyan := color.New(color.FgCyan).SprintFunc()
	fmt.Print(cyan(message + " >>> "))
	// ignore error from ReadString. No real way for this to error without memory corruption
	// (see golang source code)
	response, _ := bufio.NewReader(os.Stdin).ReadString('\n' /* delim */)

	return strings.TrimSpace(response)
}
