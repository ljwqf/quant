#!/bin/bash
# OKX量化交易系统 - 模拟盘启动脚本

echo "============================================"
echo "  启动模式: 模拟盘 (SIMULATION)"
echo "  不会使用真实资金"
echo "============================================"

# 设置环境变量（如果需要）
# source scripts/setup-env.sh

# 启动程序
go run ./cmd/trader -env simulation "$@"