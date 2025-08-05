// Copyright 2025 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package misc

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestGetFile(t *testing.T) {
	subtests := []struct {
		name     string
		setup    func(tmpDir string) (inputPath string, expectedFiles []string, err error)
		required bool
	}{
		{
			name: "symlink to directory - no infinite recursion",
			setup: func(tmpDir string) (string, []string, error) {
				// Create target directory with files
				targetDir := filepath.Join(tmpDir, "target")
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					return "", nil, err
				}

				testFile1 := filepath.Join(targetDir, "file1.txt")
				testFile2 := filepath.Join(targetDir, "file2.txt")
				if err := os.WriteFile(testFile1, []byte("content1"), 0644); err != nil {
					return "", nil, err
				}
				if err := os.WriteFile(testFile2, []byte("content2"), 0644); err != nil {
					return "", nil, err
				}

				// Create symlink pointing to target directory
				symlinkPath := filepath.Join(tmpDir, "symlink")
				if err := os.Symlink(targetDir, symlinkPath); err != nil {
					return "", nil, err
				}

				expected := []string{symlinkPath, testFile1, testFile2}
				return symlinkPath, expected, nil
			},
			required: true,
		},
		{
			name: "symlink to file - returns both symlink and target",
			setup: func(tmpDir string) (string, []string, error) {
				// Create target file
				targetFile := filepath.Join(tmpDir, "target.txt")
				if err := os.WriteFile(targetFile, []byte("content"), 0644); err != nil {
					return "", nil, err
				}

				// Create symlink pointing to target file
				symlinkPath := filepath.Join(tmpDir, "symlink.txt")
				if err := os.Symlink(targetFile, symlinkPath); err != nil {
					return "", nil, err
				}

				expected := []string{symlinkPath, targetFile}
				return symlinkPath, expected, nil
			},
			required: true,
		},
		{
			name: "regular file",
			setup: func(tmpDir string) (string, []string, error) {
				regularFile := filepath.Join(tmpDir, "regular.txt")
				if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
					return "", nil, err
				}

				expected := []string{regularFile}
				return regularFile, expected, nil
			},
			required: true,
		},
		{
			name: "regular directory",
			setup: func(tmpDir string) (string, []string, error) {
				// Create directory with files
				dirPath := filepath.Join(tmpDir, "testdir")
				if err := os.MkdirAll(dirPath, 0755); err != nil {
					return "", nil, err
				}

				file1 := filepath.Join(dirPath, "file1.txt")
				file2 := filepath.Join(dirPath, "subdir", "file2.txt")

				if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
					return "", nil, err
				}
				if err := os.MkdirAll(filepath.Dir(file2), 0755); err != nil {
					return "", nil, err
				}
				if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
					return "", nil, err
				}

				expected := []string{file1, file2}
				return dirPath, expected, nil
			},
			required: true,
		},
		{
			name: "zst compressed file fallback",
			setup: func(tmpDir string) (string, []string, error) {
				// Create a .zst file but NOT the original file
				zstFile := filepath.Join(tmpDir, "firmware.bin.zst")
				if err := os.WriteFile(zstFile, []byte("compressed content"), 0644); err != nil {
					return "", nil, err
				}

				// Request the original file (without .zst extension)
				originalFile := filepath.Join(tmpDir, "firmware.bin")

				// Expected: should find and return the .zst version
				expected := []string{zstFile}
				return originalFile, expected, nil
			},
			required: true,
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			inputPath, expectedFiles, err := st.setup(tmpDir)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			// Add timeout protection for infinite recursion test
			done := make(chan struct{})
			var files []string
			var getFileErr error

			go func() {
				defer close(done)
				files, getFileErr = getFile(inputPath, st.required)
			}()

			select {
			case <-done:
				if getFileErr != nil {
					t.Fatalf("getFile failed: %v", getFileErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("getFile appears to be in infinite recursion (timeout)")
			}

			// Sort for comparison
			sort.Strings(expectedFiles)
			sort.Strings(files)

			if !reflect.DeepEqual(expectedFiles, files) {
				t.Fatalf("expected: %q, got: %q", expectedFiles, files)
			}
		})
	}
}
