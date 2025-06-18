# Changelog

All notable changes to PhotoSorter Go will be documented in this file.

## [1.2.0] - 2024-06-18

### ✨ Major New Features

#### 📁 Native Folder Picker & Drag & Drop

- **REPLACED**: Limited directory browser with native OS folder picker
- **NEW**: Drag & drop folder selection from Windows Explorer, macOS Finder, Linux file managers
- **ENHANCED**: Users can now browse ANYWHERE on their computer, not just project directories
- **MODERN UX**: Works like file uploads on modern websites with native system dialogs
- **VISUAL FEEDBACK**: Beautiful drop zones with hover animations and selection indicators
- **DUAL INPUT**: Both source and target directories support browse buttons and drag & drop

### 🎨 UI/UX Improvements

#### Enhanced Folder Selection Experience

- **Native OS Integration**: Opens your system's standard folder picker dialog
- **Drag & Drop Zones**: Visual drop areas with hover effects and state indicators
- **Input Groups**: Modern layout with inline browse buttons
- **Real-time Feedback**: Immediate visual confirmation when folders are selected
- **Responsive Design**: Works seamlessly on desktop and mobile devices

### 🔧 Technical Improvements

#### Client-Side Folder Selection

- **HTML5 File API**: Uses modern `webkitdirectory` attribute for native folder selection
- **Removed Server Dependencies**: No more server-side directory browsing for security and performance
- **Eliminated API Endpoint**: Removed `/api/directories` - no longer needed
- **Better Security**: All folder selection happens client-side
- **Improved Performance**: Direct folder selection without API calls

---

## [1.1.0] - 2024-06-18

### ✨ New Features

#### 📅 Flexible Date Organization Formats

- Added support for multiple date organization formats:
  - `2006/01/02` - Year/Month/Day (2024/12/25) - **Default**
  - `2006/01` - Year/Month (2024/12) - Monthly organization
  - `2006` - Year Only (2024) - Yearly organization
  - `2006-01-02` - Year-Month-Day (2024-12-25) - With dashes
  - `2006-01` - Year-Month (2024-12) - Monthly with dashes
- Users can now choose the organizational structure that best fits their needs

#### ⚙️ Web Configuration Management

- **NEW**: Configuration editor in web interface
- Real-time configuration preview showing current settings
- Save/load configuration through web UI
- No need to manually edit config files anymore

#### 🔧 Enhanced Web Interface

- **Configuration Section**: Edit date format, move/copy mode, duplicate handling
- **Current Config Display**: Always shows active configuration
- **Improved Form Controls**: Better styling for selects and checkboxes
- **Real-time Updates**: Configuration changes apply immediately to operations

### 🐛 Bug Fixes

#### Critical Configuration Bugs

- **FIXED**: Web interface configuration was being ignored during operations
- **FIXED**: `date_format` setting from web UI was not applied to file organization
- **FIXED**: `move_files` setting was not respected (files were always moved regardless of setting)
- **FIXED**: Configuration passed through API requests now properly overrides default config

#### API Improvements

- **FIXED**: `/api/organize` endpoint now accepts and applies `date_format` and `move_files` parameters
- **ADDED**: `/api/config` endpoints for GET/POST configuration management
- **ADDED**: `/api/date-formats` endpoint to retrieve available format options

### 🧪 Testing & Quality

#### New Test Suite

- **ADDED**: Comprehensive test file (`test_organizer.go`) for validating functionality
- **Date Format Testing**: Validates all supported date formats work correctly
- **Move vs Copy Testing**: Ensures file operation mode is respected
- **Duplicate Handling Testing**: Validates all duplicate strategies
- **Dry Run vs Live Testing**: Confirms dry-run mode doesn't modify files

#### Test Results

- ✅ All date formats generate correct directory structures
- ✅ Move vs Copy modes work as expected
- ✅ Dry run mode preserves original files
- ✅ Live mode correctly organizes files
- ✅ Configuration validation prevents invalid settings

### 🔄 API Changes

#### New Endpoints

```
GET  /api/config          - Get current configuration
POST /api/config          - Update configuration
GET  /api/date-formats    - Get available date format options
```

#### Enhanced Endpoints

```
POST /api/organize        - Now accepts date_format and move_files parameters
```

#### Request/Response Changes

- `OrganizeRequest` now includes `date_format` and `move_files` fields
- Configuration responses include all major settings
- Better error messages for invalid configurations

### 🎨 UI/UX Improvements

#### Web Interface Enhancements

- **Configuration Section**: Dedicated area for settings management
- **Visual Config Preview**: Shows current settings at a glance
- **Improved Form Styling**: Better select dropdowns and checkboxes
- **Real-time Validation**: Input validation with visual feedback
- **Keyboard Shortcuts**: Enhanced shortcuts for power users

#### Better User Experience

- **Immediate Feedback**: Configuration changes show effects immediately
- **Validation Messages**: Clear error messages for invalid inputs
- **Visual Indicators**: Color-coded status indicators
- **Responsive Design**: Better mobile compatibility

### 📋 Configuration Options

#### Date Format Options

```yaml
date_format: "2006/01/02"    # Year/Month/Day (default)
date_format: "2006/01"       # Year/Month only
date_format: "2006"          # Year only
date_format: "2006-01-02"    # Year-Month-Day with dashes
date_format: "2006-01"       # Year-Month with dashes
```

#### File Operation Options

```yaml
processing:
  move_files: true           # Move files (default)
  move_files: false          # Copy files instead
```

### 🚀 Performance & Reliability

#### Improved Error Handling

- Better validation of configuration parameters
- More descriptive error messages
- Graceful handling of invalid date formats
- Proper cleanup on operation failures

#### Enhanced Logging

- Configuration changes are logged
- Better operation tracking
- Improved debugging information

### 📚 Documentation

#### Updated Documentation

- **README.md**: Updated with new configuration options
- **CHANGELOG.md**: This changelog for tracking changes
- **Test Documentation**: Inline documentation in test file

#### Code Comments

- Enhanced code documentation
- Better API endpoint documentation
- Improved configuration structure documentation

### 🔧 Developer Experience

#### Code Quality

- **Fixed**: Potential race conditions in configuration updates
- **Improved**: Better separation of concerns in web handlers
- **Enhanced**: More robust configuration validation
- **Added**: Comprehensive test coverage

#### Build & Development

- **Fixed**: `.gitignore` pattern that was incorrectly ignoring `cmd/photo-sorter/main.go`
- **Added**: Test utilities for validating functionality
- **Improved**: Error handling and logging throughout

---

## [1.0.0] - 2024-06-15

### Initial Release

- Core photo organization functionality
- EXIF date extraction
- Web interface with real-time progress
- Command-line interface
- Docker support
- Basic configuration system
- Multiple file format support

---

## How to Update

To get the latest version with all these improvements:

```bash
git pull origin main
go build -o photo-sorter ./cmd/photo-sorter
```

## Breaking Changes

**None** - This update is fully backwards compatible with existing configurations and usage patterns.

## Next Release Preview

Coming in v1.2.0:

- Batch operations with progress tracking
- Custom date format builder in web UI
- Advanced duplicate detection algorithms
- Integration with cloud storage services
- Enhanced metadata extraction options
