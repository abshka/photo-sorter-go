package organizer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"photo-sorter-go/internal/compressor"
	"photo-sorter-go/internal/config"
	"photo-sorter-go/internal/extractor"
	"photo-sorter-go/internal/statistics"

	"github.com/sirupsen/logrus"
)

// FileOrganizer organizes media files by date.
type LogHookFunc func(level, message string)

type FileOrganizer struct {
	config     *config.Config
	logger     *logrus.Logger
	stats      *statistics.Statistics
	extractor  extractor.DateExtractor
	workers    int
	workerPool chan struct{}
	compressor compressor.Compressor

	logHook LogHookFunc // Новый хук для проброса логов
}

// FileInfo contains information about a file to be organized.
type FileInfo struct {
	Path          string
	Size          int64
	ModTime       time.Time
	IsVideo       bool
	IsImage       bool
	Extension     string
	ThumbnailPath string
}

// OrganizedFile represents a file that has been organized.
type OrganizedFile struct {
	OriginalPath string
	NewPath      string
	Date         time.Time
	Size         int64
	Operation    string
}

// NewFileOrganizer returns a new FileOrganizer.
func NewFileOrganizer(
	cfg *config.Config,
	logger *logrus.Logger,
	stats *statistics.Statistics,
	dateExtractor extractor.DateExtractor,
	compressor compressor.Compressor,
) *FileOrganizer {
	return NewFileOrganizerWithLogHook(cfg, logger, stats, dateExtractor, compressor, nil)
}

// NewFileOrganizerWithLogHook позволяет пробрасывать логи наружу (например, в WebSocket)
func NewFileOrganizerWithLogHook(
	cfg *config.Config,
	logger *logrus.Logger,
	stats *statistics.Statistics,
	dateExtractor extractor.DateExtractor,
	compressor compressor.Compressor,
	logHook LogHookFunc,
) *FileOrganizer {
	workers := cfg.Performance.WorkerThreads
	if workers <= 0 {
		workers = 4
	}
	return &FileOrganizer{
		config:     cfg,
		logger:     logger,
		stats:      stats,
		extractor:  dateExtractor,
		workers:    workers,
		workerPool: make(chan struct{}, workers),
		compressor: compressor,
		logHook:    logHook,
	}
}

// OrganizeFiles organizes all files in the source directory.
func (fo *FileOrganizer) OrganizeFiles() error {
	fo.logger.Info("Starting file organization process")
	fo.stats.StartTime = time.Now()

	files, err := fo.discoverFiles()
	if err != nil {
		return fmt.Errorf("failed to discover files: %w", err)
	}

	if len(files) == 0 {
		fo.logger.Info("No media files found to organize")
		return nil
	}

	fo.logger.Infof("Found %d media files to process", len(files))
	fo.stats.TotalFilesFound = int64(len(files))

	if fo.config.Security.DryRun {
		fo.logger.Info("Running in dry-run mode - no files will be moved or modified")
		return fo.dryRunProcess(files)
	}

	return fo.processFiles(files)
}

// discoverFiles finds all media files in the source directory.
func (fo *FileOrganizer) discoverFiles() ([]FileInfo, error) {
	var files []FileInfo
	var mutex sync.Mutex

	err := filepath.Walk(fo.config.SourceDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fo.logger.Warnf("Error accessing path %s: %v", path, err)
			return nil
		}

		if info.IsDir() {
			fo.stats.IncrementDirectoriesScanned()
			if fo.config.Processing.SkipOrganized && fo.isAlreadyOrganized(path) {
				fo.logger.Debugf("Skipping already organized directory: %s", path)
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !fo.isSupportedFile(ext) {
			return nil
		}

		fileInfo := FileInfo{
			Path:      path,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Extension: ext,
			IsImage:   fo.config.IsImageExtension(ext),
			IsVideo:   fo.config.IsVideoExtension(ext),
		}

		if fileInfo.IsVideo && ext == ".mpg" {
			thmPath := strings.TrimSuffix(path, ext) + ".thm"
			if _, err := os.Stat(thmPath); err == nil {
				fileInfo.ThumbnailPath = thmPath
				fo.stats.IncrementThumbnailsFound()
			}
		}

		mutex.Lock()
		files = append(files, fileInfo)
		fo.stats.IncrementFilesFound()
		if fileInfo.IsVideo {
			fo.stats.IncrementVideoFilesFound()
		}
		fo.stats.IncrementFileType(strings.ToUpper(strings.TrimPrefix(ext, ".")))
		mutex.Unlock()

		if fo.config.Security.MaxFilesPerRun > 0 && len(files) >= fo.config.Security.MaxFilesPerRun {
			fo.logger.Infof("Reached maximum files limit (%d), stopping discovery", fo.config.Security.MaxFilesPerRun)
			return filepath.SkipAll
		}

		return nil
	})

	return files, err
}

// processFiles processes all discovered files.
func (fo *FileOrganizer) processFiles(files []FileInfo) error {
	var wg sync.WaitGroup
	fileChan := make(chan FileInfo, fo.config.Performance.BatchSize)

	for i := 0; i < fo.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fo.worker(fileChan)
		}()
	}

	go func() {
		defer close(fileChan)
		for _, file := range files {
			fileChan <- file
		}
	}()

	wg.Wait()

	fo.stats.Finalize()
	fo.logger.Info("File organization completed")
	return nil
}

// worker processes files from the channel.
func (fo *FileOrganizer) worker(fileChan <-chan FileInfo) {
	for file := range fileChan {
		fo.processFile(file)
	}
}

// processFile processes a single file.
func (fo *FileOrganizer) processFile(file FileInfo) {
	fo.logger.Debugf("Processing file: %s", file.Path)
	fo.stats.IncrementFilesProcessed()

	date, err := fo.extractDate(file)
	if err != nil {
		fo.logger.Warnf("Could not extract date from %s: %v", file.Path, err)
		fo.stats.IncrementFilesWithoutDates()
		fo.stats.AddError(file.Path, "date_extraction", err.Error())
		return
	}

	targetPath, err := fo.generateTargetPath(file, *date)
	if err != nil {
		fo.logger.Errorf("Could not generate target path for %s: %v", file.Path, err)
		fo.stats.IncrementFilesWithErrors()
		fo.stats.AddError(file.Path, "path_generation", err.Error())
		return
	}

	if fo.fileExistsAtTarget(file.Path, targetPath) {
		if err := fo.handleDuplicate(file, targetPath); err != nil {
			fo.logger.Errorf("Error handling duplicate for %s: %v", file.Path, err)
			fo.stats.IncrementFilesWithErrors()
			fo.stats.AddError(file.Path, "duplicate_handling", err.Error())
		}
		return
	}

	targetDir := filepath.Dir(targetPath)
	if err := fo.createDirectory(targetDir); err != nil {
		fo.logger.Errorf("Could not create directory %s: %v", targetDir, err)
		fo.stats.IncrementFilesWithErrors()
		fo.stats.AddError(file.Path, "directory_creation", err.Error())
		return
	}

	if fo.config.Security.DryRun {
		// Всегда только логируем, никаких реальных действий!
		var msg string
		if fo.config.Processing.MoveFiles {
			msg = fmt.Sprintf("DRY-RUN: Would move %s -> %s", file.Path, targetPath)
		} else {
			msg = fmt.Sprintf("DRY-RUN: Would copy %s -> %s", file.Path, targetPath)
		}
		fo.logger.Infof(msg)
		if fo.logHook != nil {
			fo.logHook("info", msg)
		}
	} else {
		if fo.config.Processing.MoveFiles {
			if err := fo.moveFile(file.Path, targetPath); err != nil {
				fo.logger.Errorf("Could not move file %s to %s: %v", file.Path, targetPath, err)
				fo.stats.IncrementFilesWithErrors()
				fo.stats.AddError(file.Path, "move_file", err.Error())
				return
			}
			fo.stats.IncrementFilesMoved()
		} else {
			if err := fo.copyFile(file.Path, targetPath); err != nil {
				fo.logger.Errorf("Could not copy file %s to %s: %v", file.Path, targetPath, err)
				fo.stats.IncrementFilesWithErrors()
				fo.stats.AddError(file.Path, "copy_file", err.Error())
				return
			}
			fo.stats.IncrementFilesCopied()
		}
	}

	if file.ThumbnailPath != "" {
		fo.processThumbnail(file, targetPath)
	}

	fo.stats.IncrementFilesOrganized()
	fo.stats.AddBytesProcessed(file.Size)
	fo.logger.Infof("Organized file: %s -> %s", file.Path, targetPath)
}

// extractDate extracts the date from a file using the configured extractor.
func (fo *FileOrganizer) extractDate(file FileInfo) (*time.Time, error) {
	if !fo.extractor.SupportsFile(file.Path) {
		return nil, fmt.Errorf("file type not supported by extractor")
	}

	date, err := fo.extractor.ExtractDate(file.Path)
	if err != nil {
		fo.stats.IncrementDateExtractionErrors()
		return nil, err
	}

	fo.stats.IncrementDateFromEXIF()
	return date, nil
}

// generateTargetPath returns the target path for a file based on its date.
func (fo *FileOrganizer) generateTargetPath(file FileInfo, date time.Time) (string, error) {
	targetDir := fo.config.GetTargetDirectory()
	dateSubdir := date.Format(fo.config.DateFormat)
	fullTargetDir := filepath.Join(targetDir, dateSubdir)
	filename := filepath.Base(file.Path)
	return filepath.Join(fullTargetDir, filename), nil
}

// fileExistsAtTarget returns true if a file already exists at the target location.
func (fo *FileOrganizer) fileExistsAtTarget(sourcePath, targetPath string) bool {
	if sourcePath == targetPath {
		return false
	}
	_, err := os.Stat(targetPath)
	return err == nil
}

// handleDuplicate handles duplicate files according to configuration.
func (fo *FileOrganizer) handleDuplicate(file FileInfo, targetPath string) error {
	fo.stats.IncrementDuplicatesFound()

	switch fo.config.Processing.DuplicateHandling {
	case "skip":
		fo.logger.Infof("Skipping duplicate file: %s", file.Path)
		fo.stats.IncrementDuplicatesSkipped()
		fo.stats.IncrementFilesSkipped()
		return nil

	case "overwrite":
		fo.logger.Infof("Overwriting existing file: %s", targetPath)
		if fo.config.Processing.MoveFiles {
			err := fo.moveFile(file.Path, targetPath)
			if err == nil {
				fo.stats.IncrementFilesMoved()
			}
			return err
		} else {
			err := fo.copyFile(file.Path, targetPath)
			if err == nil {
				fo.stats.IncrementFilesCopied()
			}
			return err
		}

	case "rename":
		newTargetPath := fo.generateUniqueFilename(targetPath)
		fo.logger.Infof("Renaming duplicate file: %s -> %s", file.Path, newTargetPath)

		if fo.config.Processing.MoveFiles {
			err := fo.moveFile(file.Path, newTargetPath)
			if err == nil {
				fo.stats.IncrementFilesMoved()
				fo.stats.IncrementDuplicatesRenamed()
			}
			return err
		} else {
			err := fo.copyFile(file.Path, newTargetPath)
			if err == nil {
				fo.stats.IncrementFilesCopied()
				fo.stats.IncrementDuplicatesRenamed()
			}
			return err
		}

	default:
		return fmt.Errorf("unknown duplicate handling strategy: %s", fo.config.Processing.DuplicateHandling)
	}
}

// generateUniqueFilename returns a unique filename by adding a counter.
func (fo *FileOrganizer) generateUniqueFilename(basePath string) string {
	dir := filepath.Dir(basePath)
	name := filepath.Base(basePath)
	ext := filepath.Ext(name)
	nameWithoutExt := strings.TrimSuffix(name, ext)

	counter := 1
	for {
		newName := fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		newPath := filepath.Join(dir, newName)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}

// processThumbnail processes the thumbnail file associated with a video.
func (fo *FileOrganizer) processThumbnail(file FileInfo, videoTargetPath string) {
	if file.ThumbnailPath == "" {
		return
	}

	videoDir := filepath.Dir(videoTargetPath)
	videoName := filepath.Base(videoTargetPath)
	videoExt := filepath.Ext(videoName)
	thmName := strings.TrimSuffix(videoName, videoExt) + ".thm"
	thmTargetPath := filepath.Join(videoDir, thmName)

	var err error
	if fo.config.Processing.MoveFiles {
		err = fo.moveFile(file.ThumbnailPath, thmTargetPath)
	} else {
		err = fo.copyFile(file.ThumbnailPath, thmTargetPath)
	}

	if err != nil {
		fo.logger.Errorf("Could not process thumbnail %s: %v", file.ThumbnailPath, err)
		fo.stats.AddError(file.ThumbnailPath, "thumbnail_processing", err.Error())
	} else {
		fo.logger.Debugf("Processed thumbnail: %s -> %s", file.ThumbnailPath, thmTargetPath)
	}
}

// createDirectory creates a directory and its parents if they do not exist.
func (fo *FileOrganizer) createDirectory(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return err
		}
		fo.stats.IncrementDirectoriesCreated()
		fo.logger.Debugf("Created directory: %s", dirPath)
	}
	return nil
}

// moveFile moves a file from source to destination.
func (fo *FileOrganizer) moveFile(sourcePath, destPath string) error {
	if fo.config.Processing.CreateBackups {
		if err := fo.createBackup(sourcePath); err != nil {
			fo.logger.Warnf("Could not create backup for %s: %v", sourcePath, err)
		}
	}
	return os.Rename(sourcePath, destPath)
}

// copyFile copies a file from source to destination.
func (fo *FileOrganizer) copyFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	return os.Chmod(destPath, sourceInfo.Mode())
}

// createBackup creates a backup of a file.
func (fo *FileOrganizer) createBackup(filePath string) error {
	backupPath := filePath + ".backup"
	return fo.copyFile(filePath, backupPath)
}

// isSupportedFile returns true if a file extension is supported.
func (fo *FileOrganizer) isSupportedFile(ext string) bool {
	return fo.config.IsImageExtension(ext) || fo.config.IsVideoExtension(ext)
}

// isAlreadyOrganized returns true if a directory appears to be already organized.
func (fo *FileOrganizer) isAlreadyOrganized(dirPath string) bool {
	dirName := filepath.Base(dirPath)
	datePatterns := []string{
		"2006",
		"2006-01",
		"2006/01",
		"2006-01-02",
		"2006/01/02",
	}

	for _, pattern := range datePatterns {
		if _, err := time.Parse(pattern, dirName); err == nil {
			return true
		}
	}

	return false
}

// dryRunProcess simulates the organization process without making changes.
func (fo *FileOrganizer) dryRunProcess(files []FileInfo) error {
	fo.logger.Info("Starting dry-run process")

	var wg sync.WaitGroup
	fileChan := make(chan FileInfo, fo.config.Performance.BatchSize)

	for i := 0; i < fo.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fo.dryRunWorker(fileChan)
		}()
	}

	go func() {
		defer close(fileChan)
		for _, file := range files {
			fileChan <- file
		}
	}()

	wg.Wait()

	fo.stats.Finalize()
	fo.logger.Info("Dry-run process completed")
	return nil
}

// dryRunWorker processes files in dry-run mode.
func (fo *FileOrganizer) dryRunWorker(fileChan <-chan FileInfo) {
	for file := range fileChan {
		fo.processDryRunFile(file)
	}
}

// processDryRunFile processes a single file in dry-run mode.
func (fo *FileOrganizer) processDryRunFile(file FileInfo) {
	fo.stats.IncrementFilesProcessed()

	date, err := fo.extractDate(file)
	if err != nil {
		msg := fmt.Sprintf("DRY-RUN: Would skip %s (no date): %v", file.Path, err)
		fo.logger.Infof(msg)
		if fo.logHook != nil {
			fo.logHook("info", msg)
		}
		fo.stats.IncrementFilesWithoutDates()
		return
	}

	targetPath, err := fo.generateTargetPath(file, *date)
	if err != nil {
		msg := fmt.Sprintf("DRY-RUN: Could not generate target path for %s: %v", file.Path, err)
		fo.logger.Errorf(msg)
		if fo.logHook != nil {
			fo.logHook("error", msg)
		}
		fo.stats.IncrementFilesWithErrors()
		return
	}

	if fo.fileExistsAtTarget(file.Path, targetPath) {
		msg := fmt.Sprintf("DRY-RUN: Would handle duplicate for %s -> %s", file.Path, targetPath)
		fo.logger.Infof(msg)
		if fo.logHook != nil {
			fo.logHook("info", msg)
		}
		fo.stats.IncrementDuplicatesFound()
	} else {
		action := "move"
		if !fo.config.Processing.MoveFiles {
			action = "copy"
		}
		msg := fmt.Sprintf("DRY-RUN: Would %s %s -> %s", action, file.Path, targetPath)
		fo.logger.Infof(msg)
		if fo.logHook != nil {
			fo.logHook("info", msg)
		}
		fo.stats.IncrementFilesOrganized()
	}
}
