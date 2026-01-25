package gpu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GPUInfo represents GPU statistics
type GPUInfo struct {
	ID              string  // GPU identifier (e.g., "GPU0", "card0")
	Name            string  // GPU product name (e.g., "Radeon 8060S Graphics")
	VRAMTotalBytes  int64   // Total VRAM in bytes
	VRAMUsedBytes   int64   // Used VRAM in bytes
	VRAMFreeBytes   int64   // Free VRAM in bytes
	VRAMUsedPercent float64 // Percentage of VRAM used
	GTTTotalBytes   int64   // Total GTT (shared) memory in bytes
	GTTUsedBytes    int64   // Used GTT memory in bytes
	Temperature     float64 // Temperature in Celsius
	Utilization     float64 // GPU utilization percentage (0-100)
	PowerUsageW     float64 // Current power usage in Watts
	ClockGPUMHz     int     // Current GPU clock speed in MHz
	ClockMemMHz     int     // Current memory clock speed in MHz
}

// getGPUProductNames fetches GPU product names from rocm-smi
// Returns a map of cardID -> product name (e.g., "card0" -> "Radeon 8060S Graphics")
func getGPUProductNames() (map[string]string, error) {
	cmd := exec.Command("rocm-smi", "--showproductname")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get product names: %w", err)
	}

	// Parse output like:
	// GPU[0]		: Card Series: 		Radeon 8060S Graphics
	productNames := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		// Look for lines like "GPU[0]		: Card Series: 		Radeon 8060S Graphics"
		if strings.Contains(line, "Card Series:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				// Extract GPU index from "GPU[0]"
				gpuPart := strings.TrimSpace(parts[0])
				if strings.HasPrefix(gpuPart, "GPU[") && strings.Contains(gpuPart, "]") {
					indexStr := strings.TrimPrefix(gpuPart, "GPU[")
					indexStr = strings.TrimSuffix(indexStr, "]")

					// Extract product name (everything after "Card Series:")
					productName := strings.TrimSpace(parts[2])

					// Map GPU[0] to card0, GPU[1] to card1, etc.
					cardID := fmt.Sprintf("card%s", indexStr)
					productNames[cardID] = productName
				}
			}
		}
	}

	return productNames, nil
}

// GetGPUInfo queries AMD GPU statistics using rocm-smi
func GetGPUInfo() ([]GPUInfo, error) {
	// Check if rocm-smi is available
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		return nil, fmt.Errorf("rocm-smi not found: %w", err)
	}

	// Get GPU product names
	productNames, err := getGPUProductNames()
	if err != nil {
		// Non-fatal: continue without product names
		productNames = make(map[string]string)
	}

	// Get all memory info (VRAM + GTT/shared memory)
	vramCmd := exec.Command("rocm-smi", "--showmeminfo", "all", "--json")
	vramOutput, err := vramCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get VRAM info: %w", err)
	}

	// Get temperature and utilization
	tempCmd := exec.Command("rocm-smi", "--showtemp", "--showuse", "--json")
	tempOutput, err := tempCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get temperature/utilization: %w", err)
	}

	// Get power consumption
	powerCmd := exec.Command("rocm-smi", "--showpower", "--json")
	powerOutput, err := powerCmd.Output()
	if err != nil {
		// Power info is optional - log but continue
		powerOutput = []byte("{}")
	}

	// Get clock frequencies
	clockCmd := exec.Command("rocm-smi", "--showclocks", "--json")
	clockOutput, err := clockCmd.Output()
	if err != nil {
		// Clock info is optional - log but continue
		clockOutput = []byte("{}")
	}

	// Parse VRAM info
	var vramData map[string]map[string]string
	if err := json.Unmarshal(vramOutput, &vramData); err != nil {
		return nil, fmt.Errorf("failed to parse VRAM JSON: %w", err)
	}

	// Parse temp/utilization info
	var tempData map[string]map[string]interface{}
	if err := json.Unmarshal(tempOutput, &tempData); err != nil {
		return nil, fmt.Errorf("failed to parse temp JSON: %w", err)
	}

	// Parse power info (optional)
	var powerData map[string]map[string]interface{}
	json.Unmarshal(powerOutput, &powerData) // Ignore errors - optional data

	// Parse clock info (optional)
	var clockData map[string]map[string]interface{}
	json.Unmarshal(clockOutput, &clockData) // Ignore errors - optional data

	var gpus []GPUInfo

	// Combine data from both queries
	for cardID, vramInfo := range vramData {
		gpu := GPUInfo{
			ID:   cardID,
			Name: productNames[cardID], // Use product name if available
		}

		// Parse VRAM total
		if totalStr, ok := vramInfo["VRAM Total Memory (B)"]; ok {
			if total, err := strconv.ParseInt(totalStr, 10, 64); err == nil {
				gpu.VRAMTotalBytes = total
			}
		}

		// Parse VRAM used
		if usedStr, ok := vramInfo["VRAM Total Used Memory (B)"]; ok {
			if used, err := strconv.ParseInt(usedStr, 10, 64); err == nil {
				gpu.VRAMUsedBytes = used
			}
		}

		// Parse GTT (Graphics Translation Table - shared memory) total
		if gttTotalStr, ok := vramInfo["GTT Total Memory (B)"]; ok {
			if gttTotal, err := strconv.ParseInt(gttTotalStr, 10, 64); err == nil {
				gpu.GTTTotalBytes = gttTotal
			}
		}

		// Parse GTT used
		if gttUsedStr, ok := vramInfo["GTT Total Used Memory (B)"]; ok {
			if gttUsed, err := strconv.ParseInt(gttUsedStr, 10, 64); err == nil {
				gpu.GTTUsedBytes = gttUsed
			}
		}

		// Calculate free memory
		gpu.VRAMFreeBytes = gpu.VRAMTotalBytes - gpu.VRAMUsedBytes
		if gpu.VRAMTotalBytes > 0 {
			gpu.VRAMUsedPercent = float64(gpu.VRAMUsedBytes) / float64(gpu.VRAMTotalBytes) * 100
		}

		// Parse temperature and utilization if available
		if tempInfo, ok := tempData[cardID]; ok {
			if temp, ok := tempInfo["Temperature (Sensor edge) (C)"].(string); ok {
				if tempVal, err := strconv.ParseFloat(temp, 64); err == nil {
					gpu.Temperature = tempVal
				}
			}
			if util, ok := tempInfo["GPU use (%)"].(string); ok {
				if utilVal, err := strconv.ParseFloat(util, 64); err == nil {
					gpu.Utilization = utilVal
				}
			}
		}

		// Parse power consumption if available
		if powerInfo, ok := powerData[cardID]; ok {
			// Try both possible power key names
			powerKey := ""
			if _, ok := powerInfo["Current Socket Graphics Package Power (W)"]; ok {
				powerKey = "Current Socket Graphics Package Power (W)"
			} else if _, ok := powerInfo["Average Graphics Package Power (W)"]; ok {
				powerKey = "Average Graphics Package Power (W)"
			}

			if powerKey != "" {
				if power, ok := powerInfo[powerKey].(string); ok {
					if powerVal, err := strconv.ParseFloat(power, 64); err == nil {
						gpu.PowerUsageW = powerVal
					}
				}
			}
		}

		// Parse clock frequencies if available
		if clockInfo, ok := clockData[cardID]; ok {
			// Try to get GPU clock (sclk) - try multiple key formats
			sclkVal := ""
			if val, ok := clockInfo["sclk clock speed:"].(string); ok {
				sclkVal = val
			} else if val, ok := clockInfo["sclk"].(string); ok {
				sclkVal = val
			}

			if sclkVal != "" {
				// Format is like "(961Mhz)" or "1700Mhz" or "1700 Mhz"
				sclkClean := strings.ReplaceAll(strings.ToLower(sclkVal), "mhz", "")
				sclkClean = strings.ReplaceAll(sclkClean, "(", "")
				sclkClean = strings.ReplaceAll(sclkClean, ")", "")
				sclkClean = strings.TrimSpace(sclkClean)
				if sclkInt, err := strconv.Atoi(sclkClean); err == nil {
					gpu.ClockGPUMHz = sclkInt
				}
			}

			// Try to get memory clock (mclk)
			mclkVal := ""
			if val, ok := clockInfo["mclk clock speed:"].(string); ok {
				mclkVal = val
			} else if val, ok := clockInfo["mclk"].(string); ok {
				mclkVal = val
			}

			if mclkVal != "" {
				// Format is like "(96Mhz)" or "96Mhz" or "96 Mhz"
				mclkClean := strings.ReplaceAll(strings.ToLower(mclkVal), "mhz", "")
				mclkClean = strings.ReplaceAll(mclkClean, "(", "")
				mclkClean = strings.ReplaceAll(mclkClean, ")", "")
				mclkClean = strings.TrimSpace(mclkClean)
				if mclkInt, err := strconv.Atoi(mclkClean); err == nil {
					gpu.ClockMemMHz = mclkInt
				}
			}
		}

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}

// FormatBytes converts bytes to human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetGPUInfoSimple returns a simple string representation of GPU stats
func GetGPUInfoSimple() (string, error) {
	gpus, err := GetGPUInfo()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for i, gpu := range gpus {
		if i > 0 {
			sb.WriteString("\n")
		}

		// Calculate total available memory
		totalAvailable := gpu.VRAMTotalBytes
		totalUsed := gpu.VRAMUsedBytes

		if gpu.GTTTotalBytes > gpu.VRAMTotalBytes*2 {
			totalAvailable = gpu.GTTTotalBytes + gpu.VRAMTotalBytes
			totalUsed = gpu.GTTUsedBytes + gpu.VRAMUsedBytes
		}

		// Use GPU name if available, otherwise fall back to ID
		gpuLabel := gpu.ID
		if gpu.Name != "" {
			gpuLabel = gpu.Name
		}

		sb.WriteString(fmt.Sprintf("%s: Memory %s / %s (%.1f%%) | Temp: %.1f°C | Util: %.0f%%",
			gpuLabel,
			FormatBytes(totalUsed),
			FormatBytes(totalAvailable),
			gpu.VRAMUsedPercent,
			gpu.Temperature,
			gpu.Utilization,
		))
	}
	return sb.String(), nil
}

// ProcessGPUInfo represents GPU VRAM usage for a specific process
type ProcessGPUInfo struct {
	PID         int    // Process ID
	ProcessName string // Process name
	VRAMUsed    int64  // VRAM used in bytes
	GPUIDs      string // GPU IDs used by this process
}

// GetProcessGPUMemory queries VRAM usage for all GPU processes using rocm-smi --showpids
func GetProcessGPUMemory() ([]ProcessGPUInfo, error) {
	// Check if rocm-smi is available
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		return nil, fmt.Errorf("rocm-smi not found: %w", err)
	}

	// Run rocm-smi --showpids to get per-process VRAM usage
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "rocm-smi", "--showpids")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get process info: %w", err)
	}

	return parseProcessGPUInfo(string(output))
}

// parseProcessGPUInfo parses the output of rocm-smi --showpids
// Example output:
// ============================ ROCm System Management Interface ============================
// ===================================== KFD Processes ======================================
// KFD process information:
// PID   	PROCESS NAME	GPU(s)	VRAM USED  	SDMA USED	CU OCCUPANCY
// 2728  	llama-server	1     	302239744  	0        	UNKNOWN
// 417281	llama-server	1     	22630404096	0        	UNKNOWN
func parseProcessGPUInfo(output string) ([]ProcessGPUInfo, error) {
	var processes []ProcessGPUInfo

	// Skip to the data section (after "PID   	PROCESS NAME" header)
	lines := strings.Split(output, "\n")
	dataStarted := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and dividers
		if line == "" || strings.HasPrefix(line, "=") {
			continue
		}

		// Look for the header line
		if strings.Contains(line, "PID") && strings.Contains(line, "PROCESS NAME") {
			dataStarted = true
			continue
		}

		// Skip non-data lines
		if !dataStarted || strings.HasPrefix(line, "KFD process") {
			continue
		}

		// Parse data line using tabs as delimiter
		fields := strings.Split(line, "\t")

		// Clean up fields (remove extra whitespace)
		var cleanFields []string
		for _, field := range fields {
			trimmed := strings.TrimSpace(field)
			if trimmed != "" {
				cleanFields = append(cleanFields, trimmed)
			}
		}

		// Need at least PID, ProcessName, GPU(s), VRAM USED
		if len(cleanFields) < 4 {
			continue
		}

		pid, err := strconv.Atoi(cleanFields[0])
		if err != nil {
			continue // Skip invalid PID
		}

		processName := cleanFields[1]
		gpuIDs := cleanFields[2]
		vramUsed, err := strconv.ParseInt(cleanFields[3], 10, 64)
		if err != nil {
			continue // Skip invalid VRAM value
		}

		processes = append(processes, ProcessGPUInfo{
			PID:         pid,
			ProcessName: processName,
			VRAMUsed:    vramUsed,
			GPUIDs:      gpuIDs,
		})
	}

	return processes, nil
}

// GetProcessVRAM returns VRAM usage for a specific PID
func GetProcessVRAM(pid int) (int64, error) {
	processes, err := GetProcessGPUMemory()
	if err != nil {
		return 0, err
	}

	for _, proc := range processes {
		if proc.PID == pid {
			return proc.VRAMUsed, nil
		}
	}

	return 0, nil // Process not using GPU or not found
}
