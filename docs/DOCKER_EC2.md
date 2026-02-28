# PostgreSQL on EC2 (Docker on EC2 only)

Postgres runs in Docker on EC2. Your local machine connects via `DATABASE_URL` – no Docker needed locally.

## 1. EC2: Install Docker & run Postgres

SSH into your EC2 instance, then:

```bash
# Install Docker (Amazon Linux 2023)
sudo yum update -y
sudo yum install docker -y
sudo systemctl start docker && sudo systemctl enable docker
sudo usermod -aG docker ec2-user
# Log out and back in

# Create project dir
mkdir -p ~/myslotmate && cd ~/myslotmate

# Create docker-compose.yml (Postgres only – no migrate service needed)
cat > docker-compose.yml << 'EOF'
services:
  postgres:
    image: postgres:16-alpine
    container_name: myslotmate-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-ChangeMe123!}
      POSTGRES_DB: myslotmate
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
EOF

# Start Postgres
export POSTGRES_PASSWORD="YourStrongPassword123!"
docker compose up -d
```

## 2. EC2: Open port 5432

- EC2 → Instances → your instance → Security tab → Security group
- Edit inbound rules → Add rule
- Type: PostgreSQL, Port: 5432, Source: Your IP (or 0.0.0.0/0 for testing)
- Save

## 3. Local: Set DATABASE_URL

In your `.env`:

```
DATABASE_URL=postgresql://postgres:YourStrongPassword123!@YOUR_EC2_PUBLIC_IP:5432/myslotmate
```

Replace `YOUR_EC2_PUBLIC_IP` with your instance’s public IP.

## 4. Local: Run migrations (no Docker)

```powershell
go run ./cmd/migrate
```

Or:

```powershell
./scripts/migrate.ps1
```

## 5. Local: Verify

```powershell
go run ./cmd/checkdb
```

---

## Summary

| Where   | What runs                          |
|---------|------------------------------------|
| **EC2** | Docker + Postgres only             |
| **Local** | Go app, migrations, checkdb – connects to EC2 via `DATABASE_URL` |
