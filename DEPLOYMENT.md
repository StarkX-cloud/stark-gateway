# 🚀 Deployment Guide - From Local to Production

## **Quick Local Testing (2 minutes)**

```bash
# Build locally
cd c:\Users\HP\Documents\MEV
go build -o starkgateway.exe

# Run gateway
.\starkgateway.exe

# In another terminal, test it
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

**Expected response:**
```json
{"jsonrpc":"2.0","result":"0x1234567","id":1}
```

---

## **Free Deployment Options**

These options let you run StarkGateway without infrastructure costs while you validate and start earning revenue.

### **Option 1: Local + Tunnel**
Use your local machine and expose the gateway with a free tunnel.

```bash
cd c:\Users\HP\Documents\MEV
go build -o starkgateway.exe
.\starkgateway.exe
```

In a second terminal:

```bash
npx --yes localtunnel --port 8545
```

This prints a public URL like `https://friendly-name.loca.lt` that your clients can use temporarily.

> Good for demos, testing, and early sign-ups. Not for long-term production.

### **Option 2: Render Free Tier**
Render offers a free web service plan that supports Docker deployments.

1. Create a free Render account.
2. Connect your GitHub repo.
3. Create a new Web Service.
4. Choose Docker deployment and set the port to `8545`.
5. Add environment variables in Render:

```text
UPSTREAM_RPC=https://eth.rpc.bloxroute.com
PORT=8545
FEE_PERCENTAGE=0.10
ENABLE_SANDWICH_PROTECTION=true
BACKRUN_SIGNER_PK=0x... (private key)
FEE_RECIPIENT=0x... (wallet address)
TRUSTED_CLIENTS=
FORCE_PRIVATE_SEND_TX=true
```

6. Deploy and use the Render URL.

### **Option 3: Railway Free Tier**
Railway offers a free tier suitable for early development and low traffic.

1. Sign up for Railway.
2. Import your GitHub repository.
3. Create a new project and select Docker.
4. Set the same environment variables.
5. Deploy and copy the Railway service URL.

### **Option 4: Fly.io Free Tier**
Fly.io gives you a small free VM for lightweight services.

1. Install Fly CLI: `curl -L https://fly.io/install.sh | sh`
2. Run `fly auth signup`.
3. Initialize the app:

```bash
fly launch --name starkgateway --dockerfile Dockerfile --no-deploy
```

4. Set environment variables:

```bash
fly secrets set UPSTREAM_RPC=https://eth.rpc.bloxroute.com PORT=8545 FEE_PERCENTAGE=0.10 ENABLE_SANDWICH_PROTECTION=true BACKRUN_SIGNER_PK=0x... FEE_RECIPIENT=0x... FORCE_PRIVATE_SEND_TX=true
```

5. Deploy with `fly deploy`.

> These free tiers are ideal until you get revenue. When traffic grows, upgrade to a paid plan or move to full production infrastructure.

---

## **Docker Deployment (Production-Ready)**

### **Build Docker Image**

```bash
cd c:\Users\HP\Documents\MEV

# Build image
docker build -t starkgateway:latest .

# Verify image
docker images | grep starkgateway
```

### **Run Docker Locally**

```bash
# Create .env file
cat > .env << 'EOF'
UPSTREAM_RPC=https://eth.rpc.bloxroute.com
PORT=8545
FEE_PERCENTAGE=0.10
ENABLE_SANDWICH_PROTECTION=true
BACKRUN_SIGNER_PK=0x... (your private key)
FEE_RECIPIENT=0x... (your wallet address)
TRUSTED_CLIENTS=
FORCE_PRIVATE_SEND_TX=true
EOF

# Run container
docker run -p 8545:8545 --env-file .env starkgateway:latest

# Test
curl http://localhost:8545/health
```

---

## **AWS EC2 Deployment (Recommended)**

### **1. Launch Instance**

```bash
# AWS Console → EC2 → Launch Instance
# Configuration:
# - Image: Ubuntu 22.04 LTS (free tier eligible)
# - Instance type: t3.large (2 CPU, 8GB RAM)
# - Storage: 50GB gp3
# - Security group: Allow inbound 8545
# - Key pair: Create or use existing

# Cost: $60/month
```

### **2. SSH into Instance**

```bash
ssh -i your-key.pem ubuntu@ec2-xxx.compute-1.amazonaws.com
```

### **3. Install Docker**

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker ubuntu

# Verify
docker --version
```

### **4. Deploy StarkGateway**

```bash
# Clone repository
git clone https://github.com/yourusername/starkgateway.git
cd starkgateway

# Create .env with production keys
cat > .env << 'EOF'
UPSTREAM_RPC=https://eth-mainnet.g.alchemy.com/v2/YOUR_ALCHEMY_KEY
PORT=8545
FEE_PERCENTAGE=0.10
ENABLE_SANDWICH_PROTECTION=true
BACKRUN_SIGNER_PK=0x... (generate new key for production)
FEE_RECIPIENT=0x... (your withdrawal wallet)
TRUSTED_CLIENTS=
FORCE_PRIVATE_SEND_TX=true
EOF

# Build image
docker build -t starkgateway:latest .

# Run container (background)
docker run -d \
  --name starkgateway \
  --restart always \
  -p 8545:8545 \
  --env-file .env \
  starkgateway:latest

# Verify running
docker logs -f starkgateway
```

### **5. Set Up Public Domain**

```bash
# Option A: Use Route53 (AWS DNS)
# - Go to Route53 console
# - Create hosted zone for your domain
# - Add A record pointing to EC2 public IP
# - Verify DNS propagation (5-10 min)

# Option B: Use Cloudflare (Better for DDoS protection)
# - Create Cloudflare account
# - Add site (point nameservers)
# - Create A record → EC2 public IP
# - Enable DDoS protection (free)

# Test DNS
nslookup starkgateway.io
```

### **6. Get SSL Certificate (Free)**

```bash
# Install Certbot
sudo apt install certbot python3-certbot-nginx -y

# Get certificate (if using Nginx reverse proxy)
sudo certbot certonly --standalone -d starkgateway.io

# Certificate stored at: /etc/letsencrypt/live/starkgateway.io/
```

### **7. Set Up Nginx Reverse Proxy (Optional but Recommended)**

```bash
# Install Nginx
sudo apt install nginx -y

# Create Nginx config
sudo tee /etc/nginx/sites-available/starkgateway > /dev/null <<'EOF'
upstream starkgateway {
    server localhost:8545;
}

server {
    listen 80;
    server_name starkgateway.io;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name starkgateway.io;

    ssl_certificate /etc/letsencrypt/live/starkgateway.io/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/starkgateway.io/privkey.pem;

    location / {
        proxy_pass http://starkgateway;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
        proxy_connect_timeout 300s;
    }
}
EOF

# Enable site
sudo ln -s /etc/nginx/sites-available/starkgateway /etc/nginx/sites-enabled/

# Test & start
sudo nginx -t
sudo systemctl start nginx
sudo systemctl enable nginx
```

### **8. Monitor Gateway**

```bash
# Check health
curl https://starkgateway.io/health

# View metrics
curl https://starkgateway.io/metrics

# Check Docker logs
docker logs -f starkgateway

# Monitor resources
docker stats starkgateway
```

---

## **DigitalOcean App Platform (Easiest)**

### **1. Push Code to GitHub**

```bash
git add .
git commit -m "Production deployment"
git push origin main
```

### **2. Create DigitalOcean Account & App**

- Go to DigitalOcean
- Create new App
- Select "GitHub" as source
- Choose your repository
- Select Dockerfile

### **3. Configure Environment Variables**

```
UPSTREAM_RPC=https://eth.rpc.bloxroute.com
FEE_PERCENTAGE=0.10
BACKRUN_SIGNER_PK=0x...
FEE_RECIPIENT=0x...
...
```

### **4. Deploy**

- Click "Create App"
- DigitalOcean builds & deploys automatically
- Your app is live at: `https://starkgateway-xxxxx.ondigitalocean.app`

**Cost:** $5-12/month

---

## **Multi-Region Deployment (Enterprise)**

### **1. Create Multi-Region Setup**

```bash
# Create instances in 3 regions
# - us-east-1 (primary)
# - eu-west-1 (backup)
# - ap-southeast-1 (Asia traffic)

# Each instance gets same Docker image + .env
```

### **2. Set Up Global Load Balancer**

```bash
# AWS CloudFront (CDN)
# - Origin: ALB across 3 regions
# - Caching: Minimal (5s TTL for RPC)
# - DDoS: AWS Shield (built-in)

# Result: Global endpoint
# https://rpc.starkgateway.io
```

### **3. Monitor with CloudWatch**

```bash
# CloudWatch dashboards show:
# - Request latency (p50, p95, p99)
# - Error rates
# - MEV captured per region
# - Cost tracking
```

---

## **Health Checks & Monitoring**

### **Basic Health Check**

```bash
# Should return 200 OK
curl -X GET http://localhost:8545/health

# Response:
{"status":"healthy"}
```

### **Set Up Alerting**

```bash
# AWS CloudWatch alarm
# - Metric: Status code != 200
# - Threshold: >5% error rate
# - Action: SNS email alert

# PagerDuty integration
# - Get alerted on production issues
# - Escalate if no ack after 5 min
```

### **Metrics Dashboard**

```bash
# Create dashboard to monitor:
# - Transactions processed
# - MEV opportunities detected
# - Bundle success rate
# - Sandwich attacks blocked
# - Revenue captured

# Tools: Grafana + Prometheus (self-hosted)
# Or: AWS CloudWatch (simpler)
```

---

## **Troubleshooting**

### **Gateway Not Starting**

```bash
# Check logs
docker logs starkgateway

# Common issues:
# 1. Private key format wrong → Verify hex format
# 2. Port 8545 already in use → Kill process: lsof -i :8545
# 3. Upstream RPC down → Try different RPC provider
```

### **Transactions Not Going Through**

```bash
# Check if bundle relay is responding
curl -X POST https://relay.flashbots.net \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# If no response → Use backup relays
# Check BUILDER_RELAYS list in main.go
```

### **High Latency**

```bash
# Profile request latency
# 1. Add timing to metrics endpoint
# 2. Check if upstream RPC is slow
# 3. Consider switching to faster RPC:
#    - Alchemy (fastest)
#    - Infura (reliable)
#    - Quicknode (dedicated)
```

---

## **Performance Tuning**

### **Optimize for Throughput**

```bash
# Docker run command with resource limits
docker run -d \
  --name starkgateway \
  --cpus="2" \
  --memory="4g" \
  --memory-swap="8g" \
  --restart always \
  -p 8545:8545 \
  --env-file .env \
  starkgateway:latest
```

### **Increase File Descriptors**

```bash
# Increase open file limit (for connection pooling)
sudo sysctl -w fs.file-max=2097152
echo "fs.file-max = 2097152" | sudo tee -a /etc/sysctl.conf

# Verify
ulimit -n
```

### **Network Optimization**

```bash
# Tune TCP settings
sudo sysctl -w net.ipv4.tcp_tw_reuse=1
sudo sysctl -w net.ipv4.ip_local_port_range="1024 65535"
sudo sysctl -w net.core.somaxconn=65535
```

---

## **Backup & Recovery**

### **Backup Private Keys**

```bash
# NEVER share your BACKRUN_SIGNER_PK
# Store in:
# 1. AWS Secrets Manager
# 2. HashiCorp Vault
# 3. Encrypted USB backup (offline)

# Generate new key for each environment
# Mainnet: production key (in vault)
# Goerli: test key (local)
```

### **Database Backup** (if using PostgreSQL for metrics)

```bash
# Daily automated backup
# pg_dump -h localhost -U postgres starkgateway > backup.sql

# Restore if needed
# psql -h localhost -U postgres starkgateway < backup.sql
```

---

## **Final Checklist**

- ✅ Gateway running and responding to requests
- ✅ Health endpoint returns 200 OK
- ✅ Metrics endpoint shows transactions processed
- ✅ SSL certificate installed (HTTPS)
- ✅ Domain DNS pointing to gateway
- ✅ Firewall allows 8545 inbound
- ✅ Private keys stored securely
- ✅ Monitoring/alerting configured
- ✅ Backup strategy in place
- ✅ Documentation updated

---

**Your gateway is now live and ready to capture MEV.**

Next: Point your first customer at it and start making money.
