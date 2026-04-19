# MySlotMate Backend Deployment Guide

## Overview
This deployment setup uses:
- **PM2** - Node.js process manager for Go applications
- **Nginx** - Reverse proxy to handle incoming requests
- **Systemd** - System daemon to keep PM2 running

## Prerequisites
- Go 1.18+
- Node.js/npm (for PM2)
- Ubuntu/Debian-based system
- Git access
- Database credentials in `.env`

## Quick Start

```bash
# Make script executable
chmod +x script.sh

# Run the complete deployment
./script.sh
```

The script will:
1. Install PM2 and Nginx (if missing)
2. Pull latest code from Git
3. Install Go dependencies
4. Run database migrations
5. Build the application
6. Start with PM2 (auto-restart enabled)
7. Configure Nginx reverse proxy
8. Show status and commands

## What Gets Deployed

### Application (PM2)
- **instances**: All available CPU cores
- **exec_mode**: Cluster mode (load balanced)
- **auto-restart**: Yes
- **max memory**: 1GB per instance

### Reverse Proxy (Nginx)
- Listens on port 80
- Forwards traffic to app running on port 5000
- Gzip compression enabled
- Max upload size: 50MB

## Important Commands

### PM2 Management
```bash
# View logs
pm2 logs myslotmate-backend

# Restart application
pm2 restart myslotmate-backend

# Stop application
pm2 stop myslotmate-backend

# Monitor resources
pm2 monit

# View all processes
pm2 status

# Delete from PM2
pm2 delete myslotmate-backend
```

### Nginx Management
```bash
# Check Nginx status
sudo systemctl status nginx

# Reload Nginx (no downtime)
sudo systemctl reload nginx

# Restart Nginx
sudo systemctl restart nginx

# Stop Nginx
sudo systemctl stop nginx

# Test configuration
sudo nginx -t
```

### System Status
```bash
# Check port 80 (Nginx)
lsof -i :80

# Check port 5000 (App)
lsof -i :5000

# View systemd PM2 service
systemctl status pm2-root
```

## Monitoring

### View Real-time Logs
```bash
pm2 logs myslotmate-backend
pm2 logs myslotmate-backend --lines 100
pm2 logs myslotmate-backend --err  # errors only
```

### Monitor System Resources
```bash
pm2 monit  # Real-time CPU/Memory usage

# Or check logs directory
tail -f logs/out.log
tail -f logs/error.log
```

## SSL/HTTPS Setup

To add SSL with Let's Encrypt:

```bash
# Install Certbot
sudo apt-get install certbot python3-certbot-nginx

# Get certificate
sudo certbot --nginx -d yourdomain.com

# Auto-renewal is set up automatically
```

Then update the Nginx config to the certificate paths.

## Environment Variables

Required in `.env`:
```
HTTP_PORT=5000
DATABASE_URL=postgresql://...
OPENAI_API_KEY=...
PINECONE_API_KEY=...
# ... other env vars
```

## Troubleshooting

### Application won't start
```bash
# Check logs
pm2 logs myslotmate-backend

# Check for port conflicts
lsof -i :5000

# Verify build succeeded
go build -o myslotmate-backend ./cmd/api/run.go
```

### Nginx showing 502 Bad Gateway
```bash
# Check if app is running
pm2 status

# Check app logs
pm2 logs myslotmate-backend

# Verify app is listening on 5000
lsof -i :5000

# Test Nginx config
sudo nginx -t
```

### Can't connect to app
```bash
# Check Nginx status
sudo systemctl status nginx

# Check if listening on port 80
sudo lsof -i :80

# Check UFW firewall
sudo ufw status
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

## Deployment Workflow

1. **Development**: Test locally
2. **Commit**: Push to main branch
3. **Deploy**: SSH into AWS instance and run `./script.sh`
4. **Monitor**: Check `pm2 logs` for issues
5. **Update**: Weekly database backups, monitor disk space

## File Structure After Deployment
```
myslotmate-backend/
├── myslotmate-backend          (compiled binary)
├── script.sh                    (deployment script)
├── pm2.config.js               (PM2 configuration)
├── logs/
│   ├── out.log                 (application output)
│   └── error.log               (application errors)
├── .env                        (environment variables)
└── ... (source code)
```

## Automation (Optional)

Add to crontab for weekly deployments:
```bash
# Edit crontab
crontab -e

# Add this line (deploy every Sunday at 2 AM)
0 2 * * 0 cd /path/to/myslotmate-backend && ./script.sh
```

## Performance Tuning

For high traffic, consider:
```bash
# Increase file descriptors
ulimit -n 65535

# PM2 configs: increase instances
pm2 start app --instances 8

# Nginx tuning in /etc/nginx/nginx.conf
worker_connections 2048;
```

## Support
For issues, check:
1. Application logs: `pm2 logs myslotmate-backend`
2. Nginx logs: `/var/log/nginx/error.log`
3. System logs: `journalctl -u pm2-root -n 50`
