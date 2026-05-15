//go:build darwin

package cli

import (
	"io"
	"os"
	"strings"
)

type exitPanic struct{ code int }

// runCapture runs fn() while capturing stdout/stderr and intercepting osExit.
// Not safe for concurrent use — do not call t.Parallel() in tests that use this.
func runCapture(fn func() error) (outStr, errStr string, exitCode int, runErr error) {
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	origOut, origErr := os.Stdout, os.Stderr
	origExit := osExit

	os.Stdout = wOut
	os.Stderr = wErr
	osExit = func(c int) { panic(exitPanic{c}) }

	func() {
		defer func() {
			if r := recover(); r != nil {
				if ep, ok := r.(exitPanic); ok {
					exitCode = ep.code
				} else {
					panic(r)
				}
			}
		}()
		runErr = fn()
	}()

	os.Stdout = origOut
	os.Stderr = origErr
	osExit = origExit

	wOut.Close()
	wErr.Close()

	var bOut, bErr strings.Builder
	io.Copy(&bOut, rOut)
	io.Copy(&bErr, rErr)
	outStr = bOut.String()
	errStr = bErr.String()
	return
}
