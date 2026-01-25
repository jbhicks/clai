#!/bin/bash
set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=================================================="
echo "llama.cpp Backend Update Script"
echo "==================================================${NC}"
echo ""

# Function to print step headers
print_step() {
    echo ""
    echo -e "${YELLOW}==> $1${NC}"
    echo ""
}

# Function to print success
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

# Function to print info
print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# Check if running as root
if [ "$EUID" -eq 0 ]; then 
   echo -e "${RED}ERROR: Do not run this script as root/sudo${NC}"
   echo "Run it as your normal user"
   exit 1
fi

# ============================================================================
# 1. Update Vulkan Backend
# ============================================================================
print_step "Step 1: Updating Vulkan Backend"

VULKAN_DIR="$HOME/llama.cpp-vulkan"
if [ ! -d "$VULKAN_DIR" ]; then
    echo -e "${RED}ERROR: Vulkan directory not found: $VULKAN_DIR${NC}"
    exit 1
fi

cd "$VULKAN_DIR"

print_info "Current commit:"
git log --oneline -1

print_info "Fetching latest changes..."
git fetch origin

COMMITS_BEHIND=$(git rev-list --count HEAD..origin/master)
if [ "$COMMITS_BEHIND" -eq 0 ]; then
    print_success "Already up to date (no new commits)"
else
    print_info "$COMMITS_BEHIND commits behind, pulling updates..."
    git pull origin master
    print_success "Repository updated"
fi

print_info "Latest commit:"
git log --oneline -1

print_info "Rebuilding Vulkan backend..."
cmake --build build --config Release -j$(nproc)
print_success "Vulkan backend rebuilt"

print_info "Testing Vulkan binary..."
VULKAN_VERSION=$($VULKAN_DIR/build/bin/llama-server --version 2>&1 | grep "version:" | head -1)
echo "$VULKAN_VERSION"
print_success "Vulkan backend working"

# ============================================================================
# 2. Update ROCm Backend
# ============================================================================
print_step "Step 2: Updating ROCm Backend"

ROCM_DIR="$HOME/llama.cpp-rocm-wmma"
if [ ! -d "$ROCM_DIR" ]; then
    echo -e "${RED}ERROR: ROCm directory not found: $ROCM_DIR${NC}"
    exit 1
fi

cd "$ROCM_DIR"

print_info "Current commit:"
git log --oneline -1

print_info "Fetching latest changes..."
git fetch origin

COMMITS_BEHIND=$(git rev-list --count HEAD..origin/master)
if [ "$COMMITS_BEHIND" -eq 0 ]; then
    print_success "Already up to date (no new commits)"
else
    print_info "$COMMITS_BEHIND commits behind, pulling updates..."
    git pull origin master
    print_success "Repository updated"
fi

print_info "Latest commit:"
git log --oneline -1

print_info "Rebuilding ROCm backend (this may take 5-10 minutes)..."
cmake --build build --config Release -j$(nproc)
print_success "ROCm backend rebuilt"

print_info "Testing ROCm binary..."
ROCM_VERSION=$($ROCM_DIR/build/bin/llama-server --version 2>&1 | grep "version:" | head -1)
echo "$ROCM_VERSION"
print_success "ROCm backend working"

# ============================================================================
# 3. Check for Mistral3 Support
# ============================================================================
print_step "Step 3: Verifying Mistral3 Support"

cd "$VULKAN_DIR"
if grep -q "mistral3" src/llama-arch.cpp 2>/dev/null; then
    print_success "Mistral3 architecture found in Vulkan source code"
else
    echo -e "${YELLOW}⚠ Mistral3 not found in Vulkan source (may need newer version)${NC}"
fi

cd "$ROCM_DIR"
if grep -q "mistral3" src/llama-arch.cpp 2>/dev/null; then
    print_success "Mistral3 architecture found in ROCm source code"
else
    echo -e "${YELLOW}⚠ Mistral3 not found in ROCm source (may need newer version)${NC}"
fi

# ============================================================================
# 4. Summary
# ============================================================================
print_step "Update Complete!"

echo ""
echo -e "${GREEN}=================================================="
echo "Summary"
echo "==================================================${NC}"
echo ""
echo "Vulkan Backend:"
echo "  Location: $VULKAN_DIR/build/bin/llama-server"
echo "  Version:  $VULKAN_VERSION"
echo ""
echo "ROCm Backend:"
echo "  Location: $ROCM_DIR/build/bin/llama-server"
echo "  Version:  $ROCM_VERSION"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "  1. Restart your benchmark server (make dev will auto-reload)"
echo "  2. Try starting the Devstral model again"
echo "  3. It should now support the 'mistral3' architecture"
echo ""
echo -e "${BLUE}To test manually:${NC}"
echo "  $VULKAN_DIR/build/bin/llama-server \\"
echo "    -m \$MODELS_PATH/Devstral-Small-2-24B-Instruct-2512-UD-Q8_K_XL.gguf \\"
echo "    --version"
echo ""
