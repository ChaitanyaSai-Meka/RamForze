package governor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type RAMInfo struct {
	TotalMiB uint64
	UsedMiB  uint64
	FreeMiB  uint64
}

type CPUInfo struct {
	UsedPct uint32
}

func GetRAMInfo() (RAMInfo, error) {
	totalBytes, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return RAMInfo{}, fmt.Errorf("could not read total RAM: %w", err)
	}
	totalMiB := totalBytes / 1024 / 1024

	usedMiB, err := getUsedRAMMiB()
	if err != nil {
		return RAMInfo{}, fmt.Errorf("could not read used RAM: %w", err)
	}

	freeMiB := uint64(0)
	if totalMiB > usedMiB {
		freeMiB = totalMiB - usedMiB
	}

	return RAMInfo{
		TotalMiB: totalMiB,
		UsedMiB:  usedMiB,
		FreeMiB:  freeMiB,
	}, nil
}


func getUsedRAMMiB() (uint64, error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, fmt.Errorf("could not run vm_stat: %w", err)
	}

	const pageSizeBytes uint64 = 4096

	var activePages, wiredPages uint64

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Pages active:") {
			activePages, err = parseVMStatLine(line)
			if err != nil {
				return 0, err
			}
		} else if strings.HasPrefix(line, "Pages wired down:") {
			wiredPages, err = parseVMStatLine(line)
			if err != nil {
				return 0, err
			}
		}
	}

	usedBytes := (activePages + wiredPages) * pageSizeBytes
	return usedBytes / 1024 / 1024, nil
}

func parseVMStatLine(line string) (uint64, error) {
	parts := strings.Split(line, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected vm_stat line format: %q", line)
	}
	raw := strings.TrimSpace(parts[1])
	raw = strings.TrimSuffix(raw, ".")
	val, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse vm_stat value %q: %w", raw, err)
	}
	return val, nil
}

func GetCPUInfo() (CPUInfo, error) {
	out, err := exec.Command("ps", "-A", "-o", "%cpu").Output()
	if err != nil {
		return CPUInfo{}, fmt.Errorf("could not run ps: %w", err)
	}

	var total float64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "%CPU" {
			continue
		}
		val, err := strconv.ParseFloat(line, 64)
		if err != nil {
			continue
		}
		total += val
	}

	if total > 100 {
		total = 100
	}

	return CPUInfo{UsedPct: uint32(total)}, nil
}


func SafeRAMMiB(info RAMInfo) uint64 {
	const bufferMiB uint64 = 1536
	if info.FreeMiB <= bufferMiB {
		return 0
	}
	return info.FreeMiB - bufferMiB
}

func SafeCPUPct(info CPUInfo) uint32 {
	const bufferPct uint32 = 15
	if info.UsedPct+bufferPct >= 100 {
		return 0
	}
	return 100 - info.UsedPct - bufferPct
}