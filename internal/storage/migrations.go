package storage

type migration struct {
	version string
	name    string
	up      string
}

var migrations = []migration{
	{
		version: "001",
		name:    "create_schema_migrations_table",
		up: `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
	{
		version: "002",
		name:    "create_manual_trades_table",
		up: `
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
			CREATE INDEX IF NOT EXISTS idx_manual_trades_order_id ON manual_trades(order_id);
			CREATE INDEX IF NOT EXISTS idx_manual_trades_symbol ON manual_trades(symbol);
			CREATE INDEX IF NOT EXISTS idx_manual_trades_status ON manual_trades(status);
			CREATE INDEX IF NOT EXISTS idx_manual_trades_created_at ON manual_trades(created_at);
		`,
	},
	{
		version: "003",
		name:    "create_ai_analyses_table",
		up: `
			CREATE TABLE IF NOT EXISTS ai_analyses (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				symbol TEXT,
				analysis_type TEXT NOT NULL,
				provider TEXT NOT NULL,
				model TEXT NOT NULL,
				prompt TEXT,
				content TEXT NOT NULL,
				risk_level TEXT NOT NULL,
				suggestions TEXT,
				warnings TEXT,
				confidence_score DECIMAL,
				prompt_tokens INTEGER,
				completion_tokens INTEGER,
				total_tokens INTEGER,
				latency_ms INTEGER,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_ai_analyses_symbol ON ai_analyses(symbol);
			CREATE INDEX IF NOT EXISTS idx_ai_analyses_type ON ai_analyses(analysis_type);
			CREATE INDEX IF NOT EXISTS idx_ai_analyses_provider ON ai_analyses(provider);
			CREATE INDEX IF NOT EXISTS idx_ai_analyses_created_at ON ai_analyses(created_at);
		`,
	},
	{
		version: "004",
		name:    "create_news_events_table",
		up: `
			CREATE TABLE IF NOT EXISTS news_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				external_id TEXT,
				title TEXT NOT NULL,
				summary TEXT,
				content TEXT,
				source TEXT NOT NULL,
				url TEXT,
				image_url TEXT,
				category TEXT,
				tags TEXT,
				importance INTEGER NOT NULL DEFAULT 1,
				sentiment TEXT,
				related_symbols TEXT,
				published_at DATETIME NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_news_events_external_id ON news_events(external_id);
			CREATE INDEX IF NOT EXISTS idx_news_events_source ON news_events(source);
			CREATE INDEX IF NOT EXISTS idx_news_events_importance ON news_events(importance);
			CREATE INDEX IF NOT EXISTS idx_news_events_published_at ON news_events(published_at);
		`,
	},
	{
		version: "005",
		name:    "create_economic_events_table",
		up: `
			CREATE TABLE IF NOT EXISTS economic_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				external_id TEXT,
				title TEXT NOT NULL,
				country TEXT,
				currency TEXT,
				indicator TEXT,
				actual DECIMAL,
				forecast DECIMAL,
				previous DECIMAL,
				unit TEXT,
				importance INTEGER NOT NULL DEFAULT 1,
				impact TEXT,
				event_time DATETIME NOT NULL,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_economic_events_external_id ON economic_events(external_id);
			CREATE INDEX IF NOT EXISTS idx_economic_events_country ON economic_events(country);
			CREATE INDEX IF NOT EXISTS idx_economic_events_importance ON economic_events(importance);
			CREATE INDEX IF NOT EXISTS idx_economic_events_event_time ON economic_events(event_time);
		`,
	},
	{
		version: "006",
		name:    "create_alert_records_table",
		up: `
			CREATE TABLE IF NOT EXISTS alert_records (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				alert_type TEXT NOT NULL,
				level TEXT NOT NULL,
				title TEXT NOT NULL,
				message TEXT NOT NULL,
				symbol TEXT,
				metadata TEXT,
				channels TEXT NOT NULL,
				read BOOLEAN NOT NULL DEFAULT 0,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_alert_records_type ON alert_records(alert_type);
			CREATE INDEX IF NOT EXISTS idx_alert_records_level ON alert_records(level);
			CREATE INDEX IF NOT EXISTS idx_alert_records_symbol ON alert_records(symbol);
			CREATE INDEX IF NOT EXISTS idx_alert_records_read ON alert_records(read);
			CREATE INDEX IF NOT EXISTS idx_alert_records_created_at ON alert_records(created_at);
		`,
	},
}
