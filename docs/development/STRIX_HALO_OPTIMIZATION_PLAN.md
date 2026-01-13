# Strix Halo Optimization & Model Implementation Plan

## System Analysis Complete ✅

**Current Status:**
- ✅ ROCm 7.0 installed at `/opt/rocm`
- ✅ GPU detected (Radeon 8060S, 80% VRAM usage)
- ✅ CLAI codebase functional with Go 1.25.2
- ❌ Missing kernel optimizations for performance
- ❌ Missing required tools (huggingface-cli, toolbox)
- ✅ Build errors fixed (Go syntax issues resolved)

## Recommended Model: Qwen3-Coder-30B-A3B

**Why this model is optimal for Strix Halo:**

1. **Performance Sweet Spot**: 30B total parameters with only 3B active parameters (MoE)
   - 78 tokens/s text generation (fastest in class)
   - 1216 tokens/s prompt processing with batching
   - Uses ~17GB VRAM efficiently

2. **Coding Excellence**: 
   - #1 ranked open-source coding model
   - Beats DeepSeek V3 on coding benchmarks
   - HumanEval: ~65% accuracy

3. **Hardware Efficiency**:
   - MoE architecture perfect for ROCm + rocWMMA
   - Fits comfortably in 128GB unified memory
   - Excellent performance with flash attention

4. **Cost-Effective**: Open source MIT license, no API costs

## Implementation Phases

### Phase 1: System Optimization (Immediate - Required)

#### 1.1 Kernel Parameters
Add to `/etc/default/grub`:
```bash
GRUB_CMDLINE_LINUX="amd_iommu=off amdgpu.gttsize=124000 ttm.pages_limit=31744000"
```

Apply changes:
```bash
sudo grub2-mkconfig -o /boot/grub2/grub.cfg
sudo reboot
```

#### 1.2 GPU Memory Configuration
Create `/etc/modprobe.d/amdgpu_llm_optimized.conf`:
```bash
# Maximize GTT for LLM usage on 128GB UMA system
options amdgpu gttsize=120000
options ttm pages_limit=31457280
options ttm page_pool_size=15728640
```

#### 1.3 Performance Profile
```bash
paru -S tuned
sudo systemctl enable --now tuned
sudo tuned-adm profile accelerator-performance
```

**Expected Impact**: 15-20% performance boost, especially for prompt processing

### Phase 2: Tool Installation

#### 2.1 HuggingFace CLI
```bash
paru -S python-huggingface-hub
pip install --upgrade hf-transfer
```

#### 2.2 Toolbox System (Alternative to Docker)
```bash
paru -S toolbox
```

### Phase 3: Model Implementation

#### 3.1 Download Qwen3-Coder-30B-A3B
```bash
# Set ROCm environment
source /opt/rocm/setenv.sh

# Download model (57GB total, ~17GB active)
HF_HUB_ENABLE_HF_TRANSFER=1 huggingface-cli download \
  unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF \
  BF16/Qwen3-Coder-30B-A3B-Instruct-BF16-00001-of-00002.gguf \
  --local-dir /home/josh/models/
```

#### 3.2 CLAI Integration
Modify `internal/llm/llm.go` to support:
- Backend selection (ROCm vs Vulkan)
- Flash attention flags (`-fa 1 --no-mmap`)
- MoE model optimizations (`-b 256` batching)
- Model-specific parameter handling

#### 3.3 Testing Commands
```bash
# ROCm backend (recommended for longer contexts)
./clai benchmark --model /path/to/Qwen3-Coder-30B.gguf --backend rocm

# Vulkan backend (faster for quick prompts)
./clai benchmark --model /path/to/Qwen3-Coder-30B.gguf --backend vulkan
```

### Phase 4: Performance Validation

#### 4.1 Benchmark Targets
- **Token Generation**: 70-80 tokens/s sustained
- **Prompt Processing**: 1000+ tokens/s with batching
- **Memory Usage**: ~17GB + reasonable context
- **Coding Quality**: Test on practical coding tasks

#### 4.2 Validation Tests
1. **Coding benchmarks**: HumanEval-style problems
2. **Context handling**: 4K, 8K, 16K token contexts
3. **Batch size optimization**: Test `-b 256` vs default
4. **Backend comparison**: ROCm vs Vulkan performance

## Alternative Models (if Qwen3 unavailable)

### DeepSeek-V3 (671B/37B active)
- HumanEval: 65.2%
- Similar performance profile
- Better reasoning capabilities
- Download: `unsloth/DeepSeek-V3-GGUF`

### Llama-4-Scout (109B/17B active)
- Newer architecture
- 20.2 tokens/s generation
- Good for large context tasks

### Qwen2.5-Coder-32B
- HumanEval: 92.7% (highest accuracy)
- Solid backup option
- Download: `unsloth/Qwen2.5-Coder-32B-Instruct-GGUF`

## Expected Performance Gains

### System Optimizations
- **Memory bandwidth**: +6% (amd_iommu=off)
- **Prompt processing**: +5-8% (tuned profile)
- **Overall throughput**: +15-20% combined

### Model Performance
- **vs current models**: 3-4x faster token generation
- **Coding accuracy**: Top-tier open-source performance
- **Resource efficiency**: MoE advantage for large models

## Configuration Files Reference

### `/etc/default/grub`
```bash
# Backup current config
sudo cp /etc/default/grub /etc/default/grub.backup

# Add to existing GRUB_CMDLINE_LINUX
GRUB_CMDLINE_LINUX="amd_iommu=off amdgpu.gttsize=124000 ttm.pages_limit=31744000"
```

### `/etc/modprobe.d/amdgpu_llm_optimized.conf`
```bash
# Created with:
sudo tee /etc/modprobe.d/amdgpu_llm_optimized.conf << 'EOF'
options amdgpu gttsize=120000
options ttm pages_limit=31457280
options ttm page_pool_size=15728640
EOF
```

### ROCm Environment
```bash
# Add to ~/.bashrc or run before CLAI
source /opt/rocm/setenv.sh
export ROCBLAS_USE_HIPBLASLT=1
```

## Troubleshooting

### If Build Errors Occur
1. Check Go version: `go version` (needs 1.22+ for `:=` shorthand)
2. Clean build: `go clean && go build`
3. Check dependencies: `go mod tidy && go mod download`

### If Model Loading Fails
1. Verify ROCm environment: `rocm-smi`
2. Check file permissions: `ls -la /path/to/model.gguf`
3. Test with smaller context: `-c 4096` vs default

### If Performance Poor
1. Verify kernel params: `cat /proc/cmdline`
2. Check GPU utilization: `watch -n 1 rocm-smi`
3. Test different backends: `--backend vulkan` vs `--backend rocm`

## Next Steps

1. **Immediate**: Apply kernel optimizations and reboot
2. **Short-term**: Install tools, download model
3. **Medium-term**: Integrate with CLAI, validate performance
4. **Long-term**: Consider multi-model support and backend auto-detection

---

**Created**: 2025-01-08  
**System**: AMD Ryzen AI Max+ 395 (Strix Halo) / 128GB RAM / Arch Linux  
**ROCm Version**: 7.0 (installed at /opt/rocm)  
**Status**: Ready for implementation