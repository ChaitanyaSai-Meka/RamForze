package handshake

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/chaitanyasai-meka/Ramforze/internal/token"
	"github.com/chaitanyasai-meka/Ramforze/pkg/types"
)

func RequestDedicatedPort(workerIP string, masterID string, masterIP string) (int, error) {
	passphrase, err := ReadPassphrase()
	if err != nil {
		return 0, fmt.Errorf("client failed to read config: %w", err)
	}

	address := fmt.Sprintf("%s:7946", workerIP)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to worker %s: %w", workerIP, err)
	}
	defer conn.Close()

	authHMAC := token.SignHandshake(masterID, passphrase)

	hello := types.HandshakeHello{
		MasterID:        masterID,
		MasterIP:        masterIP,
		ProtocolVersion: "1.0",
		AuthHMAC:        authHMAC,
	}

	if err := json.NewEncoder(conn).Encode(hello); err != nil {
		return 0, fmt.Errorf("failed to send handshake hello: %w", err)
	}

	var response types.HandshakeResponse
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return 0, fmt.Errorf("failed to read handshake response: %w", err)
	}

	if response.Status != "ACCEPTED" {
		return 0, fmt.Errorf("handshake rejected by worker: %s", response.Status)
	}

	return response.DedicatedPort, nil
}
