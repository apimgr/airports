package paths

import (
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrPathTraversal = errors.New("path traversal attempt detected")
	ErrInvalidPath   = errors.New("invalid path characters")
	ErrPathTooLong   = errors.New("path exceeds maximum length")

	validPathSegment = regexp.MustCompile(`^[a-z0-9_-]+$`)
)

func normalizePath(input string) string {
	if input == "" {
		return ""
	}
	cleaned := strings.Trim(path.Clean(input), "/")
	if strings.Contains(cleaned, "..") {
		return ""
	}
	return cleaned
}

func validatePathSegment(segment string) error {
	if segment == "" {
		return ErrInvalidPath
	}
	if len(segment) > 64 {
		return ErrPathTooLong
	}
	if segment == "." || segment == ".." {
		return ErrPathTraversal
	}
	if !validPathSegment.MatchString(segment) {
		return ErrInvalidPath
	}
	return nil
}

func validatePath(value string) error {
	if len(value) > 2048 {
		return ErrPathTooLong
	}
	if strings.Contains(value, "..") {
		return ErrPathTraversal
	}
	for _, segment := range strings.Split(strings.Trim(value, "/"), "/") {
		if segment == "" {
			continue
		}
		if err := validatePathSegment(segment); err != nil {
			return err
		}
	}
	return nil
}

func SafePath(input string) (string, error) {
	if err := validatePath(input); err != nil {
		return "", err
	}
	return normalizePath(input), nil
}

func SafeFilePath(baseDir, userPath string) (string, error) {
	safe, err := SafePath(userPath)
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(baseDir, safe))
	if err != nil {
		return "", err
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(absBase, fullPath)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrPathTraversal
	}
	return fullPath, nil
}
