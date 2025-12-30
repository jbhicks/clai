package benchmark

import (
	"testing"
)

func TestParseSplitModelFilename(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		wantIsSplit    bool
		wantPrefix     string
		wantPartNumber int
		wantTotalParts int
	}{
		{
			name:           "valid split model part 1 of 4",
			filename:       "model-name-00001-of-00004.gguf",
			wantIsSplit:    true,
			wantPrefix:     "model-name",
			wantPartNumber: 1,
			wantTotalParts: 4,
		},
		{
			name:           "valid split model part 2 of 4",
			filename:       "model-name-00002-of-00004.gguf",
			wantIsSplit:    true,
			wantPrefix:     "model-name",
			wantPartNumber: 2,
			wantTotalParts: 4,
		},
		{
			name:           "openai gpt-oss split model",
			filename:       "openai_gpt-oss-120b-Q8_0-00001-of-00002.gguf",
			wantIsSplit:    true,
			wantPrefix:     "openai_gpt-oss-120b-Q8_0",
			wantPartNumber: 1,
			wantTotalParts: 2,
		},
		{
			name:           "regular non-split model",
			filename:       "llama-7b.gguf",
			wantIsSplit:    false,
			wantPrefix:     "",
			wantPartNumber: 0,
			wantTotalParts: 0,
		},
		{
			name:           "model with numbers but not split pattern",
			filename:       "llama-7b-00001.gguf",
			wantIsSplit:    false,
			wantPrefix:     "",
			wantPartNumber: 0,
			wantTotalParts: 0,
		},
		{
			name:           "invalid part number (zero)",
			filename:       "model-00000-of-00004.gguf",
			wantIsSplit:    false,
			wantPrefix:     "",
			wantPartNumber: 0,
			wantTotalParts: 0,
		},
		{
			name:           "invalid part number exceeds total",
			filename:       "model-00005-of-00004.gguf",
			wantIsSplit:    false,
			wantPrefix:     "",
			wantPartNumber: 0,
			wantTotalParts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSplitModelFilename(tt.filename)

			if got.IsSplit != tt.wantIsSplit {
				t.Errorf("parseSplitModelFilename(%q).IsSplit = %v, want %v",
					tt.filename, got.IsSplit, tt.wantIsSplit)
			}

			if got.IsSplit {
				if got.Prefix != tt.wantPrefix {
					t.Errorf("parseSplitModelFilename(%q).Prefix = %q, want %q",
						tt.filename, got.Prefix, tt.wantPrefix)
				}
				if got.PartNumber != tt.wantPartNumber {
					t.Errorf("parseSplitModelFilename(%q).PartNumber = %d, want %d",
						tt.filename, got.PartNumber, tt.wantPartNumber)
				}
				if got.TotalParts != tt.wantTotalParts {
					t.Errorf("parseSplitModelFilename(%q).TotalParts = %d, want %d",
						tt.filename, got.TotalParts, tt.wantTotalParts)
				}
			}
		})
	}
}

func TestBuildSplitModelFilename(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		partNum    int
		totalParts int
		want       string
	}{
		{
			name:       "build part 1 of 4",
			prefix:     "model-name",
			partNum:    1,
			totalParts: 4,
			want:       "model-name-00001-of-00004.gguf",
		},
		{
			name:       "build openai gpt part 2 of 2",
			prefix:     "openai_gpt-oss-120b-Q8_0",
			partNum:    2,
			totalParts: 2,
			want:       "openai_gpt-oss-120b-Q8_0-00002-of-00002.gguf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSplitModelFilename(tt.prefix, tt.partNum, tt.totalParts)
			if got != tt.want {
				t.Errorf("buildSplitModelFilename(%q, %d, %d) = %q, want %q",
					tt.prefix, tt.partNum, tt.totalParts, got, tt.want)
			}
		})
	}
}

func TestGetSplitModelParts(t *testing.T) {
	mm := NewModelManagerForTest()

	tests := []struct {
		name      string
		modelPath string
		wantPaths []string
	}{
		{
			name:      "split model part 1 of 2",
			modelPath: "/home/josh/models/model-00001-of-00002.gguf",
			wantPaths: []string{
				"/home/josh/models/model-00001-of-00002.gguf",
				"/home/josh/models/model-00002-of-00002.gguf",
			},
		},
		{
			name:      "split model part 2 of 4",
			modelPath: "/home/josh/models/llama-00002-of-00004.gguf",
			wantPaths: []string{
				"/home/josh/models/llama-00001-of-00004.gguf",
				"/home/josh/models/llama-00002-of-00004.gguf",
				"/home/josh/models/llama-00003-of-00004.gguf",
				"/home/josh/models/llama-00004-of-00004.gguf",
			},
		},
		{
			name:      "regular non-split model",
			modelPath: "/home/josh/models/llama-7b.gguf",
			wantPaths: []string{
				"/home/josh/models/llama-7b.gguf",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mm.getSplitModelParts(tt.modelPath)

			if len(got) != len(tt.wantPaths) {
				t.Errorf("getSplitModelParts(%q) returned %d parts, want %d",
					tt.modelPath, len(got), len(tt.wantPaths))
				return
			}

			for i := range got {
				if got[i] != tt.wantPaths[i] {
					t.Errorf("getSplitModelParts(%q)[%d] = %q, want %q",
						tt.modelPath, i, got[i], tt.wantPaths[i])
				}
			}
		})
	}
}
