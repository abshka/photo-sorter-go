package compressor

import (
	"context"
	"time"
)

// CompressionParams defines parameters for the image compression process.
type CompressionParams struct {
	InputPaths []string
	TargetDir  string
	Quality    int
	Threshold  float64
	Formats    []string
}

// CompressionResult describes the result of compressing a single file.
type CompressionResult struct {
	InputPath       string
	OutputPath      string
	OriginalSize    int64
	CompressedSize  int64
	PercentageSaved float64
	Action          string
	Message         string
	Success         bool
	StartedAt       time.Time
	FinishedAt      time.Time
	Error           error
}

// Compressor defines the interface for image compression.
type Compressor interface {
	// Compress processes a list of files or directories according to the parameters.
	// Returns a slice of results for each file.
	Compress(ctx context.Context, params CompressionParams) ([]CompressionResult, error)
}
