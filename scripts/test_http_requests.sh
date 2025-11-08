#!/bin/bash

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "DEX Aggregator HTTP API Tests"
echo "============================="
echo "Project Root: $PROJECT_ROOT"

BASE_URL="http://localhost:8080"

# 测试健康检查
echo "1. Testing health check..."
curl -s "${BASE_URL}/health" | jq .

# 测试获取配置
echo -e "\n2. Testing config endpoint..."
curl -s "${BASE_URL}/config" | jq .

# 测试获取所有池子
echo -e "\n3. Testing get all pools..."
curl -s "${BASE_URL}/api/v1/pools" | jq '.count'

# 测试搜索WETH/USDT池子
echo -e "\n4. Testing pool search for WETH/USDT..."
curl -s "${BASE_URL}/api/v1/pools/search?tokenA=0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2&tokenB=0xdac17f958d2ee523a2206206994597c13d831ec7" | jq '.count'

# 测试报价API - WETH to USDT
echo -e "\n5. Testing quote API - WETH to USDT..."
curl -s -X POST "${BASE_URL}/api/v1/quote" \
  -H "Content-Type: application/json" \
  -d '{
    "tokenIn": "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
    "tokenOut": "0xdac17f958d2ee523a2206206994597c13d831ec7",
    "amountIn": "1000000000000000000",
    "maxHops": 3
  }' | jq '{amountOut: .amountOut, gasEstimate: .gasEstimate, pathsCount: (.paths | length)}'

# 测试报价API - WETH to DAI
echo -e "\n6. Testing quote API - WETH to DAI..."
curl -s -X POST "${BASE_URL}/api/v1/quote" \
  -H "Content-Type: application/json" \
  -d '{
    "tokenIn": "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2",
    "tokenOut": "0x6b175474e89094c44da98b954eedeac495271d0f", 
    "amountIn": "500000000000000000",
    "maxHops": 3
  }' | jq '{amountOut: .amountOut, bestPath: .bestPath.dexes}'

# 测试错误情况 - 无效代币地址
echo -e "\n7. Testing error case - invalid token..."
curl -s -X POST "${BASE_URL}/api/v1/quote" \
  -H "Content-Type: application/json" \
  -d '{
    "tokenIn": "invalid_address",
    "tokenOut": "0xdac17f958d2ee523a2206206994597c13d831ec7",
    "amountIn": "1000000000000000000"
  }' | jq '.'

echo -e "\nTests completed!"