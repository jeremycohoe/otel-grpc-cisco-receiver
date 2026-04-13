package ciscotelemetryreceiver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
)

// yangModelsCache is the serialized form of parsed YANG modules.
type yangModelsCache struct {
	// Version of the cache format.
	Version string `json:"version"`

	// SourceHash is the SHA-256 of the sorted list of .yang filenames + sizes
	// in the models directory. If this changes, the cache is invalidated.
	SourceHash string `json:"source_hash"`

	// SourceDir is the directory the cache was built from.
	SourceDir string `json:"source_dir"`

	// ModuleCount is the number of successfully parsed modules.
	ModuleCount int `json:"module_count"`

	// Modules contains the parsed RFC6020 modules keyed by module name.
	Modules map[string]*RFC6020Module `json:"modules"`
}

const cacheFormatVersion = "1"

// LoadYANGModelsDir loads .yang files from a directory, using a cache file for
// fast startup. On first run (or when the directory contents change), all .yang
// files are parsed with the RFC 6020/7950 parser and the results are written to
// cacheFile. Subsequent runs load from the cache in milliseconds.
//
// Only top-level .yang files are loaded (no subdirectory traversal).
func LoadYANGModelsDir(modelsDir, cacheFile string, rfcParser *RFC6020Parser, logger *zap.Logger) (int, error) {
	if modelsDir == "" {
		return 0, nil
	}

	// Default cache file location: inside the models directory.
	if cacheFile == "" {
		cacheFile = filepath.Join(modelsDir, "yang-cache.json")
	}

	// Compute a hash of the directory contents (file names + sizes).
	dirHash, fileCount, err := hashDirectory(modelsDir)
	if err != nil {
		return 0, fmt.Errorf("failed to scan models directory %s: %w", modelsDir, err)
	}
	if fileCount == 0 {
		logger.Warn("No .yang files found in models directory", zap.String("dir", modelsDir))
		return 0, nil
	}

	// Try loading from cache.
	if cached, loadErr := loadCache(cacheFile); loadErr == nil && cached.SourceHash == dirHash {
		loaded := 0
		for name, module := range cached.Modules {
			rfcParser.modules[name] = module
			loaded++
		}
		logger.Info("Loaded YANG modules from cache",
			zap.String("cache_file", cacheFile),
			zap.Int("modules", loaded))
		return loaded, nil
	}

	// Cache miss or hash mismatch — parse all .yang files.
	logger.Info("Parsing YANG models directory (first run or directory changed)",
		zap.String("dir", modelsDir),
		zap.Int("yang_files", fileCount))

	parsed, errors := parseYANGDirectory(modelsDir, rfcParser, logger)

	if errors > 0 {
		logger.Warn("Some YANG files had parse errors (non-fatal)",
			zap.Int("parsed", parsed),
			zap.Int("errors", errors))
	}

	// Write cache for next startup.
	if writeErr := writeCache(cacheFile, dirHash, modelsDir, rfcParser); writeErr != nil {
		logger.Warn("Failed to write YANG cache (non-fatal)", zap.Error(writeErr))
	} else {
		logger.Info("YANG module cache written",
			zap.String("cache_file", cacheFile),
			zap.Int("modules", parsed))
	}

	return parsed, nil
}

// hashDirectory computes a SHA-256 hash of the sorted .yang file names and sizes.
func hashDirectory(dir string) (string, int, error) {
	var entries []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip subdirectories entirely (only top-level .yang files).
		if d.IsDir() && path != dir {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".yang") {
			info, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}
			entries = append(entries, fmt.Sprintf("%s:%d", d.Name(), info.Size()))
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}

	sort.Strings(entries)
	h := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return fmt.Sprintf("%x", h), len(entries), nil
}

// loadCache reads and deserializes the YANG cache file.
func loadCache(cacheFile string) (*yangModelsCache, error) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}

	var cache yangModelsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	if cache.Version != cacheFormatVersion {
		return nil, fmt.Errorf("cache version mismatch: got %s, want %s", cache.Version, cacheFormatVersion)
	}

	return &cache, nil
}

// writeCache serializes all parsed modules to the cache file.
func writeCache(cacheFile, dirHash, sourceDir string, rfcParser *RFC6020Parser) error {
	cache := &yangModelsCache{
		Version:     cacheFormatVersion,
		SourceHash:  dirHash,
		SourceDir:   sourceDir,
		ModuleCount: len(rfcParser.modules),
		Modules:     rfcParser.modules,
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to serialize cache: %w", err)
	}

	return os.WriteFile(cacheFile, data, 0644)
}

// parseYANGDirectory reads all top-level .yang files in a directory and parses
// them with the RFC parser. Returns (parsed count, error count).
func parseYANGDirectory(dir string, rfcParser *RFC6020Parser, logger *zap.Logger) (int, int) {
	parsed := 0
	errors := 0

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip subdirectories (MIBS, BIC, etc.)
		if d.IsDir() && path != dir {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yang") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			logger.Debug("Failed to read YANG file", zap.String("file", d.Name()), zap.Error(readErr))
			errors++
			return nil
		}

		_, parseErr := rfcParser.ParseYANGModule(string(content), d.Name())
		if parseErr != nil {
			logger.Debug("Failed to parse YANG file", zap.String("file", d.Name()), zap.Error(parseErr))
			errors++
			return nil
		}

		parsed++
		return nil
	})

	return parsed, errors
}
