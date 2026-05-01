package types

import "time"

type Task struct {
	TaskID     string `json:"task_id"`
	MasterID   string `json:"master_id"`
	TokenID    string `json:"token_id"`
	TokenValue string `json:"token_value"`

	Type string `json:"type"`
	Mode string `json:"mode"`

	Payload     TaskPayload `json:"payload"`
	ResourceAsk ResourceAsk `json:"resource_ask"`

	Priority     uint8 `json:"priority"`
	ParallelSafe bool  `json:"parallel_safe"`

	CreatedAt time.Time `json:"created_at"`
}

type TaskPayload struct {
	Tool        string   `json:"tool"`
	Args        []string `json:"args"`
	InputFiles  []string `json:"input_files"`
	OutputFiles []string `json:"output_files"`
}

type ResourceAsk struct {
	RAMMiB             uint64 `json:"ram_mib"`
	CPUPct             uint32 `json:"cpu_pct"`
	MaxDurationSeconds uint32 `json:"max_duration_seconds"`
}

type TokenWorker struct {
	TokenID            string    `json:"token_id"`
	TaskID             string    `json:"task_id"`
	MasterID           string    `json:"master_id"`
	ReservedRAMMiB     uint64    `json:"reserved_ram_mib"`
	ReservedCPUPct     uint32    `json:"reserved_cpu_pct"`
	MaxDurationSeconds uint32    `json:"max_duration_seconds"`
	ExpiresAt          time.Time `json:"expires_at"`
	Status             string    `json:"status"`
}

type JournalEntry struct {
	TaskID        string      `json:"task_id"`
	WorkerID      string      `json:"worker_id"`
	WorkerIP      string      `json:"worker_ip"`
	DedicatedPort int         `json:"dedicated_port"`
	Status        string      `json:"status"`
	DispatchedAt  time.Time   `json:"dispatched_at"`
	CompletedAt   *time.Time  `json:"completed_at"`
	Result        *TaskResult `json:"result"`
}

type HandshakeHello struct {
	MasterID        string `json:"master_id"`
	MasterIP        string `json:"master_ip"`
	ProtocolVersion string `json:"protocol_version"`
	AuthHMAC        string `json:"auth_hmac"`
}

type HandshakeResponse struct {
	WorkerID      string `json:"worker_id"`
	DedicatedPort int    `json:"dedicated_port"`
	Status        string `json:"status"`
}

type NegotiationRequest struct {
	TaskID       string      `json:"task_id"`
	MasterID     string      `json:"master_id"`
	ResourceAsk  ResourceAsk `json:"resource_ask"`
	RequiredTool string      `json:"required_tool"`
	Priority     uint8       `json:"priority"`
	ParallelSafe bool        `json:"parallel_safe"`
}

type NegotiationResponse struct {
	TaskID             string     `json:"task_id"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	TokenID            string     `json:"token_id,omitempty"`
	TokenValue         string     `json:"token_value,omitempty"`
	MaxDurationSeconds uint32     `json:"max_duration_seconds,omitempty"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
}

type TaskResult struct {
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	OutputFiles []string `json:"output_files,omitempty"`
	Stdout      string   `json:"stdout,omitempty"`
	Stderr      string   `json:"stderr,omitempty"`
	ExitCode    *int     `json:"exit_code,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}
type TaskTimeout struct {
	TaskID             string `json:"task_id"`
	Status             string `json:"status"`
	MaxDurationSeconds uint32 `json:"max_duration_seconds"`
	Reason             string `json:"reason"`
}
