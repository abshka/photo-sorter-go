package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"photo-sorter-go/internal/config"
	"photo-sorter-go/internal/extractor"
	"photo-sorter-go/internal/organizer"
	"photo-sorter-go/internal/statistics"

	"github.com/sirupsen/logrus"
)

// Test configuration and functionality
func main() {
	fmt.Println("🧪 PhotoSorter Test Suite")
	fmt.Println(strings.Repeat("=", 50))

	// Test 1: Test different date formats
	fmt.Println("\n📅 Test 1: Date Format Configuration")
	testDateFormats()

	// Test 2: Test move vs copy functionality
	fmt.Println("\n📁 Test 2: Move vs Copy Configuration")
	testMoveVsCopy()

	// Test 3: Test duplicate handling
	fmt.Println("\n🔄 Test 3: Duplicate Handling")
	testDuplicateHandling()

	// Test 4: Test dry run vs live mode
	fmt.Println("\n🔧 Test 4: Dry Run vs Live Mode")
	testDryRunMode()

	fmt.Println("\n✅ All tests completed!")
}

func testDateFormats() {
	formats := []struct {
		format      string
		description string
		expected    string
	}{
		{"2006/01/02", "Year/Month/Day", "2024/12/25"},
		{"2006/01", "Year/Month", "2024/12"},
		{"2006", "Year Only", "2024"},
		{"2006-01-02", "Year-Month-Day", "2024-12-25"},
		{"2006-01", "Year-Month", "2024-12"},
	}

	testDate := time.Date(2024, 12, 25, 15, 30, 45, 0, time.UTC)

	for _, f := range formats {
		result := testDate.Format(f.format)
		status := "✅"
		if result != f.expected {
			status = "❌"
		}
		fmt.Printf("  %s %s (%s): %s -> %s\n",
			status, f.description, f.format, f.expected, result)
	}
}

func testMoveVsCopy() {
	// Create temporary test directory
	testDir, err := os.MkdirTemp("", "photosorter_test_")
	if err != nil {
		fmt.Printf("❌ Failed to create test directory: %v\n", err)
		return
	}
	defer os.RemoveAll(testDir)

	// Create test file
	testFile := filepath.Join(testDir, "test_photo.jpg")
	err = os.WriteFile(testFile, []byte("test image data"), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to create test file: %v\n", err)
		return
	}

	fmt.Printf("  📝 Created test environment: %s\n", testDir)

	// Test Move configuration
	fmt.Printf("  🔄 Testing MOVE files configuration...\n")
	testMoveOrCopyConfig(testDir, true)

	// Recreate test file for copy test
	err = os.WriteFile(testFile, []byte("test image data"), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to recreate test file: %v\n", err)
		return
	}

	// Test Copy configuration
	fmt.Printf("  📋 Testing COPY files configuration...\n")
	testMoveOrCopyConfig(testDir, false)
}

func testMoveOrCopyConfig(testDir string, moveFiles bool) {
	cfg := config.DefaultConfig()
	cfg.SourceDirectory = testDir
	cfg.Processing.MoveFiles = moveFiles
	cfg.Security.DryRun = true // Safe testing
	cfg.DateFormat = "2006"    // Simple format for testing

	log := logrus.New()
	log.SetLevel(logrus.WarnLevel) // Reduce noise
	stats := statistics.NewStatistics()
	dateExtractor := extractor.NewEXIFExtractor(log)

	org := organizer.NewFileOrganizer(cfg, log, stats, dateExtractor)

	err := org.OrganizeFiles()
	if err != nil {
		fmt.Printf("    ❌ Organization failed: %v\n", err)
		return
	}

	mode := "MOVE"
	if !moveFiles {
		mode = "COPY"
	}

	fmt.Printf("    ✅ %s mode configuration test passed\n", mode)
	fmt.Printf("    📊 Files found: %d, processed: %d\n",
		stats.TotalFilesFound, stats.TotalFilesProcessed)
}

func testDuplicateHandling() {
	strategies := []string{"rename", "skip", "overwrite"}

	for _, strategy := range strategies {
		fmt.Printf("  🔄 Testing duplicate handling: %s\n", strategy)

		cfg := config.DefaultConfig()
		cfg.Processing.DuplicateHandling = strategy

		err := cfg.Validate()
		if err != nil {
			fmt.Printf("    ❌ Invalid strategy '%s': %v\n", strategy, err)
		} else {
			fmt.Printf("    ✅ Strategy '%s' is valid\n", strategy)
		}
	}

	// Test invalid strategy
	fmt.Printf("  🔄 Testing invalid duplicate strategy...\n")
	cfg := config.DefaultConfig()
	cfg.Processing.DuplicateHandling = "invalid_strategy"

	err := cfg.Validate()
	if err != nil {
		fmt.Printf("    ✅ Correctly rejected invalid strategy: %v\n", err)
	} else {
		fmt.Printf("    ❌ Should have rejected invalid strategy\n")
	}
}

func testDryRunMode() {
	// Create temporary test directory
	testDir, err := os.MkdirTemp("", "photosorter_dryrun_")
	if err != nil {
		fmt.Printf("❌ Failed to create test directory: %v\n", err)
		return
	}
	defer os.RemoveAll(testDir)

	// Create test file
	testFile := filepath.Join(testDir, "test_photo.jpg")
	err = os.WriteFile(testFile, []byte("test image data"), 0644)
	if err != nil {
		fmt.Printf("❌ Failed to create test file: %v\n", err)
		return
	}

	originalSize := getDirectorySize(testDir)

	// Test dry run mode
	fmt.Printf("  🔧 Testing DRY RUN mode...\n")
	cfg := config.DefaultConfig()
	cfg.SourceDirectory = testDir
	cfg.Security.DryRun = true
	cfg.DateFormat = "2006"

	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)
	stats := statistics.NewStatistics()
	dateExtractor := extractor.NewEXIFExtractor(log)

	org := organizer.NewFileOrganizer(cfg, log, stats, dateExtractor)
	err = org.OrganizeFiles()

	if err != nil {
		fmt.Printf("    ❌ Dry run failed: %v\n", err)
		return
	}

	newSize := getDirectorySize(testDir)

	if originalSize == newSize {
		fmt.Printf("    ✅ Dry run mode preserved original files\n")
	} else {
		fmt.Printf("    ❌ Dry run mode modified files (original: %d, new: %d)\n",
			originalSize, newSize)
	}

	fmt.Printf("    📊 Files processed in dry run: %d\n", stats.TotalFilesProcessed)

	// Test live mode (but with copy to be safe)
	fmt.Printf("  🔧 Testing LIVE mode (copy)...\n")
	cfg.Security.DryRun = false
	cfg.Processing.MoveFiles = false // Use copy for safety

	stats2 := statistics.NewStatistics()
	org2 := organizer.NewFileOrganizer(cfg, log, stats2, dateExtractor)
	err = org2.OrganizeFiles()

	if err != nil {
		fmt.Printf("    ❌ Live mode failed: %v\n", err)
		return
	}

	finalSize := getDirectorySize(testDir)

	if finalSize > originalSize {
		fmt.Printf("    ✅ Live mode created organized copies\n")
	} else {
		fmt.Printf("    ❌ Live mode did not create expected files\n")
	}

	fmt.Printf("    📊 Files processed in live mode: %d\n", stats2.TotalFilesProcessed)
}

func getDirectorySize(dirPath string) int64 {
	var size int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0
	}

	return size
}
