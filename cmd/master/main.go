package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/chaitanyasai-meka/Ramforze/internal/ble"
)

func main() {
	fmt.Println("Starting Ramforze Master...")

	ready := make(chan struct{}, 1)
	bleErr := make(chan error, 1)

	go func() {
		bleErr <- ble.StartBLEListener(ready)
	}()

	select {
	case <-ready:
	case err := <-bleErr:
		fmt.Println("BLE listener error:", err)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Could not determine executable path:", err)
		os.Exit(1)
	}
	bridgePath := filepath.Join(filepath.Dir(exe), "blebridge")
	bleBridge := exec.Command(bridgePath, "--master")
	bleBridge.Stdout = os.Stdout
	bleBridge.Stderr = os.Stderr
	if err := bleBridge.Start(); err != nil {
		fmt.Println("Failed to start BLEBridge:", err)
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(quit)
	select {
	case err := <-bleErr:
		if err != nil {
			fmt.Println("BLE listener error:", err)
		}
	case <-quit:
		fmt.Println("Shutting down Master...")
	}

	if bleBridge.Process != nil {
		if err := bleBridge.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			fmt.Println("Failed to signal BLEBridge:", err)
		}
	}
	if err := bleBridge.Wait(); err != nil {
		fmt.Println("BLEBridge exited with error:", err)
	}
}
