package gpu

import (
	"strings"
	"testing"
)

func TestGetGPUInfo(t *testing.T) {
	gpus, err := GetGPUInfo()
	if err != nil {
		t.Skipf("GPU info not available: %v", err)
		return
	}

	if len(gpus) == 0 {
		t.Error("Expected at least one GPU")
		return
	}

	gpu := gpus[0]

	// Basic validation
	if gpu.ID == "" {
		t.Error("GPU ID should not be empty")
	}

	if gpu.VRAMTotalBytes <= 0 {
		t.Error("VRAM total should be positive")
	}

	if gpu.VRAMUsedBytes < 0 {
		t.Error("VRAM used should not be negative")
	}

	if gpu.VRAMUsedPercent < 0 || gpu.VRAMUsedPercent > 100 {
		t.Errorf("VRAM used percent should be 0-100, got %.2f", gpu.VRAMUsedPercent)
	}

	t.Logf("GPU Info: %+v", gpu)
}

func TestGetGPUInfoSimple(t *testing.T) {
	info, err := GetGPUInfoSimple()
	if err != nil {
		t.Skipf("GPU info not available: %v", err)
		return
	}

	if info == "" {
		t.Error("GPU info string should not be empty")
	}

	// Should contain key information
	if !strings.Contains(info, "Memory") {
		t.Error("GPU info should contain Memory information")
	}

	if !strings.Contains(info, "Temp") {
		t.Error("GPU info should contain temperature information")
	}

	if !strings.Contains(info, "Util") {
		t.Error("GPU info should contain utilization information")
	}

	t.Logf("GPU Info String: %s", info)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{536870912, "512.0 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestGetProcessGPUMemory(t *testing.T) {
	processes, err := GetProcessGPUMemory()
	if err != nil {
		t.Skipf("Process GPU info not available: %v", err)
		return
	}

	// Should have at least some processes if GPUs are in use
	t.Logf("Found %d GPU processes", len(processes))

	for _, proc := range processes {
		if proc.PID <= 0 {
			t.Errorf("Invalid PID: %d", proc.PID)
		}
		if proc.ProcessName == "" {
			t.Error("Process name should not be empty")
		}
		if proc.VRAMUsed < 0 {
			t.Errorf("VRAM used should not be negative: %d", proc.VRAMUsed)
		}

		t.Logf("Process %d (%s): VRAM %s on GPU %s",
			proc.PID, proc.ProcessName, FormatBytes(proc.VRAMUsed), proc.GPUIDs)
	}
}

func TestParseProcessGPUInfo(t *testing.T) {
	// Test with sample rocm-smi --showpids output
	sampleOutput := `WARNING: AMD GPU device(s) is/are in a low-power state. Check power control/runtime_status



============================ ROCm System Management Interface ============================
===================================== KFD Processes ======================================
KFD process information:
PID   	PROCESS NAME	GPU(s)	VRAM USED  	SDMA USED	CU OCCUPANCY	
2728  	llama-server	1     	302239744  	0        	UNKNOWN     	
417281	llama-server	1     	22630404096	0        	UNKNOWN     	
555979	llama-server	1     	45188182016	0        	UNKNOWN     	
==========================================================================================
================================== End of ROCm SMI Log ===================================`

	processes, err := parseProcessGPUInfo(sampleOutput)
	if err != nil {
		t.Fatalf("Failed to parse process info: %v", err)
	}

	if len(processes) != 3 {
		t.Errorf("Expected 3 processes, got %d", len(processes))
	}

	// Check first process
	if len(processes) > 0 {
		p := processes[0]
		if p.PID != 2728 {
			t.Errorf("Expected PID 2728, got %d", p.PID)
		}
		if p.ProcessName != "llama-server" {
			t.Errorf("Expected process name 'llama-server', got '%s'", p.ProcessName)
		}
		if p.VRAMUsed != 302239744 {
			t.Errorf("Expected VRAM 302239744, got %d", p.VRAMUsed)
		}
		if p.GPUIDs != "1" {
			t.Errorf("Expected GPU ID '1', got '%s'", p.GPUIDs)
		}
	}
}

func TestGetProcessVRAM(t *testing.T) {
	processes, err := GetProcessGPUMemory()
	if err != nil {
		t.Skipf("Process GPU info not available: %v", err)
		return
	}

	if len(processes) == 0 {
		t.Skip("No GPU processes running")
		return
	}

	// Test with a known PID from the list
	testPID := processes[0].PID
	vram, err := GetProcessVRAM(testPID)
	if err != nil {
		t.Fatalf("Failed to get VRAM for PID %d: %v", testPID, err)
	}

	if vram != processes[0].VRAMUsed {
		t.Errorf("Expected VRAM %d, got %d", processes[0].VRAMUsed, vram)
	}

	// Test with non-existent PID
	vramNonExistent, err := GetProcessVRAM(999999)
	if err != nil {
		t.Fatalf("Should not error for non-existent PID: %v", err)
	}
	if vramNonExistent != 0 {
		t.Errorf("Expected 0 VRAM for non-existent PID, got %d", vramNonExistent)
	}
}
