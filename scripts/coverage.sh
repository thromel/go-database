#!/bin/bash

# Coverage report generation script
set -e

echo "🧪 Generating test coverage report..."

# Clean up previous coverage files
rm -f *.txt *.out coverage.html

# Generate coverage for each package
echo "📦 Testing API package..."
go test -coverprofile=api-coverage.txt ./pkg/api

echo "📦 Testing Storage package..."
go test -coverprofile=storage-coverage.txt ./pkg/storage

echo "📦 Testing Integration..."
go test -coverprofile=integration-coverage.txt ./test/integration

# Combine coverage files
echo "🔗 Combining coverage reports..."
echo "mode: set" > coverage.txt
tail -n +2 api-coverage.txt >> coverage.txt
tail -n +2 storage-coverage.txt >> coverage.txt
tail -n +2 integration-coverage.txt >> coverage.txt

# Generate HTML report
echo "📊 Generating HTML coverage report..."
go tool cover -html=coverage.txt -o coverage.html

# Display coverage summary
echo "📈 Coverage Summary:"
go tool cover -func=coverage.txt | grep total

echo "✅ Coverage report generated: coverage.html"
echo "📁 Open coverage.html in your browser to view detailed coverage"

# Calculate package-specific coverage
echo ""
echo "📊 Package Coverage Details:"
echo "API Package:"
go tool cover -func=api-coverage.txt | grep total

echo "Storage Package:"
go tool cover -func=storage-coverage.txt | grep total

echo "Integration Tests:"
go tool cover -func=integration-coverage.txt | grep total

# Clean up individual files
rm -f api-coverage.txt storage-coverage.txt integration-coverage.txt

echo ""
echo "🎉 Coverage analysis complete!"