package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chaitanyasai-meka/Ramforze/internal/ble"
	"github.com/chaitanyasai-meka/Ramforze/internal/handshake"
	"github.com/chaitanyasai-meka/Ramforze/internal/token"
)

func getLANIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("could not list network interfaces: %w", err)
	}

	for _, i := range ifaces {
		if !strings.HasPrefix(i.Name, "en") {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no active LAN interface found")
}

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

	masterID, err := token.GenerateID()
	if err != nil {
		fmt.Println("Failed to generate master ID:", err)
		os.Exit(1)
	}
	masterIP, err := getLANIP()
	if err != nil {
		fmt.Println("Failed to detect LAN IP:", err)
		os.Exit(1)
	}

	go func() {
		for event := range peers {
			if event.Action == "add" {
				go func(e ble.BLEEvent) {
					fmt.Printf("Initiating handshake with %s (%s)\n", e.Name, e.IP)
					port, err := handshake.RequestDedicatedPort(e.IP, masterID, masterIP)
					if err != nil {
						fmt.Println("Handshake failed:", err)
						return
					}
					fmt.Printf("Handshake success. Dedicated port: %d\n", port)
				}(event)
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
