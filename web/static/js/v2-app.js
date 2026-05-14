const statusEl = document.getElementById('v2-connection-status');

function setText(id, value) {
    const el = document.getElementById(id);
    if (el) {
        el.textContent = value;
    }
}

function formatMoney(value) {
    const num = Number(value || 0);
    return num.toFixed(2);
}

function formatBool(value) {
    return value ? '是' : '否';
}

function paperStatsValue(stats, key) {
    if (!stats || typeof stats !== 'object') {
        return 0;
    }
    return stats[key] ?? stats[key.replace('_', '')] ?? 0;
}

function renderSymbolStates(status) {
    const tbody = document.getElementById('v2-symbol-states');
    if (!tbody) {
        return;
    }

    const rows = Object.entries(status)
        .filter(([key]) => key.endsWith('_state'))
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([key, state]) => {
            const symbol = key.slice(0, -'_state'.length);
            return `<tr><td>${symbol}</td><td>${state || '--'}</td></tr>`;
        });

    tbody.innerHTML = rows.length > 0 ? rows.join('') : '<tr><td colspan="2">暂无策略状态</td></tr>';
}

async function refreshV2Status() {
    try {
        const response = await fetch('/api/v2/status');
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const status = await response.json();
        statusEl.textContent = '已连接';
        statusEl.className = 'status-indicator connected';

        setText('v2-enabled', formatBool(status.enabled));
        setText('v2-mode', status.mode || '--');
        setText('v2-running', formatBool(status.running));
        setText('v2-read-only', formatBool(status.read_only));
        setText('v2-base-capital', formatMoney(status.base_capital));
        setText('v2-profit-pool', formatMoney(status.profit_pool));
        setText('v2-paper-orders', paperStatsValue(status.paper_stats, 'orders'));
        setText('v2-paper-pnl', formatMoney(paperStatsValue(status.paper_stats, 'pnl')));
        renderSymbolStates(status);
    } catch (error) {
        statusEl.textContent = '连接失败';
        statusEl.className = 'status-indicator disconnected';
    }
}

refreshV2Status();
setInterval(refreshV2Status, 5000);
