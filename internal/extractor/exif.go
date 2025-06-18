package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/sirupsen/logrus"
)

// EXIFExtractor extracts dates from image files using EXIF metadata
type EXIFExtractor struct {
	logger *logrus.Logger
	cache  *sync.Map
	stats  CacheStats
	mutex  sync.RWMutex
}

// NewEXIFExtractor creates a new EXIF date extractor
func NewEXIFExtractor(logger *logrus.Logger) *EXIFExtractor {
	return &EXIFExtractor{
		logger: logger,
		cache:  &sync.Map{},
		stats:  CacheStats{},
	}
}

// ExtractDate extracts the date from an image file using EXIF metadata
func (e *EXIFExtractor) ExtractDate(filePath string) (*time.Time, error) {
	// Check if file type is supported
	if !e.SupportsFile(filePath) {
		return nil, fmt.Errorf("file type not supported by extractor: %s", filePath)
	}

	// Get file info once for cache key and fallback
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check cache first
	if cachedDate := e.getCachedDateWithInfo(filePath, fileInfo); cachedDate != nil {
		e.incrementCacheHits()
		return cachedDate, nil
	}

	e.incrementCacheMisses()

	// Try to extract using goexif
	if date, err := e.extractWithGoExif(filePath); err == nil && date != nil {
		e.cacheDateWithInfo(filePath, fileInfo, date)
		return date, nil
	}

	// Final fallback to file modification time (using already retrieved fileInfo)
	modTime := fileInfo.ModTime()
	e.cacheDateWithInfo(filePath, fileInfo, &modTime)
	return &modTime, nil
}

// SupportsFile checks if the file is supported by this extractor
func (e *EXIFExtractor) SupportsFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExts := []string{".jpg", ".jpeg", ".png", ".tiff", ".tif", ".cr2", ".nef", ".arw", ".dng", ".raw"}

	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// GetPriority returns the priority of this extractor
func (e *EXIFExtractor) GetPriority() int {
	return 100 // High priority for EXIF data
}

// ClearCache clears the internal cache
func (e *EXIFExtractor) ClearCache() {
	e.cache = &sync.Map{}
	e.mutex.Lock()
	e.stats = CacheStats{}
	e.mutex.Unlock()
}

// GetCacheStats returns cache statistics
func (e *EXIFExtractor) GetCacheStats() CacheStats {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	stats := e.stats
	if stats.TotalQueries > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.TotalQueries)
	}
	return stats
}

// extractWithGoExif extracts date using rwcarlsen/goexif library
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

	// Try to get DateTime first
	if tm, err := x.DateTime(); err == nil {
		e.logger.Debugf("Extracted DateTime from EXIF: %v for file %s", tm, filePath)
		return &tm, nil
	}

	// Try DateTimeOriginal
	if field, err := x.Get(exif.DateTimeOriginal); err == nil {
		if dateStr, err := field.StringVal(); err == nil {
			if date := e.parseEXIFDateTime(dateStr); date != nil {
				e.logger.Debugf("Extracted DateTimeOriginal from EXIF: %v for file %s", date, filePath)
				return date, nil
			}
		}
	}

	// Try DateTimeDigitized
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

// extractFileModTime extracts the file modification time as fallback
func (e *EXIFExtractor) extractFileModTime(filePath string) (*time.Time, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}

	modTime := stat.ModTime()
	e.logger.Debugf("Using file modification time: %v for file %s", modTime, filePath)
	return &modTime, nil
}

// parseEXIFDateTime parses EXIF date time string
func (e *EXIFExtractor) parseEXIFDateTime(dateStr string) *time.Time {
	if dateStr == "" {
		return nil
	}

	// Common EXIF date formats
	formats := []string{
		"2006:01:02 15:04:05", // Standard EXIF format
		"2006-01-02 15:04:05", // Alternative format
		"2006:01:02",          // Date only
		"2006-01-02",          // Date only alternative
		time.RFC3339,          // ISO 8601
		time.RFC3339Nano,      // ISO 8601 with nanoseconds
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return &date
		}
	}

	e.logger.Debugf("Failed to parse date string: %s", dateStr)
	return nil
}

// Cache methods

func (e *EXIFExtractor) getCacheKey(filePath string, fileInfo os.FileInfo) string {
	// Include file size and mod time in cache key to detect changes
	return fmt.Sprintf("%s:%d:%d", filePath, fileInfo.Size(), fileInfo.ModTime().Unix())
}

func (e *EXIFExtractor) getCachedDate(filePath string) *time.Time {
	// Legacy method - requires stat call
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil
	}
	return e.getCachedDateWithInfo(filePath, stat)
}

func (e *EXIFExtractor) getCachedDateWithInfo(filePath string, fileInfo os.FileInfo) *time.Time {
	key := e.getCacheKey(filePath, fileInfo)
	if value, ok := e.cache.Load(key); ok {
		if date, ok := value.(time.Time); ok {
			return &date
		}
	}
	return nil
}

func (e *EXIFExtractor) cacheDate(filePath string, date *time.Time) {
	if date == nil {
		return
	}
	// Legacy method - requires stat call
	stat, err := os.Stat(filePath)
	if err != nil {
		return
	}
	e.cacheDateWithInfo(filePath, stat, date)
}

func (e *EXIFExtractor) cacheDateWithInfo(filePath string, fileInfo os.FileInfo, date *time.Time) {
	if date == nil {
		return
	}

	key := e.getCacheKey(filePath, fileInfo)
	e.cache.Store(key, *date)
}

func (e *EXIFExtractor) incrementCacheHits() {
	e.mutex.Lock()
	e.stats.Hits++
	e.stats.TotalQueries++
	e.mutex.Unlock()
}

func (e *EXIFExtractor) incrementCacheMisses() {
	e.mutex.Lock()
	e.stats.Misses++
	e.stats.TotalQueries++
	e.mutex.Unlock()
}
