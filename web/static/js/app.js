// WebSocket连接
let ws = null;
let reconnectTimer = null;
let rebalanceEvents = [];

// 图表实例
let pnlChart = null;
let strategyPerformanceChart = null;
let priceChart = null;
let indicatorChart = null;

// 图表数据
let pnlChartData = {
    labels: [],
    datasets: [{
        label: '累计盈亏 (USDT)',
        data: [],
        borderColor: '#00d4ff',
        backgroundColor: 'rgba(0, 212, 255, 0.1)',
        fill: true,
        tension: 0.4,
        pointRadius: 3,
        pointHoverRadius: 5
    }]
};

let strategyPerformanceData = {
    labels: [],
    datasets: [{
        label: '盈亏 (USDT)',
        data: [],
        backgroundColor: [
            'rgba(0, 212, 255, 0.7)',
            'rgba(124, 58, 237, 0.7)',
            'rgba(0, 255, 136, 0.7)',
            'rgba(255, 193, 7, 0.7)',
            'rgba(255, 68, 68, 0.7)'
        ],
        borderColor: [
            '#00d4ff',
            '#7c3aed',
            '#00ff88',
            '#ffc107',
            '#ff4444'
        ],
        borderWidth: 2
    }]
};

// 技术指标图表数据
let priceChartData = {
    labels: [],
    datasets: [{
        label: '价格',
        data: [],
        borderColor: '#00ff88',
        backgroundColor: 'rgba(0, 255, 136, 0.1)',
        fill: true,
        tension: 0.4,
        pointRadius: 0,
        pointHoverRadius: 5
    }]
};

let indicatorChartData = {
    labels: [],
    datasets: []
};

// 技术指标参数配置
const indicatorParamsConfig = {
    MACD: [
        { name: 'fastPeriod', label: '快线周期', type: 'number', min: 1, max: 100, default: 12 },
        { name: 'slowPeriod', label: '慢线周期', type: 'number', min: 1, max: 200, default: 26 },
        { name: 'signalPeriod', label: '信号线周期', type: 'number', min: 1, max: 100, default: 9 }
    ],
    RSI: [
        { name: 'period', label: '周期', type: 'number', min: 1, max: 100, default: 14 }
    ],
    BOLL: [
        { name: 'period', label: '周期', type: 'number', min: 1, max: 100, default: 20 },
        { name: 'deviation', label: '标准差倍数', type: 'number', min: 0.1, max: 5, step: 0.1, default: 2 }
    ],
    ATR: [
        { name: 'period', label: '周期', type: 'number', min: 1, max: 100, default: 14 }
    ],
    ADX: [
        { name: 'period', label: '周期', type: 'number', min: 1, max: 100, default: 14 }
    ],
    PRICE: []
};

// 市场数据状态
let marketData = {
    tickers: {},
    klines: {},
    orderBooks: {}
};

const MAX_REBALANCE_EVENTS = 12;

// LLM 分析相关状态
let llmLoading = false;

function getApiToken() {
    return localStorage.getItem('apiToken') || new URLSearchParams(window.location.search).get('token') || '';
}

function buildAuthHeaders(extraHeaders = {}) {
    const headers = { ...extraHeaders };
    const token = getApiToken();
    if (token) {
        headers['X-API-Token'] = token;
    }
    return headers;
}

// 初始化
document.addEventListener('DOMContentLoaded', function() {
    initTheme();
    initCharts();
    connectWebSocket();
    fetchInitialData();
    updateTime();
    setInterval(updateTime, 1000);
    // 定期更新图表数据（模拟）
    setInterval(updateChartData, 5000);
});

// 初始化图表
function initCharts() {
    const isDarkMode = document.documentElement.getAttribute('data-theme') === 'dark';
    const textColor = isDarkMode ? '#e0e0e0' : '#333';
    const gridColor = isDarkMode ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.1)';
    
    // 初始化盈亏走势图
    const pnlCtx = document.getElementById('pnl-chart');
    if (pnlCtx) {
        pnlChart = new Chart(pnlCtx, {
            type: 'line',
            data: pnlChartData,
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: textColor
                        }
                    }
                },
                scales: {
                    x: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    },
                    y: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    }
                }
            }
        });
    }
    
    // 初始化策略性能对比图
    const strategyCtx = document.getElementById('strategy-performance-chart');
    if (strategyCtx) {
        strategyPerformanceChart = new Chart(strategyCtx, {
            type: 'bar',
            data: strategyPerformanceData,
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: textColor
                        }
                    }
                },
                scales: {
                    x: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    },
                    y: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    }
                }
            }
        });
    }
    
    // 初始化价格走势图
    const priceCtx = document.getElementById('price-chart');
    if (priceCtx) {
        priceChart = new Chart(priceCtx, {
            type: 'line',
            data: priceChartData,
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: textColor
                        }
                    }
                },
                scales: {
                    x: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    },
                    y: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    }
                }
            }
        });
    }
    
    // 初始化技术指标图
    const indicatorCtx = document.getElementById('indicator-chart');
    if (indicatorCtx) {
        indicatorChart = new Chart(indicatorCtx, {
            type: 'line',
            data: indicatorChartData,
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        labels: {
                            color: textColor
                        }
                    }
                },
                scales: {
                    x: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    },
                    y: {
                        ticks: {
                            color: textColor
                        },
                        grid: {
                            color: gridColor
                        }
                    }
                }
            }
        });
    }
    
    // 初始化技术指标参数
    updateIndicatorParams();
    
    // 生成初始模拟数据
    generateInitialChartData();
    
    // 生成初始技术指标数据
    generateInitialIndicatorData();
}

// 生成初始图表数据
function generateInitialChartData() {
    // 盈亏走势图数据
    const now = new Date();
    let cumulativePnl = 0;
    for (let i = 30; i >= 0; i--) {
        const date = new Date(now);
        date.setDate(date.getDate() - i);
        const change = (Math.random() - 0.45) * 100;
        cumulativePnl += change;
        pnlChartData.labels.push(date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' }));
        pnlChartData.datasets[0].data.push(Math.round(cumulativePnl * 100) / 100);
    }
    
    // 策略性能数据
    const strategies = ['趋势跟踪', '均值回归', '网格交易', '套利策略', '机器学习'];
    strategies.forEach((name, index) => {
        strategyPerformanceData.labels.push(name);
        strategyPerformanceData.datasets[0].data.push(Math.round((Math.random() - 0.3) * 500 * 100) / 100);
    });
    
    // 更新图表
    if (pnlChart) pnlChart.update();
    if (strategyPerformanceChart) strategyPerformanceChart.update();
}

// 更新图表数据
function updateChartData() {
    // 更新盈亏走势图
    const now = new Date();
    const lastValue = pnlChartData.datasets[0].data[pnlChartData.datasets[0].data.length - 1] || 0;
    const change = (Math.random() - 0.45) * 20;
    const newValue = lastValue + change;
    
    pnlChartData.labels.push(now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }));
    pnlChartData.datasets[0].data.push(Math.round(newValue * 100) / 100);
    
    // 保持数据点数量在50个以内
    if (pnlChartData.labels.length > 50) {
        pnlChartData.labels.shift();
        pnlChartData.datasets[0].data.shift();
    }
    
    // 更新图表
    if (pnlChart) pnlChart.update();
}

// 主题切换时更新图表颜色
function updateChartTheme() {
    if (!pnlChart && !strategyPerformanceChart) return;
    
    const isDarkMode = document.documentElement.getAttribute('data-theme') === 'dark';
    const textColor = isDarkMode ? '#e0e0e0' : '#333';
    const gridColor = isDarkMode ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.1)';
    
    if (pnlChart) {
        pnlChart.options.plugins.legend.labels.color = textColor;
        pnlChart.options.scales.x.ticks.color = textColor;
        pnlChart.options.scales.y.ticks.color = textColor;
        pnlChart.options.scales.x.grid.color = gridColor;
        pnlChart.options.scales.y.grid.color = gridColor;
        pnlChart.update();
    }
    
    if (strategyPerformanceChart) {
        strategyPerformanceChart.options.plugins.legend.labels.color = textColor;
        strategyPerformanceChart.options.scales.x.ticks.color = textColor;
        strategyPerformanceChart.options.scales.y.ticks.color = textColor;
        strategyPerformanceChart.options.scales.x.grid.color = gridColor;
        strategyPerformanceChart.options.scales.y.grid.color = gridColor;
        strategyPerformanceChart.update();
    }
}

// 主题初始化
function initTheme() {
    const savedTheme = localStorage.getItem('theme') || 'dark';
    applyTheme(savedTheme);
    const themeToggleBtn = document.getElementById('theme-toggle');
    if (themeToggleBtn) {
        themeToggleBtn.addEventListener('click', toggleTheme);
    }
}

// 切换主题
function toggleTheme() {
    const currentTheme = document.documentElement.getAttribute('data-theme') || 'dark';
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
    applyTheme(newTheme);
    localStorage.setItem('theme', newTheme);
    updateChartTheme();
}

// 应用主题
function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    const themeIcon = document.querySelector('.theme-icon');
    if (themeIcon) {
        themeIcon.textContent = theme === 'dark' ? '☀️' : '🌙';
    }
}

// 连接WebSocket
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = getApiToken();
    const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws${suffix}`);

    ws.onopen = function() {
        console.log('WebSocket已连接');
        updateConnectionStatus(true);
        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
        // 订阅市场数据事件
        subscribeToMarketData();
    };

    ws.onclose = function() {
        console.log('WebSocket已断开');
        updateConnectionStatus(false);
        // 自动重连
        if (!reconnectTimer) {
            reconnectTimer = setTimeout(connectWebSocket, 3000);
        }
    };

    ws.onerror = function(error) {
        console.error('WebSocket错误:', error);
    };

    ws.onmessage = function(event) {
        const message = JSON.parse(event.data);
        handleMessage(message);
    };
}

// 处理消息
function handleMessage(message) {
    switch (message.type) {
        case 'strategy':
            updateStrategyStatus(message.data);
            break;
        case 'positions':
            updatePositions(message.data);
            break;
        case 'order':
            addOrder(message.data);
            break;
        case 'signal':
            addSignal(message.data);
            break;
        case 'rebalance_circuit':
            updateRebalanceCircuit(message.data);
            break;
        case 'rebalance_circuit_reset':
            handleRebalanceCircuitResetEvent(message.data);
            break;
        case 'rebalance_event':
            handleRebalanceEvent(message.data);
            break;
        // 新增消息类型
        case 'alert':
            handleAlert(message.data);
            break;
        case 'position_change':
            handlePositionChange(message.data);
            break;
        case 'order_update':
            handleOrderUpdate(message.data);
            break;
        case 'conditional_order':
            handleConditionalOrderEvent(message.data);
            break;
        case 'subscription':
            console.log('订阅确认:', message.data);
            break;
        // 市场数据消息
        case 'ticker':
            handleTicker(message.data);
            break;
        case 'kline':
            handleKline(message.data);
            break;
        case 'orderbook':
            handleOrderBook(message.data);
            break;
        case 'status':
            handleSystemStatus(message.data);
            break;
    }
}

// 处理提醒消息
function handleAlert(data) {
    if (!data) return;

    const levelMap = {
        'info': 'info',
        'warning': 'warning',
        'error': 'error',
        'critical': 'error'
    };

    const level = levelMap[data.level] || 'info';
    const source = data.source || 'system';
    const message = `[${source}] ${data.message}`;

    showRuntimeNotice(message, level);
    console.log('收到提醒:', data);
}

// 处理持仓变化消息
function handlePositionChange(data) {
    if (!data) return;

    fetch('/api/positions').then(r => r.json()).then(updatePositions);

    const changeTypeMap = {
        'open': '开仓',
        'close': '平仓',
        'update': '更新'
    };

    const changeType = changeTypeMap[data.change_type] || data.change_type;
    const message = `${data.symbol} ${changeType}: ${data.size}`;
    showRuntimeNotice(message, data.change_type === 'close' && data.pnl < 0 ? 'error' : 'info');
}

// 处理订单更新消息
function handleOrderUpdate(data) {
    if (!data) return;

    fetch('/api/orders').then(r => r.json()).then(updateOrders);

    const statusMap = {
        'pending': '待成交',
        'filled': '已成交',
        'partially': '部分成交',
        'cancelled': '已取消',
        'failed': '失败'
    };

    const status = statusMap[data.status] || data.status;
    const message = `${data.symbol} 订单 ${data.order_id} ${status}`;
    showRuntimeNotice(message, data.status === 'filled' ? 'success' : (data.status === 'failed' ? 'error' : 'info'));
}

// 获取初始数据
async function fetchInitialData() {
    try {
        const [status, strategies, positions, orders, signals, circuit, events] = await Promise.all([
            fetch('/api/status').then(r => r.json()),
            fetch('/api/strategies').then(r => r.json()),
            fetch('/api/positions').then(r => r.json()),
            fetch('/api/orders').then(r => r.json()),
            fetch('/api/signals').then(r => r.json()),
            fetch('/api/rebalance/circuit').then(r => r.ok ? r.json() : null),
            fetch('/api/rebalance/events').then(r => r.ok ? r.json() : [])
        ]);

        updateSystemStatus(status);
        updateStrategies(strategies);
        updatePositions(positions);
        updateOrders(orders);
        updateSignals(signals);
        updateRebalanceCircuit(circuit);
        hydrateRebalanceEvents(events);
    } catch (error) {
        console.error('获取数据失败:', error);
    }
}

// 更新连接状态
function updateConnectionStatus(connected) {
    const indicator = document.getElementById('connection-status');
    if (!indicator) return;
    if (connected) {
        indicator.textContent = '已连接';
        indicator.classList.remove('disconnected');
        indicator.classList.add('connected');
    } else {
        indicator.textContent = '未连接';
        indicator.classList.remove('connected');
        indicator.classList.add('disconnected');
    }
}

// 更新时间
function updateTime() {
    const now = new Date();
    document.getElementById('server-time').textContent = now.toLocaleString('zh-CN');
}

// 更新系统状态
function updateSystemStatus(data) {
    setValue('account-balance', formatNumber(data.account_balance));
    setValue('total-pnl', formatNumber(data.total_pnl), data.total_pnl >= 0 ? 'positive' : 'negative');
    setValue('daily-pnl', formatNumber(data.daily_pnl), data.daily_pnl >= 0 ? 'positive' : 'negative');
    setValue('win-rate', formatPercent(data.win_rate));
    setValue('total-trades', data.total_trades);
    setValue('uptime', data.uptime || '--');
    
    // 根据交易所连接状态更新显示
    updateExchangeConnectionStatus(data.exchange_connected);
}

// 更新交易所连接状态
function updateExchangeConnectionStatus(connected) {
    const indicator = document.getElementById('connection-status');
    if (connected) {
        indicator.textContent = '已连接';
        indicator.classList.remove('disconnected');
        indicator.classList.add('connected');
    } else {
        indicator.textContent = '未连接';
        indicator.classList.remove('connected');
        indicator.classList.add('disconnected');
    }
}

// 当前正在配置的策略
let currentConfigStrategy = null;

// 更新策略列表
function updateStrategies(strategies) {
    const tbody = document.getElementById('strategies-body');
    if (!strategies || strategies.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="loading">暂无策略</td></tr>';
        return;
    }

    tbody.innerHTML = strategies.map(s => `
        <tr>
            <td data-label="策略名称">${s.name}</td>
            <td data-label="状态"><span class="badge ${s.running ? 'running' : 'stopped'}">${s.running ? '运行中' : '已停止'}</span></td>
            <td data-label="盈亏" class="${s.pnl >= 0 ? 'positive' : 'negative'}">${formatNumber(s.pnl)}</td>
            <td data-label="胜率">${formatPercent(s.win_rate)}</td>
            <td data-label="交易次数">${s.trades}</td>
            <td data-label="权重">${s.weight ? (s.weight * 100).toFixed(0) + '%' : '--'}</td>
            <td data-label="最近信号">${s.last_signal || '--'}</td>
            <td data-label="操作">
                <button class="btn btn-small ${s.running ? 'btn-danger' : 'btn-success'}" 
                        onclick="toggleStrategy('${s.name}', ${!s.running})">
                    ${s.running ? '停止' : '启动'}
                </button>
                <button class="btn btn-small btn-secondary" 
                        onclick="openStrategyParamPanel('${s.name}')"
                        style="margin-left: 0.5rem;">
                    配置
                </button>
            </td>
        </tr>
    `).join('');
}

// 打开策略参数配置面板
async function openStrategyParamPanel(strategyName) {
    currentConfigStrategy = strategyName;
    const panel = document.getElementById('strategy-param-panel');
    const title = document.getElementById('param-panel-title');
    const content = document.getElementById('strategy-param-content');
    
    title.textContent = `${strategyName} - 参数配置`;
    content.innerHTML = '<div style="text-align: center; padding: 2rem;"><div class="loading-spinner" style="margin: 0 auto;"></div></div>';
    panel.classList.remove('hidden');
    
    try {
        const response = await fetch(`/api/strategy/params/${strategyName}`, {
            headers: buildAuthHeaders()
        });
        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.message || '获取参数失败');
        }
        
        renderStrategyParams(data);
    } catch (error) {
        console.error('获取策略参数失败:', error);
        content.innerHTML = `<div class="empty-state">
            <div class="empty-state-icon">⚠️</div>
            <div class="empty-state-text">加载失败</div>
            <div class="empty-state-hint">${error.message}</div>
        </div>`;
    }
}

// 渲染策略参数
function renderStrategyParams(data) {
    const content = document.getElementById('strategy-param-content');
    
    let html = '';
    
    // 策略权重和启用状态
    html += `
        <div class="param-input-group">
            <div class="param-input-item">
                <label>启用策略</label>
                <select id="param-enabled">
                    <option value="true" ${data.enabled ? 'selected' : ''}>是</option>
                    <option value="false" ${!data.enabled ? 'selected' : ''}>否</option>
                </select>
                <div class="param-description">是否启用该策略</div>
            </div>
            <div class="param-input-item">
                <label>策略权重 (0-1)</label>
                <input type="number" id="param-weight" value="${data.weight}" step="0.01" min="0" max="1">
                <div class="param-description">策略在组合中的权重占比</div>
            </div>
        </div>
    `;
    
    // 如果有参数schema，渲染参数输入
    if (data.schema && data.schema.params && data.schema.params.length > 0) {
        html += '<h4 style="margin: 1.5rem 0 1rem; color: var(--text-primary);">策略参数</h4>';
        html += '<div class="param-input-group">';
        
        data.schema.params.forEach(param => {
            const currentValue = data.params && data.params[param.name] !== undefined ? data.params[param.name] : param.default_value;
            
            html += `<div class="param-input-item">
                <label>${param.name} ${param.required ? '<span style="color: var(--danger);">*</span>' : ''}</label>`;
            
            switch (param.type) {
                case 'int':
                    html += `<input type="number" id="param-${param.name}" value="${currentValue}" 
                             step="1" ${param.min_value !== undefined ? `min="${param.min_value}"` : ''} 
                             ${param.max_value !== undefined ? `max="${param.max_value}"` : ''}>`;
                    break;
                case 'float':
                    html += `<input type="number" id="param-${param.name}" value="${currentValue}" 
                             step="0.0001" ${param.min_value !== undefined ? `min="${param.min_value}"` : ''} 
                             ${param.max_value !== undefined ? `max="${param.max_value}"` : ''}>`;
                    break;
                case 'bool':
                    html += `<select id="param-${param.name}">
                        <option value="true" ${currentValue ? 'selected' : ''}>是</option>
                        <option value="false" ${!currentValue ? 'selected' : ''}>否</option>
                    </select>`;
                    break;
                case 'string':
                    html += `<input type="text" id="param-${param.name}" value="${currentValue || ''}">`;
                    break;
                default:
                    html += `<input type="text" id="param-${param.name}" value="${currentValue || ''}">`;
            }
            
            if (param.description) {
                html += `<div class="param-description">${param.description}</div>`;
            }
            
            if (param.min_value !== undefined || param.max_value !== undefined) {
                let rangeText = [];
                if (param.min_value !== undefined) rangeText.push(`最小: ${param.min_value}`);
                if (param.max_value !== undefined) rangeText.push(`最大: ${param.max_value}`);
                if (rangeText.length > 0) {
                    html += `<div class="param-description" style="color: var(--accent-primary);">${rangeText.join(', ')}</div>`;
                }
            }
            
            html += '</div>';
        });
        
        html += '</div>';
    } else if (data.params) {
        // 如果没有schema但有参数，显示为简单文本
        html += '<h4 style="margin: 1.5rem 0 1rem; color: var(--text-primary);">策略参数</h4>';
        html += '<div class="param-input-group">';
        
        for (const [key, value] of Object.entries(data.params)) {
            html += `<div class="param-input-item">
                <label>${key}</label>
                <input type="text" id="param-${key}" value="${value || ''}">
            </div>`;
        }
        
        html += '</div>';
    }
    
    content.innerHTML = html;
}

// 关闭策略参数配置面板
function closeStrategyParamPanel() {
    const panel = document.getElementById('strategy-param-panel');
    panel.classList.add('hidden');
    currentConfigStrategy = null;
}

// 保存策略参数
async function saveStrategyParams() {
    if (!currentConfigStrategy) return;
    
    try {
        const params = {};
        const enabledInput = document.getElementById('param-enabled');
        const weightInput = document.getElementById('param-weight');
        
        // 收集所有参数
        const inputs = document.querySelectorAll('[id^="param-"]');
        inputs.forEach(input => {
            const key = input.id.replace('param-', '');
            if (key === 'enabled' || key === 'weight') return;
            
            const type = input.type;
            if (type === 'checkbox') {
                params[key] = input.checked;
            } else if (type === 'number') {
                params[key] = parseFloat(input.value);
            } else if (input.tagName === 'SELECT') {
                params[key] = input.value === 'true';
            } else {
                params[key] = input.value;
            }
        });
        
        const requestBody = {
            params: params
        };
        
        if (enabledInput) {
            requestBody.enabled = enabledInput.value === 'true';
        }
        if (weightInput) {
            requestBody.weight = parseFloat(weightInput.value);
        }
        
        const response = await fetch(`/api/strategy/params/${currentConfigStrategy}`, {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify(requestBody)
        });
        
        const result = await response.json();
        
        if (!response.ok) {
            throw new Error(result.message || '保存失败');
        }
        
        showRuntimeNotice('策略参数保存成功！', 'success');
        
        // 刷新策略列表
        fetch('/api/strategies').then(r => r.json()).then(updateStrategies);
        
        // 重新加载参数面板
        openStrategyParamPanel(currentConfigStrategy);
        
    } catch (error) {
        console.error('保存策略参数失败:', error);
        showRuntimeNotice('保存失败: ' + error.message, 'error');
    }
}

// 重置策略参数为默认值
async function resetStrategyParams() {
    if (!currentConfigStrategy) return;
    
    if (!confirm('确定要重置为默认参数吗？')) return;
    
    openStrategyParamPanel(currentConfigStrategy);
}

// 更新单个策略状态
function updateStrategyStatus(data) {
    fetch('/api/strategies').then(r => r.json()).then(updateStrategies);
}

// 更新持仓列表
function updatePositions(positions) {
    const tbody = document.getElementById('positions-body');
    if (!positions || positions.length === 0) {
        tbody.innerHTML = '<tr><td colspan="9" class="loading">暂无持仓</td></tr>';
        return;
    }

    tbody.innerHTML = positions.map(p => `
        <tr>
            <td data-label="标的">${p.symbol}</td>
            <td data-label="方向"><span class="badge ${p.side.toLowerCase()}">${p.side}</span></td>
            <td data-label="数量">${formatNumber(p.size)}</td>
            <td data-label="开仓价">${formatNumber(p.entry_price)}</td>
            <td data-label="标记价">${formatNumber(p.mark_price)}</td>
            <td data-label="未实现盈亏" class="${p.unrealized_pnl >= 0 ? 'positive' : 'negative'}">${formatNumber(p.unrealized_pnl)}</td>
            <td data-label="杠杆">${p.leverage}x</td>
            <td data-label="策略">${p.strategy}</td>
            <td data-label="操作">
                <button class="btn btn-small btn-danger" onclick="closePosition('${p.symbol}')">平仓</button>
            </td>
        </tr>
    `).join('');
}

// 更新订单列表
function updateOrders(orders) {
    const tbody = document.getElementById('orders-body');
    if (!orders || orders.length === 0) {
        tbody.innerHTML = '<tr><td colspan="10" class="loading">暂无订单</td></tr>';
        return;
    }

    tbody.innerHTML = orders.map(o => `
        <tr>
            <td data-label="订单ID">${o.order_id}</td>
            <td data-label="标的">${o.symbol}</td>
            <td data-label="方向"><span class="badge ${o.side.toLowerCase()}">${o.side}</span></td>
            <td data-label="类型">${o.type}</td>
            <td data-label="价格">${formatNumber(o.price)}</td>
            <td data-label="数量">${formatNumber(o.size)}</td>
            <td data-label="已成交">${formatNumber(o.filled_size)}</td>
            <td data-label="状态"><span class="badge ${o.status.toLowerCase()}">${o.status}</span></td>
            <td data-label="策略">${o.strategy}</td>
            <td data-label="时间">${formatTime(o.create_time)}</td>
        </tr>
    `).join('');
}

// 添加订单
function addOrder(order) {
    fetch('/api/orders').then(r => r.json()).then(updateOrders);
}

function updateRebalanceCircuit(data) {
    if (!data) {
        setValue('rebalance-circuit-state', '--');
        setValue('rebalance-circuit-reason', '--');
        setValue('rebalance-circuit-strategy', '--');
        setValue('rebalance-circuit-step', '--');
        setValue('rebalance-circuit-last-reset', '--');
        setValue('rebalance-circuit-last-reset-reason', '--');
        setValue('rebalance-circuit-cooldown', '--');
        return;
    }

    setValue('rebalance-circuit-state', data.open ? 'OPEN' : 'CLOSED', data.open ? 'negative' : 'positive');
    setValue('rebalance-circuit-reason', data.reason || '--');
    setValue('rebalance-circuit-strategy', data.strategy || '--');
    setValue('rebalance-circuit-step', data.step || '--');
    setValue('rebalance-circuit-last-reset', data.last_reset_at ? formatTime(data.last_reset_at) : '--');
    setValue('rebalance-circuit-last-reset-reason', data.last_reset_reason || '--');
    setValue('rebalance-circuit-cooldown', data.cooldown_until ? `冷却至 ${formatTime(data.cooldown_until)}` : (data.cooldown || '--'));
}

async function resetRebalanceCircuit() {
    const reason = window.prompt('请输入重置原因', 'dashboard_manual_reset') || 'dashboard_manual_reset';
    try {
        const response = await fetch('/api/rebalance/circuit/reset', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ reason })
        });
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '重置熔断失败');
        }
    } catch (error) {
        showRuntimeNotice('重置熔断失败: ' + error.message, 'error');
    }
}

function handleRebalanceCircuitResetEvent(event) {
    if (event.circuit) {
        updateRebalanceCircuit(event.circuit);
    }
    const message = event && event.message ? event.message : (event && event.success ? '再平衡熔断已重置' : '再平衡熔断重置失败');
    showRuntimeNotice(message, event && event.success ? 'success' : 'error');
}

function handleRebalanceEvent(event) {
    if (!event) {
        return;
    }
    if (event.circuit) {
        updateRebalanceCircuit(event.circuit);
    }
    addRebalanceEvent(event);
    if (event.type === 'open') {
        showRuntimeNotice(event.message || '再平衡熔断已打开', 'error');
    } else if (event.type === 'recover_started') {
        showRuntimeNotice(event.message || '开始执行再平衡恢复', 'info');
    } else if (event.type === 'recover_succeeded') {
        showRuntimeNotice(event.message || '再平衡恢复已完成', 'success');
    } else if (event.type === 'recover_failed') {
        showRuntimeNotice(event.message || '再平衡恢复失败', 'error');
    }
}

// 更新信号列表
function updateSignals(signals) {
    const tbody = document.getElementById('signals-body');
    if (!signals || signals.length === 0) {
        tbody.innerHTML = '<tr><td colspan="10" class="loading">暂无信号</td></tr>';
        return;
    }

    tbody.innerHTML = signals.map(s => `
        <tr>
            <td data-label="信号ID">${s.id}</td>
            <td data-label="策略">${s.strategy}</td>
            <td data-label="标的">${s.symbol}</td>
            <td data-label="方向"><span class="badge ${s.side.toLowerCase()}">${s.side}</span></td>
            <td data-label="价格">${formatNumber(s.price)}</td>
            <td data-label="数量">${formatNumber(s.size)}</td>
            <td data-label="置信度">${formatPercent(s.confidence)}</td>
            <td data-label="原因">${s.reason || '--'}</td>
            <td data-label="执行状态"><span class="badge ${s.executed ? 'filled' : 'pending'}">${s.executed ? '已执行' : '待执行'}</span></td>
            <td data-label="时间">${formatTime(s.time)}</td>
        </tr>
    `).join('');
}

// 添加信号
function addSignal(signal) {
    fetch('/api/signals').then(r => r.json()).then(updateSignals);
}

// 切换策略状态
async function toggleStrategy(name, start) {
    const action = start ? 'start' : 'stop';
    try {
        const response = await fetch(`/api/strategy/${action}/${name}`, { method: 'POST', headers: buildAuthHeaders() });
        const result = await response.json();
        console.log(`策略${action}结果:`, result);
        fetch('/api/strategies').then(r => r.json()).then(updateStrategies);
    } catch (error) {
        console.error('操作失败:', error);
    }
}

// 平仓
async function closePosition(symbol) {
    if (!confirm(`确定要平仓 ${symbol} 吗？`)) return;
    
    try {
        const response = await fetch(`/api/position/close/${symbol}`, { method: 'POST', headers: buildAuthHeaders() });
        const result = await response.json();
        console.log('平仓结果:', result);
        fetch('/api/positions').then(r => r.json()).then(updatePositions);
    } catch (error) {
        console.error('平仓失败:', error);
    }
}

// 切换交易面板
function toggleTradePanel() {
    const panel = document.getElementById('trade-panel');
    panel.classList.toggle('open');
    if (panel.classList.contains('open')) {
        switchPanelTab('trade');
    }
}

// 切换面板标签页
function switchPanelTab(tab) {
    document.querySelectorAll('.panel-tab').forEach(btn => {
        btn.classList.remove('active');
    });
    document.querySelectorAll('.panel-tab-content').forEach(content => {
        content.classList.add('hidden');
    });
    
    document.querySelector(`.panel-tab[onclick="switchPanelTab('${tab}')"]`).classList.add('active');
    document.getElementById(`tab-${tab}`).classList.remove('hidden');
    
    if (tab === 'orders') {
        refreshManualOrders();
    } else if (tab === 'positions') {
        refreshManualPositions();
    } else if (tab === 'timed') {
        refreshTimedOrders();
    }
}

// 切换价格输入显示
function togglePriceInput() {
    const type = document.getElementById('trade-type').value;
    const priceGroup = document.getElementById('price-input-group');
    if (type === 'limit') {
        priceGroup.style.display = 'flex';
    } else {
        priceGroup.style.display = 'none';
    }
}

// 提交手动订单
async function submitManualOrder() {
    const symbol = document.getElementById('trade-symbol').value;
    const side = document.getElementById('trade-side').value;
    const type = document.getElementById('trade-type').value;
    const price = parseFloat(document.getElementById('trade-price').value) || 0;
    const size = parseFloat(document.getElementById('trade-size').value) || 0;
    const leverage = parseInt(document.getElementById('trade-leverage').value) || 1;
    const takeProfit = parseFloat(document.getElementById('trade-tp').value) || 0;
    const stopLoss = parseFloat(document.getElementById('trade-sl').value) || 0;

    if (!size) {
        showRuntimeNotice('请输入数量', 'warning');
        return;
    }

    if (type === 'limit' && !price) {
        showRuntimeNotice('限价单必须输入价格', 'warning');
        return;
    }

    try {
        const response = await fetch('/api/manual/order', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ 
                symbol, 
                side, 
                type, 
                price, 
                size, 
                leverage, 
                take_profit: takeProfit, 
                stop_loss: stopLoss 
            })
        });
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '下单失败');
        }
        console.log('订单结果:', result);
        showRuntimeNotice('订单提交成功！', 'success');
        toggleTradePanel();
        document.getElementById('trade-price').value = '';
        document.getElementById('trade-size').value = '';
        document.getElementById('trade-tp').value = '';
        document.getElementById('trade-sl').value = '';
    } catch (error) {
        console.error('下单失败:', error);
        showRuntimeNotice('下单失败: ' + error.message, 'error');
    }
}

// 刷新手动交易订单
async function refreshManualOrders() {
    const container = document.getElementById('manual-orders-list');
    container.innerHTML = '<div class="loading-text">加载中...</div>';
    
    try {
        const response = await fetch('/api/manual/orders', {
            headers: buildAuthHeaders()
        });
        const result = await response.json();
        
        if (result.orders && result.orders.length > 0) {
            container.innerHTML = result.orders.map(order => `
                <div class="order-item">
                    <div class="order-header">
                        <span class="order-symbol">${order.symbol}</span>
                        <span class="badge ${order.status.toLowerCase()}">${order.status}</span>
                    </div>
                    <div class="order-details">
                        <div>方向: <span class="${order.side}">${order.side}</span></div>
                        <div>类型: ${order.type}</div>
                        <div>价格: ${formatNumber(order.price)}</div>
                        <div>数量: ${formatNumber(order.size)}</div>
                    </div>
                    <div class="order-actions">
                        ${order.status === 'pending' ? 
                            `<button class="btn btn-small btn-danger" onclick="cancelManualOrder('${order.order_id}')">撤销</button>` : 
                            ''}
                    </div>
                </div>
            `).join('');
        } else {
            container.innerHTML = '<div class="empty-text">暂无订单</div>';
        }
    } catch (error) {
        console.error('获取订单失败:', error);
        container.innerHTML = '<div class="error-text">获取订单失败</div>';
    }
}

// 撤销手动订单
async function cancelManualOrder(orderId) {
    if (!confirm('确定要撤销此订单吗？')) return;
    
    try {
        const response = await fetch(`/api/manual/order/${orderId}`, {
            method: 'DELETE',
            headers: buildAuthHeaders()
        });
        const result = await response.json();
        
        if (!response.ok) {
            throw new Error(result.message || '撤销失败');
        }
        
        showRuntimeNotice('订单已撤销', 'success');
        refreshManualOrders();
    } catch (error) {
        console.error('撤销订单失败:', error);
        showRuntimeNotice('撤销失败: ' + error.message, 'error');
    }
}

// 刷新手动交易持仓
async function refreshManualPositions() {
    const container = document.getElementById('manual-positions-list');
    container.innerHTML = '<div class="loading-text">加载中...</div>';
    
    try {
        const response = await fetch('/api/positions', {
            headers: buildAuthHeaders()
        });
        const positions = await response.json();
        
        if (positions && positions.length > 0) {
            container.innerHTML = positions.map(pos => `
                <div class="position-item">
                    <div class="position-header">
                        <span class="position-symbol">${pos.symbol}</span>
                        <span class="badge ${pos.side.toLowerCase()}">${pos.side}</span>
                    </div>
                    <div class="position-details">
                        <div>数量: ${formatNumber(pos.size)}</div>
                        <div>开仓价: ${formatNumber(pos.entry_price)}</div>
                        <div>标记价: ${formatNumber(pos.mark_price)}</div>
                        <div class="${pos.unrealized_pnl >= 0 ? 'positive' : 'negative'}">
                            未实现盈亏: ${formatNumber(pos.unrealized_pnl)}
                        </div>
                    </div>
                    <div class="position-actions">
                        <button class="btn btn-small btn-danger" onclick="manualClosePosition('${pos.symbol}', ${pos.size})">平仓</button>
                        <button class="btn btn-small btn-warning" onclick="openTrailingStopDialog('${pos.symbol}')">移动止损</button>
                    </div>
                </div>
            `).join('');
        } else {
            container.innerHTML = '<div class="empty-text">暂无持仓</div>';
        }
    } catch (error) {
        console.error('获取持仓失败:', error);
        container.innerHTML = '<div class="error-text">获取持仓失败</div>';
    }
}

// 手动平仓
async function manualClosePosition(symbol, size) {
    showConfirmDialog(
        '确认平仓',
        `确定要平仓 ${symbol} 吗？`,
        async () => {
            try {
                const response = await fetch('/api/manual/position/close', {
                    method: 'POST',
                    headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
                    body: JSON.stringify({ symbol, size })
                });
                const result = await response.json();
                
                if (!response.ok) {
                    throw new Error(result.message || '平仓失败');
                }
                
                showRuntimeNotice('平仓订单已提交', 'success');
                refreshManualPositions();
            } catch (error) {
                console.error('平仓失败:', error);
                showRuntimeNotice('平仓失败: ' + error.message, 'error');
            }
        }
    );
}

// 兼容旧的 submitOrder 函数
async function submitOrder() {
    await submitManualOrder();
}

// 工具函数
function setValue(id, value, className) {
    const el = document.getElementById(id);
    el.textContent = value;
    el.className = 'card-value';
    if (className) el.classList.add(className);
}

function formatNumber(num) {
    if (num === null || num === undefined) return '--';
    return num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function formatPercent(num) {
    if (num === null || num === undefined) return '--';
    return (num * 100).toFixed(2) + '%';
}

function formatTime(time) {
    if (!time) return '--';
    const date = new Date(time);
    return date.toLocaleString('zh-CN');
}

function addRebalanceEvent(event) {
    const normalized = normalizeRebalanceEvent(event);
    if (!normalized) {
        return;
    }
    const first = rebalanceEvents[0];
    if (first && first.signature === normalized.signature) {
        rebalanceEvents[0] = normalized;
    } else {
        rebalanceEvents.unshift(normalized);
    }
    if (rebalanceEvents.length > MAX_REBALANCE_EVENTS) {
        rebalanceEvents = rebalanceEvents.slice(0, MAX_REBALANCE_EVENTS);
    }
    renderRebalanceEvents();
}

function hydrateRebalanceEvents(events) {
    if (!Array.isArray(events) || !events.length) {
        renderRebalanceEvents();
        return;
    }
    rebalanceEvents = events
        .map(normalizeRebalanceEvent)
        .filter(Boolean)
        .sort((left, right) => new Date(right.timestamp).getTime() - new Date(left.timestamp).getTime())
        .slice(0, MAX_REBALANCE_EVENTS);
    renderRebalanceEvents();
}

function normalizeRebalanceEvent(event) {
    if (!event) {
        return null;
    }
    const timestamp = event.timestamp || new Date().toISOString();
    const labels = event.labels || {};
    const details = event.details || {};
    return {
        type: event.type || 'unknown',
        strategy: event.strategy || labels.strategy || details.strategy || '--',
        step: event.step || labels.step || details.step || '--',
        reason: event.reason || labels.reason || details.reason || '--',
        message: event.message || '--',
        timestamp,
        labels,
        details,
        signature: [event.type || '', event.strategy || '', event.step || '', event.reason || '', timestamp].join('|')
    };
}

function renderRebalanceEvents() {
    const container = document.getElementById('rebalance-events-list');
    if (!container) {
        return;
    }
    if (!rebalanceEvents.length) {
        container.innerHTML = '<div class="rebalance-event-empty">等待 WebSocket 事件...</div>';
        return;
    }
    container.innerHTML = rebalanceEvents.map(event => `
        <article class="rebalance-event-item ${event.type}">
            <div class="rebalance-event-header">
                <span class="rebalance-event-type ${event.type}">${formatRebalanceEventType(event.type)}</span>
                <span class="rebalance-event-time">${formatTime(event.timestamp)}</span>
            </div>
            <div class="rebalance-event-title">${event.strategy} / ${event.step}</div>
            <div class="rebalance-event-message">${event.message}</div>
            <div class="rebalance-event-meta">reason=${event.reason}</div>
        </article>
    `).join('');
}

function formatRebalanceEventType(type) {
    switch (type) {
        case 'open':
            return 'OPEN';
        case 'reset':
            return 'RESET';
        case 'recover_started':
            return 'RECOVER START';
        case 'recover_succeeded':
            return 'RECOVER OK';
        case 'recover_failed':
            return 'RECOVER FAIL';
        default:
            return String(type || '--').toUpperCase();
    }
}

// ===============================
// Toast 通知系统
// ===============================
function showRuntimeNotice(message, type = 'info', duration = 4000) {
    let container = document.getElementById('runtime-notice-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'runtime-notice-container';
        document.body.appendChild(container);
    }

    const notice = document.createElement('div');
    notice.className = `toast ${type}`;
    
    const icons = {
        success: '✓',
        error: '✕',
        warning: '⚠',
        info: 'ℹ'
    };
    
    notice.innerHTML = `
        <span class="toast-icon">${icons[type] || icons.info}</span>
        <span class="toast-content">${message}</span>
        <button class="toast-close" onclick="this.parentElement.remove()">&times;</button>
    `;
    
    container.appendChild(notice);

    const timeoutId = setTimeout(() => {
        notice.classList.add('hiding');
        setTimeout(() => {
            notice.remove();
            if (container && container.childElementCount === 0) {
                container.remove();
            }
        }, 300);
    }, duration);
    
    notice.addEventListener('click', (e) => {
        if (!e.target.classList.contains('toast-close')) {
            clearTimeout(timeoutId);
            notice.classList.add('hiding');
            setTimeout(() => {
                notice.remove();
                if (container && container.childElementCount === 0) {
                    container.remove();
                }
            }, 300);
        }
    });
}

// ===============================
// 顶部进度条
// ===============================
let progressInterval = null;

function showProgressBar() {
    let progressBar = document.getElementById('top-progress-bar');
    if (!progressBar) {
        progressBar = document.createElement('div');
        progressBar.id = 'top-progress-bar';
        document.body.appendChild(progressBar);
    }
    
    progressBar.classList.add('active');
    progressBar.style.width = '0%';
    
    let progress = 0;
    clearInterval(progressInterval);
    progressInterval = setInterval(() => {
        progress += Math.random() * 20;
        if (progress > 90) progress = 90;
        progressBar.style.width = progress + '%';
    }, 300);
}

function completeProgressBar() {
    const progressBar = document.getElementById('top-progress-bar');
    if (progressBar) {
        clearInterval(progressInterval);
        progressBar.style.width = '100%';
        setTimeout(() => {
            progressBar.classList.remove('active');
            progressBar.style.width = '0%';
        }, 300);
    }
}

// ===============================
// 全局加载遮罩
// ===============================
function showGlobalLoading(text = '加载中...') {
    let overlay = document.getElementById('global-loading-overlay');
    if (!overlay) {
        overlay = document.createElement('div');
        overlay.id = 'global-loading-overlay';
        overlay.innerHTML = `
            <div class="loading-spinner"></div>
            <div class="loading-text">${text}</div>
        `;
        document.body.appendChild(overlay);
    }
    overlay.querySelector('.loading-text').textContent = text;
    overlay.classList.add('active');
}

function hideGlobalLoading() {
    const overlay = document.getElementById('global-loading-overlay');
    if (overlay) {
        overlay.classList.remove('active');
    }
}

// ===============================
// 确认对话框
// ===============================
function showConfirmDialog(title, message, onConfirm, onCancel) {
    let overlay = document.getElementById('confirm-dialog-overlay');
    if (!overlay) {
        overlay = document.createElement('div');
        overlay.id = 'confirm-dialog-overlay';
        overlay.className = 'confirm-dialog-overlay';
        overlay.innerHTML = `
            <div class="confirm-dialog">
                <h3 id="confirm-dialog-title"></h3>
                <p id="confirm-dialog-message"></p>
                <div class="confirm-dialog-actions">
                    <button class="btn btn-secondary" id="confirm-cancel-btn">取消</button>
                    <button class="btn btn-primary" id="confirm-ok-btn">确认</button>
                </div>
            </div>
        `;
        document.body.appendChild(overlay);
        
        overlay.querySelector('#confirm-cancel-btn').addEventListener('click', () => {
            hideConfirmDialog();
            if (onCancel) onCancel();
        });
        
        overlay.querySelector('#confirm-ok-btn').addEventListener('click', () => {
            hideConfirmDialog();
            if (onConfirm) onConfirm();
        });
    }
    
    overlay.querySelector('#confirm-dialog-title').textContent = title;
    overlay.querySelector('#confirm-dialog-message').textContent = message;
    overlay.classList.add('active');
}

function hideConfirmDialog() {
    const overlay = document.getElementById('confirm-dialog-overlay');
    if (overlay) {
        overlay.classList.remove('active');
    }
}

// ===============================
// 替换 alert 为 Toast
// ===============================
function showAlert(message) {
    showRuntimeNotice(message, 'warning');
}

// ===============================
// 键盘快捷键
// ===============================
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        const paramPanel = document.getElementById('strategy-param-panel');
        if (paramPanel && !paramPanel.classList.contains('hidden')) {
            closeStrategyParamPanel();
            return;
        }
        
        hideConfirmDialog();
        hideGlobalLoading();
    }
    
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        const paramPanel = document.getElementById('strategy-param-panel');
        if (paramPanel && !paramPanel.classList.contains('hidden')) {
            saveStrategyParams();
        }
    }
});

// ===============================
// 增强的 Fetch 包装器，带进度条
// ===============================
async function fetchWithProgress(url, options = {}) {
    showProgressBar();
    try {
        const response = await fetch(url, options);
        completeProgressBar();
        return response;
    } catch (error) {
        completeProgressBar();
        throw error;
    }
}

// ===============================
// 表单验证辅助函数
// ===============================
function setInputError(input, message) {
    const wrapper = input.closest('.form-group, .param-input-item');
    if (wrapper) {
        wrapper.classList.remove('success');
        wrapper.classList.add('error');
        
        let errorMsg = wrapper.querySelector('.error-message');
        if (!errorMsg) {
            errorMsg = document.createElement('div');
            errorMsg.className = 'error-message';
            input.parentNode.appendChild(errorMsg);
        }
        errorMsg.textContent = message;
    }
}

function setInputSuccess(input, message = '') {
    const wrapper = input.closest('.form-group, .param-input-item');
    if (wrapper) {
        wrapper.classList.remove('error');
        wrapper.classList.add('success');
        
        const errorMsg = wrapper.querySelector('.error-message');
        if (errorMsg) errorMsg.remove();
        
        if (message) {
            let successMsg = wrapper.querySelector('.success-message');
            if (!successMsg) {
                successMsg = document.createElement('div');
                successMsg.className = 'success-message';
                input.parentNode.appendChild(successMsg);
            }
            successMsg.textContent = message;
        }
    }
}

function clearInputState(input) {
    const wrapper = input.closest('.form-group, .param-input-item');
    if (wrapper) {
        wrapper.classList.remove('error', 'success');
        const errorMsg = wrapper.querySelector('.error-message');
        const successMsg = wrapper.querySelector('.success-message');
        if (errorMsg) errorMsg.remove();
        if (successMsg) successMsg.remove();
    }
}

// ===============================
// 骨架屏加载功能
// ===============================
function getSkeletonTable(colCount = 8, rowCount = 3) {
    let html = '<div class="skeleton-table">';
    for (let i = 0; i < rowCount; i++) {
        html += '<div class="skeleton-row">';
        for (let j = 0; j < colCount; j++) {
            const cellClass = (j === 0 || j === colCount - 1) ? 'skeleton-cell short' : 'skeleton-cell';
            html += `<div class="skeleton ${cellClass}"></div>`;
        }
        html += '</div>';
    }
    html += '</div>';
    return html;
}

// ===============================
// 增强 resetStrategyParams，使用确认对话框
// ===============================
async function resetStrategyParams() {
    if (!currentConfigStrategy) return;
    
    showConfirmDialog(
        '重置参数',
        '确定要重置为默认参数吗？',
        () => {
            openStrategyParamPanel(currentConfigStrategy);
        }
    );
}

// LLM 分析面板标签页切换
function switchLLMTab(tab) {
    document.querySelectorAll('.llm-tab').forEach(btn => {
        btn.classList.remove('active');
    });
    document.querySelectorAll('.llm-tab-content').forEach(content => {
        content.classList.add('hidden');
    });
    
    document.querySelector(`.llm-tab[onclick="switchLLMTab('${tab}')"]`).classList.add('active');
    document.getElementById(`llm-tab-${tab}`).classList.remove('hidden');
}

// 交易分析
async function analyzeTrade() {
    const symbol = document.getElementById('llm-trade-symbol').value;
    const side = document.getElementById('llm-trade-side').value;
    const size = parseFloat(document.getElementById('llm-position-size').value) || 0;
    const price = parseFloat(document.getElementById('llm-entry-price').value) || 0;
    
    if (!symbol || !size) {
        alert('请输入交易对和数量');
        return;
    }
    
    setLLMLoading(true);
    const resultDiv = document.getElementById('llm-trade-result');
    resultDiv.innerHTML = '<div class="loading-text">AI 分析中，请稍候...</div>';
    
    try {
        const response = await fetch('/api/llm/analyze/trade', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ symbol, side, size, price })
        });
        
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '分析失败');
        }
        
        renderLLMResult(resultDiv, result);
    } catch (error) {
        resultDiv.innerHTML = `<div class="error-text">分析失败: ${error.message}</div>`;
        console.error('分析失败:', error);
    } finally {
        setLLMLoading(false);
    }
}

// 持仓分析
async function analyzePositions() {
    setLLMLoading(true);
    const resultDiv = document.getElementById('llm-positions-result');
    resultDiv.innerHTML = '<div class="loading-text">AI 分析中，请稍候...</div>';
    
    try {
        const response = await fetch('/api/llm/analyze/positions', {
            headers: buildAuthHeaders()
        });
        
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '分析失败');
        }
        
        renderLLMResult(resultDiv, result);
    } catch (error) {
        resultDiv.innerHTML = `<div class="error-text">分析失败: ${error.message}</div>`;
        console.error('分析失败:', error);
    } finally {
        setLLMLoading(false);
    }
}

// 市场分析
async function analyzeMarket() {
    const symbol = document.getElementById('llm-market-symbols').value;
    
    if (!symbol) {
        showRuntimeNotice('请输入交易对', 'warning');
        return;
    }
    
    setLLMLoading(true);
    const resultDiv = document.getElementById('llm-market-result');
    resultDiv.innerHTML = '<div class="loading-text">AI 分析中，请稍候...</div>';
    
    try {
        const response = await fetch('/api/llm/analyze/market', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ symbol })
        });
        
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '分析失败');
        }
        
        renderLLMResult(resultDiv, result);
    } catch (error) {
        resultDiv.innerHTML = `<div class="error-text">分析失败: ${error.message}</div>`;
        console.error('分析失败:', error);
    } finally {
        setLLMLoading(false);
    }
}

// 订单分析
async function analyzeOrders() {
    const analysisType = document.getElementById('llm-orders-type').value;
    const symbol = document.getElementById('llm-orders-symbol').value;
    const timeRange = document.getElementById('llm-orders-time-range').value;
    
    setLLMLoading(true);
    const resultDiv = document.getElementById('llm-orders-result');
    resultDiv.innerHTML = '<div class="loading-text">AI 分析中，请稍候...</div>';
    
    try {
        // 模拟订单数据（实际应用中应该从API获取）
        const mockOrders = [
            {
                "order_id": "123456",
                "symbol": "BTC-USDT",
                "side": "buy",
                "type": "market",
                "price": 50000,
                "size": 0.01,
                "filled_size": 0.01,
                "status": "filled",
                "create_time": new Date().toISOString(),
                "fill_price": 50050
            },
            {
                "order_id": "123457",
                "symbol": "ETH-USDT",
                "side": "sell",
                "type": "limit",
                "price": 3000,
                "size": 0.1,
                "filled_size": 0.1,
                "status": "filled",
                "create_time": new Date().toISOString(),
                "fill_price": 2995
            }
        ];
        
        const response = await fetch('/api/llm/analyze/orders', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({
                orders: mockOrders,
                time_range: timeRange,
                analysis_type: analysisType,
                symbol: symbol
            })
        });
        
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '分析失败');
        }
        
        renderLLMResult(resultDiv, result);
    } catch (error) {
        resultDiv.innerHTML = `<div class="error-text">分析失败: ${error.message}</div>`;
        console.error('分析失败:', error);
    } finally {
        setLLMLoading(false);
    }
}

// 获取历史分析记录
async function getLLMHistory() {
    setLLMLoading(true);
    const historyDiv = document.getElementById('llm-history-list');
    historyDiv.innerHTML = '<div class="loading-text">加载中...</div>';
    
    try {
        const response = await fetch('/api/llm/history', {
            headers: buildAuthHeaders()
        });
        
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '获取历史记录失败');
        }
        
        if (result.analyses && result.analyses.length > 0) {
            historyDiv.innerHTML = result.analyses.map(analysis => `
                <div class="llm-history-item">
                    <div class="llm-history-header">
                        <span class="llm-history-type">${formatAnalysisType(analysis.analysis_type)}</span>
                        <span class="llm-history-time">${formatTime(analysis.created_at)}</span>
                    </div>
                    <div class="llm-history-summary">${analysis.summary || '暂无摘要'}</div>
                    <div class="llm-history-toggle" onclick="toggleLLMDetail(${analysis.id})">
                        查看详情
                    </div>
                    <div id="llm-detail-${analysis.id}" class="llm-history-detail hidden">
                        <pre>${escapeHtml(analysis.analysis)}</pre>
                    </div>
                </div>
            `).join('');
        } else {
            historyDiv.innerHTML = '<div class="empty-text">暂无历史记录</div>';
        }
    } catch (error) {
        historyDiv.innerHTML = `<div class="error-text">获取失败: ${error.message}</div>`;
        console.error('获取历史记录失败:', error);
    } finally {
        setLLMLoading(false);
    }
}

// 切换历史记录详情显示
function toggleLLMDetail(id) {
    const detailDiv = document.getElementById(`llm-detail-${id}`);
    detailDiv.classList.toggle('hidden');
}

// 格式化分析类型
function formatAnalysisType(type) {
    const types = {
        'trade': '交易分析',
        'positions': '持仓分析',
        'market': '市场分析',
        'news': '新闻分析',
        'economic': '经济事件分析'
    };
    return types[type] || type;
}

// 渲染 LLM 分析结果
function renderLLMResult(container, result) {
    if (result.summary) {
        container.innerHTML = `
            <div class="llm-result">
                <div class="llm-result-summary">${result.summary}</div>
                ${result.analysis ? `<div class="llm-result-detail"><pre>${escapeHtml(result.analysis)}</pre></div>` : ''}
                ${result.recommendation ? `<div class="llm-result-recommendation">建议: ${result.recommendation}</div>` : ''}
            </div>
        `;
    } else {
        container.innerHTML = `
            <div class="llm-result">
                <pre>${escapeHtml(JSON.stringify(result, null, 2))}</pre>
            </div>
        `;
    }
}

// 设置 LLM 加载状态
function setLLMLoading(loading) {
    llmLoading = loading;
    const buttons = document.querySelectorAll('.llm-analyze-btn');
    buttons.forEach(btn => {
        btn.disabled = loading;
        if (loading) {
            btn.textContent = '分析中...';
        } else {
            if (btn.id === 'llm-trade-btn') {
                btn.textContent = '开始分析';
            } else if (btn.id === 'llm-positions-btn') {
                btn.textContent = '分析持仓';
            } else if (btn.id === 'llm-market-btn') {
                btn.textContent = '分析市场';
            }
        }
    });
}

// HTML 转义
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 切换LLM分析面板
function toggleLLMPanel() {
    const panel = document.getElementById('llm-panel');
    panel.classList.toggle('open');
    if (panel.classList.contains('open')) {
        switchLLMTab('trade');
    }
}

// 分析持仓（修复函数名）
async function analyzePosition() {
    return analyzePositions();
}

// 刷新LLM历史记录（修复函数名）
async function refreshLLMHistory() {
    return getLLMHistory();
}

// 提交限时单
async function submitTimedOrder() {
    const symbol = document.getElementById('timed-trade-symbol').value;
    const side = document.getElementById('timed-trade-side').value;
    const size = parseFloat(document.getElementById('timed-trade-size').value) || 0;
    const executeTime = document.getElementById('timed-trade-time').value;

    if (!size) {
        showRuntimeNotice('请输入数量', 'warning');
        return;
    }

    if (!executeTime) {
        showRuntimeNotice('请选择执行时间', 'warning');
        return;
    }

    try {
        const response = await fetch('/api/manual/timed-order', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ 
                symbol, 
                side, 
                size, 
                execute_at: executeTime 
            })
        });
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '创建限时单失败');
        }
        console.log('限时单创建结果:', result);
        showRuntimeNotice('限时单创建成功！', 'success');
        document.getElementById('timed-trade-size').value = '';
        document.getElementById('timed-trade-time').value = '';
        refreshTimedOrders();
    } catch (error) {
        console.error('创建限时单失败:', error);
        showRuntimeNotice('创建限时单失败: ' + error.message, 'error');
    }
}

// 刷新限时单列表
async function refreshTimedOrders() {
    const container = document.getElementById('timed-orders-list');
    container.innerHTML = '<div class="loading-text">加载中...</div>';
    
    try {
        const response = await fetch('/api/manual/timed-orders', {
            headers: buildAuthHeaders()
        });
        const result = await response.json();
        
        if (result.orders && result.orders.length > 0) {
            container.innerHTML = result.orders.map(order => `
                <div class="order-item">
                    <div class="order-header">
                        <span class="order-symbol">${order.symbol}</span>
                        <span class="badge ${order.status.toLowerCase()}">${order.status}</span>
                    </div>
                    <div class="order-details">
                        <div>方向: <span class="${order.side}">${order.side}</span></div>
                        <div>数量: ${formatNumber(order.size)}</div>
                        <div>执行时间: ${formatTime(order.execute_at)}</div>
                        ${order.executed_at ? `<div>执行时间: ${formatTime(order.executed_at)}</div>` : ''}
                        ${order.execute_price ? `<div>执行价格: ${formatNumber(order.execute_price)}</div>` : ''}
                    </div>
                    <div class="order-actions">
                        ${order.status === 'pending' ? 
                            `<button class="btn btn-small btn-danger" onclick="cancelTimedOrder('${order.id}')">取消</button>` : 
                            ''}
                    </div>
                </div>
            `).join('');
        } else {
            container.innerHTML = '<div class="empty-text">暂无限时单</div>';
        }
    } catch (error) {
        console.error('获取限时单失败:', error);
        container.innerHTML = '<div class="error-text">获取限时单失败</div>';
    }
}

// 取消限时单
async function cancelTimedOrder(orderId) {
    showConfirmDialog(
        '确认取消',
        '确定要取消此限时单吗？',
        async () => {
            try {
                const response = await fetch(`/api/manual/timed-order/${orderId}`, {
                    method: 'DELETE',
                    headers: buildAuthHeaders()
                });
                const result = await response.json();
                
                if (!response.ok) {
                    throw new Error(result.message || '取消失败');
                }
                
                showRuntimeNotice('限时单已取消', 'success');
                refreshTimedOrders();
            } catch (error) {
                console.error('取消限时单失败:', error);
                showRuntimeNotice('取消失败: ' + error.message, 'error');
            }
        }
    );
}

// 订阅市场数据
function subscribeToMarketData() {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        console.log('WebSocket未连接，无法订阅');
        return;
    }

    const subscribeCmd = {
        action: 'subscribe',
        events: ['ticker', 'kline', 'orderbook', 'status', 'alert']
    };

    ws.send(JSON.stringify(subscribeCmd));
    console.log('已订阅市场数据事件');
}

// 取消订阅市场数据
function unsubscribeFromMarketData() {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        return;
    }

    const unsubscribeCmd = {
        action: 'unsubscribe',
        events: ['ticker', 'kline', 'orderbook', 'status', 'alert']
    };

    ws.send(JSON.stringify(unsubscribeCmd));
}

// 处理行情数据
function handleTicker(data) {
    if (!data) return;
    
    marketData.tickers[data.symbol] = data;
    updateTickerDisplay(data);
}

// 更新行情显示
function updateTickerDisplay(data) {
    const tickerEl = document.getElementById(`ticker-${data.symbol}`);
    if (!tickerEl) return;
    
    const priceEl = tickerEl.querySelector('.ticker-price');
    if (priceEl) {
        priceEl.textContent = formatNumber(data.price);
    }
}

// 处理K线数据
function handleKline(data) {
    if (!data) return;
    
    const key = `${data.symbol}-${data.interval}`;
    if (!marketData.klines[key]) {
        marketData.klines[key] = [];
    }
    marketData.klines[key].push(data);
    
    // 保留最近100根K线
    if (marketData.klines[key].length > 100) {
        marketData.klines[key] = marketData.klines[key].slice(-100);
    }
    
    updateKlineDisplay(data);
}

// 更新K线显示
function updateKlineDisplay(data) {
    console.log('收到K线数据:', data);
}

// 处理订单簿数据
function handleOrderBook(data) {
    if (!data) return;
    
    marketData.orderBooks[data.symbol] = data;
    updateOrderBookDisplay(data);
}

// 更新订单簿显示
function updateOrderBookDisplay(data) {
    const orderBookEl = document.getElementById(`orderbook-${data.symbol}`);
    if (!orderBookEl) return;
    
    const asksEl = orderBookEl.querySelector('.orderbook-asks');
    const bidsEl = orderBookEl.querySelector('.orderbook-bids');
    
    if (asksEl) {
        asksEl.innerHTML = data.asks.slice(0, 10).map(ask => 
            `<div class="orderbook-row">
                <span class="orderbook-price">${formatNumber(ask[0])}</span>
                <span class="orderbook-size">${formatNumber(ask[1])}</span>
            </div>`
        ).join('');
    }
    
    if (bidsEl) {
        bidsEl.innerHTML = data.bids.slice(0, 10).map(bid => 
            `<div class="orderbook-row">
                <span class="orderbook-price">${formatNumber(bid[0])}</span>
                <span class="orderbook-size">${formatNumber(bid[1])}</span>
            </div>`
        ).join('');
    }
}

// 处理系统状态
function handleSystemStatus(data) {
    if (!data) return;
    
    const clientCountEl = document.getElementById('ws-client-count');
    if (clientCountEl) {
        clientCountEl.textContent = data.client_count;
    }
    
    const messageCountEl = document.getElementById('ws-message-count');
    if (messageCountEl) {
        messageCountEl.textContent = data.message_count;
    }
    
    const uptimeEl = document.getElementById('ws-uptime');
    if (uptimeEl) {
        uptimeEl.textContent = formatDuration(data.uptime);
    }
}

// 格式化持续时间
function formatDuration(durationMs) {
    if (!durationMs) return '--';
    
    const seconds = Math.floor(durationMs / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    
    if (days > 0) {
        return `${days}天${hours % 24}小时`;
    } else if (hours > 0) {
        return `${hours}小时${minutes % 60}分钟`;
    } else if (minutes > 0) {
        return `${minutes}分钟${seconds % 60}秒`;
    } else {
        return `${seconds}秒`;
    }
}

// ===============================
// 技术指标相关功能
// ===============================

// 更新技术指标参数界面
function updateIndicatorParams() {
    const indicatorType = document.getElementById('indicator-type').value;
    const paramsContainer = document.getElementById('indicator-params');
    const paramsConfig = indicatorParamsConfig[indicatorType];
    
    paramsContainer.innerHTML = '';
    
    paramsConfig.forEach(param => {
        const inputWrapper = document.createElement('div');
        inputWrapper.className = 'param-input-item';
        inputWrapper.style.display = 'flex';
        inputWrapper.style.flexDirection = 'column';
        inputWrapper.style.gap = '0.25rem';
        inputWrapper.style.flex = '1';
        inputWrapper.style.minWidth = '100px';
        
        const label = document.createElement('label');
        label.textContent = param.label;
        label.style.fontSize = '0.75rem';
        label.style.color = 'var(--text-secondary)';
        
        const input = document.createElement('input');
        input.type = param.type;
        input.id = `indicator-param-${param.name}`;
        input.value = param.default;
        if (param.min !== undefined) input.min = param.min;
        if (param.max !== undefined) input.max = param.max;
        if (param.step !== undefined) input.step = param.step;
        input.style.padding = '0.5rem';
        input.style.border = '1px solid var(--border-color)';
        input.style.borderRadius = '8px';
        input.style.background = 'var(--bg-primary)';
        input.style.color = 'var(--text-primary)';
        input.style.fontSize = '0.875rem';
        
        inputWrapper.appendChild(label);
        inputWrapper.appendChild(input);
        paramsContainer.appendChild(inputWrapper);
    });
    
    refreshIndicatorChart();
}

// 获取技术指标参数
function getIndicatorParams() {
    const indicatorType = document.getElementById('indicator-type').value;
    const paramsConfig = indicatorParamsConfig[indicatorType];
    const params = {};
    
    paramsConfig.forEach(param => {
        const input = document.getElementById(`indicator-param-${param.name}`);
        if (input) {
            params[param.name] = param.type === 'number' ? parseFloat(input.value) : input.value;
        }
    });
    
    return params;
}

// 生成初始技术指标数据
async function generateInitialIndicatorData() {
    const symbol = document.getElementById('indicator-symbol').value;
    const interval = document.getElementById('indicator-interval').value;
    const indicatorType = document.getElementById('indicator-type').value;
    
    try {
        showProgressBar();
        
        const priceResponse = await fetch(`/api/market/bars?symbol=${encodeURIComponent(symbol)}&interval=${encodeURIComponent(interval)}&limit=200`, {
            headers: buildAuthHeaders()
        });
        
        if (!priceResponse.ok) {
            throw new Error('获取K线数据失败');
        }
        
        const priceData = await priceResponse.json();
        
        priceChartData.labels = [];
        priceChartData.datasets[0].data = [];
        
        priceData.bars.forEach(bar => {
            const date = new Date(bar.timestamp);
            priceChartData.labels.push(date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }));
            priceChartData.datasets[0].data.push(bar.close);
        });
        
        await calculateAndUpdateIndicators(symbol, interval, indicatorType);
        
        if (priceChart) priceChart.update();
        if (indicatorChart) indicatorChart.update();
        
    } catch (error) {
        console.error('获取真实数据失败，使用模拟数据:', error);
        generateFallbackIndicatorData();
    } finally {
        completeProgressBar();
    }
}

function generateFallbackIndicatorData() {
    const now = new Date();
    const dataPoints = 100;
    const basePrice = 50000;
    
    priceChartData.labels = [];
    priceChartData.datasets[0].data = [];
    
    for (let i = dataPoints; i >= 0; i--) {
        const date = new Date(now.getTime() - i * 60 * 60 * 1000);
        priceChartData.labels.push(date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }));
        
        const randomChange = (Math.random() - 0.5) * 200;
        const price = basePrice + randomChange + Math.sin(i / 10) * 500;
        priceChartData.datasets[0].data.push(price);
    }
    
    updateIndicatorChartData();
    
    if (priceChart) priceChart.update();
    if (indicatorChart) indicatorChart.update();
}

async function calculateAndUpdateIndicators(symbol, interval, indicatorType) {
    const params = getIndicatorParams();
    
    const indicatorRequest = {
        symbol: symbol,
        interval: interval,
        limit: 200,
        indicators: [
            {
                name: indicatorType === 'BOLL' ? 'BOLLINGER' : indicatorType,
                params: params
            }
        ]
    };
    
    try {
        const response = await fetch('/api/indicators/calculate', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify(indicatorRequest)
        });
        
        if (!response.ok) {
            throw new Error('计算指标失败');
        }
        
        const result = await response.json();
        applyIndicatorResults(result, indicatorType);
        
    } catch (error) {
        console.error('计算指标失败，使用本地计算:', error);
        updateIndicatorChartData();
    }
}

function applyIndicatorResults(result, indicatorType) {
    const indicatorTitle = document.getElementById('indicator-chart-title');
    const priceTitle = document.getElementById('price-chart-title');
    const symbol = document.getElementById('indicator-symbol').value;
    const interval = document.getElementById('indicator-interval').value;
    
    priceTitle.textContent = `价格走势 - ${symbol} (${interval})`;
    
    indicatorChartData.labels = [...priceChartData.labels];
    indicatorChartData.datasets = [];
    
    if (!result.results || result.results.length === 0) {
        updateIndicatorChartData();
        return;
    }
    
    const indicatorResult = result.results[0];
    const params = getIndicatorParams();
    
    switch (indicatorType) {
        case 'MACD':
            indicatorTitle.textContent = `MACD (${params.fastPeriod}, ${params.slowPeriod}, ${params.signalPeriod})`;
            if (indicatorResult.macd_line) {
                indicatorChartData.datasets.push({
                    label: 'MACD线',
                    data: indicatorResult.macd_line,
                    borderColor: '#00d4ff',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0
                });
            }
            if (indicatorResult.signal_line) {
                indicatorChartData.datasets.push({
                    label: '信号线',
                    data: indicatorResult.signal_line,
                    borderColor: '#ffc107',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0
                });
            }
            if (indicatorResult.histogram) {
                indicatorChartData.datasets.push({
                    label: '柱状图',
                    data: indicatorResult.histogram,
                    type: 'bar',
                    backgroundColor: indicatorResult.histogram.map(v => v >= 0 ? 'rgba(0, 255, 136, 0.6)' : 'rgba(255, 68, 68, 0.6)'),
                    borderWidth: 0
                });
            }
            break;
            
        case 'RSI':
            indicatorTitle.textContent = `RSI (${params.period})`;
            indicatorChartData.datasets.push({
                label: 'RSI',
                data: indicatorResult.rsi || [],
                borderColor: '#7c3aed',
                backgroundColor: 'rgba(124, 58, 237, 0.1)',
                fill: true,
                tension: 0.4,
                pointRadius: 0
            });
            break;
            
        case 'BOLL':
            indicatorTitle.textContent = `布林带 (${params.period}, ${params.deviation}σ)`;
            if (indicatorResult.upper) {
                indicatorChartData.datasets.push({
                    label: '上轨',
                    data: indicatorResult.upper,
                    borderColor: '#ff4444',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0,
                    borderDash: [5, 5]
                });
            }
            if (indicatorResult.middle) {
                indicatorChartData.datasets.push({
                    label: '中轨',
                    data: indicatorResult.middle,
                    borderColor: '#ffc107',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0
                });
            }
            if (indicatorResult.lower) {
                indicatorChartData.datasets.push({
                    label: '下轨',
                    data: indicatorResult.lower,
                    borderColor: '#00ff88',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0,
                    borderDash: [5, 5]
                });
            }
            priceChartData.datasets = [
                {
                    label: '价格',
                    data: priceChartData.datasets[0].data,
                    borderColor: '#00ff88',
                    backgroundColor: 'rgba(0, 255, 136, 0.1)',
                    fill: true,
                    tension: 0.4,
                    pointRadius: 0,
                    order: 1
                },
                {
                    label: '上轨',
                    data: indicatorResult.upper || [],
                    borderColor: '#ff4444',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0,
                    borderDash: [5, 5],
                    order: 0
                },
                {
                    label: '下轨',
                    data: indicatorResult.lower || [],
                    borderColor: '#00ff88',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0,
                    borderDash: [5, 5],
                    order: 0
                }
            ];
            break;
            
        case 'ATR':
            indicatorTitle.textContent = `ATR (${params.period})`;
            indicatorChartData.datasets.push({
                label: 'ATR',
                data: indicatorResult.atr || [],
                borderColor: '#00d4ff',
                backgroundColor: 'rgba(0, 212, 255, 0.1)',
                fill: true,
                tension: 0.4,
                pointRadius: 0
            });
            break;
            
        case 'ADX':
            indicatorTitle.textContent = `ADX (${params.period})`;
            indicatorChartData.datasets.push({
                label: 'ADX',
                data: indicatorResult.adx || [],
                borderColor: '#7c3aed',
                backgroundColor: 'transparent',
                tension: 0.4,
                pointRadius: 0
            });
            if (indicatorResult.plus_di) {
                indicatorChartData.datasets.push({
                    label: '+DI',
                    data: indicatorResult.plus_di,
                    borderColor: '#00ff88',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0,
                    borderDash: [3, 3]
                });
            }
            if (indicatorResult.minus_di) {
                indicatorChartData.datasets.push({
                    label: '-DI',
                    data: indicatorResult.minus_di,
                    borderColor: '#ff4444',
                    backgroundColor: 'transparent',
                    tension: 0.4,
                    pointRadius: 0,
                    borderDash: [3, 3]
                });
            }
            break;
            
        case 'PRICE':
            indicatorTitle.textContent = '价格走势';
            indicatorChartData.datasets.push({
                label: '价格',
                data: priceChartData.datasets[0].data,
                borderColor: '#00d4ff',
                backgroundColor: 'rgba(0, 212, 255, 0.1)',
                fill: true,
                tension: 0.4,
                pointRadius: 0
            });
            priceChartData.datasets = [{
                label: '价格',
                data: priceChartData.datasets[0].data,
                borderColor: '#00ff88',
                backgroundColor: 'rgba(0, 255, 136, 0.1)',
                fill: true,
                tension: 0.4,
                pointRadius: 0,
                pointHoverRadius: 5
            }];
            break;
    }
}

// 刷新技术指标图表
async function refreshIndicatorChart() {
    const symbol = document.getElementById('indicator-symbol').value;
    const interval = document.getElementById('indicator-interval').value;
    const indicatorType = document.getElementById('indicator-type').value;
    
    try {
        showProgressBar();
        
        const priceResponse = await fetch(`/api/market/bars?symbol=${encodeURIComponent(symbol)}&interval=${encodeURIComponent(interval)}&limit=200`, {
            headers: buildAuthHeaders()
        });
        
        if (priceResponse.ok) {
            const priceData = await priceResponse.json();
            priceChartData.labels = [];
            priceChartData.datasets[0].data = [];
            
            priceData.bars.forEach(bar => {
                const date = new Date(bar.timestamp);
                priceChartData.labels.push(date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }));
                priceChartData.datasets[0].data.push(bar.close);
            });
            
            await calculateAndUpdateIndicators(symbol, interval, indicatorType);
        } else {
            updateIndicatorChartData();
        }
        
        if (priceChart) priceChart.update();
        if (indicatorChart) indicatorChart.update();
        
    } catch (error) {
        console.error('刷新指标失败:', error);
        updateIndicatorChartData();
        if (priceChart) priceChart.update();
        if (indicatorChart) indicatorChart.update();
    } finally {
        completeProgressBar();
    }
}

// ===============================
// 技术指标计算函数（简化版）
// ===============================

function calculateEMA(data, period) {
    const ema = [];
    const multiplier = 2 / (period + 1);
    
    let sum = 0;
    for (let i = 0; i < period; i++) {
        sum += data[i] || 0;
    }
    ema.push(sum / period);
    
    for (let i = period; i < data.length; i++) {
        ema.push((data[i] - ema[ema.length - 1]) * multiplier + ema[ema.length - 1]);
    }
    
    const result = new Array(Math.min(period - 1, data.length)).fill(null);
    return result.concat(ema);
}

function calculateSMA(data, period) {
    const sma = [];
    for (let i = 0; i < data.length; i++) {
        if (i < period - 1) {
            sma.push(null);
        } else {
            let sum = 0;
            for (let j = 0; j < period; j++) {
                sum += data[i - j];
            }
            sma.push(sum / period);
        }
    }
    return sma;
}

function calculateMACD(data, fastPeriod, slowPeriod, signalPeriod) {
    const fastEMA = calculateEMA(data, fastPeriod);
    const slowEMA = calculateEMA(data, slowPeriod);
    
    const macdLine = [];
    for (let i = 0; i < data.length; i++) {
        if (fastEMA[i] !== null && slowEMA[i] !== null) {
            macdLine.push(fastEMA[i] - slowEMA[i]);
        } else {
            macdLine.push(null);
        }
    }
    
    const validMacdLine = macdLine.filter(v => v !== null);
    const signalLineData = calculateEMA(validMacdLine, signalPeriod);
    const signalLine = new Array(macdLine.length - validMacdLine.length).fill(null).concat(signalLineData);
    
    const histogram = [];
    for (let i = 0; i < data.length; i++) {
        if (macdLine[i] !== null && signalLine[i] !== null) {
            histogram.push(macdLine[i] - signalLine[i]);
        } else {
            histogram.push(null);
        }
    }
    
    return { macdLine, signalLine, histogram };
}

function calculateRSI(data, period) {
    const rsi = [];
    const gains = [];
    const losses = [];
    
    for (let i = 1; i < data.length; i++) {
        const change = data[i] - data[i - 1];
        gains.push(change > 0 ? change : 0);
        losses.push(change < 0 ? -change : 0);
    }
    
    let avgGain = gains.slice(0, period).reduce((a, b) => a + b, 0) / period;
    let avgLoss = losses.slice(0, period).reduce((a, b) => a + b, 0) / period;
    
    for (let i = 0; i < period; i++) {
        rsi.push(null);
    }
    
    rsi.push(100 - (100 / (1 + avgGain / (avgLoss || 0.001))));
    
    for (let i = period; i < gains.length; i++) {
        avgGain = (avgGain * (period - 1) + gains[i]) / period;
        avgLoss = (avgLoss * (period - 1) + losses[i]) / period;
        rsi.push(100 - (100 / (1 + avgGain / (avgLoss || 0.001))));
    }
    
    return rsi;
}

function calculateStandardDeviation(data, period, sma) {
    const stdDev = [];
    for (let i = 0; i < data.length; i++) {
        if (i < period - 1 || sma[i] === null) {
            stdDev.push(null);
        } else {
            let sum = 0;
            for (let j = 0; j < period; j++) {
                sum += Math.pow(data[i - j] - sma[i], 2);
            }
            stdDev.push(Math.sqrt(sum / period));
        }
    }
    return stdDev;
}

function calculateBollingerBands(data, period, deviation) {
    const middle = calculateSMA(data, period);
    const stdDev = calculateStandardDeviation(data, period, middle);
    
    const upper = [];
    const lower = [];
    
    for (let i = 0; i < data.length; i++) {
        if (middle[i] !== null && stdDev[i] !== null) {
            upper.push(middle[i] + deviation * stdDev[i]);
            lower.push(middle[i] - deviation * stdDev[i]);
        } else {
            upper.push(null);
            lower.push(null);
        }
    }
    
    return { upper, middle, lower };
}

function calculateATR(data, period) {
    const tr = [];
    for (let i = 0; i < data.length; i++) {
        if (i === 0) {
            tr.push(null);
        } else {
            const high = data[i] * 1.01;
            const low = data[i] * 0.99;
            const prevClose = data[i - 1];
            const trueRange = Math.max(high - low, Math.abs(high - prevClose), Math.abs(low - prevClose));
            tr.push(trueRange);
        }
    }
    
    const atr = calculateSMA(tr.filter(v => v !== null), period);
    const result = new Array(data.length - atr.length).fill(null);
    return result.concat(atr);
}

function calculateADX(data, period) {
    const plusDM = [];
    const minusDM = [];
    const tr = [];
    
    for (let i = 0; i < data.length; i++) {
        if (i === 0) {
            plusDM.push(null);
            minusDM.push(null);
            tr.push(null);
        } else {
            const high = data[i] * 1.01;
            const low = data[i] * 0.99;
            const prevHigh = data[i - 1] * 1.01;
            const prevLow = data[i - 1] * 0.99;
            const prevClose = data[i - 1];
            
            const upMove = high - prevHigh;
            const downMove = prevLow - low;
            
            plusDM.push(upMove > downMove && upMove > 0 ? upMove : 0);
            minusDM.push(downMove > upMove && downMove > 0 ? downMove : 0);
            tr.push(Math.max(high - low, Math.abs(high - prevClose), Math.abs(low - prevClose)));
        }
    }
    
    const smoothedPlusDM = calculateSMA(plusDM.filter(v => v !== null), period);
    const smoothedMinusDM = calculateSMA(minusDM.filter(v => v !== null), period);
    const smoothedTR = calculateSMA(tr.filter(v => v !== null), period);
    
    const plusDI = [];
    const minusDI = [];
    const dx = [];
    
    const validLength = Math.min(smoothedPlusDM.length, smoothedMinusDM.length, smoothedTR.length);
    for (let i = 0; i < validLength; i++) {
        const trVal = smoothedTR[i] || 0.001;
        const pdi = 100 * (smoothedPlusDM[i] / trVal);
        const mdi = 100 * (smoothedMinusDM[i] / trVal);
        plusDI.push(pdi);
        minusDI.push(mdi);
        dx.push(100 * (Math.abs(pdi - mdi) / (pdi + mdi || 0.001)));
    }
    
    const adx = calculateSMA(dx, period);
    
    const padLength = data.length - adx.length;
    const pad = new Array(Math.max(0, padLength)).fill(null);
    
    return {
        adx: pad.concat(adx),
        plusDI: pad.concat(plusDI),
        minusDI: pad.concat(minusDI)
    };
}

// ==================== 条件单功能 ====================

// 切换条件类型输入框
function toggleConditionInputs() {
    const conditionalType = document.getElementById('conditional-type').value;
    const priceGroup = document.getElementById('price-condition-group');
    const priceValueGroup = document.getElementById('price-value-group');
    const timeGroup = document.getElementById('time-condition-group');

    if (conditionalType === 'price') {
        priceGroup.classList.remove('hidden');
        priceValueGroup.classList.remove('hidden');
        timeGroup.classList.add('hidden');
    } else {
        priceGroup.classList.add('hidden');
        priceValueGroup.classList.add('hidden');
        timeGroup.classList.remove('hidden');
    }
}

// 提交条件单
async function submitConditionalOrder() {
    const symbol = document.getElementById('conditional-trade-symbol').value;
    const side = document.getElementById('conditional-trade-side').value;
    const size = parseFloat(document.getElementById('conditional-trade-size').value) || 0;
    const orderType = document.getElementById('conditional-order-type').value;
    const price = parseFloat(document.getElementById('conditional-trade-price').value) || 0;
    const conditionalType = document.getElementById('conditional-type').value;

    if (!size) {
        showRuntimeNotice('请输入数量', 'warning');
        return;
    }

    if (orderType === 'limit' && !price) {
        showRuntimeNotice('限价单请输入价格', 'warning');
        return;
    }

    const payload = {
        symbol,
        side,
        size,
        order_type: orderType,
        conditional_type: conditionalType
    };

    if (conditionalType === 'price') {
        const direction = document.getElementById('price-direction').value;
        const triggerPrice = parseFloat(document.getElementById('price-value').value) || 0;

        if (!triggerPrice) {
            showRuntimeNotice('请输入触发价格', 'warning');
            return;
        }

        payload.condition = {
            direction,
            price: triggerPrice
        };
        if (orderType === 'limit') {
            payload.price = price;
        }
    } else if (conditionalType === 'time') {
        const executeTime = document.getElementById('time-value').value;

        if (!executeTime) {
            showRuntimeNotice('请选择执行时间', 'warning');
            return;
        }

        payload.condition = {
            time: executeTime
        };
        if (orderType === 'limit') {
            payload.price = price;
        }
    } else {
        showRuntimeNotice('不支持的条件类型', 'warning');
        return;
    }

    try {
        const response = await fetch('/api/manual/conditional-order', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify(payload)
        });
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '创建条件单失败');
        }
        console.log('条件单创建结果:', result);
        showRuntimeNotice('条件单创建成功！', 'success');
        document.getElementById('conditional-trade-size').value = '';
        document.getElementById('conditional-trade-price').value = '';
        document.getElementById('price-value').value = '';
        document.getElementById('time-value').value = '';
        refreshConditionalOrders();
    } catch (error) {
        console.error('创建条件单失败:', error);
        showRuntimeNotice('创建条件单失败: ' + error.message, 'error');
    }
}

// 刷新条件单列表
async function refreshConditionalOrders() {
    const container = document.getElementById('conditional-orders-list');
    container.innerHTML = '<div class="loading-text">加载中...</div>';

    try {
        const response = await fetch('/api/manual/conditional-orders?status=pending', {
            headers: buildAuthHeaders()
        });
        const result = await response.json();

        if (result.orders && result.orders.length > 0) {
            container.innerHTML = result.orders.map(order => {
                const conditionDesc = formatCondition(order.condition, order.type);
                const statusBadge = order.status.toLowerCase();
                return `
                <div class="order-item">
                    <div class="order-header">
                        <span class="order-symbol">${order.symbol}</span>
                        <span class="badge ${statusBadge}">${order.status}</span>
                    </div>
                    <div class="order-details">
                        <div>方向: <span class="${order.side}">${order.side}</span></div>
                        <div>数量: ${formatNumber(order.size)}</div>
                        <div>类型: ${order.order_type}</div>
                        <div>条件: ${conditionDesc}</div>
                        <div>创建时间: ${formatTime(order.created_at)}</div>
                        ${order.trigger_price ? `<div>触发价格: ${formatNumber(order.trigger_price)}</div>` : ''}
                        ${order.triggered_at ? `<div>触发时间: ${formatTime(order.triggered_at)}</div>` : ''}
                        ${order.order_id ? `<div>关联订单: ${order.order_id}</div>` : ''}
                        ${order.reason ? `<div>原因: ${order.reason}</div>` : ''}
                    </div>
                    <div class="order-actions">
                        ${order.status === 'pending' ?
                            `<button class="btn btn-small btn-danger" onclick="cancelConditionalOrder('${order.id}')">取消</button>` :
                            ''}
                    </div>
                </div>
                `;
            }).join('');
        } else {
            container.innerHTML = '<div class="empty-text">暂无条件单</div>';
        }
    } catch (error) {
        console.error('获取条件单失败:', error);
        container.innerHTML = '<div class="error-text">获取条件单失败</div>';
    }
}

// 格式化条件描述
function formatCondition(condition, orderType) {
    if (!condition) return '未知';
    if (orderType === 'price') {
        const dir = condition.direction === 'above' ? '≥' : '≤';
        return `价格${dir} ${formatNumber(condition.price || 0)}`;
    } else if (orderType === 'time') {
        return `时间到达 ${formatTime(condition.time)}`;
    }
    return JSON.stringify(condition);
}

// 取消条件单
async function cancelConditionalOrder(orderId) {
    showConfirmDialog(
        '确认取消',
        '确定要取消此条件单吗？',
        async () => {
            try {
                const response = await fetch(`/api/manual/conditional-order/${orderId}`, {
                    method: 'DELETE',
                    headers: buildAuthHeaders()
                });
                const result = await response.json();

                if (!response.ok) {
                    throw new Error(result.message || '取消失败');
                }

                showRuntimeNotice('条件单已取消', 'success');
                refreshConditionalOrders();
            } catch (error) {
                console.error('取消条件单失败:', error);
                showRuntimeNotice('取消失败: ' + error.message, 'error');
            }
        }
    );
}

// ==================== 移动止损 ====================

// 打开移动止损设置对话框
function openTrailingStopDialog(symbol) {
    const dialog = document.getElementById('trailing-stop-dialog');
    if (!dialog) {
        // Dialog doesn't exist yet, create it inline
        showRuntimeNotice('请在交易面板"持仓"tab中点击移动止损按钮', 'warning');
        return;
    }
    document.getElementById('trailing-stop-symbol').value = symbol;
    document.getElementById('trailing-stop-distance').value = '';
    dialog.classList.remove('hidden');
}

// 关闭移动止损对话框
function closeTrailingStopDialog() {
    const dialog = document.getElementById('trailing-stop-dialog');
    if (dialog) {
        dialog.classList.add('hidden');
    }
}

// 提交移动止损设置
async function submitTrailingStop() {
    const symbol = document.getElementById('trailing-stop-symbol').value;
    const stopDistance = parseFloat(document.getElementById('trailing-stop-distance').value) || 0;

    if (!stopDistance || stopDistance <= 0) {
        showRuntimeNotice('请输入有效的止损距离', 'warning');
        return;
    }

    try {
        const response = await fetch('/api/manual/position/trailing-stop', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ symbol, stop_distance: stopDistance })
        });
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '设置移动止损失败');
        }
        showRuntimeNotice('移动止损设置成功！', 'success');
        closeTrailingStopDialog();
    } catch (error) {
        console.error('设置移动止损失败:', error);
        showRuntimeNotice('设置移动止损失败: ' + error.message, 'error');
    }
}

// ==================== WebSocket 条件单事件处理 ====================

// 处理条件单状态变更推送
function handleConditionalOrderEvent(data) {
    if (!data) return;
    console.log('条件单状态变更:', data);
    const message = `条件单 ${data.symbol}: ${data.status}`;
    if (data.order_id) {
        showRuntimeNotice(`${message} -> 订单 ${data.order_id}`, 'info');
    } else {
        showRuntimeNotice(message, 'info');
    }
    // 刷新条件单列表
    refreshConditionalOrders();
}
