# GPT-OSS-120B Added to Testing Suite

## Update: December 27, 2025

Added **GPT-OSS-120B** to the recommended models for testing.

### Why GPT-OSS-120B?

- **Proven on Strix Halo**: Community reports show it runs "unexpectedly fast" on Strix Halo
- **Reasoning capabilities**: OpenAI's open-weight model designed for agentic tasks
- **Tool calling support**: Listed in Ollama's official tool-calling models
- **Size**: 68GB at Q4_K_M quantization (fits in Strix Halo memory)

### Updated Priority List:

| Priority | Model | Size | Notes |
|----------|-------|------|-------|
| **#1** | Hermes 3 8B | 4.9GB | Quick test - specialized for tool calling |
| **#2** | Llama 3.1 8B | 4.9GB | Quick test - Meta official support |
| **#3** | Mistral Nemo 12B | 7.1GB | Good balance of size/capability |
| **#4** | GPT-OSS-120B | 68GB | **Large but proven fast on Strix Halo** |
| #5 | Qwen 2.5 14B | 8.5GB | General purpose fallback |
| #6 | Llama 3.1 70B | 40GB | Most capable general model |

### Download GPT-OSS-120B:

```bash
./download_models.sh
# Select option 2, then choose GPT-OSS-120B from the list
```

Or directly:
```bash
cd /home/josh/models
wget -c https://huggingface.co/bartowski/gpt-oss-120b-GGUF/resolve/main/gpt-oss-120b-Q4_K_M.gguf
```

### Test GPT-OSS-120B:

```bash
./test_models.sh gpt-oss-120b-Q4_K_M.gguf
```

### Expected Performance:

Based on Reddit community reports with Strix Halo:
- **Speed**: Faster than expected for a 120B model
- **Quality**: Strong reasoning and tool use
- **Context**: Good long-context performance

### Testing Strategy:

1. **Start with smaller models** (Hermes 3, Llama 3.1 8B) to establish baseline
2. **If small models achieve >70% success**, use them (faster, easier to manage)
3. **If small models struggle**, test GPT-OSS-120B as heavy-duty option
4. GPT-OSS-120B may be overkill for simple tool calling but excellent for complex reasoning

### Disk Space Note:

Total download size for all models is now **~134GB** (was 66GB before adding GPT-OSS-120B).

Ensure you have adequate space:
```bash
df -h /home/josh/models
```
