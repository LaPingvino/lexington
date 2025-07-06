# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2025-07-06

### Major Features
- **Complete Inline Markup Support**: Fixed and enhanced inline formatting across all output formats
  - Fountain-style markup (`**bold**`, `*italic*`, `_underline_`, `***bold-italic***`) now works consistently
  - Fixed critical regex capture group issue that was preventing markup conversion
  - All output formats (HTML, LaTeX, PDF, Markdown, FDX) now properly process inline formatting

### New Features
- **FDX Inline Formatting**: Added complete inline markup support for Final Draft XML format
  - Bold text: `**bold**` → `<Text Style="Bold">bold</Text>`
  - Italic text: `*italic*` → `<Text AdornmentStyle="-1">italic</Text>`
  - Underline text: `_underline_` → `<Text Style="Underline">underline</Text>`
  - Multiple Text elements for complex formatting within paragraphs

### Fixed
- **Regex Capture Groups**: Changed all `$1` references to `${1}` in regex replacements
  - LaTeX Writer: Now generates proper LaTeX commands (`\textbf{}`, `\textit{}`, `\underline{}`)
  - HTML Writer: Now generates proper HTML tags (`<b>`, `<i>`, `<u>`)
  - PDF Writer: Now processes HTML-style markup correctly
  - Markdown Writer: Now preserves Markdown formatting with HTML fallback for underline
- **FDX Writer**: Previously showed raw fountain markup, now generates proper Final Draft styling
- **Template Processing**: Fixed placeholder approach in LaTeX to prevent conflicts with escaping

### Enhanced
- **Code Quality**: Refactored complex functions to reduce cyclomatic complexity
- **Linting**: Fixed all linting issues including line length and complexity warnings
- **Testing**: Comprehensive testing across all output formats with various markup combinations

### Technical Improvements
- Consistent inline markup processing across all writers
- Proper XML attribute handling in FDX format
- Enhanced template readability with proper line breaks
- Better error handling and code organization

## [1.0.5] - 2025-07-05

### Fixed
- **HTML Template Issues**: Fixed whitespace control and unknown element handling
  - Added proper whitespace control using `{{- .Contents -}}` for all content elements
  - Unknown elements are now properly ignored instead of displaying debug information
  - Improved HTML output readability with strategic newlines for easier debugging
- **Template Consistency**: HTML template now follows same patterns as LaTeX template fixes
- **Debug Output**: Enhanced HTML template structure for better debugging experience

### Technical Improvements
- Cleaner HTML output with proper line breaks and formatting
- Consistent whitespace handling across all HTML elements
- Better template debugging capabilities without affecting functionality
- All existing tests continue to pass with no functionality changes

## [1.0.2] - 2024-01-06

### Fixed
- **Go Version Compatibility**: Updated from Go 1.24 to Go 1.23 for better ecosystem compatibility
- **Code Quality Issues**: Fixed all golangci-lint errors and warnings
  - Resolved 18 errcheck issues by properly handling error return values
  - Fixed 7 staticcheck issues by using switch statements instead of if-else chains
  - Removed 1 unused function from fountain/parse.go
- **Error Handling**: Improved error handling in deferred file operations
- **CI/CD Compatibility**: Updated GitHub Actions workflows to use Go 1.23

### Technical Improvements
- Enhanced golangci-lint configuration for modern linter versions
- Better error propagation in temporary file handling
- Improved code maintainability with proper error checking
- All tests continue to pass with no functionality changes

## [1.0.1] - 2024-01-06

### Changed
- **Code Quality**: Significantly reduced cyclomatic complexity across the codebase
  - Refactored `main()` function from complexity 33 to <10 by breaking into focused functions
  - Refactored `fountain.Parse()` from complexity 25 to <10 using state-based approach
  - Refactored `fountain.Write()` from complexity 21 to <10 using helper functions
  - Improved code maintainability and readability without changing functionality
- **Function Organization**: Split large functions into smaller, single-purpose functions
  - `parseFlags()`, `setupIO()`, `parseInput()`, `convertOutput()` for main functionality
  - State-based parsing with `ParseState` struct for fountain parsing
  - Helper functions for each output type in fountain writing

### Technical Improvements
- Better separation of concerns with focused, testable functions
- Improved error handling with clearer error propagation
- Enhanced code structure following Go best practices
- All existing functionality preserved with 100% test compatibility

### Fixed
- **Go Report Card**: Addressed gofmt complexity warnings
- **Code Maintainability**: Reduced technical debt from high-complexity functions

## [1.0.0] - 2024-01-06

### Added
- **Dual Dialogue Support**: Complete implementation of side-by-side dual dialogue formatting
  - PDF output with proper column positioning using industry standards
  - HTML output with table-based dual dialogue structure
  - Support for parentheticals within dual dialogue
  - Multiple dual dialogue blocks within a single screenplay
- **Modern Go Features**: Updated to Go 1.24 with modern language features
  - Generic utility functions (Filter, Map, Find, Contains, Unique, GroupBy)
  - Type aliases and constants for better code readability
  - Enhanced error handling with better context and wrapping
  - Context support for graceful shutdown
- **Enhanced Output Formats**:
  - HTML-to-PDF conversion via wkhtmltopdf
  - LaTeX-to-PDF conversion via pdflatex/xelatex/lualatex
  - Configurable HTML writer with industry-standard margins
  - Improved LaTeX writer with dual dialogue support
- **Font Management**: Modern font loading using Go's embed directive
- **Configuration System**: Enhanced TOML configuration with validation
- **Version Information**: Added `-version` flag to display build information
- **GitHub Workflows**: CI/CD pipelines for testing and releasing

### Fixed
- **Dual Dialogue Rendering**: Fixed overlapping columns in PDF output
- **Title Page Detection**: Proper handling of files with and without title pages
- **Scene Heading Parsing**: Improved scene detection logic using generic utilities
- **Error Handling**: Better error messages with context throughout the application
- **Memory Management**: Fixed slice allocation issues in generic utilities

### Changed
- **Go Version**: Updated minimum requirement to Go 1.24
- **Dependencies**: Updated all dependencies to latest versions
- **Code Structure**: Modernized codebase with generics and improved patterns
- **Testing**: Enhanced test coverage with comprehensive dual dialogue tests
- **Documentation**: Updated README with improved installation and usage instructions

### Removed
- **Deprecated Code**: Removed old bindata.go in favor of embed.go
- **Legacy Patterns**: Replaced outdated error handling patterns

### Technical Improvements
- Industry-standard dual dialogue column positioning (left: 1.5"-3.5", right: 4.5"-6.5")
- Proper X/Y coordinate positioning instead of margin manipulation in PDF output
- Type-safe configuration handling with validation
- Context-aware operations with signal handling
- Comprehensive test suite with 100% coverage for critical functionality

### Performance
- Optimized dual dialogue buffer handling using generic utilities
- Improved error propagation with proper error wrapping
- Enhanced template rendering with better error context

---

This release represents the first stable version of Lexington with complete dual dialogue support and modern Go features. The application now meets professional screenplay formatting standards and provides a solid foundation for future enhancements.