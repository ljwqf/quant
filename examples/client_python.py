#!/usr/bin/env python3
"""
OKX Quant Trading System - Python API Client Example
使用 Python requests 库与 API 进行交互
"""

import requests
import json
from typing import Dict, Any, Optional


class QuantClient:
    """量化交易系统 API 客户端"""

    def __init__(self, base_url: str = "http://localhost:8080", api_token: Optional[str] = None):
        """
        初始化客户端
        
        Args:
            base_url: API 基础 URL
            api_token: API 认证令牌（可选）
        """
        self.base_url = base_url.rstrip('/')
        self.api_token = api_token
        self.session = requests.Session()

    def _get_headers(self) -> Dict[str, str]:
        """获取请求头"""
        headers = {
            "Content-Type": "application/json"
        }
        if self.api_token:
            headers["X-API-Token"] = self.api_token
        return headers

    def get_health(self) -&gt; Dict[str, Any]:
        """
        检查服务健康状态
        
        Returns:
            健康状态信息
        """
        response = self.session.get(
            f"{self.base_url}/health",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def get_status(self) -&gt; Dict[str, Any]:
        """
        获取系统状态
        
        Returns:
            系统状态信息
        """
        response = self.session.get(
            f"{self.base_url}/api/status",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def get_strategies(self) -&gt; list:
        """
        获取策略列表
        
        Returns:
            策略列表
        """
        response = self.session.get(
            f"{self.base_url}/api/strategies",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def get_positions(self) -&gt; list:
        """
        获取持仓列表
        
        Returns:
            持仓列表
        """
        response = self.session.get(
            f"{self.base_url}/api/positions",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def get_orders(self) -&gt; list:
        """
        获取订单列表
        
        Returns:
            订单列表
        """
        response = self.session.get(
            f"{self.base_url}/api/orders",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def get_metrics(self) -&gt; Dict[str, Any]:
        """
        获取系统指标
        
        Returns:
            指标数据
        """
        response = self.session.get(
            f"{self.base_url}/api/metrics",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def start_strategy(self, strategy_name: str) -&gt; Dict[str, Any]:
        """
        启动策略
        
        Args:
            strategy_name: 策略名称
            
        Returns:
            操作结果
        """
        response = self.session.post(
            f"{self.base_url}/api/strategy/start/{strategy_name}",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def stop_strategy(self, strategy_name: str) -&gt; Dict[str, Any]:
        """
        停止策略
        
        Args:
            strategy_name: 策略名称
            
        Returns:
            操作结果
        """
        response = self.session.post(
            f"{self.base_url}/api/strategy/stop/{strategy_name}",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()

    def create_order(self, symbol: str, side: str, order_type: str, 
                    price: float, size: float) -&gt; Dict[str, Any]:
        """
        创建订单
        
        Args:
            symbol: 交易对
            side: 买卖方向 (buy/sell)
            order_type: 订单类型 (limit/market)
            price: 价格（限价单必填）
            size: 数量
            
        Returns:
            订单创建结果
        """
        data = {
            "symbol": symbol,
            "side": side,
            "type": order_type,
            "price": price,
            "size": size
        }
        response = self.session.post(
            f"{self.base_url}/api/order/create",
            headers=self._get_headers(),
            json=data
        )
        response.raise_for_status()
        return response.json()

    def get_ticker(self, symbol: str = "BTC-USDT") -&gt; Dict[str, Any]:
        """
        获取行情
        
        Args:
            symbol: 交易对
            
        Returns:
            行情数据
        """
        params = {"symbol": symbol}
        response = self.session.get(
            f"{self.base_url}/api/market/ticker",
            headers=self._get_headers(),
            params=params
        )
        response.raise_for_status()
        return response.json()

    def get_bars(self, symbol: str = "BTC-USDT", 
                 interval: str = "1m", limit: int = 100) -&gt; list:
        """
        获取 K 线数据
        
        Args:
            symbol: 交易对
            interval: K线周期
            limit: 数量限制
            
        Returns:
            K线数据列表
        """
        params = {
            "symbol": symbol,
            "interval": interval,
            "limit": limit
        }
        response = self.session.get(
            f"{self.base_url}/api/market/bars",
            headers=self._get_headers(),
            params=params
        )
        response.raise_for_status()
        return response.json()

    def get_backtest_strategies(self) -&gt; Dict[str, Any]:
        """
        获取回测策略列表
        
        Returns:
            回测策略信息
        """
        response = self.session.get(
            f"{self.base_url}/api/backtest/strategies",
            headers=self._get_headers()
        )
        response.raise_for_status()
        return response.json()


def main():
    """使用示例"""
    print("=" * 60)
    print("OKX Quant Trading System - Python API Client Example")
    print("=" * 60)

    client = QuantClient(base_url="http://localhost:8080")

    print("\n1. 检查服务健康状态...")
    try:
        health = client.get_health()
        print(f"   健康状态: {health}")
    except Exception as e:
        print(f"   错误: {e}")
        print("   请确保服务已启动")
        return

    print("\n2. 获取系统状态...")
    try:
        status = client.get_status()
        print(f"   系统状态: {json.dumps(status, indent=2, ensure_ascii=False)}")
    except Exception as e:
        print(f"   错误: {e}")

    print("\n3. 获取策略列表...")
    try:
        strategies = client.get_strategies()
        print(f"   策略数量: {len(strategies)}")
    except Exception as e:
        print(f"   错误: {e}")

    print("\n4. 获取系统指标...")
    try:
        metrics = client.get_metrics()
        print(f"   指标类型: {list(metrics.keys())}")
    except Exception as e:
        print(f"   错误: {e}")

    print("\n5. 获取回测策略...")
    try:
        backtest_strategies = client.get_backtest_strategies()
        print(f"   回测策略: {json.dumps(backtest_strategies, indent=2, ensure_ascii=False)}")
    except Exception as e:
        print(f"   错误: {e}")

    print("\n" + "=" * 60)
    print("示例完成!")
    print("=" * 60)


if __name__ == "__main__":
    main()

