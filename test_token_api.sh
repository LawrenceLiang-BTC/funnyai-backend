#!/bin/bash

# FunnyAI Token System API 测试脚本

BASE_URL="http://localhost:8080/api/v1"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "FunnyAI Token System API 测试"
echo "========================================"
echo ""

# 1. 健康检查
echo -e "${YELLOW}1. 健康检查${NC}"
result=$(curl -s http://localhost:8080/health)
if [[ $result == *"ok"* ]]; then
    echo -e "${GREEN}✓ 后端服务正常运行${NC}"
else
    echo -e "${RED}✗ 后端服务异常${NC}"
    echo "Response: $result"
fi
echo ""

# 2. 排行榜 API（公开）
echo -e "${YELLOW}2. 排行榜 API (公开)${NC}"
result=$(curl -s "$BASE_URL/token/leaderboard")
if [[ $result == *"period"* ]]; then
    echo -e "${GREEN}✓ 排行榜 API 正常${NC}"
    echo "Response: $result"
else
    echo -e "${RED}✗ 排行榜 API 异常${NC}"
    echo "Response: $result"
fi
echo ""

# 3. 激励池统计 API（公开）
echo -e "${YELLOW}3. 激励池统计 API (公开)${NC}"
result=$(curl -s "$BASE_URL/token/pool/stats")
if [[ $result == *"poolBalance"* ]]; then
    echo -e "${GREEN}✓ 激励池统计 API 正常${NC}"
    echo "Response: $result"
else
    echo -e "${RED}✗ 激励池统计 API 异常${NC}"
    echo "Response: $result"
fi
echo ""

# 4. 余额 API（需要认证）
echo -e "${YELLOW}4. 余额 API (需要认证)${NC}"
result=$(curl -s "$BASE_URL/token/balance")
if [[ $result == *"需要登录"* ]] || [[ $result == *"Unauthorized"* ]] || [[ $result == *"Authorization"* ]]; then
    echo -e "${GREEN}✓ 余额 API 正确要求认证${NC}"
else
    echo -e "${RED}✗ 余额 API 认证检查异常${NC}"
    echo "Response: $result"
fi
echo ""

# 5. 充值地址 API（需要认证）
echo -e "${YELLOW}5. 充值地址 API (需要认证)${NC}"
result=$(curl -s "$BASE_URL/token/deposit/address")
if [[ $result == *"需要登录"* ]] || [[ $result == *"Unauthorized"* ]] || [[ $result == *"Authorization"* ]]; then
    echo -e "${GREEN}✓ 充值地址 API 正确要求认证${NC}"
else
    echo -e "${RED}✗ 充值地址 API 认证检查异常${NC}"
    echo "Response: $result"
fi
echo ""

# 6. 提现 API（需要认证）
echo -e "${YELLOW}6. 提现 API (需要认证)${NC}"
result=$(curl -s -X POST "$BASE_URL/token/withdraw" -H "Content-Type: application/json" -d '{"amount":"100000"}')
if [[ $result == *"需要登录"* ]] || [[ $result == *"Unauthorized"* ]] || [[ $result == *"Authorization"* ]]; then
    echo -e "${GREEN}✓ 提现 API 正确要求认证${NC}"
else
    echo -e "${RED}✗ 提现 API 认证检查异常${NC}"
    echo "Response: $result"
fi
echo ""

# 7. 打赏 API（需要认证）
echo -e "${YELLOW}7. 打赏 API (需要认证)${NC}"
result=$(curl -s -X POST "$BASE_URL/token/tip/1" -H "Content-Type: application/json" -d '{"amount":"100000"}')
if [[ $result == *"需要登录"* ]] || [[ $result == *"Unauthorized"* ]] || [[ $result == *"Authorization"* ]]; then
    echo -e "${GREEN}✓ 打赏 API 正确要求认证${NC}"
else
    echo -e "${RED}✗ 打赏 API 认证检查异常${NC}"
    echo "Response: $result"
fi
echo ""

echo "========================================"
echo "测试完成"
echo "========================================"
echo ""
echo "代币系统配置信息："
echo "- 合约地址: 0x3c471D10F11142C52DE4f3A3953c39d8AAaeFfFf"
echo "- 网络: BSC (BNB Smart Chain)"
echo "- 打赏抽成: 5%"
echo "- 提现手续费: 2%"
echo "- 最低充提: 100,000 代币"
echo ""
echo "平台钱包地址: 0x19F44844AE56D49AAb0b6F4d214A1fdd21c6D236"
