# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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