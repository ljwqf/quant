// WebSocket连接
let ws = null;
let reconnectTimer = null;
let rebalanceEvents = [];

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
    connectWebSocket();
    fetchInitialData();
    updateTime();
    setInterval(updateTime, 1000);
});

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
        case 'status':
            updateSystemStatus(message.data);
            break;
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
    }
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

// 更新策略列表
function updateStrategies(strategies) {
    const tbody = document.getElementById('strategies-body');
    if (!strategies || strategies.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="loading">暂无策略</td></tr>';
        return;
    }

    tbody.innerHTML = strategies.map(s => `
        <tr>
            <td>${s.name}</td>
            <td><span class="badge ${s.running ? 'running' : 'stopped'}">${s.running ? '运行中' : '已停止'}</span></td>
            <td class="${s.pnl >= 0 ? 'positive' : 'negative'}">${formatNumber(s.pnl)}</td>
            <td>${formatPercent(s.win_rate)}</td>
            <td>${s.trades}</td>
            <td>${s.last_signal || '--'}</td>
            <td>
                <button class="btn btn-small ${s.running ? 'btn-danger' : 'btn-success'}" 
                        onclick="toggleStrategy('${s.name}', ${!s.running})">
                    ${s.running ? '停止' : '启动'}
                </button>
            </td>
        </tr>
    `).join('');
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
            <td>${p.symbol}</td>
            <td><span class="badge ${p.side.toLowerCase()}">${p.side}</span></td>
            <td>${formatNumber(p.size)}</td>
            <td>${formatNumber(p.entry_price)}</td>
            <td>${formatNumber(p.mark_price)}</td>
            <td class="${p.unrealized_pnl >= 0 ? 'positive' : 'negative'}">${formatNumber(p.unrealized_pnl)}</td>
            <td>${p.leverage}x</td>
            <td>${p.strategy}</td>
            <td>
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
            <td>${o.order_id}</td>
            <td>${o.symbol}</td>
            <td><span class="badge ${o.side.toLowerCase()}">${o.side}</span></td>
            <td>${o.type}</td>
            <td>${formatNumber(o.price)}</td>
            <td>${formatNumber(o.size)}</td>
            <td>${formatNumber(o.filled_size)}</td>
            <td><span class="badge ${o.status.toLowerCase()}">${o.status}</span></td>
            <td>${o.strategy}</td>
            <td>${formatTime(o.create_time)}</td>
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
        alert('重置熔断失败: ' + error.message);
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
            <td>${s.id}</td>
            <td>${s.strategy}</td>
            <td>${s.symbol}</td>
            <td><span class="badge ${s.side.toLowerCase()}">${s.side}</span></td>
            <td>${formatNumber(s.price)}</td>
            <td>${formatNumber(s.size)}</td>
            <td>${formatPercent(s.confidence)}</td>
            <td>${s.reason || '--'}</td>
            <td><span class="badge ${s.executed ? 'filled' : 'pending'}">${s.executed ? '已执行' : '待执行'}</span></td>
            <td>${formatTime(s.time)}</td>
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
        alert('请输入数量');
        return;
    }

    if (type === 'limit' && !price) {
        alert('限价单必须输入价格');
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
        alert('订单提交成功！');
        toggleTradePanel();
        document.getElementById('trade-price').value = '';
        document.getElementById('trade-size').value = '';
        document.getElementById('trade-tp').value = '';
        document.getElementById('trade-sl').value = '';
    } catch (error) {
        console.error('下单失败:', error);
        alert('下单失败: ' + error.message);
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
        
        alert('订单已撤销');
        refreshManualOrders();
    } catch (error) {
        console.error('撤销订单失败:', error);
        alert('撤销失败: ' + error.message);
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
    if (!confirm(`确定要平仓 ${symbol} 吗？`)) return;
    
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
        
        alert('平仓订单已提交');
        refreshManualPositions();
    } catch (error) {
        console.error('平仓失败:', error);
        alert('平仓失败: ' + error.message);
    }
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

function showRuntimeNotice(message, type = 'info') {
    let container = document.getElementById('runtime-notice-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'runtime-notice-container';
        container.style.position = 'fixed';
        container.style.top = '16px';
        container.style.right = '16px';
        container.style.zIndex = '2000';
        container.style.display = 'flex';
        container.style.flexDirection = 'column';
        container.style.gap = '8px';
        document.body.appendChild(container);
    }

    const notice = document.createElement('div');
    notice.textContent = message;
    notice.style.padding = '10px 14px';
    notice.style.borderRadius = '8px';
    notice.style.color = '#fff';
    notice.style.boxShadow = '0 8px 20px rgba(0,0,0,0.15)';
    notice.style.maxWidth = '360px';
    notice.style.fontSize = '14px';
    notice.style.background = type === 'success' ? '#157347' : (type === 'error' ? '#b02a37' : '#0d6efd');
    container.appendChild(notice);

    setTimeout(() => {
        notice.remove();
        if (container && container.childElementCount === 0) {
            container.remove();
        }
    }, 4000);
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
    const symbol = document.getElementById('llm-market-symbol').value;
    
    if (!symbol) {
        alert('请输入交易对');
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
        alert('请输入数量');
        return;
    }

    if (!executeTime) {
        alert('请选择执行时间');
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
        alert('限时单创建成功！');
        document.getElementById('timed-trade-size').value = '';
        document.getElementById('timed-trade-time').value = '';
        refreshTimedOrders();
    } catch (error) {
        console.error('创建限时单失败:', error);
        alert('创建限时单失败: ' + error.message);
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
    if (!confirm('确定要取消此限时单吗？')) return;
    
    try {
        const response = await fetch(`/api/manual/timed-order/${orderId}`, {
            method: 'DELETE',
            headers: buildAuthHeaders()
        });
        const result = await response.json();
        
        if (!response.ok) {
            throw new Error(result.message || '取消失败');
        }
        
        alert('限时单已取消');
        refreshTimedOrders();
    } catch (error) {
        console.error('取消限时单失败:', error);
        alert('取消失败: ' + error.message);
    }
}
