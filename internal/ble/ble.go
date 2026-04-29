package ble

import (
	"bufio"
	"encoding/json"
	"path/filepath"
	"fmt"
	"net"
	"os"
)

type BLEEvent struct {
    Action string `json:"action"`
    IP     string `json:"peer_ip"`
    Port   int    `json:"port"`
    Name   string `json:"name"`
}

func StartBLEListener() error {

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home directory: %w", err)
	}

	socketPath := filepath.Join(home, ".ramforze", "ble.sock")

	os.Remove(socketPath)

	ln, err := net.Listen("unix",socketPath)
	if err != nil {
		return fmt.Errorf("could not listen on BLE socket: %w", err)
	}
	defer ln.Close()

	fmt.Println("BLE socket listening on", socketPath)

	for {
		conn,err := ln.Accept()
		if err != nil {
			fmt.Printf("BLE socket accept error: %v\n", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
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
		
		fmt.Printf("Received BLE event: action=%s, IP=%s, port=%d, name=%s\n",
			event.Action, event.IP, event.Port, event.Name)
	}
}