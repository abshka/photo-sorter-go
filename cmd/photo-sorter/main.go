package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"photo-sorter-go/internal/compressor"
	"photo-sorter-go/internal/config"
	"photo-sorter-go/internal/extractor"
	"photo-sorter-go/internal/logger"
	"photo-sorter-go/internal/organizer"
	"photo-sorter-go/internal/statistics"
	"photo-sorter-go/internal/web"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	sourceDir string
	targetDir string
	dryRun    bool
	verbose   bool
	quiet     bool
	version   string
	buildTime string
	port      int
)

// rootCmd is the base command for the CLI.
var rootCmd = &cobra.Command{
	Use:   "photo-sorter",
	Short: "Automatically organize photos and videos by date",
	Long: `PhotoSorter is a tool that automatically organizes your photos and videos
by extracting date information from EXIF metadata and sorting them into
date-based directory structures.

Features:
- Organizes photos by EXIF date information
- Handles video files and their thumbnails (MPG/THM pairs)
- Supports various image formats (JPEG, PNG, TIFF, RAW)
- Configurable date folder structures
- Duplicate handling strategies
- Dry-run mode for safe testing
- Comprehensive logging and statistics`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOrganize(args)
	},
}

// scanCmd scans a directory and shows statistics without organizing files.
var scanCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan directory and show statistics without organizing files",
	Long: `Scan the specified directory (or current directory) and display
statistics about found media files without actually organizing them.
This is useful for understanding what files would be processed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScan(args)
	},
}

// testExifCmd tests EXIF extraction on a specific file.
var testExifCmd = &cobra.Command{
	Use:   "test-exif <file>",
	Short: "Test EXIF extraction on a specific file",
	Long: `Tests EXIF extraction on a specific file and shows detailed metadata information.
This is useful for debugging date extraction issues.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTestExif(args[0])
	},
}

// serveCmd starts the web interface server.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web interface server",
	Long: `Starts a web server with a graphical interface for PhotoSorter.
The web interface allows you to:
- Browse and select directories
- Configure sorting options
- Monitor sorting progress in real-time
- View statistics and results

Access the interface at http://localhost:<port> (default: 8080)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe()
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")

	rootCmd.Flags().StringVar(&sourceDir, "source", "", "source directory containing media files")
	rootCmd.Flags().StringVar(&targetDir, "target", "", "target directory for organized files (default: organize in place)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "simulate organization without making changes")

	serveCmd.Flags().IntVar(&port, "port", 8080, "port to run web server on")

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(testExifCmd)
	rootCmd.AddCommand(serveCmd)
}

// initConfig loads configuration file and environment variables.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.photo-sorter")
		viper.AddConfigPath("/etc/photo-sorter")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}
}

// runOrganize executes the main organization logic.
func runOrganize(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if dryRun {
		cfg.Security.DryRun = true
	}

	log := setupLogger(cfg)
	stats := statistics.NewStatistics()
	dateExtractor := extractor.NewEXIFExtractor(log)

	compressor := compressor.NewDefaultCompressor()
	org := organizer.NewFileOrganizer(cfg, log, stats, dateExtractor, compressor)

	err = org.OrganizeFiles()
	if err != nil {
		return fmt.Errorf("organization failed: %w", err)
	}

	if !quiet {
		fmt.Println("\n" + stats.GetSummary())
	}

	return nil
}

// runScan scans the directory and prints statistics.
func runScan(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	scanDir := cfg.SourceDirectory
	if len(args) > 0 {
		scanDir = args[0]
	}

	cfg.SourceDirectory = scanDir
	cfg.Security.DryRun = true

	fmt.Fprintf(os.Stderr, "Scanning directory: %s\n", scanDir)

	log := setupLogger(cfg)
	stats := statistics.NewStatistics()
	dateExtractor := extractor.NewEXIFExtractor(log)

	compressor := compressor.NewDefaultCompressor()
	org := organizer.NewFileOrganizer(cfg, log, stats, dateExtractor, compressor)

	err = org.OrganizeFiles()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if !quiet {
		fmt.Println("\n==================================================")
		fmt.Println("SCAN RESULTS")
		fmt.Println("==================================================")
		fmt.Println("\n" + stats.GetSummary())
	}

	return nil
}

// runTestExif tests EXIF extraction for a given file.
func runTestExif(filePath string) error {
	if !fileExists(filePath) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	fmt.Printf("Testing EXIF extraction for: %s\n", filePath)

	log := logrus.New()
	dateExtractor := extractor.NewEXIFExtractor(log)
	date, err := dateExtractor.ExtractDate(filePath)

	if err != nil {
		fmt.Printf("Error extracting date: %v\n", err)
		return nil
	}

	if date.IsZero() {
		fmt.Println("No date found in EXIF data")
	} else {
		fmt.Printf("Extracted date: %s\n", date.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// runServe starts the web server and handles graceful shutdown.
func runServe() error {
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "CONFIG LOAD ERROR: %v\n", err)
		cfg = config.DefaultConfig()
		cfg.SourceDirectory = "."
		cfg.Security.DryRun = true
	}

	log := setupLogger(cfg)
	compressor := compressor.NewDefaultCompressor()
	server := web.NewServer(cfg, log, compressor)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.Start(port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	fmt.Printf("🚀 PhotoSorter Web Interface started!\n")
	fmt.Printf("📱 Open your browser and go to: http://localhost:%d\n", port)
	fmt.Printf("🛑 Press Ctrl+C to stop the server\n\n")

	<-sigChan
	fmt.Println("\n🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	fmt.Println("✅ Server stopped gracefully")
	return nil
}

// loadConfig loads configuration and applies CLI overrides.
func loadConfig(args []string) (*config.Config, error) {
	cfg, err := config.LoadConfig("")
	if err != nil {
		return nil, err
	}

	if sourceDir != "" {
		cfg.SourceDirectory = sourceDir
	}

	if targetDir != "" {
		cfg.TargetDirectory = &targetDir
	}

	if cfg.SourceDirectory == "" && len(args) > 0 {
		cfg.SourceDirectory = args[0]
	}

	if cfg.SourceDirectory == "" {
		cfg.SourceDirectory = "."
	}

	if !dirExists(cfg.SourceDirectory) {
		return nil, fmt.Errorf("source directory does not exist: %s", cfg.SourceDirectory)
	}

	return cfg, nil
}

// setupLogger configures and returns a logger.
func setupLogger(cfg *config.Config) *logrus.Logger {
	loggerCfg := logger.LoggerConfig{
		Level:      cfg.Logging.Level,
		FilePath:   cfg.Logging.FilePath,
		MaxSize:    cfg.Logging.MaxSize,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAge:     cfg.Logging.MaxAge,
		Compress:   cfg.Logging.Compress,
		Console:    !quiet,
	}

	if verbose {
		loggerCfg.Level = "debug"
	}
	if quiet {
		loggerCfg.Level = "error"
	}

	log, err := logger.NewLogger(loggerCfg)
	if err != nil {
		log = logrus.New()
		log.SetLevel(logrus.InfoLevel)
	}

	return log
}

// fileExists returns true if the given path exists and is a file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists returns true if the given path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
