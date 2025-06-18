package extractor

import (
	"time"
)

// DateExtractor defines the interface for extracting dates from files
type DateExtractor interface {
	// ExtractDate extracts the date from a file
	ExtractDate(filePath string) (*time.Time, error)

	// SupportsFile checks if the extractor supports the given file
	SupportsFile(filePath string) bool

	// GetPriority returns the priority of this extractor (higher = more preferred)
	GetPriority() int
}

// CachedDateExtractor extends DateExtractor with caching capabilities
type CachedDateExtractor interface {
	DateExtractor

	// ClearCache clears the internal cache
	ClearCache()

	// GetCacheStats returns cache statistics
	GetCacheStats() CacheStats
}

// DateExtractorFactory creates date extractors
type DateExtractorFactory interface {
	// CreateExtractor creates a date extractor for the given file type
	CreateExtractor(fileType FileType) DateExtractor

	// GetExtractors returns all available extractors ordered by priority
	GetExtractors() []DateExtractor
}

// FileType represents the type of file being processed
type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeJPEG
	FileTypePNG
	FileTypeTIFF
	FileTypeRAW
	FileTypeVideo
)

// CacheStats contains statistics about cache performance
type CacheStats struct {
	Hits         int64
	Misses       int64
	Size         int
	MaxSize      int
	HitRate      float64
	TotalQueries int64
}

// DateSource represents the source of extracted date
type DateSource int

const (
	DateSourceUnknown DateSource = iota
	DateSourceEXIFDateTime
	DateSourceEXIFDateTimeOriginal
	DateSourceEXIFDateTimeDigitized
	DateSourceVideoMetadata
	DateSourceThumbnail
	DateSourceFileModTime
	DateSourceFileName
)

// ExtractedDate contains the extracted date and its source
type ExtractedDate struct {
	Date   time.Time
	Source DateSource
	Raw    string // Original raw value
}

// String returns a human-readable description of the date source
func (ds DateSource) String() string {
	switch ds {
	case DateSourceEXIFDateTime:
		return "EXIF DateTime"
	case DateSourceEXIFDateTimeOriginal:
		return "EXIF DateTimeOriginal"
	case DateSourceEXIFDateTimeDigitized:
		return "EXIF DateTimeDigitized"
	case DateSourceVideoMetadata:
		return "Video Metadata"
	case DateSourceThumbnail:
		return "Thumbnail EXIF"
	case DateSourceFileModTime:
		return "File Modification Time"
	case DateSourceFileName:
		return "File Name"
	default:
		return "Unknown"
	}
}

// FileType methods

func (ft FileType) String() string {
	switch ft {
	case FileTypeJPEG:
		return "JPEG"
	case FileTypePNG:
		return "PNG"
	case FileTypeTIFF:
		return "TIFF"
	case FileTypeRAW:
		return "RAW"
	case FileTypeVideo:
		return "Video"
	default:
		return "Unknown"
	}
}

// IsImage returns true if the file type is an image
func (ft FileType) IsImage() bool {
	switch ft {
	case FileTypeJPEG, FileTypePNG, FileTypeTIFF, FileTypeRAW:
		return true
	default:
		return false
	}
}

// IsVideo returns true if the file type is a video
func (ft FileType) IsVideo() bool {
	return ft == FileTypeVideo
}
