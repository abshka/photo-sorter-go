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

	"github.com/barasher/go-exiftool"
	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

// DefaultCompressor is the default implementation of the Compressor interface.
type DefaultCompressor struct{}

// NewDefaultCompressor creates a new DefaultCompressor instance.
func NewDefaultCompressor() *DefaultCompressor {
	return &DefaultCompressor{}
}

// Compress performs image compression according to the provided parameters.
func (c *DefaultCompressor) Compress(ctx context.Context, params CompressionParams) ([]CompressionResult, error) {
	startGlobal := time.Now()
	files, err := collectImageFiles(params.InputPaths, params.Formats)
	if err != nil {
		return nil, fmt.Errorf("collect files: %w", err)
	}
	if len(files) == 0 {
		return nil, nil
	}

	filesToCompress, err := filterUncompressedImages(files, runtime.NumCPU())
	if err != nil {
		return nil, fmt.Errorf("filter uncompressed: %w", err)
	}
	if len(filesToCompress) == 0 {
		return nil, nil
	}

	if params.TargetDir != "" {
		if err := os.MkdirAll(params.TargetDir, 0755); err != nil {
			return nil, fmt.Errorf("create target dir: %w", err)
		}
	}

	numWorkers := max(runtime.NumCPU(), 2)
	type job struct {
		index int
		path  string
	}
	type result struct {
		index int
		res   CompressionResult
	}

	jobs := make(chan job, len(filesToCompress))
	results := make(chan result, len(filesToCompress))

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

	for i, path := range filesToCompress {
		jobs <- job{index: i, path: path}
	}
	close(jobs)

	wg.Wait()
	close(results)

	resArr := make([]CompressionResult, len(filesToCompress))
	for r := range results {
		resArr[r.index] = r.res
	}

	_ = startGlobal
	return resArr, nil
}

// collectImageFiles recursively collects all files with supported extensions.
func collectImageFiles(inputPaths []string, formats []string) ([]string, error) {
	var files []string
	extSet := make(map[string]struct{})
	for _, f := range formats {
		extSet[strings.ToLower(f)] = struct{}{}
	}
	visit := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
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

// filterUncompressedImages filters out files that already have Software=PhotoSorter in EXIF (JPEG/JPG).
func filterUncompressedImages(files []string, numWorkers int) ([]string, error) {
	type result struct {
		path string
		keep bool
	}
	jobs := make(chan string, len(files))
	results := make(chan result, len(files))

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			for path := range jobs {
				ext := strings.ToLower(filepath.Ext(path))
				keep := true
				if ext == ".jpg" || ext == ".jpeg" {
					keep = !hasPhotoSorterSoftwareFlag(path)
				}
				results <- result{path: path, keep: keep}
			}
		}()
	}
	for _, path := range files {
		jobs <- path
	}
	close(jobs)

	wg.Wait()
	close(results)

	var filtered []string
	for r := range results {
		if r.keep {
			filtered = append(filtered, r.path)
		}
	}
	return filtered, nil
}

// hasPhotoSorterSoftwareFlag returns true if the EXIF Software tag contains "PhotoSorter".
func hasPhotoSorterSoftwareFlag(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return false
	}
	tag, err := x.Get(exif.Software)
	if err != nil {
		return false
	}
	val, err := tag.StringVal()
	if err != nil {
		return false
	}
	return strings.Contains(val, "PhotoSorter")
}

// compressOne compresses a single file and returns a CompressionResult.
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

	img, err := imaging.Open(inputPath)
	if err != nil {
		res.Action = "error"
		res.Message = fmt.Sprintf("open error: %v", err)
		res.Error = err
		res.FinishedAt = time.Now()
		fmt.Printf("Compression error for %s: %s\n", inputPath, res.Message)
		return res
	}

	outPath := filepath.Join(params.TargetDir, filepath.Base(inputPath))
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		res.Action = "error"
		res.Message = fmt.Sprintf("mkdir error: %v", err)
		res.Error = err
		res.FinishedAt = time.Now()
		return res
	}
	res.OutputPath = outPath

	tmpPath := outPath + ".tmp"
	var saveErr error

	var buf bytes.Buffer
	err = imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(params.Quality))
	if err != nil {
		saveErr = fmt.Errorf("encode error: %w", err)
	} else {
		err = os.WriteFile(tmpPath, buf.Bytes(), 0644)
		if err != nil {
			saveErr = fmt.Errorf("write tmp file error: %w", err)
		} else {
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

// copyFile copies file src to dst.
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

// ioCopy is an alias for io.Copy.
func ioCopy(dst *os.File, src *os.File) (written int64, err error) {
	return io.Copy(dst, src)
}

// copyExifAndSetPhotoSorterMark copies EXIF from src to dst and sets Software=PhotoSorter Compressed using exiftool.
func copyExifAndSetPhotoSorterMark(src, dst string) error {
	cmdCopy := exec.Command("exiftool", "-TagsFromFile", src, "-overwrite_original", dst)
	if err := cmdCopy.Run(); err != nil {
		return fmt.Errorf("exiftool copy failed: %v", err)
	}
	cmdSet := exec.Command("exiftool", "-overwrite_original", "-Software=PhotoSorter Compressed", dst)
	if err := cmdSet.Run(); err != nil {
		return fmt.Errorf("exiftool set Software failed: %v", err)
	}
	return nil
}

// hasPhotoSorterMarkExiftool checks if the EXIF Software tag contains "PhotoSorter" using exiftool.
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
