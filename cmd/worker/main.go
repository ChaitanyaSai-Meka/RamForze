package main

import (
	"fmt"

	"github.com/chaitanyasai-meka/Ramforze/internal/handshake"
)

func main() {
	fmt.Println("Starting Ramforze Worker...")

	server, err := handshake.NewServer("worker-uuid-hardcoded-for-now")
	if err != nil {
		fmt.Println("Failed to init handshake server:", err)
		return
	}

	if err := server.ListenAndServe(); err != nil {
		fmt.Println("Handshake server error:", err)
	}
}
