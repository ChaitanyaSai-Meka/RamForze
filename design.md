# Ramforze: System Design Document

> **One line:** Ramforze is a local compute-sharing tool that transparently offloads heavy tasks from a resource-starved machine to a nearby idle one through peer discovery, token-gated resource negotiation, and isolated task execution.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [What We Are Building](#2-what-we-are-building)
3. [Scope and Constraints](#3-scope-and-constraints)
4. [System Roles](#4-system-roles)
5. [High-Level Architecture](#5-high-level-architecture)
6. [Component Design](#6-component-design)
   - 6.1 [BLE Discovery Layer](#61-ble-discovery-layer)
   - 6.2 [Transport Layer](#62-transport-layer)
   - 6.3 [Handshake and Port Allocation](#63-handshake-and-port-allocation)
   - 6.4 [The Governor](#64-the-governor)
   - 6.5 [Capability Token System](#65-capability-token-system)
   - 6.6 [Task Model](#66-task-model)
   - 6.7 [Master Task Journal](#67-master-task-journal)
   - 6.8 [Worker Priority Queue](#68-worker-priority-queue)
   - 6.9 [Result Return Flow](#69-result-return-flow)
7. [The Three Execution Modes](#7-the-three-execution-modes)
   - 7.1 [Transparent Mode](#71-transparent-mode)
   - 7.2 [Assisted Mode](#72-assisted-mode)
   - 7.3 [Manual Mode](#73-manual-mode)
8. [Fault Tolerance](#8-fault-tolerance)
9. [Security Model](#9-security-model)
10. [UI Layer](#10-ui-layer)
11. [Project Structure](#11-project-structure)
12. [Data Formats](#12-data-formats)
13. [Future Milestones](#13-future-milestones)

---

## 1. Problem Statement

A developer's machine is often underpowered for heavy tasks: compiling large codebases, running intensive builds, processing large datasets. Meanwhile, a nearby machine sits largely idle. That idle compute is completely wasted. There is no lightweight, peer-to-peer tool that lets two ordinary machines share compute locally, without cloud infrastructure, without complex setup, and without requiring the user to understand distributed systems.

Ramforze solves this.

---

## 2. What We Are Building

Ramforze is a **local distributed task dispatcher** for macOS. It allows one machine (the Master) to offload compute-heavy tasks to a nearby machine (the Worker) over a local peer-to-peer connection. The Worker negotiates and approves resource allocation via a Governor goroutine, issues a one-time capability token to authorize the task, and executes it in an isolated resource envelope. The Master tracks all task state in a persistent journal. The result travels back to the Master when the task completes.

The user experience goal: it should feel like your machine got faster, not like you are operating a distributed system.

---

## 3. Scope and Constraints

| Item | Decision |
|---|---|
| **Platform** | macOS only (MVP). Windows and Linux after. |
| **Network** | Same LAN (same router). Cross-router support is a future milestone. |
| **Discovery** | Bluetooth Low Energy (BLE) for peer discovery. |
| **Transport** | TCP over LAN for all task and result data. |
| **Serialization** | JSON. Human-readable, easy to debug during development. |
| **GUI** | SwiftUI frontend + Go backend, communicating over a local Unix socket. |
| **Language** | Go for all backend logic. Swift for all UI. |
| **Pre-condition** | All execution modes require a pre-established connection. No connection means no offloading. |
| **Cross-platform tasks** | Not supported in MVP. The Worker must have the required tools installed locally. |

---

## 4. System Roles

### Master
The machine that owns the task and needs help. It:
- Discovers available Workers via BLE.
- Negotiates resource allocation with the Worker's Governor.
- Holds and maintains the Task Journal.
- Splits work into task chunks and dispatches them.
- Receives results and re-queues on failure.

### Worker
The machine providing compute. It:
- Runs a Governor goroutine that controls all resource decisions.
- Allocates a dedicated port per connected Master.
- Executes approved task chunks in isolated resource envelopes.
- Returns results to the Master.
- Can serve multiple Masters simultaneously, subject to resource availability.

### Governor
A goroutine inside the Worker process. It is the single authority over the Worker's resources. Nothing runs on the Worker without a valid capability token issued by the Governor. It:
- Receives resource negotiation requests from Masters.
- Checks available CPU, RAM, required tool availability, and accepts the Master's time estimate.
- Issues matched capability token pairs: one sent to the Master, one stored internally.
- Reserves the approved resources against that token for the token's full duration.
- Expires tokens on task completion, timeout, or connection loss.

---

## 5. High-Level Architecture

This diagram shows the full lifecycle: BLE discovery, TCP connection, negotiation, token issuance, task dispatch, and result return.

```
MASTER                                              WORKER
+-----------------------------------+              +-----------------------------------+
|                                   |              |                                   |
|  [ SwiftUI Frontend ]             |              |  [ SwiftUI Frontend ]             |
|       | Unix Socket               |              |       | Unix Socket               |
|  [ Go Backend ]                   |              |  [ Go Backend ]                   |
|       |                           |              |       |                           |
|  [ BLE Scanner ]--BLE advert.-----|--------------|--[ BLE Advertiser ]               |
|       |                           |              |       |                           |
|       | (extracts Worker IP)       |              |       |                           |
|       v                           |              |       v                           |
|  [ Handshake Client ]             |              |  [ Handshake Server :7946 ]       |
|       |---HELLO + auth_hmac------>|              |<---HELLO + auth_hmac---|          |
|       |<--dedicated port :7947----|              |----port :7947--------->|          |
|       |                           |              |                                   |
|  +---------------------------+    |              |  +----------------------------+   |
|  | Task Dispatcher           |    |              |  | Governor                   |   |
|  |                           |    |              |  |                            |   |
|  | 1. Build task chunk       |    |              |  | On negotiation request:    |   |
|  | 2. Send negotiation req   |----|--TCP :7947-->|->| - Check RAM headroom       |   |
|  |    (ram, cpu, tool,       |    |              |  | - Check CPU headroom       |   |
|  |     max_duration_seconds) |    |              |  | - Check tool availability  |   |
|  |                           |    |              |  | - Accept time estimate     |   |
|  |                           |    |              |  |                            |   |
|  |                           |    |              |  | If approved:               |   |
|  |                           |    |              |  | - Reserve resources        |   |
|  |                           |    |              |  | - Generate token pair      |   |
|  |                           |    |              |  |   K_worker stored locally  |   |
|  |                           |    |              |  |   K_master sent to Master  |   |
|  |<--K_master (task_id,      |<---|----K_master--|--|   (task_id, status,       |   |
|  |   status, token_id,       |    |              |  |    token_id, token_value, |   |
|  |   token_value,            |    |              |  |    max_duration_seconds,  |   |
|  |   max_duration_seconds,   |    |              |  |    expires_at)            |   |
|  |   expires_at)             |    |              |  +----------------------------+   |
|  |                           |    |              |                                   |
|  | 3. Write task to journal  |    |              |  +----------------------------+   |
|  |    (status: dispatched)   |    |              |  | Worker Priority Queue      |   |
|  | 4. Attach K_master to     |    |              |  |                            |   |
|  |    task envelope          |    |              |  | On task + K_master arrival:|   |
|  | 5. Send task envelope  -->|----|--TCP :7947-->|->| - Validate K_master vs     |   |
|  |                           |    |              |  |   K_worker (HMAC check)    |   |
|  |                           |    |              |  | - Check expires_at         |   |
|  |                           |    |              |  | - Check resource headroom  |   |
|  |                           |    |              |  | - Queue or run task        |   |
|  |                           |    |              |  |                            |   |
|  |                           |    |              |  | On completion:             |   |
|  |                           |    |              |  | - Token expires            |   |
|  |                           |    |              |  | - Resources freed          |   |
|  |                           |    |              |  | - Result sent to Master    |   |
|  |<--result (files + output)-|<---|----result----|--|                            |   |
|  |                           |    |              |  +----------------------------+   |
|  | 6. Update journal         |    |              |                                   |
|  |    (status: completed)    |    |              |  [ Port Registry ]                |
|  +---------------------------+    |              |  master-A -> :7947  active        |
|                                   |              |  master-B -> :7948  active        |
|  [ Task Journal ]                 |              |                                   |
|  (~/.ramforze/journal.ndjson)     |              |                                   |
+-----------------------------------+              +-----------------------------------+
```

**Key flow in plain words:**

1. Worker advertises over BLE. Master scans, extracts the Worker's LAN IP and handshake port automatically. No manual IP entry.
2. Master connects to Worker on the fixed handshake port `:7946`. Authenticates using a HMAC of its ID signed with the shared passphrase. Worker verifies and allocates a dedicated port (e.g. `:7947`) for all future communication with this Master.
3. Master builds a task chunk and sends a negotiation request to the Governor on `:7947`, describing the RAM need, CPU need, required tool, and `max_duration_seconds`.
4. Governor checks RAM headroom, CPU headroom, and tool availability. If all pass, it generates a token pair. `K_worker` is stored internally with the reserved resource allocation. `K_master` (containing `task_id`, `status`, `token_id`, `token_value`, `max_duration_seconds`, and `expires_at`) is sent back to the Master.
5. Master writes the task to the journal with status `dispatched`, attaches `K_master` to the task envelope, and sends the task to the Worker on `:7947`.
6. Worker validates `K_master` against `K_worker` via HMAC check and expiry check. On success, it allocates the reserved resources and executes the task. If the task is still running when `expires_at` is reached, the Governor kills it and notifies the Master.
7. On completion, the token expires, resources are freed, and the result (output files + stdout/stderr) is sent back to the Master.
8. Master updates the journal to `completed`.

---

## 6. Component Design

### 6.1 BLE Discovery Layer

Bluetooth Low Energy is used exclusively for peer discovery. No task data travels over BLE. It is low-power, always-on, and does not require a shared router, making it the right choice for the discovery phase.

**Worker side (Advertising):**
When the Worker starts Ramforze, it begins BLE advertising. The BLE advertisement payload contains:
```
CBAdvertisementDataServiceUUIDsKey : ramforze-service (custom UUID)
CBAdvertisementDataLocalNameKey    : "<hostname-prefix>|<LAN-IP>|7946"
                                     e.g. "MacBoo|192.168.1.42|7946"
```

**Master side (Scanning):**
The Master runs a BLE scanner, filters for the Ramforze service UUID, and extracts the Worker's LAN IP and handshake port by splitting the advertised local name string on `|`. No manual IP entry is needed. Once the Master has the Worker's IP and port, BLE's job is done. All further communication is TCP.

```text
parts    = localName.split("|")
hostname = parts[0]  // "MacBook"
ip       = parts[1]  // "192.168.1.42"
port     = parts[2]  // "7946"
```

**macOS implementation note:**
BLE on macOS is handled by a Swift subprocess via CoreBluetooth. The Swift process communicates discovered peer IP and port to the Go backend over a Unix socket at `~/.ramforze/ble.sock`. This avoids cgo bindings and CoreBluetooth memory management in Go.

Note: macOS `CBPeripheralManager` only supports `CBAdvertisementDataLocalNameKey` and `CBAdvertisementDataServiceUUIDsKey` reliably in advertisement data. Manufacturer data is silently ignored. IP and port are therefore packed into `CBAdvertisementDataLocalNameKey` as a pipe-delimited string, and the Master splits on `|` to recover all three fields. The advertised hostname prefix is sanitized to remove `|` and truncated to fit BLE advertisement size limits.

```text
macOS BLE Architecture:

+---------------------------+        +---------------------------+
|   Swift Process           |        |   Go Backend              |
|   (BLEBridge)             |        |                           |
|                           |        |                           |
|   CoreBluetooth           |        |   internal/ble            |
|   - CBCentralManager      |        |   - Reads peer discovery  |
|     (Master: scanning)    | Unix   |     events from socket    |
|   - CBPeripheralManager   | socket |   - Extracts IP + port    |
|     (Worker: advertising) |------->|   - Triggers TCP handshake |
|                           |        |                           |
|   On peer found:          |        |   Socket path:            |
|   sends NDJSON events:    |        |   ~/.ramforze/ble.sock    |
|   {"action":"add",        |        |                           |
|    "peer_ip":"192.x.x.x", |        |                           |
|    "port":7946,           |        |                           |
|    "name":"Arjun"}        |        |                           |
|   {"action":"remove",     |        |                           |
|    "peer_ip":"192.x.x.x"} |        |                           |
+---------------------------+        +---------------------------+
```

**Connection loss detection:**
BLE advertisement stops when the Worker process exits or the machine sleeps. The Master detects this either via the BLE scan (device disappears from scan results) or via TCP keepalive on the data connection. Either signal triggers the fault tolerance flow.

---

### 6.2 Transport Layer

All task data, negotiation, and results travel over **TCP on the LAN**. TCP is chosen because it guarantees ordered and reliable delivery, connection drops are detectable via keepalive, and no additional libraries are needed in Go.

**TCP Keepalive** is enabled on all connections with:
- Idle time before first probe: 10 seconds
- Probe interval: 5 seconds
- Probe count before declaring dead: 3

This means a dead connection is detected within approximately 25 seconds maximum.

---

### 6.3 Handshake and Port Allocation

Every communication channel begins on the Worker's fixed **handshake port `:7946`**. This is the only port that is fixed and known in advance.

**Flow:**

```
1. Master connects to Worker on :7946

2. Master sends HELLO with passphrase HMAC for authentication:
   {
     "master_id": "<uuid>",
     "master_ip": "192.168.1.10",
     "protocol_version": "1.0",
     "auth_hmac": "<hmac-of-master_id-using-passphrase>"
   }

3. Worker verifies auth_hmac against the shared passphrase.
   If verification fails, connection is rejected immediately.

4. Worker's Governor allocates a dedicated port for this Master
   (from a pool, e.g., 7947 to 8946).

5. Worker responds:
   {
     "worker_id": "<uuid>",
     "dedicated_port": 7947,
     "status": "connected"
   }

6. Master closes the handshake connection.
   All future communication (negotiation, task transfer, results)
   travels on :7947, the Master's dedicated port.
```

**Port Registry on the Worker:**
```
master_id        dedicated_port    state
master-uuid-A    :7947             active
master-uuid-B    :7948             active
```

When a Master disconnects, its dedicated port is closed and returned to the available pool.

**UI Display on Startup:**
```
Ramforze is running.
  Your IP address  : 192.168.1.42
  Handshake port   : 7946
  Share this IP with your peer to connect.
```

---

### 6.4 The Governor

The Governor is a goroutine inside the Worker's Go backend. It is the **single authority** over all resource decisions on the Worker. No task is executed without the Governor's approval.

All memory values in this document use binary units: `1 GiB = 1024 MiB`.

**What the Governor checks before issuing a token:**

1. **RAM headroom:** not just current free RAM, but safe-to-offer RAM. The Governor models current usage and reserves a buffer for the Worker's own OS and applications, then calculates what can safely be offered.
   ```
   safe_to_offer_RAM = total_RAM - used_RAM - system_buffer (e.g., 1536 MiB)
   ```

2. **CPU headroom:** the Governor measures current CPU utilization and calculates the available percentage.
   ```
   safe_to_offer_CPU = 100% - current_utilization% - system_buffer (e.g., 15%)
   ```

3. **Tool availability:** the Governor checks whether the binary required by the task is installed on the Worker.
   ```
   task.required_tool = "clang"
   Governor checks: exec.LookPath("clang") -> found / not found
   ```

4. **Time window:** the Master provides `max_duration_seconds` as its estimate for how long the task will take. The Governor accepts this as the token's validity window. `expires_at` is computed as `negotiation_time + max_duration_seconds`. If the task is still running when `expires_at` is reached, the Governor kills it and notifies the Master.

If all four checks pass, the Governor issues a capability token, marks those resources as reserved, and subtracts the reserved amount from the available pool for any subsequent requests until the token expires or the task completes.

---

### 6.5 Capability Token System

The capability token system is the security and isolation backbone of Ramforze. Every task execution requires a valid, one-time token. Tokens cannot be reused, guessed, or forged.

**Token Generation:**

The Governor generates a token pair when it approves a resource negotiation request.

```
token_id    = UUID v4 (random, unique per task)
hmac_secret = shared passphrase established during handshake
token_value = HMAC-SHA256(
  message = token_id | task_id | master_id | expires_at(RFC3339 UTC),
  key     = hmac_secret
)
Note: string fields must not contain "|". Output is hex-encoded SHA-256 (64 chars / 32 bytes).
```

**K_worker** is stored internally by the Governor:
```json
{
  "token_id": "a3f9...",
  "task_id": "task-001",
  "master_id": "master-uuid-A",
  "reserved_ram_mib": 2048,
  "reserved_cpu_pct": 40,
  "max_duration_seconds": 120,
  "expires_at": "2024-01-01T12:05:00Z",
  "status": "pending"
}
```

`max_duration_seconds` is the Master's estimate, accepted verbatim by the Governor. `expires_at` is computed as `negotiation_timestamp + max_duration_seconds`. Both fields are stored so the Governor can enforce the execution ceiling and display remaining time in the Worker UI.

**K_master** is sent to the Master:
```json
{
  "task_id": "task-001",
  "status": "approved",
  "token_id": "a3f9...",
  "token_value": "<hmac-signature>",
  "max_duration_seconds": 120,
  "expires_at": "2024-01-01T12:05:00Z"
}
```

The Master receives `K_master`, sees the approved time window, and knows the hard deadline for this task. The Master UI can display a countdown for the task using `expires_at`.

**Token Usage Flow:**

```
Master -> sends task envelope to Worker's dedicated port
          K_master fields (token_id, token_value) included in envelope

Worker -> extracts token_id from envelope
       -> looks up K_worker from Governor's store
       -> recomputes HMAC and compares with token_value
       -> checks expires_at (is it still valid?)
       -> if valid: allocates reserved resources, begins execution
       -> if invalid or expired: rejects, returns error to Master
```

**Token Expiry Conditions:**

- Task completes successfully: token marked `expired`, resources freed.
- Task exceeds `max_duration_seconds`: Governor kills the task process, marks token `expired`, notifies Master to re-queue locally.
- Master disconnects mid-task: TCP keepalive detects drop, Governor aborts task, token marked `expired`, resources freed.
- Worker process exits: all tokens invalidated on restart.

**One token per task. One task per token. No exceptions.**

For every new task, the Master must initiate a fresh negotiation with the Governor. A previous token cannot be reused even if it has not yet expired.

---

### 6.6 Task Model

A task is the fundamental unit of work in Ramforze. Every task is serialized to JSON by the Master before dispatch. The Master is responsible for tagging every field before the task is sent. The Worker never infers task properties on its own.

**Task Schema:**
```json
{
  "task_id": "task-uuid-001",
  "master_id": "master-uuid-A",
  "token_id": "a3f9...",
  "token_value": "<hmac-signature>",

  "type": "compilation",
  "mode": "transparent",

  "payload": {
    "tool": "clang",
    "args": ["-O2", "-c", "main.c", "-o", "main.o"],
    "input_files": ["main.c"],
    "output_files": ["main.o"]
  },

  "resource_ask": {
    "ram_mib": 2048,
    "cpu_pct": 40,
    "max_duration_seconds": 120
  },

  "priority": 2,
  "parallel_safe": true,
  "created_at": "2024-01-01T12:00:00Z"
}
```

**Field Definitions:**

| Field | Description |
|---|---|
| `task_id` | UUID v4. Globally unique. Written to journal before dispatch. |
| `master_id` | Identifies which Master owns this task. |
| `token_id` / `token_value` | Capability token issued by the Governor. Worker validates both before execution. |
| `type` | Task category: `compilation`, `script`, `command`. |
| `mode` | Which execution mode triggered this task: `transparent`, `assisted`, `manual`. |
| `payload.tool` | The binary to run on the Worker (e.g. `clang`). |
| `payload.args` | Arguments to pass to the tool. |
| `payload.input_files` | Files the Master will transfer to the Worker before execution. |
| `payload.output_files` | Files the Worker will transfer back to the Master after execution. |
| `resource_ask.ram_mib` | RAM the Master estimates this task needs. |
| `resource_ask.cpu_pct` | CPU percentage the Master estimates this task needs. |
| `resource_ask.max_duration_seconds` | Master's time estimate. Baked into the token. Task is killed if it exceeds this window. |
| `priority` | Integer 1 to 5. Higher means more urgent. Used by the Worker's priority queue. |
| `parallel_safe` | Boolean. If true, this task can be queued alongside another running task. If false and the Worker is busy, the task is rejected. |

---

### 6.7 Master Task Journal

The Task Journal is a persistent journal file maintained by the Master. It is the **source of truth** for all task state. A new task entry is written to disk **before** a task is dispatched to the Worker, and later status changes update that task's stored record. This ensures that if the Master crashes mid-dispatch, the journal records what was sent and to whom, enabling full recovery on restart.

**Journal Entry Schema:**
```json
{
  "task_id": "task-uuid-001",
  "worker_id": "worker-uuid-B",
  "worker_ip": "192.168.1.42",
  "dedicated_port": 7947,
  "status": "dispatched",
  "dispatched_at": "2024-01-01T12:00:00Z",
  "completed_at": null,
  "result": null
}
```

**Status Lifecycle:**
```
created -> dispatched -> completed
                      -> failed    -> re-queued -> dispatched (retry)
                      -> timed_out -> re-queued
```

**On Master crash and restart:**

1. Master reads the journal on startup.
2. Any task with status `dispatched` is considered potentially lost.
3. Master attempts to reconnect to the recorded Worker IP and port.
4. If reconnection succeeds: Master queries the Worker for task status by `task_id`.
5. If Worker confirms completion: journal entry updated to `completed`, result retrieved.
6. If Worker has no record (crashed too, or task was lost): task re-queued locally.
7. If reconnection fails: task re-queued locally.

The journal is stored as a newline-delimited JSON file containing the current record for each tracked task. Status updates replace the stored file atomically so the journal remains crash-safe during rewrites. It is stored at:
```
~/.ramforze/journal.ndjson
```

---

### 6.8 Worker Priority Queue

The Worker maintains an internal priority queue of pending tasks. This queue handles scenarios where multiple Masters are sending tasks simultaneously, or where a second task arrives while the Worker is already busy.

**Decision tree when a new task arrives:**

```
Incoming task (with valid token)
              |
Does the Worker have enough headroom RIGHT NOW
to run it alongside all currently running tasks?
              |
         YES  |  NO
              |
    Accept    |  Is the task parallel_safe?
    and run   |
              |   YES              NO
              |
          Queue it           Reject -> send
          by priority        rejection to Master
```

When a running task completes and resources are freed, the Governor re-evaluates the queue. It picks the highest priority task that fits in the newly available headroom, validates its token (still within expiry), and dispatches it. Tasks of equal priority are ordered by arrival time (FIFO).

**Rejection response to Master:**
```json
{
  "task_id": "task-uuid-001",
  "status": "rejected",
  "reason": "insufficient_resources_and_not_parallel_safe"
}
```

The Master receives this, re-queues the task chunk to run locally, and logs the event in the journal.

---

### 6.9 Result Return Flow

When the Worker completes a task:

1. Worker writes output files to a temporary staging directory.
2. Worker sends a `TASK_COMPLETE` message to the Master on the Master's dedicated port.
3. Master receives the message and opens a file transfer sub-channel on the same dedicated port.
4. Worker streams output files to the Master over TCP.
5. Master verifies received files and updates the journal entry to `completed`.
6. Master confirms receipt. Worker deletes staging files, token is marked expired, reserved resources are freed.

---

## 7. The Three Execution Modes

All three modes share the same underlying pipeline: negotiate with Governor, receive token, attach token to task, execute on Worker, return result. They differ only in **how a task enters the system**.

A pre-established connection is required for all three modes. If no Worker is connected, every mode falls back to local execution silently.

### 7.1 Transparent Mode

The user does nothing. Ramforze intercepts the task before the OS allocates resources to it.

**How it works (compiler wrapper):**

Ramforze installs a wrapper binary on the Master machine that shadows the actual compiler in the system PATH.

```
Normal flow:
  go build -> /usr/bin/clang -> compiles locally

Ramforze flow:
  go build -> /usr/local/bin/ramforze-clang (wrapper)
    |
    -> If Worker is connected and Governor approved:
         Master transfers source file to Worker
         Worker compiles
         Worker returns .o file
         go build continues, never knew anything changed
    |
    -> If not connected, or Governor rejected, or Worker timed out:
         Falls through to /usr/bin/clang
         Compiles locally
         User sees no Ramforze-specific error (silent fallback)
```

**Supported transparent task type (MVP):** Compilation jobs (Go, C, C++). Each source file is an independent compilation unit, making it a natural, parallel-safe task chunk.

---

### 7.2 Assisted Mode

Ramforze monitors the Master's system resources in the background. When a process crosses a sustained resource threshold and a Worker is available with headroom, Ramforze surfaces a native macOS notification offering to offload.

**Trigger condition:**
```
Process RAM usage exceeds 70% of total RAM
AND this condition is sustained for more than 30 seconds
AND a Worker is connected with sufficient available headroom
```

**Notification:**
```
+-----------------------------------------------+
| Ramforze                                      |
|                                               |
| "go build" is using 5632 MiB of RAM.         |
| Your peer has 8192 MiB available.            |
|                                               |
| [Offload compilation]         [Dismiss]       |
+-----------------------------------------------+
```

User clicks "Offload compilation" and Ramforze handles the rest. One action from the user.

---

### 7.3 Manual Mode

The user explicitly selects a command or script to run on the Worker's machine. This is a **remote execution launcher**. The user picks what to run before it runs. It is not a running process migration tool.

**UI flow:**
```
Master UI -> "Manual" tab
  -> Input: command or script path
  -> Input: arguments
  -> Input: any input files to send
  -> Button: "Run on [Worker name]"

Ramforze:
  -> creates task from user input
  -> negotiates with Governor
  -> dispatches on approval
  -> streams stdout and stderr back to Master UI in real time
```

**Example use case:** User wants to run a heavy data processing script. They point Ramforze at the script, attach an input file, click run. The script executes on the Worker and output appears in the Ramforze terminal view on the Master.

---

## 8. Fault Tolerance

### Worker drops mid-task

1. Master's TCP keepalive detects severed connection (within approximately 25 seconds).
2. Master reads the journal and finds the task was `dispatched` to this Worker.
3. Master marks task as `failed` in the journal and re-queues it locally.
4. Master UI shows: "Connection to [Worker] lost. Task re-queued locally."

### Master crashes mid-task

1. Worker detects TCP connection drop via keepalive.
2. Worker's Governor aborts the running task process.
3. Governor expires the capability token and frees reserved resources.
4. Worker returns to idle. No resource leak.
5. On Master restart: journal recovery flow runs (see Section 6.7).

### Task exceeds max_duration_seconds

1. Governor's background watcher checks token expiry every 5 seconds.
2. If a task is still running when `expires_at` is reached:
   - Governor sends SIGKILL to the task process.
   - Governor sends a `TASK_TIMEOUT` message to the Master.
   - Master marks the task `timed_out` in the journal and re-queues it locally.

### Worker rejects task (insufficient resources)

1. Worker sends rejection response with reason.
2. Master logs the rejection in the journal.
3. Master re-queues the task chunk to run locally.
4. No partial execution occurred on the Worker.

### Required tool not available on Worker

1. Governor's tool check fails during negotiation.
2. Governor sends: `{ "task_id": "uuid", "status": "rejected", "reason": "tool_not_available", "tool": "clang" }`.
3. Master re-queues locally and surfaces a warning in the UI.
4. This task type is not attempted on this Worker again for the session.

---

## 9. Security Model

### Shared Passphrase (Handshake Authentication)

During the handshake on `:7946`, the Master sends a HMAC of its `master_id` computed using the shared passphrase as the key. The Worker verifies it. If verification fails, the connection is rejected immediately. This prevents any machine on the same LAN from connecting to a Worker without prior authorization.

The passphrase is set by the user in the Ramforze UI on both machines before the first connection. Auto-generation with QR code sharing is a future polish milestone.

### Capability Token Forgery Prevention

Tokens are HMAC-SHA256 signed using the shared passphrase. A Master cannot fabricate a valid token. It must go through the Governor's negotiation flow for every single task. The Worker verifies the HMAC signature on every task envelope before execution begins.

### Resource Isolation

Each task runs with the CPU and RAM ceiling that the Governor approved. The Governor reserves these resources at negotiation time and does not issue overlapping tokens that would exceed safe headroom. The Worker's own OS and applications are protected by the system buffer baked into the Governor's headroom calculation.

### No Internet Communication

Ramforze never communicates outside the LAN. There are no external servers, no telemetry, and no cloud dependencies. All data stays between the two machines.

---

## 10. UI Layer

### Stack

- **Frontend:** SwiftUI (macOS native)
- **Backend:** Go binary
- **Bridge:** Unix domain socket at `~/.ramforze/ramforze.sock`

SwiftUI sends commands and receives state updates over the Unix socket. The Go backend handles all networking, task management, BLE, and journal operations. Swift never touches the network directly.

### Master UI: Key Screens

**Dashboard:**
```
+------------------------------------------------+
|  Ramforze                               Live   |
+------------------------------------------------+
|  Your IP : 192.168.1.10                        |
|  Connected: Arjun's MacBook (192.168.1.42)     |
+------------------------------------------------+
|  ACTIVE TASKS                                  |
|  task-001  compilation  main.c  -> Worker      |
|  [||||||||  ] 80%  ~12s remaining              |
+------------------------------------------------+
|  LOCAL RESOURCES        WORKER RESOURCES       |
|  RAM  ||||  6144 / 8192 MiB  RAM  ||   4096 / 16384 MiB |
|  CPU  |||   62%          CPU  |     18%        |
+------------------------------------------------+
|  [Manual Mode]    [Settings]                   |
+------------------------------------------------+
```

**Manual Mode panel:**
```
  Command : [________________________________]
  Args    : [________________________________]
  Files   : [+ Add input file              ]
            [  Run on Arjun's MacBook      ]
```

**Assisted Mode notification:** Native macOS banner via UNUserNotificationCenter.

### Worker UI: Key Screens

**Governor Status:**
```
+------------------------------------------------+
|  Ramforze  (Worker Mode)                Live   |
+------------------------------------------------+
|  Accepting connections on :7946                |
|  Your IP : 192.168.1.42  (share this)          |
+------------------------------------------------+
|  CONNECTED MASTERS                             |
|  Chaitanya's Mac   :7947   1 task running      |
|  (no other masters)                            |
+------------------------------------------------+
|  RESOURCES                                     |
|  RAM  ||||||||||   10240 / 16384 MiB               |
|  CPU  |||          28%                         |
|  Offering to peers: 4096 MiB RAM, 35% CPU         |
+------------------------------------------------+
```

---

## 11. Project Structure

```
ramforze/
├── cmd/
│   ├── master/           # Master entrypoint
│   └── worker/           # Worker entrypoint
├── internal/
│   ├── ble/              # BLE advertising and scanning
│   ├── transport/        # TCP connection management, keepalive
│   ├── handshake/        # Port allocation, connection registry, auth
│   ├── governor/         # Resource negotiation, token issuance, expiry watcher
│   ├── token/            # UUID + HMAC token generation and verification
│   ├── journal/          # Task journal read/write (ndjson)
│   ├── queue/            # Worker priority queue
│   ├── dispatcher/       # Task chunking and dispatch logic
│   ├── modes/
│   │   ├── transparent/  # Compiler wrapper integration
│   │   ├── assisted/     # Resource monitor and notification trigger
│   │   └── manual/       # Remote execution launcher
│   ├── result/           # Result receipt and file transfer
│   └── socket/           # Unix socket bridge to SwiftUI
├── pkg/
│   └── types/            # Shared types: Task, Token, JournalEntry, etc.
├── swift/                # SwiftUI frontend (Xcode project)
│   ├── Views/
│   ├── Models/
│   ├── Bridge/           # Unix socket client in Swift
│   └── BLEBridge/        # CoreBluetooth advertiser and scanner
├── scripts/
│   └── install-wrappers.sh   # Installs ramforze-clang etc. into PATH
└── design.md
```

---

## 12. Data Formats

### Handshake HELLO (Master to Worker on :7946)
```json
{
  "master_id": "uuid",
  "master_ip": "192.168.1.10",
  "protocol_version": "1.0",
  "auth_hmac": "<hmac-of-master_id-using-passphrase>"
}
```

### Handshake RESPONSE (Worker to Master)
```json
{
  "worker_id": "uuid",
  "dedicated_port": 7947,
  "status": "connected"
}
```

### Negotiation REQUEST (Master to Governor on dedicated port)
```json
{
  "task_id": "uuid",
  "master_id": "uuid",
  "resource_ask": {
    "ram_mib": 2048,
    "cpu_pct": 40,
    "max_duration_seconds": 120
  },
  "required_tool": "clang",
  "priority": 2,
  "parallel_safe": true
}
```

### Negotiation RESPONSE: Approved (Governor to Master)
```json
{
  "task_id": "uuid",
  "status": "approved",
  "token_id": "uuid",
  "token_value": "<hmac-signature>",
  "max_duration_seconds": 120,
  "expires_at": "2024-01-01T12:05:00Z"
}
```

### Negotiation RESPONSE: Rejected (Governor to Master)
```json
{
  "task_id": "uuid",
  "status": "rejected",
  "reason": "insufficient_resources_and_not_parallel_safe"
}
```

### Task Envelope (Master to Worker on dedicated port)
```json
{
  "task_id": "uuid",
  "master_id": "uuid",
  "token_id": "uuid",
  "token_value": "<hmac-signature>",
  "type": "compilation",
  "mode": "transparent",
  "payload": {
    "tool": "clang",
    "args": ["-O2", "-c", "main.c", "-o", "main.o"],
    "input_files": ["main.c"],
    "output_files": ["main.o"]
  },
  "resource_ask": {
    "ram_mib": 2048,
    "cpu_pct": 40,
    "max_duration_seconds": 120
  },
  "priority": 2,
  "parallel_safe": true,
  "created_at": "2024-01-01T12:00:00Z"
}
```

### Task Result: Success (Worker to Master)
```json
{
  "task_id": "uuid",
  "status": "success",
  "output_files": ["main.o"],
  "stdout": "...",
  "stderr": "",
  "exit_code": 0
}
```

### Task Result: Failure (Worker to Master)
```json
{
  "task_id": "uuid",
  "status": "failed",
  "reason": "tool_exited_nonzero",
  "exit_code": 1,
  "stderr": "error: use of undeclared identifier 'foo'"
}
```

### Task Timeout Notification (Governor to Master)
```json
{
  "task_id": "uuid",
  "status": "timed_out",
  "max_duration_seconds": 120,
  "reason": "task_exceeded_max_duration"
}
```

This is a separate wire message from `TaskResult`, not a result variant.

---

## 13. Future Milestones

These are explicitly out of scope for MVP but are natural next steps once the core system is working.

| Milestone | Description |
|---|---|
| **Cross-router support** | NAT traversal via STUN/TURN or a lightweight relay server. Enables peers on different routers. |
| **Wi-Fi Direct / P2P transport** | Replace LAN TCP with true peer-to-peer Wi-Fi. No shared router required. |
| **Passphrase QR code** | Generate a QR code in the Worker UI encoding the IP and passphrase. Master scans to connect instantly. |
| **Windows support** | Port Go backend. Replace SwiftUI with a cross-platform UI (e.g. Wails). |
| **Linux support** | Same as Windows. BLE is simpler on Linux via BlueZ. |
| **Additional transparent task types** | Rust (cargo build), Make, Gradle. Each requires its own wrapper binary. |
| **Multi-chunk parallelism** | Master splits one large compilation into N chunks dispatched to multiple Workers simultaneously. |
| **Worker pool** | Master maintains connections to more than one Worker for higher parallelism. |
| **Task history and analytics** | Persistent log of all completed tasks: time saved, data transferred, success rate. Visualized in UI. |
| **MessagePack serialization** | Replace JSON with MessagePack for lower wire overhead on large task payloads. |
