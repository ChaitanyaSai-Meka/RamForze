package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chaitanyasai-meka/Ramforze/internal/handshake"
	"github.com/chaitanyasai-meka/Ramforze/internal/token"
)

func getOrCreateWorkerID() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	dir := filepath.Join(homeDir, ".ramforze")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("could not create ramforze directory: %w", err)
	}
	path := filepath.Join(dir, "worker_id")
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}
	id, err := token.GenerateID()
	if err != nil {
		return "", fmt.Errorf("could not generate worker ID: %w", err)
	}
	if err := os.WriteFile(path, []byte(id), 0600); err != nil {
		return "", fmt.Errorf("could not persist worker ID: %w", err)
	}
	return id, nil
}

func main() {
	fmt.Println("Starting Ramforze Worker...")

	workerID, err := getOrCreateWorkerID()
	if err != nil {
		fmt.Println("Failed to get worker ID:", err)
		return
	}

	server, err := handshake.NewServer(workerID)
	if err != nil {
		fmt.Println("Failed to init handshake server:", err)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Could not determine executable path:", err)
		return
	}
	bridgePath := filepath.Join(filepath.Dir(exe), "blebridge")

	bleBridge := exec.Command(bridgePath, "--worker")
	bleBridge.Stdout = os.Stdout
	bleBridge.Stderr = os.Stderr
	if err := bleBridge.Start(); err != nil {
		fmt.Println("Failed to start BLEBridge:", err)
		return
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
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
	case err := <-serverErr:
		if err != nil {
			fmt.Println("Handshake server error:", err)
		}
	case err := <-bridgeExited:
		bridgeAlreadyExited = true
		fmt.Println("BLEBridge exited unexpectedly:", err)
	case <-quit:
		fmt.Println("Shutting down Worker...")
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
