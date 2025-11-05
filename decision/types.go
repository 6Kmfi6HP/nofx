package decision

import (
	"nofx/market"
	"time"
)

// ==================== AI 决策相关类型 ====================

// AIDecisionSignal AI 决策信号 - 纯粹的 AI 输出
// 这是 AI 层的标准输出格式，只包含决策核心信息
type AIDecisionSignal struct {
	Symbol     string  `json:"symbol"`     // 币种
	Action     string  `json:"action"`     // 动作: BUY, SELL, HOLD
	Confidence float64 `json:"confidence"` // 信心度 (0.0-1.0)
	Reasoning  string  `json:"reasoning"`  // 决策理由（简短）
}

// AIAnalysisResult AI 分析结果
// 包含 AI 的完整分析过程和输出
type AIAnalysisResult struct {
	// 市场状态判断
	MarketState     string  `json:"market_state"`      // 趋势/震荡/突破
	MarketConfidence float64 `json:"market_confidence"` // 市场状态信心度

	// 交易机会识别
	TradingOpportunities []AIDecisionSignal `json:"trading_opportunities"`

	// 思维链（AI 的分析过程）
	CoTTrace string `json:"cot_trace"`

	// 分析时间戳
	Timestamp time.Time `json:"timestamp"`
}

// ==================== 三层架构数据流类型 ====================

// TradingContext 交易上下文 - 传递给各层的完整信息
type TradingContext struct {
	// 时间信息
	CurrentTime    string `json:"current_time"`
	RuntimeMinutes int    `json:"runtime_minutes"`
	CallCount      int    `json:"call_count"`

	// 账户信息
	Account AccountInfo `json:"account"`

	// 持仓信息
	Positions []PositionInfo `json:"positions"`

	// 候选币种
	CandidateCoins []CandidateCoin `json:"candidate_coins"`

	// 市场数据映射（不序列化，内部使用）
	MarketDataMap map[string]*market.Data `json:"-"`

	// OI Top 数据映射
	OITopDataMap map[string]*OITopData `json:"-"`

	// 历史表现分析
	Performance interface{} `json:"-"`

	// 杠杆配置
	BTCETHLeverage  int `json:"-"`
	AltcoinLeverage int `json:"-"`
}

// StrategyDecision 策略决策 - 上层策略控制层的输出
// 包含完整的交易参数（在 AI 决策基础上计算出的具体参数）
type StrategyDecision struct {
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"` // open_long, open_short, close_long, close_short, hold, wait
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	Confidence      int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD         float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning       string  `json:"reasoning"`

	// 风险评估信息（由策略层计算）
	RiskRewardRatio  float64 `json:"risk_reward_ratio,omitempty"`
	MarginRequired   float64 `json:"margin_required,omitempty"`
	LiquidationPrice float64 `json:"liquidation_price,omitempty"`
}

// ==================== 共享数据类型 ====================

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长 Top 数据（用于 AI 决策参考）
type OITopData struct {
	Rank              int     // OI Top 排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// ==================== 兼容旧代码的类型别名 ====================

// Context 旧代码兼容：交易上下文
// 为了保持向后兼容，保留原有的 Context 类型名
type Context = TradingContext

// Decision 旧代码兼容：AI 的交易决策
// 实际上这应该是 StrategyDecision，但为了兼容保留
type Decision = StrategyDecision

// FullDecision AI 的完整决策（包含思维链）- 旧代码兼容
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // 系统提示词（发送给AI的系统prompt）
	UserPrompt   string     `json:"user_prompt"`   // 发送给AI的输入prompt
	CoTTrace     string     `json:"cot_trace"`     // 思维链分析（AI输出）
	Decisions    []Decision `json:"decisions"`     // 具体决策列表
	Timestamp    time.Time  `json:"timestamp"`
}
