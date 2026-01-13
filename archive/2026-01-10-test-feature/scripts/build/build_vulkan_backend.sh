#!/bin/bash
set -e  # Exit on any error

echo "=================================================="
echo "Vulkan Backend Builder for llama.cpp"
echo "=================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$EUID" -eq 0 ]; then 
   echo -e "${RED}ERROR: Do not run this script as root/sudo${NC}"
   echo "Run it as your normal user - it will prompt for sudo when needed"
   exit 1
fi

# Step 1: Install Vulkan dependencies
echo -e "${YELLOW}Step 1: Installing Vulkan dependencies...${NC}"
echo "This requires sudo access to install packages via pacman"
echo ""

PACKAGES=(
    vulkan-headers
    vulkan-icd-loader
    vulkan-tools
    vulkan-validation-layers
    shaderc
    cmake
    git
)

# Check which packages are already installed
MISSING_PACKAGES=()
for pkg in "${PACKAGES[@]}"; do
    if ! pacman -Q "$pkg" &> /dev/null; then
        MISSING_PACKAGES+=("$pkg")
    fi
done

if [ ${#MISSING_PACKAGES[@]} -eq 0 ]; then
    echo -e "${GREEN}✓ All Vulkan dependencies already installed${NC}"
else
    echo "Missing packages: ${MISSING_PACKAGES[*]}"
    echo ""
    sudo pacman -S --needed --noconfirm "${MISSING_PACKAGES[@]}"
    echo -e "${GREEN}✓ Vulkan dependencies installed${NC}"
fi
echo ""

# Step 2: Clone llama.cpp if not exists
echo -e "${YELLOW}Step 2: Setting up llama.cpp source...${NC}"
VULKAN_DIR="$HOME/llama.cpp-vulkan"

if [ -d "$VULKAN_DIR" ]; then
    echo "Directory $VULKAN_DIR already exists"
    read -p "Do you want to remove and re-clone? (y/N): " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing existing directory..."
        rm -rf "$VULKAN_DIR"
    else
        echo "Using existing directory"
    fi
fi

if [ ! -d "$VULKAN_DIR" ]; then
    echo "Cloning llama.cpp to $VULKAN_DIR..."
    git clone https://github.com/ggerganov/llama.cpp "$VULKAN_DIR"
    echo -e "${GREEN}✓ Repository cloned${NC}"
else
    echo -e "${GREEN}✓ Using existing repository${NC}"
fi
echo ""

# Step 3: Build with Vulkan support
echo -e "${YELLOW}Step 3: Building llama.cpp with Vulkan support...${NC}"
cd "$VULKAN_DIR"

# Clean previous build if exists
if [ -d "build" ]; then
    echo "Cleaning previous build..."
    rm -rf build
fi

echo "Running CMake configuration..."
cmake -B build -DGGML_VULKAN=ON

echo ""
echo "Building (this may take 5-10 minutes)..."
echo "Using $(nproc) CPU cores"
cmake --build build --config Release -j$(nproc)

echo -e "${GREEN}✓ Build complete${NC}"
echo ""

# Step 4: Verify the build
echo -e "${YELLOW}Step 4: Verifying build...${NC}"
LLAMA_SERVER_BIN="$VULKAN_DIR/build/bin/llama-server"

if [ ! -f "$LLAMA_SERVER_BIN" ]; then
    echo -e "${RED}ERROR: llama-server binary not found at $LLAMA_SERVER_BIN${NC}"
    exit 1
fi

echo "Testing binary..."
VERSION_OUTPUT=$("$LLAMA_SERVER_BIN" --version 2>&1 || true)
echo "$VERSION_OUTPUT"
echo ""

if echo "$VERSION_OUTPUT" | grep -qi "vulkan"; then
    echo -e "${GREEN}✓ Vulkan support confirmed in binary${NC}"
else
    echo -e "${YELLOW}⚠ Warning: 'vulkan' not found in version output${NC}"
    echo "This is normal - Vulkan support may not show in --version"
fi
echo ""

# Step 5: Check Vulkan runtime
echo -e "${YELLOW}Step 5: Checking Vulkan runtime...${NC}"
if command -v vulkaninfo &> /dev/null; then
    echo "Testing Vulkan with vulkaninfo..."
    if vulkaninfo --summary &> /dev/null; then
        echo -e "${GREEN}✓ Vulkan runtime working${NC}"
        echo ""
        echo "Available Vulkan devices:"
        vulkaninfo --summary 2>/dev/null | grep -A 2 "GPU id"
    else
        echo -e "${RED}⚠ Vulkan runtime test failed${NC}"
        echo "This may indicate missing GPU drivers"
    fi
else
    echo -e "${YELLOW}⚠ vulkaninfo not found (optional)${NC}"
fi
echo ""

# Step 6: Summary
echo "=================================================="
echo -e "${GREEN}Build Complete!${NC}"
echo "=================================================="
echo ""
echo "Binary location:"
echo "  $LLAMA_SERVER_BIN"
echo ""
echo "Expected path in clai code:"
echo "  /home/josh/llama.cpp-vulkan/build/bin/llama-server"
echo ""
if [ "$LLAMA_SERVER_BIN" = "/home/josh/llama.cpp-vulkan/build/bin/llama-server" ]; then
    echo -e "${GREEN}✓ Path matches - clai will automatically detect this backend${NC}"
else
    echo -e "${YELLOW}⚠ Path mismatch - you may need to update model_manager.go${NC}"
fi
echo ""
echo "Next steps:"
echo "  1. Restart your benchmark server (make dev will auto-restart)"
echo "  2. Visit http://localhost:8081"
echo "  3. Backend selector should now show both 'ROCm' and 'Vulkan'"
echo "  4. Try starting a model with Vulkan backend"
echo ""
echo "To test manually:"
echo "  $LLAMA_SERVER_BIN --help"
echo ""
