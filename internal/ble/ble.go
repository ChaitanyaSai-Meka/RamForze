package ble

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

type BLEEvent struct {
	Action string `json:"action"`
	IP     string `json:"peer_ip"`
	Port   int    `json:"port"`
	Name   string `json:"name"`
}

func StartBLEListener(ready chan<- struct{}, peers chan<- BLEEvent) error {

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home directory: %w", err)
	}

	dir := filepath.Join(home, ".ramforze")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not create .ramforze directory: %w", err)
	}

	socketPath := filepath.Join(dir, "ble.sock")

	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not remove existing BLE socket: %w", err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("could not listen on BLE socket: %w", err)
	}
	defer ln.Close()
	if err := os.Chmod(socketPath, 0600); err != nil {
		ln.Close()
		return fmt.Errorf("could not secure BLE socket permissions: %w", err)
	}

	if ready != nil {
		select {
		case ready <- struct{}{}:
		default:
		}
	}

	fmt.Println("BLE socket listening on", socketPath)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				fmt.Printf("BLE socket accept temporary error: %v\n", err)
				continue
			}
			return fmt.Errorf("BLE socket accept error: %w", err)
		}
		go handleConnection(conn, peers)
	}
}

func handleConnection(conn net.Conn, peers chan<- BLEEvent) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()

		var event BLEEvent
		err := json.Unmarshal(line, &event)
		if err != nil {
			fmt.Printf("Failed to parse BLE event: %v\n", err)
			continue
		}

		switch event.Action {
		case "add", "remove":
			if peers != nil {
				peers <- event
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("BLE socket read error: %v\n", err)
	}
}
