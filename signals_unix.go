//go:build !windows

package main

import (
	"os"
	"syscall"
)

// shutdownSignals são os sinais que disparam o shutdown gracioso no Unix.
var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
