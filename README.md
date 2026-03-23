# OKX量化交易系统

这是一个基于Go语言开发的OKX量化交易系统，支持自动交易、风险管理、回测和监控功能。

## 功能特性

- **自动交易**：基于OKX v5 API实现的自动交易功能
- **风险管理**：实现止损止盈、仓位限制、每日亏损限制等风险控制
- **回测系统**：基于历史数据测试交易策略，生成绩效报告
- **监控系统**：实时监控交易状态、系统指标和告警通知
- **容器化部署**：支持Docker容器化部署和运维
- **SmartFilter**：基于链上数据的市场状态分析和策略过滤
- **多数据源支持**：支持Santiment和CryptoQuant等链上数据源

## 项目结构

```
okx-quant/
├── cmd/                # 命令行入口
│   └── main.go         # 主入口文件
├── configs/            # 配置文件
│   └── config.yaml     # 主配置文件
├── internal/           # 内部包
│   ├── backtest/       # 回测系统
│   ├── config/         # 配置管理
│   ├── exchange/       # 交易所接口
│   │   └── okx/        # OKX API实现
│   ├── monitoring/     # 监控系统
│   └── risk/           # 风险管理
├── pkg/                # 公共包
│   ├── logger/         # 日志系统
│   └── types/          # 核心类型定义
├── Dockerfile          # Docker构建文件
├── docker-compose.yml  # Docker Compose配置
├── prometheus.yml      # Prometheus配置
├── go.mod              # Go模块文件
├── go.sum              # Go依赖校验文件
└── README.md           # 项目说明文档
```

## 安装与运行

### 环境要求

- Go 1.21+
- Docker (可选，用于容器化部署)
- Docker Compose (可选，用于容器化部署)

### 本地运行

1. 克隆项目

```bash
git clone https://github.com/yourusername/okx-quant.git
cd okx-quant
```

2. 安装依赖

```bash
go mod download
```

3. 配置API密钥

编辑 `configs/config.yaml` 文件，填写OKX API密钥信息：

```yaml
exchange:
  okx:
    api_key: "your_api_key"
    secret_key: "your_secret_key"
    passphrase: "your_passphrase"
```

4. 运行应用

```bash
go run cmd/main.go
```

### 容器化部署

1. 构建镜像

```bash
docker-compose build
```

2. 运行容器

```bash
docker-compose up -d
```

3. 查看日志

```bash
docker-compose logs -f
```

## 配置说明

配置文件位于 `configs/config.yaml`，主要包含以下部分：

- **basic**：基本配置，如应用名称、环境、日志级别等
- **exchange**：交易所配置，包括OKX API密钥和URL
- **risk**：风险管理配置，如最大仓位、止损止盈参数等
- **backtest**：回测配置，如初始余额、数据目录等
- **monitoring**：监控配置，如检查间隔、告警阈值等
- **strategy**：策略配置
- **server**：服务器配置，如端口、主机等

### 环境变量配置

除了配置文件外，还需要设置以下环境变量：

- **OKX API 配置**：
  - `OKX_API_KEY`：OKX API密钥
  - `OKX_SECRET_KEY`：OKX API密钥
  - `OKX_PASSPHRASE`：OKX API密钥

- **SmartFilter 配置**：
  - `SMART_FILTER_SOURCE`：数据源类型（file, http, env, auto）
  - `SMART_FILTER_FILE_PATH`：文件数据源路径（默认：data/smart_filter_data.json）
  - `SMART_FILTER_REFRESH_ENABLED`：是否启用自动刷新（默认：true）
  - `SMART_FILTER_REFRESH_INTERVAL`：刷新间隔（默认：5m）

- **CryptoQuant 配置**：
  - `CRYPTOQUANT_API_KEY`：CryptoQuant API密钥（从 https://cryptoquant.com/account/settings 获取）

- **Santiment 配置**（可选）：
  - `SANTIMENT_API_KEY`：Santiment API密钥（从 https://api.santiment.net/ 获取）

## 回测系统使用

1. 准备历史数据

将历史K线数据保存到 `data/` 目录，格式为CSV文件，包含以下字段：
- timestamp：时间戳（毫秒）
- open：开盘价
- high：最高价
- low：最低价
- close：收盘价
- volume：成交量
- interval：时间周期

2. 实现策略

创建一个实现 `backtest.Strategy` 接口的策略文件：

```go
package strategies

import (
	"github.com/yourusername/okx-quant/pkg/types"
)

type MyStrategy struct {
	// 策略参数
}

func (s *MyStrategy) Name() string {
	return "MyStrategy"
}

func (s *MyStrategy) Init(params map[string]interface{}) error {
	// 初始化策略
	return nil
}

func (s *MyStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	// 策略逻辑
	return nil, nil
}

func (s *MyStrategy) GetParameters() map[string]interface{} {
	return nil
}
```

3. 运行回测

在代码中使用回测引擎：

```go
strategy := &strategies.MyStrategy{}
engine := backtest.NewEngine(strategy, 10000)

// 加载历史数据
if err := engine.LoadDataFromFile("data/btc_usdt_1h.csv", "BTC/USDT"); err != nil {
	log.Fatal(err)
}

// 运行回测
if err := engine.Run(); err != nil {
	log.Fatal(err)
}

// 获取回测结果
result := engine.GetResult()
fmt.Printf("回测结果: %+v\n", result)
```

## 监控系统

监控系统会实时监控以下指标：

- 账户余额和权益
- 持仓状态
- 交易历史
- 风险指标
- 系统健康状态

当指标超过阈值时，会通过以下渠道发送告警：

- 控制台
- Webhook

## 风险管理

风险管理模块会检查以下风险：

- 仓位限制：控制单个持仓的最大大小
- 止损止盈：自动设置止损和止盈订单
- 每日亏损限制：控制每日最大亏损
- 最大回撤：控制最大资金回撤
- 交易频率：控制每日最大交易次数

## SmartFilter 使用指南

SmartFilter 是一个基于链上数据的市场状态分析和策略过滤模块，用于评估市场状态并指导交易策略。

### 数据源配置

SmartFilter 支持以下数据源：

1. **CryptoQuant**（推荐）：
   - 免费计划：50 req/day，最高分辨率1天，历史数据7天
   - 配置：设置 `CRYPTOQUANT_API_KEY` 环境变量
   - 运行数据获取脚本：`scripts/fetch_cryptoquant.bat`

2. **Santiment**（可选）：
   - 付费服务，提供更全面的链上数据
   - 配置：设置 `SANTIMENT_API_KEY` 环境变量
   - 运行数据获取脚本：`scripts/fetch_santiment.bat`

3. **文件数据源**：
   - 手动创建或通过脚本生成数据文件
   - 路径：`data/smart_filter_data.json`

4. **环境变量**：
   - 直接通过环境变量设置数据
   - 变量：`SMART_FILTER_NETFLOW`、`SMART_FILTER_SOPR`、`SMART_FILTER_MVRV`

### 数据源配置

SmartFilter 现在直接从 CryptoQuant API 获取数据，无需运行外部脚本。

#### CryptoQuant 配置

1. **注册 CryptoQuant 账号**：
   - 访问 https://cryptoquant.com/
   - 注册免费账号
   - 获取 API Key

2. **配置文件设置**：
   编辑 `configs/config.yaml` 文件，添加以下配置：
   ```yaml
   strategy:
     smart_filter:
       source: "cryptoquant"
       cryptoquant:
         api_key: "your_cryptoquant_api_key"
         asset: "btc"  # 默认资产类型
         assets:  # 多个资产类型（可选）
           - "btc"
           - "eth"
   ```

3. **多资产支持**：
   - 系统默认使用 `asset` 字段指定的资产类型
   - `assets` 字段用于配置多个资产类型，系统会按顺序尝试获取数据
   - 如果某个资产获取失败，系统会自动尝试下一个资产

4. **启动系统**：
   系统会自动从 CryptoQuant API 获取数据，无需运行额外脚本

### 自动数据刷新

SmartFilter 会自动从配置的数据源获取最新数据，默认每5分钟刷新一次。

## 注意事项

1. **API密钥安全**：不要将API密钥提交到版本控制系统
2. **策略测试**：在实盘交易前，一定要通过回测系统充分测试策略
3. **风险控制**：合理设置风险管理参数，避免过度交易和大额亏损
4. **系统监控**：定期检查系统运行状态和监控告警
5. **API限制**：注意CryptoQuant免费计划的限制：
   - 50 req/day API调用限制
   - 最高分辨率：1天
   - 历史数据：7天
   - 仅个人使用
6. **数据缓存**：系统内置1小时缓存机制，以遵守API调用限制

## 许可证

MIT License
