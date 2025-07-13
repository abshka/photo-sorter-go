package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DateFormatOption defines a predefined date format option.
type DateFormatOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Format      string `json:"format"`
	Example     string `json:"example"`
	Description string `json:"description"`
}

// CompressorConfig holds image compression settings.
type CompressorConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	Quality   int      `mapstructure:"quality"`
	Threshold float64  `mapstructure:"threshold"`
	Formats   []string `mapstructure:"formats"`
	// OutputDir string   `mapstructure:"output_dir"` // Deprecated
}

// Config is the main configuration structure.
type Config struct {
	SourceDirectory     string            `mapstructure:"source_directory" validate:"required"`
	TargetDirectory     *string           `mapstructure:"target_directory"`
	DateFormat          string            `mapstructure:"date_format"`
	SupportedExtensions []string          `mapstructure:"supported_extensions"`
	Processing          ProcessingConfig  `mapstructure:"processing"`
	Video               VideoConfig       `mapstructure:"video"`
	Performance         PerformanceConfig `mapstructure:"performance"`
	Security            SecurityConfig    `mapstructure:"security"`
	Logging             LoggingConfig     `mapstructure:"logging"`
	Compressor          CompressorConfig  `mapstructure:"compressor"`
}

// ProcessingConfig holds file processing settings.
type ProcessingConfig struct {
	MoveFiles         bool   `mapstructure:"move_files"`
	DuplicateHandling string `mapstructure:"duplicate_handling"`
	SkipOrganized     bool   `mapstructure:"skip_organized"`
	CreateBackups     bool   `mapstructure:"create_backups"`
}

// VideoConfig holds video processing settings.
type VideoConfig struct {
	MPGProcessing        MPGProcessingConfig `mapstructure:"mpg_processing"`
	ExtractVideoMetadata bool                `mapstructure:"extract_video_metadata"`
	SupportedExtensions  []string            `mapstructure:"supported_extensions"`
}

// MPGProcessingConfig holds MPG/THM merging settings.
type MPGProcessingConfig struct {
	EnableMerging       bool `mapstructure:"enable_merging"`
	DeleteTHMAfterMerge bool `mapstructure:"delete_thm_after_merge"`
	CreateBackup        bool `mapstructure:"create_backup"`
}

// PerformanceConfig holds performance tuning settings.
type PerformanceConfig struct {
	BatchSize     int  `mapstructure:"batch_size"`
	WorkerThreads int  `mapstructure:"worker_threads"`
	ShowProgress  bool `mapstructure:"show_progress"`
	CacheSize     int  `mapstructure:"cache_size"`
}

// SecurityConfig holds security and safety settings.
type SecurityConfig struct {
	DryRun             bool `mapstructure:"dry_run"`
	ConfirmBeforeStart bool `mapstructure:"confirm_before_start"`
	MaxFilesPerRun     int  `mapstructure:"max_files_per_run"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// GetAvailableDateFormats returns all available date format options.
func GetAvailableDateFormats() []DateFormatOption {
	return []DateFormatOption{
		{
			ID:          "year_month_day",
			Name:        "Year/Month/Day",
			Format:      "2006/01/02",
			Example:     "2024/12/25",
			Description: "Full date structure with year, month, and day folders",
		},
		{
			ID:          "year_month",
			Name:        "Year/Month",
			Format:      "2006/01",
			Example:     "2024/12",
			Description: "Monthly organization with year and month folders only",
		},
		{
			ID:          "year_only",
			Name:        "Year Only",
			Format:      "2006",
			Example:     "2024",
			Description: "Yearly organization with only year folders",
		},
		{
			ID:          "year_dash_month_dash_day",
			Name:        "Year-Month-Day",
			Format:      "2006-01-02",
			Example:     "2024-12-25",
			Description: "Full date structure with dashes",
		},
		{
			ID:          "year_dash_month",
			Name:        "Year-Month",
			Format:      "2006-01",
			Example:     "2024-12",
			Description: "Monthly organization with dashes",
		},
	}
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() *Config {
	return &Config{
		DateFormat: "2006/01/02",
		SupportedExtensions: []string{
			".jpg", ".jpeg", ".png", ".tiff", ".tif",
			".cr2", ".nef", ".arw", ".dng", ".raw",
		},
		Processing: ProcessingConfig{
			MoveFiles:         true,
			DuplicateHandling: "rename",
			SkipOrganized:     true,
			CreateBackups:     false,
		},
		Video: VideoConfig{
			MPGProcessing: MPGProcessingConfig{
				EnableMerging:       true,
				DeleteTHMAfterMerge: false,
				CreateBackup:        true,
			},
			ExtractVideoMetadata: true,
			SupportedExtensions: []string{
				".mp4", ".avi", ".mov", ".mpg", ".thm",
			},
		},
		Performance: PerformanceConfig{
			BatchSize:     100,
			WorkerThreads: 4,
			ShowProgress:  true,
			CacheSize:     1000,
		},
		Security: SecurityConfig{
			DryRun:             false,
			ConfirmBeforeStart: true,
			MaxFilesPerRun:     0,
		},
		Logging: LoggingConfig{
			Level:      "info",
			FilePath:   "photo-sorter.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     30,
			Compress:   true,
		},
		Compressor: CompressorConfig{
			Enabled:   true,
			Quality:   85,
			Threshold: 1.01,
			Formats:   []string{".jpg", ".jpeg", ".png", ".webp"},
		},
	}
}

// LoadConfig loads configuration from file and environment variables.
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.photo-sorter")
		viper.AddConfigPath("/etc/photo-sorter")
	}

	viper.SetEnvPrefix("PHOTO_SORTER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Validate checks the configuration for correctness.
func (c *Config) Validate() error {
	if c.SourceDirectory == "" {
		return fmt.Errorf("source_directory is required")
	}

	if !isValidPath(c.SourceDirectory) {
		return fmt.Errorf("source_directory does not exist or is not accessible: %s", c.SourceDirectory)
	}

	if c.TargetDirectory != nil && *c.TargetDirectory != "" {
		if !isValidPath(*c.TargetDirectory) {
			return fmt.Errorf("target_directory does not exist or is not accessible: %s", *c.TargetDirectory)
		}
	}

	if c.DateFormat == "" {
		c.DateFormat = "2006/01/02"
	}

	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)
	testFormatted := testTime.Format(c.DateFormat)
	if testFormatted == c.DateFormat {
		return fmt.Errorf("invalid date format: %s", c.DateFormat)
	}

	validStrategies := map[string]bool{
		"rename":    true,
		"skip":      true,
		"overwrite": true,
	}
	if !validStrategies[c.Processing.DuplicateHandling] {
		return fmt.Errorf("invalid duplicate_handling strategy: %s (valid: rename, skip, overwrite)",
			c.Processing.DuplicateHandling)
	}

	c.SupportedExtensions = normalizeExtensions(c.SupportedExtensions)
	c.Video.SupportedExtensions = normalizeExtensions(c.Video.SupportedExtensions)

	if c.Performance.BatchSize <= 0 {
		c.Performance.BatchSize = 100
	}
	if c.Performance.WorkerThreads <= 0 {
		c.Performance.WorkerThreads = 4
	}
	if c.Performance.CacheSize <= 0 {
		c.Performance.CacheSize = 1000
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[strings.ToLower(c.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s (valid: debug, info, warn, error)", c.Logging.Level)
	}

	return nil
}

// GetTargetDirectory returns the target directory or the source directory if target is not set.
func (c *Config) GetTargetDirectory() string {
	if c.TargetDirectory != nil && *c.TargetDirectory != "" {
		return *c.TargetDirectory
	}
	return c.SourceDirectory
}

// IsInPlaceOrganization returns true if files are organized in place.
func (c *Config) IsInPlaceOrganization() bool {
	return c.TargetDirectory == nil || *c.TargetDirectory == "" ||
		*c.TargetDirectory == c.SourceDirectory
}

// GetAllSupportedExtensions returns all supported extensions for images and videos.
func (c *Config) GetAllSupportedExtensions() []string {
	all := make([]string, 0, len(c.SupportedExtensions)+len(c.Video.SupportedExtensions))
	all = append(all, c.SupportedExtensions...)
	all = append(all, c.Video.SupportedExtensions...)
	return all
}

// IsImageExtension returns true if the extension is for an image file.
func (c *Config) IsImageExtension(ext string) bool {
	ext = strings.ToLower(ext)
	return slices.Contains(c.SupportedExtensions, ext)
}

// IsVideoExtension returns true if the extension is for a video file.
func (c *Config) IsVideoExtension(ext string) bool {
	ext = strings.ToLower(ext)
	return slices.Contains(c.Video.SupportedExtensions, ext)
}

// isValidPath checks if the given path exists and is a directory.
func isValidPath(path string) bool {
	if path == "" {
		return false
	}

	expandedPath := os.ExpandEnv(path)
	if strings.HasPrefix(expandedPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		expandedPath = filepath.Join(home, expandedPath[1:])
	}

	stat, err := os.Stat(expandedPath)
	return err == nil && stat.IsDir()
}

// normalizeExtensions returns a normalized slice of file extensions.
func normalizeExtensions(extensions []string) []string {
	normalized := make([]string, len(extensions))
	for i, ext := range extensions {
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalized[i] = ext
	}
	return normalized
}
