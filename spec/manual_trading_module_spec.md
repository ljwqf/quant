# 手动交易模块规格说明书

## 1. 项目概述

### 1.1 项目背景

在现有OKX量化交易系统的基础上，增加手动交易功能模块，同时集成大语言模型（LLM）API，为用户的手动交易提供智能分析、风险提醒和决策支持。

### 1.2 项目目标

- 提供完整的手动交易功能界面和API
- 集成大语言模型进行交易分析和风险提醒
- 结合技术指标、新闻事件、宏观经济数据进行综合分析
- 在用户进行手动交易时提供实时提醒和建议

### 1.3 目标用户

- 量化交易系统用户
- 需要人工干预和手动交易的交易者
- 希望获得AI辅助决策的交易者

***

## 2. 功能需求

### 2.1 手动交易核心功能

#### 2.1.1 订单管理

| 功能    | 说明          |
| ----- | ----------- |
| 创建市价单 | 立即以当前市场价格成交 |
| 创建限价单 | 以指定价格挂单     |
| 创建条件单 | 价格达到指定条件时触发 |
| 撤销订单  | 撤销未成交订单     |
| 查询订单  | 查看当前订单和历史订单 |

#### 2.1.2 持仓管理

| 功能     | 说明         |
| ------ | ---------- |
| 查看持仓   | 实时查看当前所有持仓 |
| 平仓     | 全部或部分平仓    |
| 调整杠杆   | 调整持仓杠杆倍数   |
| 止盈止损设置 | 为持仓设置止盈止损  |
| 移动止损   | 设置追踪止损     |

#### 2.1.3 市场数据

| 功能   | 说明             |
| ---- | -------------- |
| 实时行情 | 显示最新价格、涨跌幅、成交量 |
| K线图表 | 多种时间周期K线展示     |
| 订单簿  | 买卖盘深度展示        |
| 成交记录 | 最近成交明细         |

### 2.2 大模型分析功能

#### 2.2.1 技术分析

- 基于当前价格走势进行技术指标分析（MA、MACD、RSI、布林带等）
- 识别支撑位和阻力位
- 趋势判断和反转信号识别
- 多时间周期综合分析

#### 2.2.2 新闻与事件分析

- 获取近期加密货币相关新闻
- 分析新闻对市场的潜在影响
- 识别重要事件（项目升级、监管政策等）
- 事件时间线梳理

#### 2.2.3 宏观经济数据分析

- 美联储利率决策分析
- CPI、PPI等经济指标解读
- 美元指数走势影响分析
- 全球金融市场联动分析

#### 2.2.4 交易提醒与建议

- **入场前提醒**: 技术面、基本面风险提示
- **持仓中提醒**: 价格异动、止盈止损提醒
- **平仓建议**: 基于分析的平仓时机建议
- **风险预警**: 极端行情、重大事件预警

### 2.3 数据采集模块

#### 2.3.1 新闻数据源

| 数据源       | 内容类型       | 更新频率 |
| --------- | ---------- | ---- |
| 加密货币新闻API | 行业新闻、项目动态  | 实时   |
| 财经日历      | 宏观经济事件     | 每日   |
| 社交媒体      | 社区情绪、KOL观点 | 实时   |

#### 2.3.2 经济数据来源

- 美联储官网
- 各国统计局
- 财经数据服务商API

#### 2.3.3 技术指标计算

- 基于历史K线数据计算各类技术指标
- 实时更新指标数据
- 指标异常检测

***

## 3. 非功能需求

### 3.1 性能要求

- 订单响应时间 < 500ms
- 大模型分析响应时间 < 10s
- 支持至少10个并发用户
- 行情数据延迟 < 1s

### 3.2 可用性要求

- 系统可用性 ≥ 99.5%
- 数据持久化，故障后可恢复
- 完善的错误处理和重试机制

### 3.3 安全性要求

- API密钥加密存储
- 交易操作需二次确认
- 操作日志完整记录
- 敏感信息脱敏展示

### 3.4 可扩展性

- 模块化设计，易于扩展新功能
- 支持接入多家大模型API
- 支持接入更多新闻数据源

***

## 4. 系统架构设计

### 4.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Web 前端界面                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │  交易面板    │  │  分析面板    │  │  提醒面板    │    │
│  └──────────────┘  └──────────────┘  └──────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      API 服务器层                             │
│  ┌──────────────────┐  ┌──────────────────┐                │
│  │  手动交易API     │  │  分析服务API     │                │
│  └──────────────────┘  └──────────────────┘                │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│   交易所接口   │    │  大模型服务    │    │  数据采集服务  │
│   (OKX)       │    │  (LLM API)    │    │  (新闻/经济)   │
└───────────────┘    └───────────────┘    └───────────────┘
                              │
                              ▼
                    ┌───────────────┐
                    │   数据存储     │
                    │  (交易/分析)   │
                    └───────────────┘
```

### 4.2 模块划分

#### 4.2.1 手动交易模块 (`internal/manualtrading/`)

- `manager.go` - 手动交易管理器主入口
- `order_manager.go` - 订单管理
- `position_manager.go` - 持仓管理
- `market_data.go` - 市场数据

#### 4.2.2 大模型分析模块 (`internal/llmanalysis/`)

- `client.go` - 大模型API客户端（支持OpenAI、Claude、阿里百炼等）
- `providers/openai.go` - OpenAI提供商
- `providers/claude.go` - Claude提供商
- `providers/qwen.go` - 阿里百炼提供商
- `analyzer.go` - 分析引擎
- `prompts.go` - 提示词模板

#### 4.2.3 数据采集模块 (`internal/datacollector/`)

- `news_collector.go` - 新闻采集
- `economic_collector.go` - 经济数据采集
- `technical_indicator.go` - 技术指标计算

#### 4.2.4 提醒服务模块 (`internal/alertservice/`)

- `alert_engine.go` - 提醒引擎
- `notification.go` - 通知发送

#### 4.2.5 数据存储模块 (`internal/storage/`)

- `database.go` - SQLite数据库管理
- `migrations.go` - 数据库迁移
- `repository/manual_trade_repo.go` - 手动交易数据访问
- `repository/ai_analysis_repo.go` - AI分析数据访问
- `repository/news_event_repo.go` - 新闻事件数据访问

***

## 5. API 设计

### 5.1 手动交易 API

#### 5.1.1 创建订单

```
POST /api/manual/order
{
  "symbol": "BTC-USDT-SWAP",
  "side": "buy",
  "type": "market",
  "size": 0.1,
  "price": null,
  "leverage": 10
}
```

#### 5.1.2 撤销订单

```
DELETE /api/manual/order/{order_id}
```

#### 5.1.3 查询订单

```
GET /api/manual/orders?status=open
```

#### 5.1.4 平仓

```
POST /api/manual/position/close
{
  "symbol": "BTC-USDT-SWAP",
  "size": 0.1
}
```

#### 5.1.5 设置止盈止损

```
POST /api/manual/position/tp-sl
{
  "symbol": "BTC-USDT-SWAP",
  "take_profit": 50000,
  "stop_loss": 48000
}
```

### 5.2 分析服务 API

#### 5.2.1 获取交易分析

```
POST /api/analysis/trade
{
  "symbol": "BTC-USDT-SWAP",
  "side": "buy",
  "size": 0.1
}

Response:
{
  "analysis": "基于当前技术分析...",
  "risk_level": "medium",
  "suggestions": [...],
  "warnings": [...]
}
```

#### 5.2.2 获取持仓分析

```
GET /api/analysis/position/{symbol}
```

#### 5.2.3 获取市场概览

```
GET /api/analysis/market-overview
```

#### 5.2.4 获取新闻和事件

```
GET /api/news?limit=20
```

### 5.3 WebSocket 事件

#### 5.3.1 行情更新

```json
{
  "type": "ticker",
  "data": {
    "symbol": "BTC-USDT-SWAP",
    "price": 49500.0,
    "change_24h": 2.5
  }
}
```

#### 5.3.2 AI 提醒

```json
{
  "type": "ai_alert",
  "data": {
    "level": "warning",
    "title": "价格突破关键阻力位",
    "message": "BTC价格突破50000美元整数关口...",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

***

## 6. 数据库设计 (SQLite)

### 6.1 数据库表设计

#### 6.1.1 手动交易记录表 (`manual_trades`)

| 字段                    | 类型       | 约束                                  | 说明                                                       |
| --------------------- | -------- | ----------------------------------- | -------------------------------------------------------- |
| id                    | INTEGER  | PRIMARY KEY AUTOINCREMENT           | 主键                                                       |
| order\_id             | TEXT     | NOT NULL                            | 交易所订单ID                                                  |
| symbol                | TEXT     | NOT NULL                            | 交易对                                                      |
| side                  | TEXT     | NOT NULL                            | 买卖方向 (buy/sell)                                          |
| type                  | TEXT     | NOT NULL                            | 订单类型 (market/limit)                                      |
| price                 | DECIMAL  | <br />                              | 价格                                                       |
| size                  | DECIMAL  | NOT NULL                            | 数量                                                       |
| filled\_size          | DECIMAL  | DEFAULT 0                           | 已成交数量                                                    |
| status                | TEXT     | NOT NULL                            | 状态 (pending/partially\_filled/filled/cancelled/rejected) |
| leverage              | INTEGER  | DEFAULT 1                           | 杠杆倍数                                                     |
| take\_profit          | DECIMAL  | <br />                              | 止盈价格                                                     |
| stop\_loss            | DECIMAL  | <br />                              | 止损价格                                                     |
| ai\_analysis\_id      | INTEGER  | <br />                              | 关联的AI分析ID                                                |
| ai\_analysis\_summary | TEXT     | <br />                              | AI分析摘要                                                   |
| created\_at           | DATETIME | NOT NULL DEFAULT CURRENT\_TIMESTAMP | 创建时间                                                     |
| updated\_at           | DATETIME | NOT NULL DEFAULT CURRENT\_TIMESTAMP | 更新时间                                                     |

**索引:**

- `idx_manual_trades_order_id` ON `manual_trades`(`order_id`)
- `idx_manual_trades_symbol` ON `manual_trades`(`symbol`)
- `idx_manual_trades_status` ON `manual_trades`(`status`)
- `idx_manual_trades_created_at` ON `manual_trades`(`created_at`)

#### 6.1.2 AI 分析记录表 (`ai_analyses`)

| 字段                 | 类型       | 约束                                  | 说明                                                         |
| ------------------ | -------- | ----------------------------------- | ---------------------------------------------------------- |
| id                 | INTEGER  | PRIMARY KEY AUTOINCREMENT           | 主键                                                         |
| symbol             | TEXT     | <br />                              | 交易对                                                        |
| analysis\_type     | TEXT     | NOT NULL                            | 分析类型 (trade\_precheck/position\_overview/market\_overview) |
| provider           | TEXT     | NOT NULL                            | LLM提供商 (openai/claude/qwen)                                |
| model              | TEXT     | NOT NULL                            | 使用的模型                                                      |
| prompt             | TEXT     | <br />                              | 发送的提示词                                                     |
| content            | TEXT     | NOT NULL                            | 分析内容                                                       |
| risk\_level        | TEXT     | NOT NULL                            | 风险等级 (low/medium/high/critical)                            |
| suggestions        | TEXT     | <br />                              | 建议 (JSON格式)                                                |
| warnings           | TEXT     | <br />                              | 警告 (JSON格式)                                                |
| confidence\_score  | DECIMAL  | <br />                              | 置信度 (0-1)                                                  |
| prompt\_tokens     | INTEGER  | <br />                              | 提示词token数                                                  |
| completion\_tokens | INTEGER  | <br />                              | 完成token数                                                   |
| total\_tokens      | INTEGER  | <br />                              | 总token数                                                    |
| latency\_ms        | INTEGER  | <br />                              | 响应延迟(毫秒)                                                   |
| created\_at        | DATETIME | NOT NULL DEFAULT CURRENT\_TIMESTAMP | 创建时间                                                       |

**索引:**

- `idx_ai_analyses_symbol` ON `ai_analyses`(`symbol`)
- `idx_ai_analyses_type` ON `ai_analyses`(`analysis_type`)
- `idx_ai_analyses_provider` ON `ai_analyses`(`provider`)
- `idx_ai_analyses_created_at` ON `ai_analyses`(`created_at`)

#### 6.1.3 新闻事件表 (`news_events`)

| 字段               | 类型       | 约束                                  | 说明                             |
| ---------------- | -------- | ----------------------------------- | ------------------------------ |
| id               | INTEGER  | PRIMARY KEY AUTOINCREMENT           | 主键                             |
| external\_id     | TEXT     | <br />                              | 外部ID                           |
| title            | TEXT     | NOT NULL                            | 标题                             |
| summary          | TEXT     | <br />                              | 摘要                             |
| content          | TEXT     | <br />                              | 内容                             |
| source           | TEXT     | NOT NULL                            | 来源 (cryptopanic/coindesk/rss)  |
| url              | TEXT     | <br />                              | 原文链接                           |
| image\_url       | TEXT     | <br />                              | 图片链接                           |
| category         | TEXT     | <br />                              | 分类                             |
| tags             | TEXT     | <br />                              | 标签 (逗号分隔)                      |
| importance       | INTEGER  | NOT NULL DEFAULT 1                  | 重要程度 (1-5)                     |
| sentiment        | TEXT     | <br />                              | 情绪 (positive/neutral/negative) |
| related\_symbols | TEXT     | <br />                              | 相关交易对 (逗号分隔)                   |
| published\_at    | DATETIME | NOT NULL                            | 发布时间                           |
| created\_at      | DATETIME | NOT NULL DEFAULT CURRENT\_TIMESTAMP | 抓取时间                           |

**索引:**

- `idx_news_events_external_id` ON `news_events`(`external_id`)
- `idx_news_events_source` ON `news_events`(`source`)
- `idx_news_events_importance` ON `news_events`(`importance`)
- `idx_news_events_published_at` ON `news_events`(`published_at`)

#### 6.1.4 经济事件表 (`economic_events`)

| 字段           | 类型       | 约束                                  | 说明                     |
| ------------ | -------- | ----------------------------------- | ---------------------- |
| id           | INTEGER  | PRIMARY KEY AUTOINCREMENT           | 主键                     |
| external\_id | TEXT     | <br />                              | 外部ID                   |
| title        | TEXT     | NOT NULL                            | 事件标题                   |
| country      | TEXT     | <br />                              | 国家                     |
| currency     | TEXT     | <br />                              | 货币                     |
| indicator    | TEXT     | <br />                              | 经济指标                   |
| actual       | DECIMAL  | <br />                              | 实际值                    |
| forecast     | DECIMAL  | <br />                              | 预测值                    |
| previous     | DECIMAL  | <br />                              | 前值                     |
| unit         | TEXT     | <br />                              | 单位                     |
| importance   | INTEGER  | NOT NULL DEFAULT 1                  | 重要程度 (1-3)             |
| impact       | TEXT     | <br />                              | 影响程度 (low/medium/high) |
| event\_time  | DATETIME | NOT NULL                            | 事件时间                   |
| created\_at  | DATETIME | NOT NULL DEFAULT CURRENT\_TIMESTAMP | 创建时间                   |

**索引:**

- `idx_economic_events_external_id` ON `economic_events`(`external_id`)
- `idx_economic_events_country` ON `economic_events`(`country`)
- `idx_economic_events_importance` ON `economic_events`(`importance`)
- `idx_economic_events_event_time` ON `economic_events`(`event_time`)

#### 6.1.5 提醒记录表 (`alert_records`)

| 字段          | 类型       | 约束                                  | 说明                                                     |
| ----------- | -------- | ----------------------------------- | ------------------------------------------------------ |
| id          | INTEGER  | PRIMARY KEY AUTOINCREMENT           | 主键                                                     |
| alert\_type | TEXT     | NOT NULL                            | 提醒类型 (price\_move/indicator\_break/news\_event/tp\_sl) |
| level       | TEXT     | NOT NULL                            | 级别 (info/warning/error/critical)                       |
| title       | TEXT     | NOT NULL                            | 标题                                                     |
| message     | TEXT     | NOT NULL                            | 消息内容                                                   |
| symbol      | TEXT     | <br />                              | 相关交易对                                                  |
| metadata    | TEXT     | <br />                              | 元数据 (JSON格式)                                           |
| channels    | TEXT     | NOT NULL                            | 发送渠道 (逗号分隔)                                            |
| read        | BOOLEAN  | NOT NULL DEFAULT 0                  | 是否已读                                                   |
| created\_at | DATETIME | NOT NULL DEFAULT CURRENT\_TIMESTAMP | 创建时间                                                   |

**索引:**

- `idx_alert_records_type` ON `alert_records`(`alert_type`)
- `idx_alert_records_level` ON `alert_records`(`level`)
- `idx_alert_records_symbol` ON `alert_records`(`symbol`)
- `idx_alert_records_read` ON `alert_records`(`read`)
- `idx_alert_records_created_at` ON `alert_records`(`created_at`)

### 6.2 数据库迁移脚本示例

```sql
-- migration 001_initial_schema.up.sql
CREATE TABLE IF NOT EXISTS manual_trades (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    type TEXT NOT NULL,
    price DECIMAL,
    size DECIMAL NOT NULL,
    filled_size DECIMAL DEFAULT 0,
    status TEXT NOT NULL,
    leverage INTEGER DEFAULT 1,
    take_profit DECIMAL,
    stop_loss DECIMAL,
    ai_analysis_id INTEGER,
    ai_analysis_summary TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_manual_trades_order_id ON manual_trades(order_id);
CREATE INDEX idx_manual_trades_symbol ON manual_trades(symbol);
CREATE INDEX idx_manual_trades_status ON manual_trades(status);
CREATE INDEX idx_manual_trades_created_at ON manual_trades(created_at);

-- ... 创建其他表 ...
```

***

## 7. 配置管理

### 7.1 配置项 (`config.yaml`)

```yaml
database:
  enable: true
  type: "sqlite"
  path: "./data/quant.db"

manual_trading:
  enable: true
  risk_check: true
  order_confirmation: true

llm:
  enable: true
  provider: "openai"  # openai / claude / qwen
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      base_url: "https://api.openai.com/v1"
      model: "gpt-4"
      temperature: 0.7
      max_tokens: 2000
    claude:
      api_key: "${CLAUDE_API_KEY}"
      base_url: "https://api.anthropic.com"
      model: "claude-3-opus-20240229"
      temperature: 0.7
      max_tokens: 2000
    qwen:
      api_key: "${QWEN_API_KEY}"
      base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      model: "qwen-max"
      temperature: 0.7
      max_tokens: 2000
  timeout: 30s

data_collector:
  news:
    enable: true
    sources:
      - "cryptopanic"
      - "coindesk"
    refresh_interval: 5m
  economic:
    enable: true
    refresh_interval: 1h

alert:
  enable: true
  channels:
    - "web"
    - "webhook"
  price_change_threshold: 0.05
  check_interval: 1m
```

***

## 8. 实施计划

### 8.1 阶段一：数据库与基础架构 (Week 1)

- [ ] SQLite数据库设计和迁移
- [ ] 数据访问层实现
- [ ] 配置管理更新
- [ ] 手动交易模块基础架构

### 8.2 阶段二：核心手动交易功能 (Week 2)

- [ ] 订单管理功能
- [ ] 持仓管理功能
- [ ] API接口实现
- [ ] Web界面交易面板

### 8.3 阶段三：大模型集成 (Week 3-4)

- [ ] 大模型客户端框架（支持多提供商）
- [ ] OpenAI提供商实现
- [ ] Claude提供商实现
- [ ] 阿里百炼提供商实现
- [ ] 提示词模板设计
- [ ] 分析引擎实现
- [ ] API接口和Web界面

### 8.4 阶段四：数据采集 (Week 5)

- [ ] 新闻数据源集成
- [ ] 经济数据采集
- [ ] 技术指标计算
- [ ] 数据存储和缓存

### 8.5 阶段五：提醒服务 (Week 6)

- [ ] 提醒引擎实现
- [ ] WebSocket实时推送
- [ ] 多渠道通知

### 8.6 阶段六：测试优化 (Week 7)

- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能优化
- [ ] 用户体验优化

***

## 9. 风险与应对

| 风险         | 影响 | 概率 | 应对措施           |
| ---------- | -- | -- | -------------- |
| 大模型API费用过高 | 高  | 中  | 实现请求缓存、限制调用频率  |
| 大模型响应延迟    | 中  | 中  | 异步处理、超时机制、缓存   |
| 新闻数据准确性    | 中  | 低  | 多源验证、人工审核标记    |
| 系统安全性      | 高  | 低  | 权限控制、操作审计、加密存储 |

***

## 10. 验收标准

### 10.1 功能验收

- [ ] 能够成功创建市价单和限价单
- [ ] 能够查看和管理持仓
- [ ] 大模型能够返回有意义的分析结果
- [ ] 能够正常获取和展示新闻
- [ ] 提醒功能正常触发

### 10.2 性能验收

- [ ] 订单创建响应时间 < 500ms
- [ ] 大模型分析响应时间 < 10s (95%请求)
- [ ] 支持10个并发用户无明显卡顿

### 10.3 质量验收

- [ ] 单元测试覆盖率 ≥ 80%
- [ ] 无严重bug
- [ ] 代码符合项目规范

