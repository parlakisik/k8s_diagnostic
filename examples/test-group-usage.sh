#!/bin/bash
# Example script demonstrating test-group functionality

# Set script to exit on error
set -e

# Ensure we're in the project root
cd "$(dirname "$0")/.."

echo "Building k8s-diagnostic tool..."
make build

# Basic networking test group usage
echo -e "\n\033[1;34m=== Running all networking tests with test-group ===\033[0m"
./build/k8s-diagnostic test --test-group networking

# Running with custom namespace and verbose output
echo -e "\n\033[1;34m=== Running networking test group with custom namespace and verbose output ===\033[0m"
./build/k8s-diagnostic test --test-group networking -n custom-test-ns --verbose

# Invalid group fallback demonstration
echo -e "\n\033[1;34m=== Demonstrating fallback behavior with invalid test group ===\033[0m"
./build/k8s-diagnostic test --test-group invalid -n invalid-test-group

echo -e "\n\033[1;32m✅ Test Group feature demonstrated successfully!\033[0m"
