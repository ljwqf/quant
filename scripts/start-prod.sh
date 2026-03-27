#!/bin/bash
# OKX量化交易系统 - 实盘启动脚本

echo "============================================"
echo "  警告: 实盘模式 (PRODUCTION)"
echo "  将使用真实资金!"
echo "============================================"

# 确认操作
read -p "确认启动实盘模式? (输入 yes 继续): " confirm
if [ "$confirm" != "yes" ]; then
    echo "已取消"
    exit 1
fi

# 检查环境变量
if [ -z "$OKX_API_KEY" ] || [ -z "$OKX_SECRET_KEY" ] || [ -z "$OKX_PASSPHRASE" ]; then
    echo "错误: 请先设置 OKX API 环境变量"
    echo "  export OKX_API_KEY=xxx"
    echo "  export OKX_SECRET_KEY=xxx"
    echo "  export OKX_PASSPHRASE=xxx"
    exit 1
fi

# 设置环境变量（如果需要）
# source scripts/setup-env.sh

# 启动程序
go run ./cmd/trader -env production "$@"