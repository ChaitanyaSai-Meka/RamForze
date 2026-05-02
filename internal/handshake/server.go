package handshake

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/chaitanyasai-meka/Ramforze/internal/token"
	"github.com/chaitanyasai-meka/Ramforze/pkg/types"
)

type Server struct {
	Passphrase string
	Pool       *PortPool
	Registry   *Registry
	WorkerID   string
}

func NewServer(workerID string) (*Server, error) {
	pass, err := ReadPassphrase()
	if err != nil {
		return nil, err
	}

	pool := NewPortPool()
	return &Server{
		Passphrase: pass,
		Pool:       pool,
		Registry:   NewRegistry(pool),
		WorkerID:   workerID,
	}, nil
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", ":7946")
	if err != nil {
		return fmt.Errorf("failed to listen on :7946: %w", err)
	}
	defer listener.Close()
	fmt.Println("Worker handshake server listening on :7946")

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				fmt.Printf("Handshake accept temporary error: %v\n", err)
				continue
			}
			return fmt.Errorf("handshake accept error: %w", err)
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	var hello types.HandshakeHello
	if err := json.NewDecoder(conn).Decode(&hello); err != nil {
		fmt.Printf("Invalid handshake payload: %v\n", err)
		return
	}

	isValid := token.VerifyHandshake(hello.MasterID, s.Passphrase, hello.AuthHMAC)
	if !isValid {
		json.NewEncoder(conn).Encode(types.HandshakeResponse{Status: "REJECTED_UNAUTHORIZED"})
		return
	}

	port, err := s.Pool.Allocate()
	if err != nil {
		json.NewEncoder(conn).Encode(types.HandshakeResponse{Status: "REJECTED_NO_PORTS"})
		return
	}

	response := types.HandshakeResponse{
		WorkerID:      s.WorkerID,
		DedicatedPort: port,
		Status:        "ACCEPTED",
	}

	if err := json.NewEncoder(conn).Encode(response); err != nil {
		fmt.Printf("Failed to send response: %v\n", err)
		s.Pool.Release(port)
		return
	}

	s.Registry.Register(hello.MasterID, port)
	fmt.Printf("Successfully registered Master %s to port %d\n", hello.MasterID, port)
}
