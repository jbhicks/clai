# Model Loading Monitoring - 120B Model with 131K Context

## Model Details
- **Model:** kldzj_gpt-oss-120b-heretic-Q8_0 (120B parameters, MoE with 128 experts)
- **Size:** 59 GB (split GGUF, 2 parts)
- **Configuration:** 
  - Context: `-c 131072` (131K tokens)
  - Flash Attention: `-fa on`
  - GPU offload: `-ngl 999` (all 37/37 layers)
  - Batch size: `-b 2048 -ub 512`

## Hardware
- **CPU:** Strix Halo (32 threads)
- **GPU:** Radeon 8060S Graphics (gfx1151, ROCm 6.18.2)
- **RAM:** 128 GB unified memory architecture
- **Build:** llama.cpp-rocm-wmma (optimized for AMD with WMMA)

## Loading Timeline

### Start: 2026-01-01 19:42:57
- Process started on port 8082
- PID: 3593421

### First Hour
- **00:01:45** - RAM: 48.1 GB, CPU: 74.1% - Model tensors loading
- **00:06:49** - RAM: 51.5 GB, CPU: 91.1% - Tensors loaded, KV cache init started
- **00:12:33** - RAM: 50.9 GB, CPU: 95.1% - KV cache allocation phase
- **01:00:00** - RAM: ~51 GB, CPU: 96-99% - Still in KV cache init

### Second Hour  
- **02:00:28** - RAM: 52.6 GB, CPU: 99.3% - Still loading (SLOW!)
- **02:03:45** - RAM: 50.1 GB, CPU: 99.3% - Memory fluctuating

**Status:** Single-threaded KV cache initialization taking extremely long

## Root Cause Analysis

The loading is stuck in **Flash Attention KV cache initialization** for the 131K context window. Evidence:

1. ✅ Model tensors loaded successfully (59.85 GB to GPU in ~10 minutes)
2. ❌ KV cache allocation taking 2+ hours and counting
3. 🔍 Only main thread (PID 3593421) using 99% CPU - single-threaded bottleneck
4. 🔍 No VMM support on Strix Halo gfx1151 - less efficient memory allocation

## Monitoring Setup

### Automated Monitor
- **Script:** `/home/josh/clai/monitor_model_load.sh`
- **Duration:** 12 hours (until 2026-01-02 09:49:10 AM)
- **Check interval:** Every 5 minutes
- **Log file:** `/home/josh/clai/model_load_monitor.log`

### Quick Status Check
- **Script:** `/home/josh/clai/check_load_status.sh`
- **Usage:** Run anytime to see current status
- Shows: CPU, RAM, elapsed time, health status, and recent log entries

### Live UI
- **URL:** http://localhost:8081
- **GPU Status section** shows real-time:
  - ⏳ Loading indicator
  - CPU usage percentage
  - RAM usage in GB
  - GPU memory usage

## Expected Outcomes

### Best Case
- Model finishes loading in 3-5 hours total
- Final RAM usage: ~60 GB
- Status changes from ⏳ to 🟢
- Health endpoint returns 200 OK

### Likely Case  
- Model takes 6-8 hours to load
- May hit memory limits or timeout
- Will document exact load time for this configuration

### Worst Case
- Never finishes (stuck in infinite loop)
- Requires kill and restart with smaller context
- Alternative: `-c 8192` loads in <5 minutes

## Lessons Learned

1. **131K context is impractical** for loading (even if inference works)
2. **Flash Attention has significant init overhead** for large contexts
3. **No VMM on Strix Halo** impacts large memory allocations
4. **Recommend starting with 8K-32K context** for reasonable load times

## Next Steps After Completion/Timeout

1. Document final load time (or timeout)
2. Test with smaller contexts (8K, 16K, 32K, 64K)
3. Measure load time vs context size relationship
4. Consider disabling FA during load (`-fa off`) for faster startup

---

**Monitor Status:** Running
**Next Check:** Every 5 minutes
**End Time:** 2026-01-02 09:49:10 AM MST (12 hours from start)
