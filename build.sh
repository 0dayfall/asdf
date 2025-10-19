#!/bin/bash

# ASDF WebFinger Server - Build and Test Script
set -e

echo "🏗️  Building ASDF WebFinger Server..."

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}📦 Downloading dependencies...${NC}"
go mod download
go mod tidy

echo -e "${YELLOW}🔧 Running go vet...${NC}"
go vet ./...

echo -e "${YELLOW}🏗️  Building all packages...${NC}"
go build ./...

echo -e "${YELLOW}🏗️  Building main binary...${NC}"
go build -o asdf ./cmd/asdf

echo -e "${YELLOW}🧪 Running tests...${NC}"
go test ./... -v

echo -e "${YELLOW}📊 Test coverage...${NC}"
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

echo -e "${YELLOW}🐳 Validating Docker Compose...${NC}"
if command -v docker-compose &> /dev/null; then
    docker-compose config --quiet
    echo -e "${GREEN}✅ Docker Compose configuration is valid${NC}"
else
    echo -e "${YELLOW}⚠️  Docker Compose not available, skipping validation${NC}"
fi

echo -e "${YELLOW}📝 Generating dependency tree...${NC}"
go mod graph > dependencies.txt

# Summary
echo ""
echo -e "${GREEN}🎉 Build Summary:${NC}"
echo -e "${GREEN}✅ Dependencies downloaded${NC}"
echo -e "${GREEN}✅ Code vetted${NC}"
echo -e "${GREEN}✅ All packages built successfully${NC}"
echo -e "${GREEN}✅ Main binary created: ./asdf${NC}"
echo -e "${GREEN}✅ All tests passed${NC}"
echo -e "${GREEN}✅ Coverage report generated: coverage.html${NC}"
echo ""

# Binary info
if [ -f "./asdf" ]; then
    echo -e "${YELLOW}📄 Binary information:${NC}"
    ls -lh ./asdf
    echo ""
fi

echo -e "${GREEN}🚀 Ready to deploy!${NC}"
echo ""
echo "Next steps:"
echo "  • Copy .env.example to .env and configure"
echo "  • Generate TLS certificates (see README.md)"
echo "  • Run: docker-compose up --build"
echo "  • Or run locally: ./asdf"