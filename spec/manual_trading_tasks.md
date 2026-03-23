# 手动交易模块实施任务列表

## 阶段一：数据库与基础架构 (Week 1)

### 1.1 数据库设计与实现
- [ ] 设计 SQLite 数据库 schema
- [ ] 创建 `internal/storage/` 目录
- [ ] 创建 `internal/storage/database.go` - SQLite 数据库管理
- [ ] 创建 `internal/storage/migrations.go` - 数据库迁移脚本
- [ ] 创建 `internal/storage/repository/manual_trade_repo.go` - 手动交易数据访问
- [ ] 创建 `internal/storage/repository/ai_analysis_repo.go` - AI 分析数据访问
- [ ] 创建 `internal/storage/repository/news_event_repo.go` - 新闻事件数据访问

### 1.2 配置管理
- [ ] 在 `internal/config/config.go` 中添加数据库配置结构体
- [ ] 在 `internal/config/config.go` 中添加 LLM 多提供商配置结构体
- [ ] 在 `internal/config/config.go` 中添加手动交易、数据采集、提醒服务配置
- [ ] 在 `configs/config.yaml` 中添加相应的配置项
- [ ] 添加配置验证逻辑

### 1.3 手动交易模块基础架构
- [ ] 创建 `internal/manualtrading/` 目录
- [ ] 创建 `internal/manualtrading/manager.go` - 手动交易管理器主入口
- [ ] 创建 `internal/manualtrading/order_manager.go` - 订单管理
- [ ] 创建 `internal/manualtrading/position_manager.go` - 持仓管理
- [ ] 创建 `internal/manualtrading/market_data.go` - 市场数据

---

## 阶段二：核心手动交易功能 (Week 2)

### 2.1 API 接口实现
- [ ] 在 `internal/api/server.go` 中添加手动交易相关路由
- [ ] 实现创建订单接口 (`POST /api/manual/order`)
- [ ] 实现撤销订单接口 (`DELETE /api/manual/order/{order_id}`)
- [ ] 实现查询订单接口 (`GET /api/manual/orders`)
- [ ] 实现平仓接口 (`POST /api/manual/position/close`)
- [ ] 实现设置止盈止损接口 (`POST /api/manual/position/tp-sl`)

### 2.2 Web 界面更新
- [ ] 增强 `web/index.html` 中的手动交易面板
- [ ] 添加市场数据展示区域
- [ ] 添加持仓管理界面
- [ ] 添加订单历史界面
- [ ] 更新 `web/static/js/app.js` 添加手动交易相关逻辑
- [ ] 更新 `web/static/css/style.css` 添加样式

### 2.3 集成到主程序
- [ ] 在 `cmd/trader/main.go` 中初始化数据库
- [ ] 在 `cmd/trader/main.go` 中初始化手动交易模块
- [ ] 注册手动交易 API 处理器
- [ ] 连接到现有交易所客户端

---

## 阶段三：大模型集成 (Week 3-4)

### 3.1 大模型客户端框架
- [ ] 创建 `internal/llmanalysis/` 目录
- [ ] 创建 `internal/llmanalysis/client.go` - 大模型 API 客户端统一接口
- [ ] 创建 `internal/llmanalysis/providers/` 目录
- [ ] 创建 `internal/llmanalysis/providers/provider.go` - 提供商接口定义
- [ ] 实现请求重试和错误处理
- [ ] 实现请求缓存机制

### 3.2 各提供商实现
- [ ] 创建 `internal/llmanalysis/providers/openai.go` - OpenAI 提供商
- [ ] 创建 `internal/llmanalysis/providers/claude.go` - Claude (Anthropic) 提供商
- [ ] 创建 `internal/llmanalysis/providers/qwen.go` - 阿里百炼 (Qwen) 提供商

### 3.3 提示词模板
- [ ] 创建 `internal/llmanalysis/prompts.go`
- [ ] 技术分析提示词模板
- [ ] 新闻事件分析提示词模板
- [ ] 宏观经济数据分析提示词模板
- [ ] 交易决策建议提示词模板

### 3.4 分析引擎
- [ ] 创建 `internal/llmanalysis/analyzer.go`
- [ ] 实现交易前分析功能
- [ ] 实现持仓分析功能
- [ ] 实现市场概览分析功能
- [ ] 集成技术指标计算

### 3.5 API 接口
- [ ] 添加分析服务 API 路由
- [ ] 实现获取交易分析接口 (`POST /api/analysis/trade`)
- [ ] 实现获取持仓分析接口 (`GET /api/analysis/position/{symbol}`)
- [ ] 实现获取市场概览接口 (`GET /api/analysis/market-overview`)

### 3.6 Web 界面
- [ ] 添加 AI 分析面板
- [ ] 展示分析结果
- [ ] 显示风险等级和建议

---

## 阶段四：数据采集 (Week 5)

### 4.1 数据采集模块
- [ ] 创建 `internal/datacollector/` 目录
- [ ] 创建 `internal/datacollector/news_collector.go`
- [ ] 创建 `internal/datacollector/economic_collector.go`
- [ ] 创建 `internal/datacollector/technical_indicator.go`

### 4.2 新闻数据源集成
- [ ] 集成 CryptoPanic API
- [ ] 集成 CoinDesk RSS
- [ ] 实现新闻数据存储（SQLite）
- [ ] 实现新闻重要性评分

### 4.3 经济数据采集
- [ ] 集成财经日历 API
- [ ] 实现经济数据存储（SQLite）
- [ ] 实现事件提醒

### 4.4 技术指标计算
- [ ] 实现 MA、MACD、RSI、布林带等指标计算
- [ ] 实时更新指标数据
- [ ] 指标异常检测

### 4.5 API 接口
- [ ] 实现获取新闻接口 (`GET /api/news`)

---

## 阶段五：提醒服务 (Week 6)

### 5.1 提醒服务模块
- [ ] 创建 `internal/alertservice/` 目录
- [ ] 创建 `internal/alertservice/alert_engine.go`
- [ ] 创建 `internal/alertservice/notification.go`

### 5.2 提醒引擎
- [ ] 实现价格异动提醒
- [ ] 实现技术指标突破提醒
- [ ] 实现新闻事件提醒
- [ ] 实现止盈止损提醒

### 5.3 WebSocket 推送
- [ ] 在 `internal/api/websocket.go` 中添加 AI 提醒事件
- [ ] 实现实时推送

### 5.4 多渠道通知
- [ ] Web 界面通知
- [ ] Webhook 通知
- [ ] （可选）邮件/短信通知

---

## 阶段六：测试优化 (Week 7)

### 6.1 单元测试
- [ ] 数据存储模块单元测试
- [ ] 手动交易模块单元测试
- [ ] LLM 分析模块单元测试
- [ ] 数据采集模块单元测试
- [ ] 提醒服务模块单元测试

### 6.2 集成测试
- [ ] 完整手动交易流程测试
- [ ] 大模型分析集成测试（各提供商）
- [ ] 端到端测试

### 6.3 性能优化
- [ ] API 响应时间优化
- [ ] 数据库查询优化
- [ ] 缓存策略优化

### 6.4 用户体验优化
- [ ] 界面交互优化
- [ ] 加载状态提示
- [ ] 错误提示优化

---

## 依赖管理

### Go 依赖
- [ ] SQLite 驱动 (modernc.org/sqlite)
- [ ] SQL 构建器 (可选，如 github.com/Masterminds/squirrel)
- [ ] OpenAI SDK 或通用 HTTP 客户端
- [ ] 新闻 API 客户端
