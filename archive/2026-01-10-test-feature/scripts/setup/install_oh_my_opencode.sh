#!/bin/bash
# Script to install oh-my-opencode globally
# WARNING: This uses sudo which can be a security risk

echo "WARNING: Installing npm packages globally with sudo can be a security risk."
echo "Consider using nvm or a node version manager instead."
echo ""

read -p "Do you want to continue with sudo installation? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Installation cancelled."
    exit 1
fi

echo "Installing oh-my-opencode globally..."
sudo npm install -g oh-my-opencode@latest

echo "Installation complete."
echo "You can now run: opencode"
echo ""
echo "To test oh-my-opencode features, try:"
echo "opencode run 'ultrawork: help me refactor this code'"
