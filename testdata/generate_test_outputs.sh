#!/bin/bash

# Test Output Generation Script
# This script generates all output formats from the test input files
# for manual validation and testing purposes.

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Lexington Test Output Generation ===${NC}"

# Create output directory if it doesn't exist
mkdir -p testdata/output

# Test files and their descriptions
declare -A test_files=(
    ["basic_screenplay.fountain"]="Basic screenplay with standard elements"
    ["dual_dialogue.fountain"]="Dual dialogue comprehensive test"
    ["simple_dual.fountain"]="Simple dual dialogue test"
    ["complex_screenplay.fountain"]="Advanced formatting test"
    ["no_title.fountain"]="No title page edge case"
    ["fountain_example.fountain"]="Original fountain example"
)

# Output formats to test
formats=("html" "lex" "fountain" "markdown" "epub" "docx" "odt" "pdf")

# Pandoc-only formats (require pandoc)
pandoc_formats=("epub" "docx" "odt" "markdown")

echo -e "${YELLOW}Checking dependencies...${NC}"

# Check if pandoc is available for pandoc formats
pandoc_available=false
if command -v pandoc &> /dev/null; then
    pandoc_available=true
    echo -e "${GREEN}✓ Pandoc found${NC}"
else
    echo -e "${YELLOW}⚠ Pandoc not found - skipping pandoc formats${NC}"
fi

# Check if we're in the right directory
if [[ ! -f "main.go" ]]; then
    echo -e "${RED}Error: Please run this script from the lexington project root directory${NC}"
    exit 1
fi

echo

# Generate outputs for each test file
for test_file in "${!test_files[@]}"; do
    echo -e "${GREEN}Processing: ${test_file}${NC}"
    echo -e "${YELLOW}  Description: ${test_files[$test_file]}${NC}"

    input_path="testdata/input/$test_file"
    base_name="${test_file%.*}"

    # Check if input file exists
    if [[ ! -f "$input_path" ]]; then
        echo -e "${RED}  ✗ Input file not found: $input_path${NC}"
        continue
    fi

    # Generate each format
    for format in "${formats[@]}"; do
        output_file="testdata/output/${base_name}.${format}"

        # Skip pandoc formats if pandoc is not available
        if [[ " ${pandoc_formats[*]} " =~ " ${format} " ]] && [[ "$pandoc_available" = false ]]; then
            echo -e "${YELLOW}  ⊝ Skipping $format (pandoc not available)${NC}"
            continue
        fi

        # Special case for PDF - might fail if dependencies missing
        if [[ "$format" = "pdf" ]]; then
            echo -e "${YELLOW}  → Generating $format...${NC}"
            if go run main.go -i "$input_path" -to "$format" -o "$output_file" 2>/dev/null; then
                echo -e "${GREEN}  ✓ Generated $format${NC}"
            else
                echo -e "${YELLOW}  ⚠ PDF generation failed (missing dependencies?)${NC}"
            fi
            continue
        fi

        echo -e "${YELLOW}  → Generating $format...${NC}"
        if go run main.go -i "$input_path" -to "$format" -o "$output_file" 2>/dev/null; then
            echo -e "${GREEN}  ✓ Generated $format${NC}"
        else
            echo -e "${RED}  ✗ Failed to generate $format${NC}"
        fi
    done

    echo
done

echo -e "${GREEN}=== Generation Complete ===${NC}"
echo
echo -e "${YELLOW}Generated files are in: testdata/output/${NC}"
echo -e "${YELLOW}You can now manually inspect the outputs for correctness.${NC}"
echo
echo -e "${GREEN}Useful commands for inspection:${NC}"
echo -e "  ${YELLOW}# View HTML in browser${NC}"
echo -e "  open testdata/output/basic_screenplay.html"
echo -e "  ${YELLOW}# Check lex structure${NC}"
echo -e "  cat testdata/output/dual_dialogue.lex"
echo -e "  ${YELLOW}# Compare fountain round-trip${NC}"
echo -e "  diff testdata/input/fountain_example.fountain testdata/output/fountain_example.fountain"
echo

# List generated files
echo -e "${GREEN}Generated files:${NC}"
if ls testdata/output/ &> /dev/null; then
    ls -la testdata/output/ | grep -v "^total" | awk '{print "  " $9 " (" $5 " bytes)"}'
else
    echo -e "${YELLOW}  No files generated${NC}"
fi

echo
echo -e "${GREEN}To clean up generated files:${NC}"
echo -e "  rm -rf testdata/output/*"
