/**
 * OKX Quant Trading System - JavaScript API Client Example
 * 使用 JavaScript fetch/axios 与 API 进行交互
 */

class QuantClient {
  /**
   * 初始化客户端
   * @param {string} baseUrl - API 基础 URL
   * @param {string} [apiToken] - API 认证令牌（可选）
   */
  constructor(baseUrl = "http://localhost:8080", apiToken = null) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.apiToken = apiToken;
  }

  /**
   * 获取请求头
   * @returns {Object} 请求头
   */
  _getHeaders() {
    const headers = {
      "Content-Type": "application/json"
    };
    if (this.apiToken) {
      headers["X-API-Token"] = this.apiToken;
    }
    return headers;
  }

  /**
   * 发送 HTTP 请求
   * @param {string} method - HTTP 方法
   * @param {string} path - API 路径
   * @param {Object} [body] - 请求体
   * @param {Object} [params] - 查询参数
   * @returns {Promise&lt;Object&gt;} 响应数据
   */
  async _request(method, path, body = null, params = null) {
    let url = `${this.baseUrl}${path}`;
    
    if (params) {
      const searchParams = new URLSearchParams(params);
      url += `?${searchParams.toString()}`;
    }

    const options = {
      method,
      headers: this._getHeaders()
    };

    if (body) {
      options.body = JSON.stringify(body);
    }

    const response = await fetch(url, options);
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    return await response.json();
  }

  /**
   * 检查服务健康状态
   * @returns {Promise&lt;Object&gt;} 健康状态信息
   */
  async getHealth() {
    return this._request("GET", "/health");
  }

  /**
   * 获取系统状态
   * @returns {Promise&lt;Object&gt;} 系统状态信息
   */
  async getStatus() {
    return this._request("GET", "/api/status");
  }

  /**
   * 获取策略列表
   * @returns {Promise&lt;Array&gt;} 策略列表
   */
  async getStrategies() {
    return this._request("GET", "/api/strategies");
  }

  /**
   * 获取持仓列表
   * @returns {Promise&lt;Array&gt;} 持仓列表
   */
  async getPositions() {
    return this._request("GET", "/api/positions");
  }

  /**
   * 获取订单列表
   * @returns {Promise&lt;Array&gt;} 订单列表
   */
  async getOrders() {
    return this._request("GET", "/api/orders");
  }

  /**
   * 获取系统指标
   * @returns {Promise&lt;Object&gt;} 指标数据
   */
  async getMetrics() {
    return this._request("GET", "/api/metrics");
  }

  /**
   * 启动策略
   * @param {string} strategyName - 策略名称
   * @returns {Promise&lt;Object&gt;} 操作结果
   */
  async startStrategy(strategyName) {
    return this._request("POST", `/api/strategy/start/${strategyName}`);
  }

  /**
   * 停止策略
   * @param {string} strategyName - 策略名称
   * @returns {Promise&lt;Object&gt;} 操作结果
   */
  async stopStrategy(strategyName) {
    return this._request("POST", `/api/strategy/stop/${strategyName}`);
  }

  /**
   * 创建订单
   * @param {string} symbol - 交易对
   * @param {string} side - 买卖方向 (buy/sell)
   * @param {string} orderType - 订单类型 (limit/market)
   * @param {number} price - 价格（限价单必填）
   * @param {number} size - 数量
   * @returns {Promise&lt;Object&gt;} 订单创建结果
   */
  async createOrder(symbol, side, orderType, price, size) {
    const data = {
      symbol,
      side,
      type: orderType,
      price,
      size
    };
    return this._request("POST", "/api/order/create", data);
  }

  /**
   * 获取行情
   * @param {string} [symbol] - 交易对
   * @returns {Promise&lt;Object&gt;} 行情数据
   */
  async getTicker(symbol = "BTC-USDT") {
    return this._request("GET", "/api/market/ticker", null, { symbol });
  }

  /**
   * 获取 K 线数据
   * @param {string} [symbol] - 交易对
   * @param {string} [interval] - K线周期
   * @param {number} [limit] - 数量限制
   * @returns {Promise&lt;Array&gt;} K线数据列表
   */
  async getBars(symbol = "BTC-USDT", interval = "1m", limit = 100) {
    return this._request("GET", "/api/market/bars", null, { 
      symbol, 
      interval, 
      limit 
    });
  }

  /**
   * 获取回测策略列表
   * @returns {Promise&lt;Object&gt;} 回测策略信息
   */
  async getBacktestStrategies() {
    return this._request("GET", "/api/backtest/strategies");
  }
}

// 使用示例
async function main() {
  console.log("=".repeat(60));
  console.log("OKX Quant Trading System - JavaScript API Client Example");
  console.log("=".repeat(60));

  const client = new QuantClient("http://localhost:8080");

  console.log("\n1. 检查服务健康状态...");
  try {
    const health = await client.getHealth();
    console.log("   健康状态:", health);
  } catch (e) {
    console.log("   错误:", e.message);
    console.log("   请确保服务已启动");
    return;
  }

  console.log("\n2. 获取系统状态...");
  try {
    const status = await client.getStatus();
    console.log("   系统状态:", JSON.stringify(status, null, 2));
  } catch (e) {
    console.log("   错误:", e.message);
  }

  console.log("\n3. 获取策略列表...");
  try {
    const strategies = await client.getStrategies();
    console.log("   策略数量:", strategies.length);
  } catch (e) {
    console.log("   错误:", e.message);
  }

  console.log("\n4. 获取系统指标...");
  try {
    const metrics = await client.getMetrics();
    console.log("   指标类型:", Object.keys(metrics));
  } catch (e) {
    console.log("   错误:", e.message);
  }

  console.log("\n5. 获取回测策略...");
  try {
    const backtestStrategies = await client.getBacktestStrategies();
    console.log("   回测策略:", JSON.stringify(backtestStrategies, null, 2));
  } catch (e) {
    console.log("   错误:", e.message);
  }

  console.log("\n" + "=".repeat(60));
  console.log("示例完成!");
  console.log("=".repeat(60));
}

// 如果在 Node.js 环境中运行
if (typeof window === 'undefined') {
  main().catch(console.error);
}

// 导出类供其他模块使用
if (typeof module !== 'undefined' &amp;&amp; module.exports) {
  module.exports = QuantClient;
}

