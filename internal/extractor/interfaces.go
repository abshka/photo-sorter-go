package extractor

import (
	"time"
)

// DateExtractor is the interface for extracting dates from files.
type DateExtractor interface {
	ExtractDate(filePath string) (*time.Time, error)
	SupportsFile(filePath string) bool
	GetPriority() int
}

// CachedDateExtractor extends DateExtractor with caching capabilities.
type CachedDateExtractor interface {
	DateExtractor
	ClearCache()
	GetCacheStats() CacheStats
}

// DateExtractorFactory creates date extractors.
type DateExtractorFactory interface {
	CreateExtractor(fileType FileType) DateExtractor
	GetExtractors() []DateExtractor
}

// FileType represents the type of file being processed.
type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeJPEG
	FileTypePNG
	FileTypeTIFF
	FileTypeRAW
	FileTypeVideo
)

// CacheStats contains statistics about cache performance.
type CacheStats struct {
	Hits         int64
	Misses       int64
	Size         int
	MaxSize      int
	HitRate      float64
	TotalQueries int64
}

// DateSource represents the source of the extracted date.
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

// ExtractedDate contains the extracted date and its source.
type ExtractedDate struct {
	Date   time.Time
	Source DateSource
	Raw    string
}

// String returns a human-readable description of the date source.
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

// String returns the string representation of the FileType.
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

// IsImage reports whether the file type is an image.
func (ft FileType) IsImage() bool {
	switch ft {
	case FileTypeJPEG, FileTypePNG, FileTypeTIFF, FileTypeRAW:
		return true
	default:
		return false
	}
}

// IsVideo reports whether the file type is a video.
func (ft FileType) IsVideo() bool {
	return ft == FileTypeVideo
}
