package terminal

import "fmt"

func asrsErrorf(format string, args ...interface{}) error {
	return fmt.Errorf("terminal_server: "+format, args...)
}
