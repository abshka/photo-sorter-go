package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/sirupsen/logrus"
)

// EXIFExtractor extracts dates from image files using EXIF metadata.
type EXIFExtractor struct {
	logger *logrus.Logger
	cache  *sync.Map
	stats  CacheStats
	mutex  sync.RWMutex
}

// NewEXIFExtractor returns a new EXIFExtractor.
func NewEXIFExtractor(logger *logrus.Logger) *EXIFExtractor {
	return &EXIFExtractor{
		logger: logger,
		cache:  &sync.Map{},
		stats:  CacheStats{},
	}
}

// ExtractDate returns the date from an image file using EXIF metadata.
// If EXIF data is not available, it falls back to the file modification time.
func (e *EXIFExtractor) ExtractDate(filePath string) (*time.Time, error) {
	if !e.SupportsFile(filePath) {
		return nil, fmt.Errorf("file type not supported by extractor: %s", filePath)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if cachedDate := e.getCachedDateWithInfo(filePath, fileInfo); cachedDate != nil {
		e.incrementCacheHits()
		return cachedDate, nil
	}

	e.incrementCacheMisses()

	if date, err := e.extractWithGoExif(filePath); err == nil && date != nil {
		e.cacheDateWithInfo(filePath, fileInfo, date)
		return date, nil
	}

	modTime := fileInfo.ModTime()
	e.cacheDateWithInfo(filePath, fileInfo, &modTime)
	return &modTime, nil
}

// SupportsFile reports whether the file is supported by this extractor.
func (e *EXIFExtractor) SupportsFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExts := []string{".jpg", ".jpeg", ".png", ".tiff", ".tif", ".cr2", ".nef", ".arw", ".dng", ".raw"}

	return slices.Contains(supportedExts, ext)
}

// GetPriority returns the priority of this extractor.
func (e *EXIFExtractor) GetPriority() int {
	return 100
}

// ClearCache removes all entries from the internal cache and resets statistics.
func (e *EXIFExtractor) ClearCache() {
	e.cache = &sync.Map{}
	e.mutex.Lock()
	e.stats = CacheStats{}
	e.mutex.Unlock()
}

// GetCacheStats returns cache statistics for this extractor.
func (e *EXIFExtractor) GetCacheStats() CacheStats {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	stats := e.stats
	if stats.TotalQueries > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.TotalQueries)
	}
	return stats
}

// extractWithGoExif extracts the date using the rwcarlsen/goexif library.
func (e *EXIFExtractor) extractWithGoExif(filePath string) (*time.Time, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode EXIF: %w", err)
	}

	if tm, err := x.DateTime(); err == nil {
		e.logger.Debugf("Extracted DateTime from EXIF: %v for file %s", tm, filePath)
		return &tm, nil
	}

	if field, err := x.Get(exif.DateTimeOriginal); err == nil {
		if dateStr, err := field.StringVal(); err == nil {
			if date := e.parseEXIFDateTime(dateStr); date != nil {
				e.logger.Debugf("Extracted DateTimeOriginal from EXIF: %v for file %s", date, filePath)
				return date, nil
			}
		}
	}

	if field, err := x.Get(exif.DateTimeDigitized); err == nil {
		if dateStr, err := field.StringVal(); err == nil {
			if date := e.parseEXIFDateTime(dateStr); date != nil {
				e.logger.Debugf("Extracted DateTimeDigitized from EXIF: %v for file %s", date, filePath)
				return date, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid date found in EXIF using goexif")
}

// parseEXIFDateTime parses an EXIF date time string and returns a time.Time pointer.
// Returns nil if parsing fails.
func (e *EXIFExtractor) parseEXIFDateTime(dateStr string) *time.Time {
	if dateStr == "" {
		return nil
	}

	formats := []string{
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05",
		"2006:01:02",
		"2006-01-02",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return &date
		}
	}

	e.logger.Debugf("Failed to parse date string: %s", dateStr)
	return nil
}

// getCacheKey returns a cache key for the given file path and file info.
func (e *EXIFExtractor) getCacheKey(filePath string, fileInfo os.FileInfo) string {
	return fmt.Sprintf("%s:%d:%d", filePath, fileInfo.Size(), fileInfo.ModTime().Unix())
}

// getCachedDateWithInfo returns the cached date for the given file path and file info, or nil if not found.
func (e *EXIFExtractor) getCachedDateWithInfo(filePath string, fileInfo os.FileInfo) *time.Time {
	key := e.getCacheKey(filePath, fileInfo)
	if value, ok := e.cache.Load(key); ok {
		if date, ok := value.(time.Time); ok {
			return &date
		}
	}
	return nil
}

// cacheDateWithInfo stores the date in the cache for the given file path and file info.
func (e *EXIFExtractor) cacheDateWithInfo(filePath string, fileInfo os.FileInfo, date *time.Time) {
	if date == nil {
		return
	}

	key := e.getCacheKey(filePath, fileInfo)
	e.cache.Store(key, *date)
}

// incrementCacheHits increments the cache hit counter.
func (e *EXIFExtractor) incrementCacheHits() {
	e.mutex.Lock()
	e.stats.Hits++
	e.stats.TotalQueries++
	e.mutex.Unlock()
}

// incrementCacheMisses increments the cache miss counter.
func (e *EXIFExtractor) incrementCacheMisses() {
	e.mutex.Lock()
	e.stats.Misses++
	e.stats.TotalQueries++
	e.mutex.Unlock()
}
