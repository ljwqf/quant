#!/bin/bash
# OKX 量化交易系统环境变量配置
# 使用方法: source scripts/setup-env.sh

# ============================================
# OKX 交易所凭证
# ============================================
export OKX_API_KEY="your-okx-api-key-here"
export OKX_SECRET_KEY="your-okx-secret-key-here"
export OKX_PASSPHRASE="your-okx-passphrase-here"

# ============================================
# CryptoQuant API Key (SmartFilter使用)
# ============================================
export CRYPTOQUANT_API_KEY="your-cryptoquant-api-key-here"

# ============================================
# LLM 提供商 API Keys (可选)
# ============================================
export OPENAI_API_KEY=""
export CLAUDE_API_KEY=""
export QWEN_API_KEY=""

# ============================================
# 加密密钥 (用于敏感数据加密, 可选)
# ============================================
export ENCRYPTION_KEY=""

echo "环境变量已设置"
echo "请确保已替换上述占位符为真实凭证"