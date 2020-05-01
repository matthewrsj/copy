package cmdlineutils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Lockout creates a lockfile with file name fName in the OS temp directory. Hold on to the file reference that is
// returned for use in unlocking the system. Should prevent the software from running if an error is returned.
//
// Example usage like so
// lockF, err := Lockout(lFileName)
// if err != nil {
//     log.Fatalf("lockfile creation: %v\n", err)
// }
//
// Complete solution with all lockout functions
// lockF, err := Lockout(lFileName)
// if err != nil {
//     log.Fatalf("lockfile creation: %v\n", err)
// }
//
// c := make(chan os.Signal, 1)
// signal.Notify(c, os.Interrupt)   // CTRL-C
// signal.Notify(c, syscall.SIGHUP) // terminal exits
// go UnLockoutWithSignalAndExit(f, c, log.New(os.Stdout, "", 0))
//
// defer func() {
//     if err := UnLockout(f) {
//         log.Printf("remove lockfile %v\n", err)
//     }
// }()
func Lockout(fName string) (*os.File, error) {
	lf := filepath.Join(os.TempDir(), fName)
	if _, err := os.Stat(lf); err == nil {
		return nil, fmt.Errorf("another instance of this software may be running, lockfile %s exists", lf)
	}

	f, err := os.Create(lf)
	if err != nil {
		return nil, fmt.Errorf("obtain lock: %v", err)
	}

	return f, err
}

// UnLockoutWithSignalAndExit watches c for a signal. When the signal is received it removes the lockfile and exits the
// program. Logs to l log.Logger if exiting with a failure.
//
// WARNING: exits program when signal is received.
//
// This function should be called in a goroutine like so
// c := make(chan os.Signal, 1)
// signal.Notify(c, os.Interrupt)   // CTRL-C
// signal.Notify(c, syscall.SIGHUP) // terminal exits
// go UnLockoutWithSignalAndExit(f, c, log.New(os.Stdout, "", 0))
func UnLockoutWithSignalAndExit(f *os.File, c chan os.Signal, l *log.Logger) {
	<-c
	if err := UnLockout(f); err != nil {
		l.Fatalf("remove lockfile: %v", err)
	}
	os.Exit(0)
}

// UnLockout removes f before the function exits. Should be deferred after the lockfile is created.
// Example usage like so
// defer func() {
//     if err := UnLockout(f) {
//         log.Printf("remove lockfile %v\n", err)
//     }
// }()
func UnLockout(f *os.File) error {
	_ = f.Close()
	if err := os.Remove(f.Name()); err != nil {
		return fmt.Errorf("remove lockfile %s: %v", f.Name(), err)
	}

	return nil
}
