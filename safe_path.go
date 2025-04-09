package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func safePath(baseDir, filePath string) (string, error) {
	cleanPath := filepath.Clean(filePath)
	fullPath := filepath.Join(baseDir, cleanPath)
	if !strings.HasPrefix(fullPath, baseDir) {
		return "", fmt.Errorf("path outside allowed directory")
	}

	return fullPath, nil
}
