package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileScanner(t *testing.T) {
	baseDir := "/tmp/test"
	scanner := NewFileScanner(baseDir)

	assert.NotNil(t, scanner)
	assert.Equal(t, baseDir, scanner.baseDir)
	assert.Equal(t, "*.jsonl", scanner.pattern)
	assert.Equal(t, 10, scanner.concurrent)
}

func TestFileScannerScanEmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	files, err := scanner.Scan()

	require.NoError(t, err)
	assert.Empty(t, files, "Empty directory should return no files")
}

func TestFileScannerScanNonExistentDirectory(t *testing.T) {
	nonExistentDir := "/path/that/does/not/exist"
	scanner := NewFileScanner(nonExistentDir)

	files, err := scanner.Scan()

	// Scanner handles errors gracefully by skipping them
	require.NoError(t, err, "Scanner should handle non-existent directory gracefully")
	assert.Empty(t, files, "Non-existent directory should return no files")
}

func TestFileScannerScanWithJSONLFiles(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	// Create test files
	testFiles := []struct {
		path     string
		isJSONL  bool
	}{
		{"session1.jsonl", true},
		{"session2.jsonl", true},
		{"session3.JSONL", true}, // Test case insensitive
		{"data.json", false},
		{"readme.txt", false},
		{"subdir/session4.jsonl", true},
		{"subdir/other.log", false},
	}

	expectedJSONLFiles := []string{}
	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file.path)
		dir := filepath.Dir(fullPath)
		
		// Create directory if it doesn't exist
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		
		// Create file
		err = os.WriteFile(fullPath, []byte("test content"), 0644)
		require.NoError(t, err)
		
		if file.isJSONL {
			expectedJSONLFiles = append(expectedJSONLFiles, fullPath)
		}
	}

	files, err := scanner.Scan()

	require.NoError(t, err)
	assert.Len(t, files, len(expectedJSONLFiles), "Should find all JSONL files")
	
	// Verify all expected files are found
	for _, expectedFile := range expectedJSONLFiles {
		assert.Contains(t, files, expectedFile, "Should contain expected JSONL file")
	}
	
	// Verify no non-JSONL files are included
	for _, file := range files {
		assert.True(t, strings.HasSuffix(strings.ToLower(file), ".jsonl"), 
			"All returned files should be JSONL files")
	}
}

func TestFileScannerScanNestedDirectories(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	// Create nested directory structure
	testStructure := []string{
		"level1/session1.jsonl",
		"level1/level2/session2.jsonl",
		"level1/level2/level3/session3.jsonl",
		"other1/session4.jsonl",
		"other1/other2/session5.jsonl",
	}

	for _, path := range testStructure {
		fullPath := filepath.Join(tempDir, path)
		dir := filepath.Dir(fullPath)
		
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		
		err = os.WriteFile(fullPath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	files, err := scanner.Scan()

	require.NoError(t, err)
	assert.Len(t, files, len(testStructure), "Should find all JSONL files in nested directories")
	
	// Verify all paths are found
	for _, expectedPath := range testStructure {
		expectedFullPath := filepath.Join(tempDir, expectedPath)
		assert.Contains(t, files, expectedFullPath, "Should find file in nested directory")
	}
}

func TestFileScannerScanMixedFileTypes(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	// Create various file types
	fileTypes := []struct {
		name    string
		isJSONL bool
	}{
		{"session.jsonl", true},
		{"config.json", false},
		{"data.csv", false},
		{"log.txt", false},
		{"backup.jsonl.bak", false}, // Should not match
		{"test.JSONL", true},        // Case insensitive
		{".jsonl", true},           // Hidden file with .jsonl extension
		{"file.jsonl.old", false},   // .jsonl not at the end
	}

	expectedCount := 0
	for _, file := range fileTypes {
		fullPath := filepath.Join(tempDir, file.name)
		err := os.WriteFile(fullPath, []byte("content"), 0644)
		require.NoError(t, err)
		
		if file.isJSONL {
			expectedCount++
		}
	}

	files, err := scanner.Scan()

	require.NoError(t, err)
	assert.Len(t, files, expectedCount, "Should only find files ending with .jsonl")
}

func TestFileScannerScanSymlinks(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	// Create a real file
	realFile := filepath.Join(tempDir, "real.jsonl")
	err := os.WriteFile(realFile, []byte("real content"), 0644)
	require.NoError(t, err)

	// Create a symlink to the real file
	symlinkFile := filepath.Join(tempDir, "symlink.jsonl")
	err = os.Symlink(realFile, symlinkFile)
	if err != nil {
		// Skip symlink test on systems that don't support symlinks
		t.Skipf("Skipping symlink test: %v", err)
	}

	files, err := scanner.Scan()

	require.NoError(t, err)
	// Should find both the real file and the symlink
	assert.GreaterOrEqual(t, len(files), 1, "Should find at least the real file")
	assert.Contains(t, files, realFile, "Should find the real file")
}

func TestFileScannerScanPermissionDenied(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	// Create a file
	testFile := filepath.Join(tempDir, "test.jsonl")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create a subdirectory and make it unreadable
	restrictedDir := filepath.Join(tempDir, "restricted")
	err = os.MkdirAll(restrictedDir, 0755)
	require.NoError(t, err)

	// Create a file in the restricted directory
	restrictedFile := filepath.Join(restrictedDir, "restricted.jsonl")
	err = os.WriteFile(restrictedFile, []byte("restricted content"), 0644)
	require.NoError(t, err)

	// Remove read permission from the directory (Unix only)
	if os.Getenv("GOOS") != "windows" {
		err = os.Chmod(restrictedDir, 0000)
		require.NoError(t, err)
		
		// Restore permissions after test
		defer func() {
			os.Chmod(restrictedDir, 0755)
		}()
	}

	files, err := scanner.Scan()

	// Scanner should continue even with permission errors
	require.NoError(t, err)
	assert.Contains(t, files, testFile, "Should find accessible files")
	
	// On Unix, the restricted file should not be found due to permission error
	// On Windows, it might still be found depending on the system
	if os.Getenv("GOOS") != "windows" {
		assert.NotContains(t, files, restrictedFile, "Should not find files in restricted directories")
	}
}

func TestFileScannerScanLargeDirectory(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	// Create many files to test performance
	numFiles := 100
	expectedJSONLFiles := 0

	for i := 0; i < numFiles; i++ {
		var filename string
		if i%3 == 0 {
			filename = filepath.Join(tempDir, fmt.Sprintf("session%d.jsonl", i))
			expectedJSONLFiles++
		} else if i%3 == 1 {
			filename = filepath.Join(tempDir, fmt.Sprintf("data%d.json", i))
		} else {
			filename = filepath.Join(tempDir, fmt.Sprintf("log%d.txt", i))
		}
		
		err := os.WriteFile(filename, []byte("content"), 0644)
		require.NoError(t, err)
	}

	files, err := scanner.Scan()

	require.NoError(t, err)
	assert.Len(t, files, expectedJSONLFiles, "Should find all JSONL files in large directory")
}

func TestFileScannerScanWithEmptyFiles(t *testing.T) {
	tempDir := t.TempDir()
	scanner := NewFileScanner(tempDir)

	// Create empty JSONL files
	emptyFiles := []string{
		"empty1.jsonl",
		"empty2.jsonl",
		"subdir/empty3.jsonl",
	}

	for _, file := range emptyFiles {
		fullPath := filepath.Join(tempDir, file)
		dir := filepath.Dir(fullPath)
		
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		
		// Create empty file
		err = os.WriteFile(fullPath, []byte{}, 0644)
		require.NoError(t, err)
	}

	files, err := scanner.Scan()

	require.NoError(t, err)
	assert.Len(t, files, len(emptyFiles), "Should find empty JSONL files")
	
	for _, expectedFile := range emptyFiles {
		expectedFullPath := filepath.Join(tempDir, expectedFile)
		assert.Contains(t, files, expectedFullPath, "Should find empty file")
	}
}

func TestFileScannerPattern(t *testing.T) {
	scanner := NewFileScanner("/tmp")
	
	// Test that the pattern is set correctly
	assert.Equal(t, "*.jsonl", scanner.pattern)
}

func TestFileScannerConcurrency(t *testing.T) {
	scanner := NewFileScanner("/tmp")
	
	// Test that concurrency is set to default value
	assert.Equal(t, 10, scanner.concurrent)
}