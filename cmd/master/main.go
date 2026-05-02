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
	"github.com/chaitanyasai-meka/Ramforze/internal/handshake"
)

func main() {
	fmt.Println("Starting Ramforze Master...")

	ready := make(chan struct{}, 1)
	bleErr := make(chan error, 1)
	peers := make(chan ble.BLEEvent, 10)

	go func() {
		bleErr <- ble.StartBLEListener(ready, peers)
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

	go func() {
		for event := range peers {
			if event.Action == "add" {
				fmt.Printf("Initiating handshake with %s (%s)\n", event.Name, event.IP)
				port, err := handshake.RequestDedicatedPort(event.IP, "master-uuid-hardcoded-for-now", "master-ip-hardcoded-for-now")
				if err != nil {
					fmt.Println("Handshake failed:", err)
					continue
				}
				fmt.Printf("Handshake success. Dedicated port: %d\n", port)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(quit)

	bridgeExited := make(chan error, 1)
	go func() {
		bridgeExited <- bleBridge.Wait()
	}()

	bridgeAlreadyExited := false
	select {
	case err := <-bleErr:
		if err != nil {
			fmt.Println("BLE listener error:", err)
		}
	case err := <-bridgeExited:
		bridgeAlreadyExited = true
		fmt.Println("BLEBridge exited unexpectedly:", err)
	case <-quit:
		fmt.Println("Shutting down Master...")
	}

	if bleBridge.Process != nil {
		if err := bleBridge.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			fmt.Println("Failed to signal BLEBridge:", err)
		}
	}
	if !bridgeAlreadyExited {
		if err := bleBridge.Wait(); err != nil {
			fmt.Println("BLEBridge exited with error:", err)
		}
	}
}
