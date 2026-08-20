package legacymigrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type LegacyTXTManifest struct {
	root  string
	files map[int64]string
}

func LoadLegacyTXTManifest(path string) (*LegacyTXTManifest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("MOONBOOK_LEGACY_TXT_MANIFEST is required for TXT migration")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read legacy TXT manifest: %w", err)
	}
	var raw struct {
		Root  string            `json:"root"`
		Files map[string]string `json:"files"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode legacy TXT manifest: %w", err)
	}
	root := strings.TrimSpace(raw.Root)
	if root == "" {
		root = filepath.Dir(path)
	} else if !filepath.IsAbs(root) {
		root = filepath.Join(filepath.Dir(path), root)
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("resolve legacy TXT root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve legacy TXT root symlinks: %w", err)
	}
	manifest := &LegacyTXTManifest{root: resolvedRoot, files: make(map[int64]string, len(raw.Files))}
	for rawID, rawPath := range raw.Files {
		id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("legacy TXT manifest task ID %q is invalid", rawID)
		}
		entry := filepath.Clean(strings.TrimSpace(rawPath))
		if entry == "." || filepath.IsAbs(entry) {
			return nil, fmt.Errorf("legacy TXT manifest path for task %d must be relative", id)
		}
		if _, exists := manifest.files[id]; exists {
			return nil, fmt.Errorf("legacy TXT manifest task %d is duplicated", id)
		}
		manifest.files[id] = entry
	}
	return manifest, nil
}

func (manifest *LegacyTXTManifest) Resolve(taskID int64) (string, error) {
	if manifest == nil {
		return "", errors.New("legacy TXT manifest is required")
	}
	entry, ok := manifest.files[taskID]
	if !ok {
		return "", os.ErrNotExist
	}
	path, err := filepath.EvalSymlinks(filepath.Join(manifest.root, entry))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(manifest.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("legacy TXT file resolves outside the configured root")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("legacy TXT path is not a regular file")
	}
	return path, nil
}
