package statistics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Statistics contains all statistics for the photo sorting operation.
type Statistics struct {
	TotalFilesFound     int64
	TotalFilesProcessed int64
	FilesOrganized      int64
	FilesMoved          int64
	FilesCopied         int64
	FilesSkipped        int64
	FilesWithErrors     int64
	FilesWithoutDates   int64

	VideoFilesFound     int64
	VideoFilesProcessed int64
	ThumbnailsFound     int64
	VideoPairsFound     int64
	MPGTHMMerged        int64
	MPGTHMErrors        int64

	DuplicatesFound    int64
	DuplicatesRenamed  int64
	DuplicatesSkipped  int64
	DuplicatesReplaced int64

	StartTime       time.Time
	EndTime         time.Time
	Duration        time.Duration
	FilesPerSecond  float64
	BytesProcessed  int64
	AverageFileSize int64

	CacheHits    int64
	CacheMisses  int64
	CacheHitRate float64

	DirectoriesCreated int64
	DirectoriesScanned int64

	Errors []StatError

	mutex sync.RWMutex

	FileTypeStats map[string]int64

	DateExtractionStats DateExtractionStats
}

// StatError represents an error that occurred during processing.
type StatError struct {
	FilePath  string
	Operation string
	Error     string
	Timestamp time.Time
}

// DateExtractionStats contains statistics about date extraction methods.
type DateExtractionStats struct {
	FromEXIF         int64
	FromVideoMeta    int64
	FromThumbnail    int64
	FromFileName     int64
	FromModTime      int64
	ExtractionErrors int64
}

// NewStatistics returns a new Statistics instance.
func NewStatistics() *Statistics {
	return &Statistics{
		StartTime:           time.Now(),
		FileTypeStats:       make(map[string]int64),
		Errors:              make([]StatError, 0),
		DateExtractionStats: DateExtractionStats{},
	}
}

// IncrementFilesFound increases the count of found files by 1.
func (s *Statistics) IncrementFilesFound() {
	atomic.AddInt64(&s.TotalFilesFound, 1)
}

// IncrementFilesProcessed increases the count of processed files by 1.
func (s *Statistics) IncrementFilesProcessed() {
	atomic.AddInt64(&s.TotalFilesProcessed, 1)
}

// IncrementFilesOrganized increases the count of organized files by 1.
func (s *Statistics) IncrementFilesOrganized() {
	atomic.AddInt64(&s.FilesOrganized, 1)
}

// IncrementFilesMoved increases the count of moved files by 1.
func (s *Statistics) IncrementFilesMoved() {
	atomic.AddInt64(&s.FilesMoved, 1)
}

// IncrementFilesCopied increases the count of copied files by 1.
func (s *Statistics) IncrementFilesCopied() {
	atomic.AddInt64(&s.FilesCopied, 1)
}

// IncrementFilesSkipped increases the count of skipped files by 1.
func (s *Statistics) IncrementFilesSkipped() {
	atomic.AddInt64(&s.FilesSkipped, 1)
}

// IncrementFilesWithErrors increases the count of files with errors by 1.
func (s *Statistics) IncrementFilesWithErrors() {
	atomic.AddInt64(&s.FilesWithErrors, 1)
}

// IncrementFilesWithoutDates increases the count of files without dates by 1.
func (s *Statistics) IncrementFilesWithoutDates() {
	atomic.AddInt64(&s.FilesWithoutDates, 1)
}

// IncrementVideoFilesFound increases the count of found video files by 1.
func (s *Statistics) IncrementVideoFilesFound() {
	atomic.AddInt64(&s.VideoFilesFound, 1)
}

// IncrementVideoFilesProcessed increases the count of processed video files by 1.
func (s *Statistics) IncrementVideoFilesProcessed() {
	atomic.AddInt64(&s.VideoFilesProcessed, 1)
}

// IncrementThumbnailsFound increases the count of found thumbnails by 1.
func (s *Statistics) IncrementThumbnailsFound() {
	atomic.AddInt64(&s.ThumbnailsFound, 1)
}

// IncrementVideoPairsFound increases the count of found video pairs by 1.
func (s *Statistics) IncrementVideoPairsFound() {
	atomic.AddInt64(&s.VideoPairsFound, 1)
}

// IncrementMPGTHMMerged increases the count of merged MPG/THM pairs by 1.
func (s *Statistics) IncrementMPGTHMMerged() {
	atomic.AddInt64(&s.MPGTHMMerged, 1)
}

// IncrementMPGTHMErrors increases the count of MPG/THM errors by 1.
func (s *Statistics) IncrementMPGTHMErrors() {
	atomic.AddInt64(&s.MPGTHMErrors, 1)
}

// IncrementDuplicatesFound increases the count of found duplicates by 1.
func (s *Statistics) IncrementDuplicatesFound() {
	atomic.AddInt64(&s.DuplicatesFound, 1)
}

// IncrementDuplicatesRenamed increases the count of renamed duplicates by 1.
func (s *Statistics) IncrementDuplicatesRenamed() {
	atomic.AddInt64(&s.DuplicatesRenamed, 1)
}

// IncrementDuplicatesSkipped increases the count of skipped duplicates by 1.
func (s *Statistics) IncrementDuplicatesSkipped() {
	atomic.AddInt64(&s.DuplicatesSkipped, 1)
}

// IncrementDuplicatesReplaced increases the count of replaced duplicates by 1.
func (s *Statistics) IncrementDuplicatesReplaced() {
	atomic.AddInt64(&s.DuplicatesReplaced, 1)
}

// IncrementDirectoriesCreated increases the count of created directories by 1.
func (s *Statistics) IncrementDirectoriesCreated() {
	atomic.AddInt64(&s.DirectoriesCreated, 1)
}

// IncrementDirectoriesScanned increases the count of scanned directories by 1.
func (s *Statistics) IncrementDirectoriesScanned() {
	atomic.AddInt64(&s.DirectoriesScanned, 1)
}

// IncrementCacheHits increases the cache hit count by 1.
func (s *Statistics) IncrementCacheHits() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.CacheHits++
}

// IncrementCacheMisses increases the cache miss count by 1.
func (s *Statistics) IncrementCacheMisses() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.CacheMisses++
}

// UpdateCacheHitRate updates the cache hit rate based on current hits and misses.
func (s *Statistics) UpdateCacheHitRate() {
	hits := atomic.LoadInt64(&s.CacheHits)
	misses := atomic.LoadInt64(&s.CacheMisses)
	total := hits + misses
	if total > 0 {
		s.CacheHitRate = float64(hits) / float64(total)
	}
}

// IncrementDateFromEXIF increases the count of dates extracted from EXIF by 1.
func (s *Statistics) IncrementDateFromEXIF() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.DateExtractionStats.FromEXIF++
}

// IncrementDateFromVideoMeta increases the count of dates extracted from video metadata by 1.
func (s *Statistics) IncrementDateFromVideoMeta() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.DateExtractionStats.FromVideoMeta++
}

// IncrementDateFromThumbnail increases the count of dates extracted from thumbnails by 1.
func (s *Statistics) IncrementDateFromThumbnail() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.DateExtractionStats.FromThumbnail++
}

// IncrementDateFromFileName increases the count of dates extracted from filenames by 1.
func (s *Statistics) IncrementDateFromFileName() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.DateExtractionStats.FromFileName++
}

// IncrementDateFromModTime increases the count of dates extracted from modification time by 1.
func (s *Statistics) IncrementDateFromModTime() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.DateExtractionStats.FromModTime++
}

// IncrementDateExtractionErrors increases the count of date extraction errors by 1.
func (s *Statistics) IncrementDateExtractionErrors() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.DateExtractionStats.ExtractionErrors++
}

// IncrementFileType increases the count for a specific file type by 1.
func (s *Statistics) IncrementFileType(fileType string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.FileTypeStats[fileType]++
}

// AddBytesProcessed adds the given number of bytes to the total bytes processed.
func (s *Statistics) AddBytesProcessed(bytes int64) {
	atomic.AddInt64(&s.BytesProcessed, bytes)
}

// Finalize calculates final statistics such as duration, files per second, and average file size.
func (s *Statistics) Finalize() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)

	totalProcessed := atomic.LoadInt64(&s.TotalFilesProcessed)
	bytesProcessed := atomic.LoadInt64(&s.BytesProcessed)

	if s.Duration.Seconds() > 0 {
		s.FilesPerSecond = float64(totalProcessed) / s.Duration.Seconds()
	}

	if totalProcessed > 0 {
		s.AverageFileSize = bytesProcessed / totalProcessed
	}

	s.UpdateCacheHitRate()
}

// AddError records an error that occurred during processing.
func (s *Statistics) AddError(filePath, operation, errorMsg string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Errors = append(s.Errors, StatError{
		FilePath:  filePath,
		Operation: operation,
		Error:     errorMsg,
		Timestamp: time.Now(),
	})
}

// GetSummary returns a formatted summary of all statistics.
func (s *Statistics) GetSummary() string {
	return fmt.Sprintf(`Photo Sorter Statistics Summary:

Files:
		Total Found: %d
		Total Processed: %d
		Organized: %d
		Moved: %d
		Copied: %d
		Skipped: %d
		Errors: %d
		Without Dates: %d

Videos:
		Videos Found: %d
		Videos Processed: %d
		Thumbnails Found: %d
		Video Pairs: %d
		MPG/THM Merged: %d
		MPG/THM Errors: %d

Duplicates:
		Found: %d
		Renamed: %d
		Skipped: %d
		Replaced: %d

Performance:
		Duration: %v
		Files/Second: %.2f
		Bytes Processed: %s
		Average File Size: %s

Cache:
		Hits: %d
		Misses: %d
		Hit Rate: %.2f%%

Date Extraction:
		From EXIF: %d
		From Video Metadata: %d
		From Thumbnail: %d
		From Filename: %d
		From ModTime: %d
		Extraction Errors: %d

Directories:
		Created: %d
		Scanned: %d`,
		atomic.LoadInt64(&s.TotalFilesFound),
		atomic.LoadInt64(&s.TotalFilesProcessed),
		atomic.LoadInt64(&s.FilesOrganized),
		atomic.LoadInt64(&s.FilesMoved),
		atomic.LoadInt64(&s.FilesCopied),
		atomic.LoadInt64(&s.FilesSkipped),
		atomic.LoadInt64(&s.FilesWithErrors),
		atomic.LoadInt64(&s.FilesWithoutDates),
		atomic.LoadInt64(&s.VideoFilesFound),
		atomic.LoadInt64(&s.VideoFilesProcessed),
		atomic.LoadInt64(&s.ThumbnailsFound),
		atomic.LoadInt64(&s.VideoPairsFound),
		atomic.LoadInt64(&s.MPGTHMMerged),
		atomic.LoadInt64(&s.MPGTHMErrors),
		atomic.LoadInt64(&s.DuplicatesFound),
		atomic.LoadInt64(&s.DuplicatesRenamed),
		atomic.LoadInt64(&s.DuplicatesSkipped),
		atomic.LoadInt64(&s.DuplicatesReplaced),
		s.Duration,
		s.FilesPerSecond,
		formatBytes(atomic.LoadInt64(&s.BytesProcessed)),
		formatBytes(s.AverageFileSize),
		atomic.LoadInt64(&s.CacheHits),
		atomic.LoadInt64(&s.CacheMisses),
		s.CacheHitRate*100,
		s.DateExtractionStats.FromEXIF,
		s.DateExtractionStats.FromVideoMeta,
		s.DateExtractionStats.FromThumbnail,
		s.DateExtractionStats.FromFileName,
		s.DateExtractionStats.FromModTime,
		s.DateExtractionStats.ExtractionErrors,
		atomic.LoadInt64(&s.DirectoriesCreated),
		atomic.LoadInt64(&s.DirectoriesScanned))
}

// GetFileTypeBreakdown returns a formatted breakdown of file types processed.
func (s *Statistics) GetFileTypeBreakdown() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.FileTypeStats) == 0 {
		return "No file type statistics available"
	}

	result := "File Type Breakdown:\n"
	for fileType, count := range s.FileTypeStats {
		result += fmt.Sprintf("  %s: %d\n", fileType, count)
	}
	return result
}

// GetErrorSummary returns a summary of errors that occurred during processing.
func (s *Statistics) GetErrorSummary() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.Errors) == 0 {
		return "No errors occurred during processing"
	}

	result := fmt.Sprintf("Errors (%d total):\n", len(s.Errors))
	for i, err := range s.Errors {
		if i >= 10 {
			result += fmt.Sprintf("  ... and %d more errors\n", len(s.Errors)-10)
			break
		}
		result += fmt.Sprintf("  [%s] %s: %s - %s\n",
			err.Timestamp.Format("15:04:05"),
			err.Operation,
			err.FilePath,
			err.Error)
	}
	return result
}

// formatBytes returns a human-readable string for a byte count.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetTotalFilesProcessed returns the total number of files processed.
func (s *Statistics) GetTotalFilesProcessed() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.TotalFilesProcessed
}

// GetFilesOrganized returns the total number of files organized.
func (s *Statistics) GetFilesOrganized() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.FilesOrganized
}

// GetFilesWithErrors returns the total number of files with errors.
func (s *Statistics) GetFilesWithErrors() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return int64(len(s.Errors))
}

// GetDuration returns the total duration of the operation.
func (s *Statistics) GetDuration() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Duration
}

// GetFilesPerSecond returns the average number of files processed per second.
func (s *Statistics) GetFilesPerSecond() float64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.FilesPerSecond
}
