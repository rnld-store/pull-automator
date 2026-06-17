//go:build windows

package main

import "os"

// shutdownSignals no Windows: apenas Ctrl+C (os.Interrupt) é entregue de forma
// confiável ao processo; parar um serviço do Windows também encerra o processo.
var shutdownSignals = []os.Signal{os.Interrupt}
