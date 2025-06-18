package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

// rootCmd represents the base command when called without any subcommands
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

// scanCmd represents the scan command
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

// testExifCmd represents the test-exif command
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

// serveCmd represents the serve command
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

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")

	// Root command flags
	rootCmd.Flags().StringVar(&sourceDir, "source", "", "source directory containing media files")
	rootCmd.Flags().StringVar(&targetDir, "target", "", "target directory for organized files (default: organize in place)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "simulate organization without making changes")

	// Serve command flags
	serveCmd.Flags().IntVar(&port, "port", 8080, "port to run web server on")

	// Add subcommands
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(testExifCmd)
	rootCmd.AddCommand(serveCmd)
}

// initConfig reads in config file and ENV variables
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in current directory, home directory, and /etc
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.photo-sorter")
		viper.AddConfigPath("/etc/photo-sorter")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}
}

func runOrganize(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override dry-run if flag is set
	if dryRun {
		cfg.Security.DryRun = true
	}

	log := setupLogger(cfg)
	stats := statistics.NewStatistics()
	dateExtractor := extractor.NewEXIFExtractor(log)

	org := organizer.NewFileOrganizer(cfg, log, stats, dateExtractor)

	err = org.OrganizeFiles()
	if err != nil {
		return fmt.Errorf("organization failed: %w", err)
	}

	// Print statistics
	if !quiet {
		fmt.Println("\n" + stats.GetSummary())
	}

	return nil
}

func runScan(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// For scan command, determine the source directory
	scanDir := cfg.SourceDirectory
	if len(args) > 0 {
		// Use directory from command line argument if provided
		scanDir = args[0]
	}

	// Update config with scan directory
	cfg.SourceDirectory = scanDir
	cfg.Security.DryRun = true // Scan is always dry-run

	fmt.Fprintf(os.Stderr, "Scanning directory: %s\n", scanDir)

	log := setupLogger(cfg)
	stats := statistics.NewStatistics()
	dateExtractor := extractor.NewEXIFExtractor(log)

	org := organizer.NewFileOrganizer(cfg, log, stats, dateExtractor)

	err = org.OrganizeFiles()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Print scan results
	if !quiet {
		fmt.Println("\n==================================================")
		fmt.Println("SCAN RESULTS")
		fmt.Println("==================================================")
		fmt.Println("\n" + stats.GetSummary())
	}

	return nil
}

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

func runServe() error {
	// For web interface, create a minimal config if none exists
	cfg, err := config.LoadConfig("")
	if err != nil {
		// Use default config for web interface
		cfg = config.DefaultConfig()
		cfg.SourceDirectory = "."  // Default to current directory for web interface
		cfg.Security.DryRun = true // Safe default for web interface
	}

	log := setupLogger(cfg)
	server := web.NewServer(cfg, log)

	// Create a channel to listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.Start(port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	fmt.Printf("🚀 PhotoSorter Web Interface started!\n")
	fmt.Printf("📱 Open your browser and go to: http://localhost:%d\n", port)
	fmt.Printf("🛑 Press Ctrl+C to stop the server\n\n")

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\n🛑 Shutting down server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown the server
	if err := server.Stop(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	fmt.Println("✅ Server stopped gracefully")
	return nil
}

func loadConfig(args []string) (*config.Config, error) {
	cfg, err := config.LoadConfig("")
	if err != nil {
		return nil, err
	}

	// Override source directory if provided via flag
	if sourceDir != "" {
		cfg.SourceDirectory = sourceDir
	}

	// Override target directory if provided via flag
	if targetDir != "" {
		cfg.TargetDirectory = &targetDir
	}

	// If no source directory is set and we have args, use the first arg
	if cfg.SourceDirectory == "" && len(args) > 0 {
		cfg.SourceDirectory = args[0]
	}

	// Default to current directory if no source is specified
	if cfg.SourceDirectory == "" {
		cfg.SourceDirectory = "."
	}

	// Validate source directory exists
	if !dirExists(cfg.SourceDirectory) {
		return nil, fmt.Errorf("source directory does not exist: %s", cfg.SourceDirectory)
	}

	return cfg, nil
}

func setupLogger(cfg *config.Config) *logrus.Logger {
	// Create logger config
	loggerCfg := logger.LoggerConfig{
		Level:      cfg.Logging.Level,
		FilePath:   cfg.Logging.FilePath,
		MaxSize:    cfg.Logging.MaxSize,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAge:     cfg.Logging.MaxAge,
		Compress:   cfg.Logging.Compress,
		Console:    !quiet,
	}

	// Override log level based on flags
	if verbose {
		loggerCfg.Level = "debug"
	}
	if quiet {
		loggerCfg.Level = "error"
	}

	log, err := logger.NewLogger(loggerCfg)
	if err != nil {
		// Fallback to basic logger
		log = logrus.New()
		log.SetLevel(logrus.InfoLevel)
	}

	return log
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

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
