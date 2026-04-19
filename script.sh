#!/bin/bash

set -e  # Exit on any error

echo "🚀 MySlotMate Backend Deployment Script"
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Git Pull
echo -e "${YELLOW}📥 Step 1: Pulling latest changes from Git...${NC}"
git pull origin main
echo -e "${GREEN}✅ Git pull completed${NC}"

# Step 2: Go Get
echo -e "${YELLOW}📦 Step 2: Installing Go dependencies...${NC}"
go get ./...
echo -e "${GREEN}✅ Dependencies installed${NC}"

# Step 3: Database Migration
echo -e "${YELLOW}🗄️  Step 3: Running database migrations...${NC}"
go run ./cmd/migrate/run.go
echo -e "${GREEN}✅ Migrations completed${NC}"

# Step 4: Build
echo -e "${YELLOW}🔨 Step 4: Building the application...${NC}"
go build -o myslotmate-backend ./cmd/api/run.go
echo -e "${GREEN}✅ Build completed${NC}"

# Step 5: Run
echo -e "${YELLOW}▶️  Step 5: Starting the server...${NC}"
./myslotmate-backend

echo -e "${GREEN}✅ Server running!${NC}"
