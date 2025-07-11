package compressor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	exiftool "github.com/barasher/go-exiftool"
	"github.com/disintegration/imaging"
)

// DefaultCompressor — базовая реализация интерфейса Compressor.
type DefaultCompressor struct{}

// NewDefaultCompressor создает новый экземпляр DefaultCompressor.
func NewDefaultCompressor() *DefaultCompressor {
	return &DefaultCompressor{}
}

// Compress реализует базовую логику компрессии изображений.
func (c *DefaultCompressor) Compress(ctx context.Context, params CompressionParams) ([]CompressionResult, error) {
	startGlobal := time.Now()
	files, err := collectImageFiles(params.InputPaths, params.Formats)
	if err != nil {
		return nil, fmt.Errorf("collect files: %w", err)
	}
	if len(files) == 0 {
		return nil, nil
	}

	// Создаем целевую директорию, если нужно (TargetDir)
	if params.TargetDir != "" {
		if err := os.MkdirAll(params.TargetDir, 0755); err != nil {
			return nil, fmt.Errorf("create target dir: %w", err)
		}
	}

	// Worker pool
	numWorkers := max(runtime.NumCPU(), 2)
	type job struct {
		index int
		path  string
	}
	type result struct {
		index int
		res   CompressionResult
	}

	jobs := make(chan job, len(files))
	results := make(chan result, len(files))

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				r := compressOne(j.path, params)
				results <- result{index: j.index, res: r}
			}
		}()
	}

	for i, path := range files {
		jobs <- job{index: i, path: path}
	}
	close(jobs)

	wg.Wait()
	close(results)

	// Собираем результаты в правильном порядке
	resArr := make([]CompressionResult, len(files))
	for r := range results {
		resArr[r.index] = r.res
	}

	_ = startGlobal // можно добавить в CompressionResult общее время, если потребуется
	return resArr, nil
}

// collectImageFiles рекурсивно собирает все файлы с поддерживаемыми расширениями.
func collectImageFiles(inputPaths []string, formats []string) ([]string, error) {
	var files []string
	extSet := make(map[string]struct{})
	for _, f := range formats {
		extSet[strings.ToLower(f)] = struct{}{}
	}
	visit := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // игнорируем ошибки доступа
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := extSet[ext]; ok {
			files = append(files, path)
		}
		return nil
	}
	for _, in := range inputPaths {
		info, err := os.Stat(in)
		if err != nil {
			continue
		}
		if info.IsDir() {
			_ = filepath.WalkDir(in, visit)
		} else {
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if _, ok := extSet[ext]; ok {
				files = append(files, in)
			}
		}
	}
	return files, nil
}

// compressOne сжимает один файл и возвращает CompressionResult.
func compressOne(inputPath string, params CompressionParams) CompressionResult {
	start := time.Now()
	res := CompressionResult{
		InputPath: inputPath,
		StartedAt: start,
	}
	info, err := os.Stat(inputPath)
	if err != nil {
		res.Action = "error"
		res.Message = fmt.Sprintf("stat error: %v", err)
		res.Error = err
		res.FinishedAt = time.Now()
		fmt.Printf("Compression error for %s: %s\n", inputPath, res.Message)
		return res
	}
	res.OriginalSize = info.Size()

	extOrig := filepath.Ext(inputPath)
	ext := strings.ToLower(extOrig)

	// --- Проверка метки PhotoSorter в EXIF (только для JPEG) ---
	if ext == ".jpg" || ext == ".jpeg" {
		hasMark, err := hasPhotoSorterMarkExiftool(inputPath)
		if err == nil && hasMark {
			res.Action = "skipped"
			res.Message = "Already compressed by PhotoSorter"
			res.Success = true
			res.FinishedAt = time.Now()
			return res
		}
	}

	// Открываем изображение
	img, err := imaging.Open(inputPath)
	if err != nil {
		res.Action = "error"
		res.Message = fmt.Sprintf("open error: %v", err)
		res.Error = err
		res.FinishedAt = time.Now()
		fmt.Printf("Compression error for %s: %s\n", inputPath, res.Message)
		return res
	}

	// Всегда сохраняем в TargetDir без структуры
	outPath := filepath.Join(params.TargetDir, filepath.Base(inputPath))
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		res.Action = "error"
		res.Message = fmt.Sprintf("mkdir error: %v", err)
		res.Error = err
		res.FinishedAt = time.Now()
		return res
	}
	res.OutputPath = outPath

	// Сохраняем с нужным качеством
	tmpPath := outPath + ".tmp"
	// extOrig := filepath.Ext(inputPath)
	var saveErr error

	// Создаем буфер для сжатого изображения
	var buf bytes.Buffer
	// Кодируем изображение в JPEG с нужным качеством, не полагаясь на расширение файла
	err = imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(params.Quality))
	if err != nil {
		saveErr = fmt.Errorf("encode error: %w", err)
	} else {
		// Записываем сжатые данные во временный файл
		err = os.WriteFile(tmpPath, buf.Bytes(), 0644)
		if err != nil {
			saveErr = fmt.Errorf("write tmp file error: %w", err)
		} else {
			// Копируем EXIF только для оригинальных JPEG файлов и добавляем метку через exiftool
			if strings.ToLower(extOrig) == ".jpg" || strings.ToLower(extOrig) == ".jpeg" {
				exifErr := copyExifAndSetPhotoSorterMark(inputPath, tmpPath)
				if exifErr != nil {
					res.Message = fmt.Sprintf("warning: exif not copied/marked: %v", exifErr)
				}
			}
		}
	}

	if saveErr != nil {
		res.Action = "error"
		res.Message = fmt.Sprintf("save error: %v", saveErr)
		res.Error = saveErr
		res.FinishedAt = time.Now()
		fmt.Printf("Compression error for %s: %s\n", inputPath, res.Message)
		return res
	}

	// Сравниваем размеры
	origSize := res.OriginalSize
	compInfo, err := os.Stat(tmpPath)
	if err != nil {
		res.Action = "error"
		res.Message = fmt.Sprintf("stat compressed error: %v", err)
		res.Error = err
		res.FinishedAt = time.Now()
		_ = os.Remove(tmpPath)
		fmt.Printf("Compression error for %s: %s\n", inputPath, res.Message)
		return res
	}
	compSize := compInfo.Size()
	res.CompressedSize = compSize

	threshold := params.Threshold
	if threshold <= 0 {
		threshold = 1.01
	}
	if float64(compSize) >= float64(origSize)*threshold {
		// Сохраняем оригинал
		copyErr := copyFile(inputPath, outPath)
		if copyErr != nil {
			res.Action = "error"
			res.Message = fmt.Sprintf("copy original error: %v", copyErr)
			res.Error = copyErr
			res.FinishedAt = time.Now()
			_ = os.Remove(tmpPath)
			fmt.Printf("Compression error for %s: %s\n", inputPath, res.Message)
			return res
		}
		res.Action = "original"
		res.Message = "Compressed file not smaller than original, saved original"
		res.PercentageSaved = 0
		_ = os.Remove(tmpPath)
	} else {
		// Перемещаем tmp -> outPath
		moveErr := os.Rename(tmpPath, outPath)
		if moveErr != nil {
			res.Action = "error"
			res.Message = fmt.Sprintf("rename error: %v", moveErr)
			res.Error = moveErr
			res.FinishedAt = time.Now()
			return res
		}
		res.Action = "compressed"
		res.Message = "Image compressed"
		res.PercentageSaved = float64(origSize-compSize) * 100 / float64(origSize)
	}
	res.Success = (res.Action == "compressed" || res.Action == "original")
	res.FinishedAt = time.Now()
	return res
}

// copyFile копирует файл src -> dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	_, err = ioCopy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

// ioCopy — алиас для io.Copy, чтобы не тянуть весь io пакет в начало.
func ioCopy(dst *os.File, src *os.File) (written int64, err error) {
	return io.Copy(dst, src)
}

// copyExifAndSetPhotoSorterMark копирует EXIF из src в dst и устанавливает Software=PhotoSorter Compressed через exiftool.
func copyExifAndSetPhotoSorterMark(src, dst string) error {
	// Копируем EXIF из src в dst (через exiftool)
	cmdCopy := exec.Command("exiftool", "-TagsFromFile", src, "-overwrite_original", dst)
	if err := cmdCopy.Run(); err != nil {
		return fmt.Errorf("exiftool copy failed: %v", err)
	}
	// Устанавливаем Software
	cmdSet := exec.Command("exiftool", "-overwrite_original", "-Software=PhotoSorter Compressed", dst)
	if err := cmdSet.Run(); err != nil {
		return fmt.Errorf("exiftool set Software failed: %v", err)
	}
	return nil
}

// hasPhotoSorterMarkExiftool проверяет, есть ли в EXIF Software метка PhotoSorter через exiftool
func hasPhotoSorterMarkExiftool(path string) (bool, error) {
	et, err := exiftool.NewExiftool()
	if err != nil {
		return false, err
	}
	defer et.Close()
	files := et.ExtractMetadata(path)
	if len(files) == 0 || files[0].Err != nil {
		return false, files[0].Err
	}
	sw := files[0].Fields["Software"]
	if swStr, ok := sw.(string); ok && strings.Contains(swStr, "PhotoSorter Compressed") {
		return true, nil
	}
	return false, nil
}
