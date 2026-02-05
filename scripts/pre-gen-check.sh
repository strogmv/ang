#!/bin/bash
# ============================================================================
# PRE-GENERATION VALIDATION SCRIPT
# Run this before `make gen` to catch errors early
# Usage: ./scripts/pre-gen-check.sh
# ============================================================================
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=============================================="
echo "ANG Pre-Generation Validation"
echo "=============================================="

# Step 1: Check CUE syntax
echo -e "\n${YELLOW}>>> Step 1: Validating CUE schemas...${NC}"
if command -v cue &> /dev/null; then
    if cue vet ./cue/... 2>&1; then
        echo -e "${GREEN}✓ CUE schemas are valid${NC}"
    else
        echo -e "${RED}✗ CUE validation failed${NC}"
        exit 1
    fi

    echo -e "\n${YELLOW}>>> Step 1.5: Running CUE lint rules...${NC}"
    if cue vet ./cue/lint/... 2>&1; then
        echo -e "${GREEN}✓ CUE lint rules passed${NC}"
    else
        echo -e "${RED}✗ CUE lint rules failed${NC}"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠ CUE not installed, skipping CUE validation${NC}"
fi

# Step 2: Check Go compiler builds
echo -e "\n${YELLOW}>>> Step 2: Building ANG compiler...${NC}"
if go build -o /tmp/ang_check ./cmd/ang 2>&1; then
    echo -e "${GREEN}✓ Compiler builds successfully${NC}"
else
    echo -e "${RED}✗ Compiler build failed${NC}"
    exit 1
fi

# Step 2.5: Run compiler unit tests
echo -e "\n${YELLOW}>>> Step 2.5: Running compiler unit tests...${NC}"
if go test ./compiler/ir/... ./compiler/normalizer/... 2>&1; then
    echo -e "${GREEN}✓ Compiler tests passed${NC}"
else
    echo -e "${RED}✗ Compiler tests failed${NC}"
    exit 1
fi

# Step 3: Dry-run template parsing (check for syntax errors)
echo -e "\n${YELLOW}>>> Step 3: Checking template syntax...${NC}"
TEMPLATE_ERRORS=0
for tmpl in templates/*.tmpl templates/**/*.tmpl; do
    if [ -f "$tmpl" ]; then
        # Basic syntax check - look for unclosed template tags
        OPEN_COUNT=$(grep -o '{{' "$tmpl" 2>/dev/null | wc -l || echo 0)
        CLOSE_COUNT=$(grep -o '}}' "$tmpl" 2>/dev/null | wc -l || echo 0)
        if [ "$OPEN_COUNT" -ne "$CLOSE_COUNT" ]; then
            echo -e "${RED}✗ Unbalanced template tags in $tmpl ({{ count: $OPEN_COUNT, }} count: $CLOSE_COUNT)${NC}"
            TEMPLATE_ERRORS=$((TEMPLATE_ERRORS + 1))
        fi
    fi
done

if [ $TEMPLATE_ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ Template syntax looks OK${NC}"
else
    echo -e "${RED}✗ Found $TEMPLATE_ERRORS template issues${NC}"
    exit 1
fi

# Step 4: Check for breaking changes in CUE
echo -e "\n${YELLOW}>>> Step 4: Checking for schema changes...${NC}"
if [ -f ".gen-hash" ]; then
    OLD_HASH=$(cat .gen-hash)
    NEW_HASH=$(find cue/ -name "*.cue" -exec md5sum {} \; | sort | md5sum | cut -d' ' -f1)
    if [ "$NEW_HASH" != "$OLD_HASH" ]; then
        echo -e "${YELLOW}⚠ CUE schemas changed since last generation${NC}"
        echo "  Old hash: $OLD_HASH"
        echo "  New hash: $NEW_HASH"
    else
        echo -e "${GREEN}✓ No CUE schema changes detected${NC}"
    fi
else
    echo -e "${YELLOW}⚠ No previous hash found (first run?)${NC}"
fi

# Step 5: Check disk space
echo -e "\n${YELLOW}>>> Step 5: Checking disk space...${NC}"
FREE_SPACE=$(df -m . | tail -1 | awk '{print $4}')
if [ "$FREE_SPACE" -lt 100 ]; then
    echo -e "${RED}✗ Low disk space: ${FREE_SPACE}MB free${NC}"
    exit 1
else
    echo -e "${GREEN}✓ Disk space OK (${FREE_SPACE}MB free)${NC}"
fi

# Step 6: Check for potential conflicts
echo -e "\n${YELLOW}>>> Step 6: Checking for build conflicts...${NC}"
CONFLICTS=0
if [ -f "ang_new" ]; then
    echo -e "${YELLOW}⚠ Found temporary binary 'ang_new'. Suggest removing.${NC}"
    CONFLICTS=1
fi
if [ -f "server_bin" ]; then
    echo -e "${YELLOW}⚠ Found 'server_bin'. Ensure it is not running if you plan to rebuild.${NC}"
fi
# Check for conflicting casing (e.g., ApiKey vs APIKey files)
if find internal/domain -name "*apikey.go" | grep -q "apikey"; then
     if find internal/domain -name "*APIKey.go" 2>/dev/null; then
        echo -e "${RED}✗ Found ambiguous filenames (case sensitivity conflict). Clean internal/domain.${NC}"
        CONFLICTS=1
     fi
fi

if [ $CONFLICTS -eq 0 ]; then
    echo -e "${GREEN}✓ No obvious conflicts found${NC}"
else
    echo -e "${YELLOW}⚠ Some cleanup might be required${NC}"
fi

# Cleanup
rm -f /tmp/ang_check

echo -e "\n${GREEN}=============================================="
echo "All pre-flight checks passed!"
echo "Run 'make gen' to generate code."
echo "==============================================${NC}"

# Save current hash for next comparison
find cue/ -name "*.cue" -exec md5sum {} \; | sort | md5sum | cut -d' ' -f1 > .gen-hash
