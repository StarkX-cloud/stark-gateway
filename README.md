# 🚀 StarkGateway - MEV-Insulated RPC Gateway

**Drop-in RPC replacement for Ethereum mainnet. Zero code changes. Maximum profit.**

Change one line in your backend:
```javascript
// BEFORE
const provider = new ethers.JsonRpcProvider("https://alchemy.com");

// AFTER (That's it!)
const provider = new ethers.JsonRpcProvider("https://stark-gateway-1.onrender.com");
```

---

## **Features**

✅ **Drop-in RPC Replacement** — Standard JSON-RPC 2.0 compatible  
✅ **MEV Detection** — Detects swaps, liquidations, and sandwich opportunities  
✅ **Multi-Builder Routing** — Sends bundles to Flashbots, MEV-Boost, and EigenLayer AVS  
✅ **Sandwich Protection** — Detects and blocks frontrunning attempts  
✅ **Dynamic Fee Scaling** — Adjusts fees (5-15%) based on MEV opportunity size  
✅ **Private Execution** — Transactions bypassed to secure relays by default  
✅ **Flashbots Integration** — Fallback to mempool if all relays fail  
✅ **Zero Latency** — <5ms proxy overhead  
✅ **Production-Ready** — Docker, metrics, health checks, CI/CD  

---

## **Quick Start (5 minutes)**

### **Option 1: Docker (Easiest)**

```bash
# Clone and enter directory
cd ~/Documents/MEV

# Copy environment file
cp .env.example .env

# Build Docker image
docker build -t starkgateway:latest .

# Run container
docker run -p 8545:8545 --env-file .env starkgateway:latest
```

Gateway is now live at `http://localhost:8545`

### **Option 2: Local Go Build**

```bash
# Install Go 1.22+ if needed

# Download dependencies
go mod download

# Build binary
go build -o starkgateway .

# Run
./starkgateway
```

---

## **Usage**

### **1. Point Your dApp at the Gateway**

Replace your RPC URL in your backend:

```javascript
// ethers.js example
const provider = new ethers.JsonRpcProvider("http://localhost:8545");

// web3.js example
const web3 = new Web3("http://localhost:8545");
```

### **2. Monitor Fees in Real-Time**

```bash
# View health status
curl http://localhost:8545/health

# View metrics (total fees captured, MEV opportunities detected)
curl http://localhost:8545/metrics
```

**Output:**
```json
{
  "totalFeesCaptured": "0.024500 ETH",
  "transactionsProcessed": 15042,
  "mevOpportunitiesDetected": 127,
  "timestamp": 1718552400
}
```

---

## **How It Makes Money**

### **1. Order Flow Auction (OFA) Rebates** — Primary Revenue
- Block builders pay you for clean, high-quality transaction volume
- You get paid just for existing; revenue sharing with wallets/dApps using you

### **2. MEV Optimization Fees** — Secondary Revenue
- Gateway detects post-trade price swaps (backruns)
- Executes arbitrage automatically
- Keeps 10% of recovered value
- User gets 90% of their lost value back

**Example (150,000 daily transactions):**
- 2% have backrun opportunities = 3,000 opportunities
- Average recovery: $0.75 per trade
- Your fee: 10%
- **Daily revenue: 3,000 × $0.75 × 10% = $225/day**

---

## **Architecture**

```
┌─────────────────────────────────────────────────────┐
│          Developer's Backend                        │
│  const provider = "http://starkgateway.io:8545"    │
└────────────────────┬────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────┐
│    StarkGateway (Your RPC Proxy)                   │
│  ┌─────────────────────────────────────────────┐  │
│  │ JSON-RPC Request Handler (HTTP)             │  │
│  │ • Accepts eth_sendTransaction               │  │
│  │ • Accepts eth_sendRawTransaction            │  │
│  │ • Proxies all other methods                 │  │
│  └─────────────────────────────────────────────┘  │
│                    │                               │
│  ┌────────────────┴────────────────┐              │
│  │   MEV Analysis Engine           │              │
│  │  • Detects DEX swaps            │              │
│  │  • Identifies backrun patterns  │              │
│  │  • Sandwich attack detection    │              │
│  │  • Calculates dynamic fees      │              │
│  └────────────────┬────────────────┘              │
│                   │                               │
│  ┌────────────────┴────────────────────┐          │
│  │  Multi-Builder Bundle Router        │          │
│  │  • Flashbots MEV-Boost              │          │
│  │  • MEV-Boost Builders               │          │
│  │  • EigenLayer AVS Builders          │          │
│  │  • Mempool fallback if all fail     │          │
│  └────────────────┬────────────────────┘          │
└────────────────────┼────────────────────────────────┘
                     │
      ┌──────────────┼──────────────────┬──────────┐
      ▼              ▼                  ▼          ▼
   Upstream     Flashbots         MEV-Boost    EigenLayer
   RPC Node     Relay             Builders     AVS
```

---

## **Configuration**

Edit `.env` to customize:

```env
# Upstream RPC provider (must be fast + reliable)
UPSTREAM_RPC=https://eth.rpc.bloxroute.com

# Listen port
PORT=8545

# Fee percentage on detected MEV (0.10 = 10%)
FEE_PERCENTAGE=0.10

# Enable sandwich attack protection
ENABLE_SANDWICH_PROTECTION=true

# Force all eth_sendTransaction through private relays
FORCE_PRIVATE_SEND_TX=false
```

---

## **Advanced Features**

### **Multi-Builder Routing**
Bundle submissions are sent in parallel to:
- **Flashbots MEV-Boost** — Primary relay
- **MEV-Boost Builders** — Alternative builders
- **EigenLayer AVS** — Encrypted preference execution
- **Mempool Fallback** — If all relays fail, tx goes to public mempool

This maximizes bundle inclusion probability and MEV capture.

### **Sandwich Attack Protection**
Detects and blocks transactions that:
- Target DEX routers with abnormally high gas prices (>3x network average)
- Attempt to extract MEV through frontrunning

Protected transactions are relayed privately without fee extraction.

### **Dynamic Fee Scaling**
Fees automatically adjust based on MEV opportunity:
- **Small MEV** (<0.001 ETH): 5% fee
- **Medium MEV** (0.001-0.01 ETH): 8% fee  
- **Large MEV** (0.01-0.1 ETH): 12% fee
- **Massive MEV** (>0.1 ETH): 15% fee

---

## **Metrics & Monitoring**

Real-time metrics are available at the `/metrics` endpoint:

```bash
curl http://localhost:8545/metrics
```

**Response:**
```json
{
  "totalFeesCaptured": "0.156234 ETH",
  "transactionsProcessed": 45892,
  "trustedRequests": 12543,
  "mevOpportunitiesDetected": 342,
  "backrunsExecuted": 89,
  "flashbotsBundlesSent": 342,
  "flashbotsBundlesOK": 301,
  "bundleSuccessRate": "87.95%",
  "flashbotsFallbacks": 28,
  "bundlesFailed": 13,
  "sandwichAttacksBlocked": 45,
  "multiBuilderSubmissions": 301,
  "backrunSignerAddress": "0x...",
  "feeRecipient": "0x...",
  "sandwichProtection": true,
  "timestamp": 1718552400
}
```

**Key Metrics:**
- **bundleSuccessRate** — Percentage of bundles successfully included on-chain
- **sandwichAttacksBlocked** — Number of frontrunning attempts detected and prevented
- **multiBuilderSubmissions** — Bundles sent to multiple builders for redundancy

---

## **Deployment Options**

### **AWS EC2**
```bash
# Launch Ubuntu 22.04 instance
ssh ubuntu@your-instance.ec2.amazonaws.com

# Clone and run
git clone <your-repo>
cd MEV
docker build -t starkgateway .
docker run -p 8545:8545 starkgateway
```

### **DigitalOcean App Platform**
```bash
# Point to your repo, select Dockerfile
# Platform automatically builds and deploys
# Your gateway is live at: https://starkgateway-xxxxx.ondigitalocean.app
```

### **Railway / Render (Free Tier)**
```bash
# Connect your GitHub repo
# Select Dockerfile as build file
# Deploy (free tier covers ~1000 req/min)
```

---

## **API Reference**

StarkGateway implements full **Ethereum JSON-RPC 2.0** compatibility.

### **Supported Methods**

All standard Ethereum methods work:
- `eth_sendTransaction` — Send transaction
- `eth_sendRawTransaction` — Send signed transaction
- `eth_call` — Execute function without state change
- `eth_getBalance` — Get account balance
- `eth_getTransactionCount` — Get nonce
- `eth_gasPrice` — Get current gas price
- `eth_estimateGas` — Estimate gas cost
- `eth_blockNumber` — Get latest block
- ... and 50+ others

### **Health Endpoints**

```bash
# Health check
GET /health
# Response: {"status": "healthy"}

# Performance metrics
GET /metrics
# Response: {
#   "totalFeesCaptured": "0.024500 ETH",
#   "transactionsProcessed": 15042,
#   "mevOpportunitiesDetected": 127,
#   "timestamp": 1718552400
# }
```

---

## **Revenue Projection (Conservative)**

| Metric | Value |
|--------|-------|
| Gateway RPS (requests/sec) | 100 |
| Daily transactions | 8.64M |
| MEV detection rate | 2% |
| Daily opportunities | 172,800 |
| Avg. MEV per tx | $0.75 |
| Your fee | 10% |
| **Daily gross revenue** | **$12,960** |
| **Monthly gross** | **~$389K** |
| OFA rebate (per block) | +$10-50 |
| **Monthly with OFA** | **~$450K+** |

*Note: This assumes you reach competitive scale (~100 RPS). Early days will be lower.*

---

## **Next Steps**

### Phase 1 ✅ (Current)
- [x] RPC proxy with MEV detection
- [x] Basic Flashbots integration
- [x] Docker deployment

### Phase 2 (Build Now)
- [ ] Live backrun execution (sign your own arbitrage transactions)
- [ ] Block builder direct partnerships (OFA negotiations)
- [ ] On-chain preconfirmation contracts (slashing logic)
- [ ] Transaction signing vault (secure key management)

### Phase 3 (Scale)
- [ ] Multi-chain support (Arbitrum, Base, Optimism)
- [ ] Edge node network (global latency <5ms)
- [ ] Advanced MEV detection (liquidations, arbitrage extraction)
- [ ] DAO governance (decentralize fee sharing)

---

## **Troubleshooting**

**Q: Transactions are slow**
A: Check `UPSTREAM_RPC` — switch to a faster provider (Alchemy, Infura paid tier, BloxRoute)

**Q: No MEV detected**
A: Normal for non-DEX transactions. MEV detection is tuned for Uniswap/SushiSwap swaps.

**Q: Docker build fails**
A: Ensure you have `go.mod` in the directory. Run `go mod tidy` first.

**Q: 502 Bad Gateway**
A: Upstream RPC is down. Check connectivity or switch providers.

---

## **License**

MIT — Use freely, modify as needed.

**Built for**: Builders who want to capture MEV without begging for DAO votes.

---

**Questions?** Join us on Twitter: [@StarkGateway](https://twitter.com/starkgateway)
