# OKX Quant Trading System - API 客户端示例

本目录包含多语言 API 客户端示例，方便开发者快速接入 OKX 量化交易系统。

## 目录

- [Python 客户端](#python-客户端)
- [JavaScript 客户端](#javascript-客户端)
- [Go 客户端](#go-客户端)

---

## Python 客户端

### 依赖安装
```bash
pip install requests
```

### 使用示例
```python
from client_python import QuantClient

client = QuantClient(
    base_url="http://localhost:8080",
    api_token="your-api-token"  # 可选
)

# 检查健康状态
health = client.get_health()
print(health)

# 获取系统状态
status = client.get_status()
print(status)

# 获取行情
ticker = client.get_ticker("BTC-USDT")
print(ticker)

# 创建订单
result = client.create_order(
    symbol="BTC-USDT",
    side="buy",
    order_type="limit",
    price=50000.0,
    size=0.01
)
print(result)
```

---

## JavaScript 客户端

### 使用示例 (浏览器)
```html
&lt;script src="client_javascript.js"&gt;&lt;/script&gt;
&lt;script&gt;
  const client = new QuantClient(
    "http://localhost:8080",
    "your-api-token"  // 可选
  );

  // 检查健康状态
  client.getHealth().then(health =&gt; {
    console.log(health);
  });

  // 获取系统状态
  client.getStatus().then(status =&gt; {
    console.log(status);
  });
&lt;/script&gt;
```

### 使用示例 (Node.js)
```javascript
const QuantClient = require('./client_javascript');

const client = new QuantClient(
  "http://localhost:8080",
  "your-api-token"  // 可选
);

async function main() {
  const health = await client.getHealth();
  console.log(health);

  const status = await client.getStatus();
  console.log(status);
}

main();
```

---

## Go 客户端

### 编译和运行
```bash
cd examples
go run client_go.go
```

### 使用示例
```go
package main

import (
    "fmt"
    "encoding/json"
)

func main() {
    client := NewQuantClient(
        "http://localhost:8080",
        "your-api-token"  // 可选
    )

    health, err := client.GetHealth()
    if err != nil {
        panic(err)
    }
    fmt.Printf("健康状态: %v\n", health)

    status, err := client.GetStatus()
    if err != nil {
        panic(err)
    }
    statusJSON, _ := json.MarshalIndent(status, "", "  ")
    fmt.Printf("系统状态: %s\n", statusJSON)
}
```

---

## API 端点列表

### 系统端点
- `GET /health` - 健康检查
- `GET /ready` - 就绪检查
- `GET /api/status` - 系统状态

### 策略端点
- `GET /api/strategies` - 获取策略列表
- `POST /api/strategy/start/{name}` - 启动策略
- `POST /api/strategy/stop/{name}` - 停止策略

### 交易端点
- `GET /api/positions` - 获取持仓
- `GET /api/orders` - 获取订单
- `POST /api/order/create` - 创建订单
- `POST /api/position/close/{symbol}` - 平仓

### 市场数据端点
- `GET /api/market/ticker` - 获取行情
- `GET /api/market/bars` - 获取 K 线
- `GET /api/market/orderbook` - 获取订单簿

### 监控端点
- `GET /api/metrics` - 获取系统指标
- `GET /api/metrics/prometheus` - Prometheus 格式指标

### 回测端点
- `GET /api/backtest/strategies` - 获取回测策略列表
- `POST /api/backtest/start` - 启动回测
- `GET /api/backtest/results/{taskId}` - 获取回测结果
- `GET /api/backtest/report/{taskId}` - 获取回测报告

---

## 认证

如果服务配置了 API Token 认证，需要在请求头中添加：
```
X-API-Token: your-api-token
```

---

## Swagger UI

可以通过 Swagger UI 查看完整的 API 文档：
```
http://localhost:8080/swagger/index.html
```

---

## 注意事项

1. 确保服务已启动并监听在正确的端口
2. 如果服务配置了 CORS，需要在服务端配置允许的源
3. 生产环境请使用 HTTPS
4. 妥善保管 API Token，不要泄露

