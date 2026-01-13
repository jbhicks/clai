#!/bin/bash

# Disk cleanup script for freeing up space
# Cleans pacman cache and checks btrfs filesystem usage

set -e

echo "=========================================="
echo "Disk Cleanup Script"
echo "=========================================="
echo ""

echo "Current disk usage:"
df -h /
echo ""

echo "----------------------------------------"
echo "Step 1: Cleaning pacman package cache"
echo "----------------------------------------"

# Check if paccache is available, if not use manual cleanup
if command -v paccache &> /dev/null; then
    echo "Keeping only the latest version of each package..."
    sudo paccache -rk1
else
    echo "paccache not found, using alternative method..."
    echo "Installing pacman-contrib..."
    sudo pacman -S --noconfirm pacman-contrib
    echo "Cleaning cache..."
    sudo paccache -rk1
fi
echo ""

echo "----------------------------------------"
echo "Step 2: Checking btrfs filesystem usage"
echo "----------------------------------------"
sudo btrfs filesystem usage /
echo ""

echo "----------------------------------------"
echo "Step 3: Final disk usage"
echo "----------------------------------------"
df -h /
echo ""

echo "=========================================="
echo "Cleanup complete!"
echo "=========================================="
