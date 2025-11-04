package layers

import "time"

// ========================================
// 三层架构核心类型定义
// ========================================

// MarketCondition 市场状态
type MarketCondition string

const (
	MarketTrending    MarketCondition = "trending"     // 趋势市场
	MarketRanging     MarketCondition = "ranging"      // 震荡市场
	MarketVolatile    MarketCondition = "volatile"     // 高波动市场
	MarketConsolidate MarketCondition = "consolidate"  // 整理市场
	MarketBreakout    MarketCondition = "breakout"     // 突破市场
)

// TradingOpportunity 交易机会类型
type TradingOpportunity string

const (
	OpportunityLongEntry   TradingOpportunity = "long_entry"    // 做多入场
	OpportunityShortEntry  TradingOpportunity = "short_entry"   // 做空入场
	OpportunityLongExit    TradingOpportunity = "long_exit"     // 多单出场
	OpportunityShortExit   TradingOpportunity = "short_exit"    // 空单出场
	OpportunityScalp       TradingOpportunity = "scalp"         // 剥头皮
	OpportunityNone        TradingOpportunity = "none"          // 无机会
)

// Direction 交易方向
type Direction string

const (
	DirectionLong  Direction = "long"   // 做多
	DirectionShort Direction = "short"  // 做空
	DirectionWait  Direction = "wait"   // 观望
)

// ========================================
// 底层数据层 - 输出数据结构
// ========================================

// CleanedMarketData 清洗后的市场数据（底层 → AI层）
type CleanedMarketData struct {
	Symbol            string    `json:"symbol"`
	Timestamp         time.Time `json:"timestamp"`

	// 价格数据
	CurrentPrice      float64   `json:"current_price"`
	PriceChange1h     float64   `json:"price_change_1h"`
	PriceChange4h     float64   `json:"price_change_4h"`
	PriceChange24h    float64   `json:"price_change_24h"`

	// 技术指标
	EMA20             float64   `json:"ema_20"`
	EMA50             float64   `json:"ema_50"`
	MACD              float64   `json:"macd"`
	MACDSignal        float64   `json:"macd_signal"`
	RSI7              float64   `json:"rsi_7"`
	RSI14             float64   `json:"rsi_14"`
	ATR               float64   `json:"atr"`

	// 成交量和持仓量
	Volume24h         float64   `json:"volume_24h"`
	VolumeChange      float64   `json:"volume_change"`
	OpenInterest      float64   `json:"open_interest"`
	OIChange          float64   `json:"oi_change"`
	FundingRate       float64   `json:"funding_rate"`

	// 市场情绪
	LongShortRatio    float64   `json:"long_short_ratio"`
	TopTraderRatio    float64   `json:"top_trader_ratio"`

	// 数据质量标记
	DataQuality       float64   `json:"data_quality"` // 0-1
	IsValid           bool      `json:"is_valid"`

	// 压缩的历史数据（650字符以内，供AI使用）
	CompressedSummary string    `json:"compressed_summary"`
}

// RiskMetrics 风险指标（底层计算）
type RiskMetrics struct {
	Symbol              string  `json:"symbol"`

	// 仓位风险
	MaxPositionSizeUSD  float64 `json:"max_position_size_usd"`  // 最大仓位（USD）
	RecommendedLeverage int     `json:"recommended_leverage"`   // 建议杠杆

	// 止损止盈
	StopLossPrice       float64 `json:"stop_loss_price"`        // 止损价格
	TakeProfitPrice     float64 `json:"take_profit_price"`      // 止盈价格
	MaxLossUSD          float64 `json:"max_loss_usd"`           // 最大亏损

	// 保证金
	RequiredMargin      float64 `json:"required_margin"`        // 所需保证金
	MarginUsagePercent  float64 `json:"margin_usage_percent"`   // 保证金使用率

	// 风险评估
	RiskLevel           string  `json:"risk_level"`             // low/medium/high/extreme
	CanTrade            bool    `json:"can_trade"`              // 是否可交易
	RiskReason          string  `json:"risk_reason"`            // 风险原因
}

// ========================================
// AI层 - 输出数据结构
// ========================================

// AIDecision AI决策（AI层 → 执行层）
type AIDecision struct {
	Symbol            string             `json:"symbol"`
	Timestamp         time.Time          `json:"timestamp"`

	// 市场状态判断
	MarketCondition   MarketCondition    `json:"market_condition"`
	ConditionReason   string             `json:"condition_reason"`  // 100字以内

	// 交易机会识别
	Opportunity       TradingOpportunity `json:"opportunity"`
	OpportunityReason string             `json:"opportunity_reason"` // 100字以内

	// 方向和信心度
	Direction         Direction          `json:"direction"`
	Confidence        float64            `json:"confidence"`        // 0.7-1.0

	// AI思维链（可选，调试用）
	ChainOfThought    string             `json:"chain_of_thought,omitempty"`

	// 元数据
	ModelUsed         string             `json:"model_used"`
	ResponseTimeMs    int64              `json:"response_time_ms"`
}

// ========================================
// 执行层 - 输出数据结构
// ========================================

// ExecutionPlan 执行计划（执行层 → 交易所）
type ExecutionPlan struct {
	Symbol            string    `json:"symbol"`
	Timestamp         time.Time `json:"timestamp"`

	// 订单参数
	Action            string    `json:"action"` // open_long/open_short/close_long/close_short
	Quantity          float64   `json:"quantity"`
	QuantityUSD       float64   `json:"quantity_usd"`
	Leverage          int       `json:"leverage"`

	// 止损止盈
	StopLoss          float64   `json:"stop_loss"`
	TakeProfit        float64   `json:"take_profit"`

	// 风控参数
	MaxSlippagePercent float64  `json:"max_slippage_percent"`
	TimeoutSeconds    int       `json:"timeout_seconds"`

	// 二次风控验证结果
	RiskCheckPassed   bool      `json:"risk_check_passed"`
	RiskCheckReason   string    `json:"risk_check_reason"`

	// 执行优先级
	Priority          string    `json:"priority"` // high/normal/low

	// 来源决策
	SourceDecision    *AIDecision `json:"source_decision,omitempty"`
}

// OrderResult 订单执行结果
type OrderResult struct {
	Success           bool      `json:"success"`
	OrderID           string    `json:"order_id"`
	FilledQuantity    float64   `json:"filled_quantity"`
	AvgPrice          float64   `json:"avg_price"`
	ExecutionTimeMs   int64     `json:"execution_time_ms"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
}

// ========================================
// 配置结构
// ========================================

// LayerConfig 三层架构配置
type LayerConfig struct {
	// 底层配置
	DataLayer DataLayerConfig `json:"data_layer"`

	// AI层配置
	AILayer AILayerConfig `json:"ai_layer"`

	// 执行层配置
	ExecutionLayer ExecutionLayerConfig `json:"execution_layer"`
}

// DataLayerConfig 底层配置
type DataLayerConfig struct {
	// 数据源
	DataSources       []string `json:"data_sources"`  // ["binance", "hyperliquid"]

	// 数据质量阈值
	MinDataQuality    float64  `json:"min_data_quality"` // 0.8

	// 风险参数
	MaxAccountRiskPercent float64 `json:"max_account_risk_percent"` // 2%
	MaxSingleTradeRiskPercent float64 `json:"max_single_trade_risk_percent"` // 1%
	DefaultLeverage   int      `json:"default_leverage"` // 3
	MaxLeverage       int      `json:"max_leverage"`     // 5

	// 熔断机制
	CircuitBreakerEnabled bool `json:"circuit_breaker_enabled"`
	MaxDailyLossPercent   float64 `json:"max_daily_loss_percent"` // 5%
	MaxConsecutiveLosses  int     `json:"max_consecutive_losses"` // 3
}

// AILayerConfig AI层配置
type AILayerConfig struct {
	// AI提供商
	Provider          string   `json:"provider"` // "deepseek", "qwen", "custom"
	Model             string   `json:"model"`
	APIKey            string   `json:"api_key"`
	BaseURL           string   `json:"base_url,omitempty"`

	// 决策参数
	MinConfidence     float64  `json:"min_confidence"`     // 0.75
	EnableChainOfThought bool  `json:"enable_chain_of_thought"`
	MaxPromptLength   int      `json:"max_prompt_length"`  // 650

	// 频率控制
	MaxDecisionsPerHour int    `json:"max_decisions_per_hour"` // 2
	CooldownMinutes     int    `json:"cooldown_minutes"`       // 30
}

// ExecutionLayerConfig 执行层配置
type ExecutionLayerConfig struct {
	// 风控参数
	EnableSecondaryRiskCheck bool    `json:"enable_secondary_risk_check"`
	MaxSlippagePercent       float64 `json:"max_slippage_percent"` // 0.5%
	OrderTimeoutSeconds      int     `json:"order_timeout_seconds"` // 30

	// 仓位管理
	EnablePositionSizing     bool    `json:"enable_position_sizing"`
	PositionSizingMethod     string  `json:"position_sizing_method"` // "fixed", "kelly", "volatility"

	// 执行模式
	DryRun                   bool    `json:"dry_run"` // 模拟执行
	RequireManualConfirmation bool   `json:"require_manual_confirmation"`
}
