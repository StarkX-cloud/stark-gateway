package main

// Example 1: Basic JSON-RPC call (works with any standard Ethereum library)
//
// JavaScript (ethers.js):
// const provider = new ethers.JsonRpcProvider("http://localhost:8545");
// const balance = await provider.getBalance("0x...");
//
// JavaScript (web3.js):
// const web3 = new Web3("http://localhost:8545");
// const balance = await web3.eth.getBalance("0x...");
//
// Python (web3.py):
// from web3 import Web3
// w3 = Web3(Web3.HTTPProvider("http://localhost:8545"))
// balance = w3.eth.get_balance("0x...")
//

// Example 2: Raw JSON-RPC over HTTP
//
// GET request:
// curl -X POST http://localhost:8545 \
//   -H "Content-Type: application/json" \
//   -d '{
//     "jsonrpc": "2.0",
//     "method": "eth_getBalance",
//     "params": ["0x1234567890123456789012345678901234567890", "latest"],
//     "id": 1
//   }'
//

// Example 3: Send a transaction (gateway detects MEV automatically)
//
// const tx = {
//   to: "0xUniswapRouterAddress",
//   data: "0x38ed1739...", // Uniswap swap signature
//   value: ethers.parseEther("1.0"),
//   gasLimit: 300000
// };
//
// // User sends transaction
// const receipt = await signer.sendTransaction(tx);
//
// // Your gateway:
// // 1. ✅ Detects it's a Uniswap swap
// // 2. ✅ Calculates MEV opportunity ($0.75)
// // 3. ✅ Takes 10% fee ($0.075)
// // 4. ✅ Bundles it with Flashbots
// // 5. ✅ Executes in next block
// //
// // User never knows MEV was recovered!
//

// Example 4: Monitor gateway metrics
//
// curl http://localhost:8545/metrics
//
// Response:
// {
//   "totalFeesCaptured": "0.024500 ETH",
//   "transactionsProcessed": 15042,
//   "mevOpportunitiesDetected": 127,
//   "timestamp": 1718552400
// }
//
