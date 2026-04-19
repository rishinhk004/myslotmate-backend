#!/bin/bash

set -e  # Exit on any error

echo "🚀 MySlotMate Backend Deployment Script"
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
APP_NAME="myslotmate-backend"
APP_PORT=${HTTP_PORT:-5000}
NGINX_CONF="/etc/nginx/sites-available/$APP_NAME"
NGINX_ENABLED="/etc/nginx/sites-enabled/$APP_NAME"

# Step 0: Install PM2 & Nginx (if not already installed)
echo -e "${YELLOW}📦 Step 0: Installing PM2 and Nginx...${NC}"
if ! command -v pm2 &> /dev/null; then
  npm install -g pm2
  pm2 startup
  pm2 save
fi

if ! command -v nginx &> /dev/null; then
  sudo apt-get update
  sudo apt-get install -y nginx
fi
echo -e "${GREEN}✅ PM2 and Nginx ready${NC}"

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
go build -o $APP_NAME ./cmd/api/run.go
echo -e "${GREEN}✅ Build completed${NC}"

# Step 5: Stop old PM2 process (if running)
echo -e "${YELLOW}🛑 Step 5: Stopping old PM2 process...${NC}"
pm2 delete $APP_NAME 2>/dev/null || true
echo -e "${GREEN}✅ Old process stopped${NC}"

# Step 6: Start with PM2
echo -e "${YELLOW}▶️  Step 6: Starting application with PM2...${NC}"
pm2 start ./$APP_NAME --name=$APP_NAME --instances=max --exec-mode=cluster
pm2 save
echo -e "${GREEN}✅ Application started with PM2${NC}"

# Step 7: Configure Nginx Reverse Proxy
echo -e "${YELLOW}🌐 Step 7: Configuring Nginx reverse proxy...${NC}"
sudo tee $NGINX_CONF > /dev/null <<EOF
upstream myslotmate_backend {
    server 127.0.0.1:$APP_PORT;
    keepalive 64;
}

server {
    listen 80;
    server_name api.myslotmate.com;
    client_max_body_size 50M;

    gzip on;
    gzip_types text/plain text/css text/javascript application/json application/javascript;

    location / {
        proxy_pass http://myslotmate_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_cache_bypass \$http_upgrade;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
EOF
echo -e "${GREEN}✅ Nginx configuration created${NC}"

# Step 8: Enable Nginx site
echo -e "${YELLOW}🔗 Step 8: Enabling Nginx site...${NC}"
sudo ln -sf $NGINX_CONF $NGINX_ENABLED 2>/dev/null || true
sudo rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
echo -e "${GREEN}✅ Nginx site enabled${NC}"

# Step 9: Test & Reload Nginx
echo -e "${YELLOW}🧪 Step 9: Testing and reloading Nginx...${NC}"
sudo nginx -t
sudo systemctl reload nginx
echo -e "${GREEN}✅ Nginx reloaded${NC}"

# Step 10: Verify Status
echo -e "${BLUE}📊 Step 10: Verification${NC}"
echo -e "${BLUE}PM2 Status:${NC}"
pm2 status
echo -e "\n${BLUE}Nginx Status:${NC}"
sudo systemctl status nginx --no-pager

echo -e "\n${GREEN}✅ Deployment Complete!${NC}"
echo -e "${BLUE}Visit http://localhost or your domain to access the API${NC}"
echo -e "${YELLOW}PM2 Commands:${NC}"
echo -e "  pm2 logs $APP_NAME        - View logs"
echo -e "  pm2 restart $APP_NAME     - Restart app"
echo -e "  pm2 stop $APP_NAME        - Stop app"
echo -e "  pm2 monit                 - Monitor resources"
