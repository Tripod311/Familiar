package main

import (
	"os"
	"path/filepath"
)

func ResolveAppPath(relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return filepath.Clean(relativePath), nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil {
		return "", err
	}

	appDir := filepath.Dir(executablePath)

	return filepath.Join(appDir, relativePath), nil
}
