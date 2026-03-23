// WebSocket连接
let ws = null;
let reconnectTimer = null;
let rebalanceEvents = [];

const MAX_REBALANCE_EVENTS = 12;

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
}

// 提交订单
async function submitOrder() {
    const symbol = document.getElementById('trade-symbol').value;
    const side = document.getElementById('trade-side').value;
    const type = document.getElementById('trade-type').value;
    const price = parseFloat(document.getElementById('trade-price').value) || 0;
    const size = parseFloat(document.getElementById('trade-size').value) || 0;

    if (!size) {
        alert('请输入数量');
        return;
    }

    if (type === 'limit' && !price) {
        alert('限价单必须输入价格');
        return;
    }

    try {
        const response = await fetch('/api/order/create', {
            method: 'POST',
            headers: buildAuthHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ symbol, side, type, price, size })
        });
        const result = await response.json();
        console.log('订单结果:', result);
        toggleTradePanel();
        document.getElementById('trade-price').value = '';
        document.getElementById('trade-size').value = '';
    } catch (error) {
        console.error('下单失败:', error);
    }
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
