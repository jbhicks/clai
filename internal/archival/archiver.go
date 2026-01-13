package archival

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"clai/internal/logger"
)

// Archiver manages archiving of completed feature work
type Archiver struct {
	projectRoot string
	archiveRoot string
	compression bool
	maxArchives int
}

// NewArchiver creates a new archiver instance
func NewArchiver(projectRoot string) *Archiver {
	return &Archiver{
		projectRoot: projectRoot,
		archiveRoot: filepath.Join(projectRoot, "archive"),
		compression: true,
		maxArchives: 50, // Keep last 50 archives
	}
}

// ArchiveFeature archives a completed feature to a dated directory
func (a *Archiver) ArchiveFeature(featureName string) error {
	logger.Info("Starting archival of feature: %s", featureName)

	// Create archive directory name with date and feature name
	timestamp := time.Now().Format("2006-01-02")
	archiveName := fmt.Sprintf("%s-%s", timestamp, sanitizeFilename(featureName))
	archivePath := filepath.Join(a.archiveRoot, archiveName)

	// Ensure archive directory exists
	if err := os.MkdirAll(archivePath, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Archive .clai directory
	if err := a.archiveDirectory(".clai", archivePath); err != nil {
		return fmt.Errorf("failed to archive .clai directory: %w", err)
	}

	// Archive generated code and other artifacts
	artifacts := []string{
		"model_test_results",
		"reference/ralph",
		"scripts",
		"docs/performance",
		"docs/ui",
		"docs/downloads",
		"docs/testing",
		"docs/development",
	}

	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(a.projectRoot, artifact)); err == nil {
			if err := a.archiveDirectory(artifact, archivePath); err != nil {
				logger.Warn("Failed to archive %s: %v", artifact, err)
			}
		}
	}

	// Create archive metadata
	if err := a.createArchiveMetadata(archivePath, featureName); err != nil {
		logger.Warn("Failed to create archive metadata: %v", err)
	}

	// Compress if enabled
	if a.compression {
		if err := a.compressArchive(archivePath); err != nil {
			logger.Warn("Failed to compress archive: %v", err)
		}
	}

	// Clean up old archives
	if err := a.cleanupOldArchives(); err != nil {
		logger.Warn("Failed to cleanup old archives: %v", err)
	}

	logger.Info("Successfully archived feature '%s' to: %s", featureName, archivePath)
	return nil
}

// ArchiveOnBranchChange archives current work when switching branches
func (a *Archiver) ArchiveOnBranchChange(oldBranch, newBranch string) error {
	logger.Info("Branch change detected: %s -> %s", oldBranch, newBranch)

	// Archive the old branch's work
	featureName := fmt.Sprintf("branch-%s", oldBranch)
	return a.ArchiveFeature(featureName)
}

// archiveDirectory copies a directory to the archive
func (a *Archiver) archiveDirectory(srcDir, archivePath string) error {
	srcPath := filepath.Join(a.projectRoot, srcDir)
	destPath := filepath.Join(archivePath, srcDir)

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Copy directory recursively
	return a.copyDir(srcPath, destPath)
}

// copyDir recursively copies a directory
func (a *Archiver) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip certain files/directories
		if strings.Contains(path, ".git") || strings.Contains(path, "node_modules") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return a.copyFile(path, destPath, info.Mode())
	})
}

// copyFile copies a single file
func (a *Archiver) copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return os.Chmod(dst, mode)
}

// createArchiveMetadata creates a metadata file for the archive
func (a *Archiver) createArchiveMetadata(archivePath, featureName string) error {
	metadata := map[string]interface{}{
		"featureName": featureName,
		"archivedAt":  time.Now().Format(time.RFC3339),
		"version":     "1.0",
		"contents": []string{
			".clai/",
			"model_test_results/",
			"reference/ralph/",
			"scripts/",
			"docs/performance/",
			"docs/ui/",
			"docs/downloads/",
			"docs/testing/",
			"docs/development/",
		},
	}

	// Write metadata as JSON
	metadataPath := filepath.Join(archivePath, "archive-metadata.json")
	file, err := os.Create(metadataPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(metadata)
}

// compressArchive compresses an archive directory into a tar.gz file
func (a *Archiver) compressArchive(archivePath string) error {
	dirName := filepath.Base(archivePath)
	parentDir := filepath.Dir(archivePath)
	tarPath := filepath.Join(parentDir, dirName+".tar.gz")

	file, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return filepath.Walk(archivePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(parentDir, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})
}

// cleanupOldArchives removes old archives to prevent disk space issues
func (a *Archiver) cleanupOldArchives() error {
	entries, err := os.ReadDir(a.archiveRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Archive directory doesn't exist yet
		}
		return err
	}

	// Collect archive directories (skip files)
	var archives []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), "-") {
			archives = append(archives, entry)
		}
	}

	// Sort by modification time (newest first)
	for i := 0; i < len(archives)-1; i++ {
		for j := i + 1; j < len(archives); j++ {
			infoI, _ := archives[i].Info()
			infoJ, _ := archives[j].Info()
			if infoI.ModTime().Before(infoJ.ModTime()) {
				archives[i], archives[j] = archives[j], archives[i]
			}
		}
	}

	for i := a.maxArchives; i < len(archives); i++ {
		archivePath := filepath.Join(a.archiveRoot, archives[i].Name())
		logger.Info("Removing old archive: %s", archivePath)
		if err := os.RemoveAll(archivePath); err != nil {
			logger.Warn("Failed to remove old archive %s: %v", archivePath, err)
		}
	}

	return nil
}

// ListArchives returns a list of available archives
func (a *Archiver) ListArchives() ([]ArchiveInfo, error) {
	entries, err := os.ReadDir(a.archiveRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []ArchiveInfo{}, nil
		}
		return nil, err
	}

	var archives []ArchiveInfo
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".tar.gz") {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			archives = append(archives, ArchiveInfo{
				Name:         entry.Name(),
				Path:         filepath.Join(a.archiveRoot, entry.Name()),
				CreatedAt:    info.ModTime(),
				IsCompressed: strings.HasSuffix(entry.Name(), ".tar.gz"),
				Size:         info.Size(),
			})
		}
	}

	return archives, nil
}

// RestoreArchive restores an archive to the project
func (a *Archiver) RestoreArchive(archiveName string) error {
	archivePath := filepath.Join(a.archiveRoot, archiveName)

	// Check if it's a compressed archive
	if strings.HasSuffix(archiveName, ".tar.gz") {
		return a.decompressAndRestore(archivePath)
	}

	// Copy archive back to project root
	return a.restoreFromDirectory(archivePath)
}

// decompressAndRestore decompresses a tar.gz archive and restores it
func (a *Archiver) decompressAndRestore(tarPath string) error {
	// For now, just log that decompression is needed
	logger.Info("Archive %s is compressed, decompression not yet implemented", tarPath)
	return fmt.Errorf("compressed archive restoration not yet implemented")
}

// restoreFromDirectory copies files from archive directory back to project
func (a *Archiver) restoreFromDirectory(archivePath string) error {
	logger.Info("Restoring from archive: %s", archivePath)

	return filepath.Walk(archivePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(archivePath, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(a.projectRoot, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return a.copyFile(path, destPath, info.Mode())
	})
}

// SetCompression enables or disables compression for archives
func (a *Archiver) SetCompression(enabled bool) {
	a.compression = enabled
}

// SetMaxArchives sets the maximum number of archives to keep
func (a *Archiver) SetMaxArchives(max int) {
	a.maxArchives = max
}

// ArchiveInfo represents information about an archived feature
type ArchiveInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	CreatedAt    time.Time `json:"createdAt"`
	IsCompressed bool      `json:"isCompressed"`
	Size         int64     `json:"size"`
}

// sanitizeFilename sanitizes a string for use as a filename
func sanitizeFilename(name string) string {
	result := strings.ReplaceAll(name, " ", "-")
	result = strings.ReplaceAll(result, "/", "-")
	result = strings.ReplaceAll(result, "\\", "-")
	result = strings.ReplaceAll(result, ":", "-")
	result = strings.ReplaceAll(result, "*", "-")
	result = strings.ReplaceAll(result, "?", "-")
	result = strings.ReplaceAll(result, "\"", "-")
	result = strings.ReplaceAll(result, "<", "-")
	result = strings.ReplaceAll(result, ">", "-")
	result = strings.ReplaceAll(result, "|", "-")

	// Limit length
	if len(result) > 50 {
		result = result[:50]
	}

	return strings.Trim(result, "-")
}
