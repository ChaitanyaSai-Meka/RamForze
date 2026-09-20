package workergovernor

import (
	"fmt"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type ResourceStats struct {
	RAMHeadroomMiB float64
	CPUHeadroomPercent float64
}

const (
    ramBufferMiB     = 1536
    cpuBufferPercent = 15
)

func HeadRoomStats() (*ResourceStats,error){
	memStats,err:= getMemoryStats()
	if err != nil {
		return nil,err
	
	}
	cpuStats,err:= getCpuStats()
	if err != nil {
		return nil,err
	}
	return &ResourceStats{
		RAMHeadroomMiB: float64(memStats.Available)/(1024*1024) - ramBufferMiB,
		CPUHeadroomPercent: (100 - cpuBufferPercent) - cpuStats,
	},nil
}

func getMemoryStats() (*mem.VirtualMemoryStat,error){
	memInfo,err := mem.VirtualMemory()
	if err != nil {
		return nil,fmt.Errorf("failed to get memory stats: %w", err)
	}
	return memInfo,nil
}

func getCpuStats() (float64,error) {
	cpuInfo,err:= cpu.Percent(0,false)
	if err != nil {
		return 0,fmt.Errorf("failed to get CPU Stats: %w", err)
	}
	return cpuInfo[0],nil
}

