# PhotoSorter Go

A powerful command-line tool for automatically organizing photos and videos by date using EXIF metadata. This is a Go port of the original Python PhotoSorter with enhanced performance and cross-platform compatibility.

## Features

- **Flexible Date Organization**: Multiple date folder structure options:
  - Year/Month/Day (2024/12/25) - Default
  - Year/Month (2024/12) - Monthly organization
  - Year Only (2024) - Yearly organization
  - With dashes: 2024-12-25, 2024-12
- **Web Configuration Management**: Edit all settings through the web interface - no config file editing needed
- **Move or Copy Files**: Choose whether to move files or create organized copies
- **EXIF Metadata Extraction**: Extracts dates from image EXIF data with multiple fallback strategies
- **Video Support**: Handles video files and their associated thumbnail files (MPG/THM pairs)
- **Multiple File Formats**: Supports JPEG, PNG, TIFF, RAW formats (CR2, NEF, ARW, DNG), and video files
- **Flexible Configuration**: YAML-based configuration with web-based editor
- **Duplicate Handling**: Configurable strategies for handling duplicate files (rename, skip, overwrite)
- **Dry Run Mode**: Test organization without making any changes
- **Performance Optimized**: Multi-threaded processing with caching for large photo collections
- **Comprehensive Logging**: Structured logging with rotation and detailed statistics
- **Cross-Platform**: Works on Windows, macOS, and Linux
- **Web Interface**: Modern browser-based interface with real-time progress monitoring and configuration management

## Installation

### From Source

```bash
git clone https://github.com/abshka/photo-sorter-go.git
cd photo-sorter-go
go build -o photo-sorter ./cmd/photo-sorter
```

### Binary Releases

Download the latest binary for your platform from the [releases page](https://github.com/abshka/photo-sorter-go/releases).

## Quick Start

### Command Line Interface

1. **Basic usage** - organize photos in the current directory:

   ```bash
   ./photo-sorter
   ```

2. **Organize a specific directory**:

   ```bash
   ./photo-sorter /path/to/your/photos
   ```

3. **Dry run** - see what would happen without making changes:

   ```bash
   ./photo-sorter --dry-run /path/to/your/photos
   ```

4. **Move to a different target directory**:
   ```bash
   ./photo-sorter --source /path/to/photos --target /path/to/organized
   ```

### Web Interface

Start the web server:

```bash
./photo-sorter serve --port 8080
```

Then open your browser to `http://localhost:8080` to access the graphical interface.

**Web Interface Features:**

- **Configuration Editor**: Change date formats, move/copy mode, and duplicate handling
- **Real-time Config Preview**: See current settings at a glance
- Visual directory browser
- Real-time progress monitoring via WebSocket
- Interactive controls for start/stop operations
- Live statistics and logs
- Dry run mode for safe testing
- Responsive design for desktop and mobile

## Commands

### Main Command

```bash
photo-sorter [flags] [directory]
```

**Flags:**

- `--config`: Path to configuration file
- `--dry-run`: Simulate without making changes
- `--source`: Source directory
- `--target`: Target directory
- `--verbose`: Enable debug logging
- `--quiet`: Suppress non-error output

### Scan Command

```bash
photo-sorter scan [directory]
```

Scans a directory and shows statistics without organizing files.

### Test EXIF Command

```bash
photo-sorter test-exif <file>
```

Tests EXIF extraction on a specific file and shows detailed metadata information.

### Web Server Command

```bash
photo-sorter serve [flags]
```

Starts the web interface server.

**Flags:**

- `--port`: Port to run web server on (default: 8080)

## Configuration

PhotoSorter can be configured in two ways:

1. **Web Interface** (Recommended): Use the built-in configuration editor in the web interface
2. **YAML File**: Copy `config.example.yaml` to `config.yaml` and modify as needed

### Configuration Locations

PhotoSorter looks for configuration files in the following order:

1. File specified with `--config` flag
2. `./config.yaml` (current directory)
3. `$HOME/.photo-sorter/config.yaml`
4. `/etc/photo-sorter/config.yaml`

### Date Organization Formats

Choose from multiple organizational structures:

```yaml
# Full date structure (default)
date_format: "2006/01/02"    # Creates: 2024/12/25/

# Monthly organization
date_format: "2006/01"       # Creates: 2024/12/

# Yearly organization
date_format: "2006"          # Creates: 2024/

# Date with dashes
date_format: "2006-01-02"    # Creates: 2024-12-25/
date_format: "2006-01"       # Creates: 2024-12/
```

### Key Configuration Options

```yaml
# Required: Source directory containing photos
source_directory: "/path/to/your/photos"

# Optional: Target directory (default: organize in place)
target_directory: "/path/to/organized/photos"

# Date folder format - choose from options above
date_format: "2006/01/02"

# File processing options
processing:
  move_files: true # true = move files, false = copy files
  duplicate_handling: "rename" # rename, skip, or overwrite
  skip_organized: true # Skip already organized folders

# Performance settings
performance:
  worker_threads: 4
  batch_size: 100
  cache_size: 1000

# Security settings
security:
  dry_run: false
  confirm_before_start: true
  max_files_per_run: 0 # 0 = no limit
```

## Supported Formats

### Image Formats

- JPEG (.jpg, .jpeg)
- PNG (.png)
- TIFF (.tiff, .tif)
- RAW formats:
  - Canon (.cr2)
  - Nikon (.nef)
  - Sony (.arw)
  - Adobe DNG (.dng)
  - Generic (.raw)

### Video Formats

- MP4 (.mp4)
- AVI (.avi)
- QuickTime (.mov)
- MPEG (.mpg)
- Thumbnail files (.thm)

## Date Extraction Logic

PhotoSorter uses a multi-tiered approach to extract dates:

1. **EXIF Metadata** (highest priority):

   - `DateTime`
   - `DateTimeOriginal`
   - `DateTimeDigitized`

2. **Video Metadata**:

   - `creation_time`
   - `date` fields from ffprobe

3. **Thumbnail EXIF** (for video files):

   - Extract date from associated THM files

4. **File Modification Time** (fallback):
   - Uses file system modification date

## Directory Structure Examples

### Year/Month/Day Structure (2006/01/02)

```
organized_photos/
├── 2023/
│   ├── 01/
│   │   ├── 01/
│   │   │   ├── IMG_001.jpg
│   │   │   └── VID_001.mp4
│   │   └── 15/
│   │       └── IMG_002.jpg
│   └── 12/
│       └── 25/
│           └── Christmas_2023.jpg
└── 2024/
    └── 01/
        └── 01/
            └── NewYear_2024.jpg
```

### Year/Month Structure (2006/01)

```
organized_photos/
├── 2023/
│   ├── 01/
│   │   ├── IMG_001.jpg
│   │   ├── IMG_002.jpg
│   │   └── VID_001.mp4
│   └── 12/
│       └── Christmas_2023.jpg
└── 2024/
    └── 01/
        └── NewYear_2024.jpg
```

### Year Only Structure (2006)

```
organized_photos/
├── 2023/
│   ├── IMG_001.jpg
│   ├── IMG_002.jpg
│   ├── VID_001.mp4
│   └── Christmas_2023.jpg
└── 2024/
    └── NewYear_2024.jpg
```

## Performance Considerations

For large photo collections (10,000+ files), consider:

- Increasing `worker_threads` (up to number of CPU cores)
- Adjusting `batch_size` based on available memory
- Using SSD storage for better I/O performance

## Building from Source

### Prerequisites

- Go 1.21 or later
- Git

### Build Steps

```bash
# Clone the repository
git clone https://github.com/abshka/photo-sorter-go.git
cd photo-sorter-go

# Download dependencies
go mod download

# Build for your platform
go build -o photo-sorter ./cmd/photo-sorter
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Troubleshooting

### Configuration Issues

**Web configuration not working**

- Ensure you click "Save Configuration" after making changes
- Check the "Current Config" display shows your changes
- Try refreshing the page and reloading configuration

**Files still moved when copy mode selected**

- Verify "Move files" checkbox is unchecked in web interface
- Save configuration after changing the setting
- Check that the organize request uses the updated settings

### Common Issues

**"No date found in EXIF data"**

- File may not have EXIF data or date fields
- Try using file modification time as fallback
- Use `photo-sorter test-exif <file>` to debug

**"Permission denied"**

- Ensure write permissions on target directory
- Check source file permissions
- Run with appropriate user privileges

**Web interface connection issues**

- Check that the server is running on the correct port
- Verify firewall settings allow local connections
- Try refreshing the browser page

### Testing Your Setup

Run the built-in test suite to verify functionality:

```bash
go run test_organizer.go
```

This will test:

- All date format options work correctly
- Move vs copy functionality
- Duplicate handling strategies
- Dry run vs live mode

### Getting Help

- Check the [issues page](https://github.com/abshka/photo-sorter-go/issues)
- Run with `--verbose` flag for detailed logging
- Use `photo-sorter test-exif` to debug specific files
- Use the test suite to verify your configuration works
