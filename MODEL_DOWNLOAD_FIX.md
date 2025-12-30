# Model Download Fix - Autocomplete Selection Issue

## Problem
When selecting a model from the autocomplete dropdown, **nothing happened**. No download started.

### Root Cause
The autocomplete returned **HuggingFace repository names** (e.g., `MaziyarPanahi/Qwen3-4B-GGUF`), but the download handler expected **direct .gguf file URLs** (e.g., `https://huggingface.co/.../file.gguf`).

The validation check at `download_manager.go:304` rejected repository names because they didn't contain `huggingface.co`:

```go
// OLD CODE - Failed for repo names
if !strings.Contains(downloadURL, "huggingface.co") {
    return error
}
```

## Solution

### 1. Enhanced Download Handler (`internal/benchmark/download_manager.go`)

**Added repository detection:**
- Checks if input is a URL (`http://` or `https://`) or a repository name
- If it's a repository name, fetch the list of .gguf files from HuggingFace API
- Present user with a file selection interface

**New functions:**
- `fetchRepoFiles(repoID string)` - Fetches files from HuggingFace repository API
- Uses endpoint: `https://huggingface.co/api/models/{repo}/tree/main`
- Filters for `.gguf` files only
- Returns file names, URLs, and sizes

### 2. File Selection UI

When you select a repository from autocomplete and click "Download", you now see:

```
Select a file to download from MaziyarPanahi/Qwen3-4B-GGUF:

┌─────────────────────────────────────┐
│ Qwen3-4B.Q2_K.gguf                  │
│ 1.6 GB                              │
├─────────────────────────────────────┤
│ Qwen3-4B.Q3_K_M.gguf                │
│ 1.9 GB                              │
├─────────────────────────────────────┤
│ Qwen3-4B.Q4_K_M.gguf                │
│ 2.3 GB                              │
└─────────────────────────────────────┘
```

Each button triggers the actual download with the direct file URL.

## How It Works Now

### Flow 1: Repository Selection
```
1. User types "qwen" in search → autocomplete shows repos
2. User selects "MaziyarPanahi/Qwen3-4B-GGUF" → fills input
3. User clicks "Download" button
4. Backend detects it's a repo name (no http://)
5. Backend calls HuggingFace API to list files
6. UI shows file selection buttons
7. User clicks a file button
8. Download starts with direct URL
```

### Flow 2: Direct URL
```
1. User pastes full URL: https://huggingface.co/.../file.gguf
2. User clicks "Download" button
3. Backend detects it's a URL (starts with https://)
4. Download starts immediately
```

## Testing

### Test Repository Selection
```bash
curl -X POST "http://localhost:8080/api/models/download" \
  -d "url=MaziyarPanahi/Qwen3-4B-GGUF"
```

**Expected:** HTML file selection UI with list of .gguf files

### Test Direct URL Download
```bash
curl -X POST "http://localhost:8080/api/models/download" \
  -d "url=https://huggingface.co/MaziyarPanahi/Qwen3-4B-GGUF/resolve/main/Qwen3-4B.Q2_K.gguf"
```

**Expected:** Success message: "Download started for Qwen3-4B.Q2_K.gguf"

### Check Download Progress
```bash
curl "http://localhost:8080/api/models/downloads"
```

**Expected:** Active downloads list with progress bars, speed, percentage

## Code Changes

### Modified Files
- `internal/benchmark/download_manager.go`:
  - Added `encoding/json` import
  - Added `RepoFile` struct
  - Added `fetchRepoFiles()` function
  - Modified `handleDownloadModel()` to handle both repos and URLs

### New Features
1. **Smart input detection** - Distinguishes between repo names and direct URLs
2. **Repository file browser** - Lists all .gguf files in a repo
3. **File size display** - Shows human-readable sizes (GB, MB)
4. **Interactive selection** - Click to download specific quantization

## User Experience

### Before
```
1. Search → Select repo → Click Download → ❌ Error: "Only Hugging Face URLs supported"
2. User confused - has to manually find .gguf file URL
```

### After
```
1. Search → Select repo → Click Download → ✅ Shows file list
2. Click desired file → ✅ Download starts with progress bar
```

## Future Enhancements

Potential improvements:
1. **Auto-select** if only one .gguf file in repo
2. **Recommended quantizations** - Highlight Q4_K_M or Q5_K_M (best quality/size)
3. **Model size warnings** - Alert if model won't fit in available VRAM
4. **Folder organization** - Group files by quantization type
5. **Search within files** - Filter .gguf files by quantization (Q4, Q5, etc.)

## Related Documentation
- HuggingFace API: https://huggingface.co/docs/hub/api
- GGUF quantization: https://github.com/ggerganov/llama.cpp/blob/master/examples/quantize/README.md
