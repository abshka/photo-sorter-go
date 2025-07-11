package compressor

import (
	"context"
	"time"
)

// CompressionParams описывает параметры для процесса сжатия изображений.
type CompressionParams struct {
	InputPaths []string // Пути к файлам или директориям для обработки
	TargetDir  string   // Директория для сохранения сжатых файлов (теперь всегда target_directory)
	Quality    int      // Качество JPEG/WebP (1-100)
	Threshold  float64  // Если размер сжатого >= оригинал * threshold, сохранять оригинал
	Formats    []string // Поддерживаемые форматы (расширения)
}

// CompressionResult описывает результат сжатия одного файла.
type CompressionResult struct {
	InputPath       string    // Исходный путь
	OutputPath      string    // Путь к сжатому файлу
	OriginalSize    int64     // Размер исходного файла (байт)
	CompressedSize  int64     // Размер сжатого файла (байт)
	PercentageSaved float64   // Процент экономии (0-100)
	Action          string    // "compressed", "original", "skipped", "error"
	Message         string    // Сообщение или описание ошибки
	Success         bool      // Было ли успешно сжато
	StartedAt       time.Time // Время начала обработки
	FinishedAt      time.Time // Время окончания обработки
	Error           error     // Ошибка (если есть)
}

// Compressor определяет интерфейс для сжатия изображений.
type Compressor interface {
	// Compress обрабатывает список файлов/директорий согласно параметрам.
	// Возвращает срез результатов по каждому файлу.
	Compress(ctx context.Context, params CompressionParams) ([]CompressionResult, error)
}
