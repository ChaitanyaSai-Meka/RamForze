package main

import (
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "syscall"

    "github.com/chaitanyasai-meka/Ramforze/internal/ble"
)

func main() {
    fmt.Println("Starting Ramforze Master...")

    go func() {
        if err := ble.StartBLEListener(); err != nil {
            fmt.Println("BLE listener error:", err)
            os.Exit(1)
        }
    }()

    bleBridge := exec.Command("./blebridge", "--master")
    bleBridge.Stdout = os.Stdout
    bleBridge.Stderr = os.Stderr
    if err := bleBridge.Start(); err != nil {
        fmt.Println("Failed to start BLEBridge:", err)
        os.Exit(1)
    }

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
    <-quit

    fmt.Println("Shutting down Master...")
    bleBridge.Process.Kill()
}