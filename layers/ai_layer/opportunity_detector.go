package ai_layer

import (
	"encoding/json"
	"fmt"
	"nofx/layers"
	"nofx/mcp"
	"time"
)

// OpportunityDetector 交易机会识别器（AI层）
// 职责：交易机会识别（现在该做什么？）
type OpportunityDetector struct {
	config    layers.AILayerConfig
	mcpClient *mcp.Client
}

// NewOpportunityDetector 创建交易机会识别器
func NewOpportunityDetector(config layers.AILayerConfig) (*OpportunityDetector, error) {
	provider := mcp.Provider(config.Provider)
	client := mcp.NewClient(provider, config.APIKey, config.BaseURL, config.Model)

	return &OpportunityDetector{
		config:    config,
		mcpClient: client,
	}, nil
}

// DetectOpportunity 识别交易机会
// 输入：市场状态、清洗后的市场数据
// 输出：交易机会类型和原因
func (od *OpportunityDetector) DetectOpportunity(
	marketCondition layers.MarketCondition,
	marketData *layers.CleanedMarketData,
) (layers.TradingOpportunity, string, error) {
	if marketData == nil {
		return layers.OpportunityNone, "", fmt.Errorf("market data is nil")
	}

	// 构建AI提示词
	systemPrompt := `你是交易机会识别专家。识别当前交易机会，只返回JSON：
{"opportunity":"long_entry/short_entry/long_exit/short_exit/scalp/none","reason":"100字以内"}`

	userPrompt := fmt.Sprintf(`市场状态：%s
市场数据：%s

识别交易机会：
- long_entry: 做多入场
- short_entry: 做空入场
- long_exit: 多单出场
- short_exit: 空单出场
- scalp: 剥头皮
- none: 无机会

只返回JSON`, marketCondition, marketData.CompressedSummary)

	// 调用AI
	startTime := time.Now()
	response, err := od.mcpClient.SendMessage(systemPrompt, userPrompt)
	if err != nil {
		return layers.OpportunityNone, "", fmt.Errorf("AI call failed: %w", err)
	}

	responseTime := time.Since(startTime).Milliseconds()
	fmt.Printf("[AI Layer] Opportunity detection took %dms\n", responseTime)

	// 解析AI响应
	var result struct {
		Opportunity string `json:"opportunity"`
		Reason      string `json:"reason"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return od.parseOpportunityFromText(response)
	}

	// 验证opportunity
	opportunity := layers.TradingOpportunity(result.Opportunity)
	if !isValidOpportunity(opportunity) {
		return layers.OpportunityNone, "AI返回无效机会类型", nil
	}

	return opportunity, result.Reason, nil
}

// DetectOpportunityWithTechnicals 使用技术指标识别机会（快速版）
func (od *OpportunityDetector) DetectOpportunityWithTechnicals(
	marketCondition layers.MarketCondition,
	marketData *layers.CleanedMarketData,
) (layers.TradingOpportunity, string) {
	// 根据市场状态选择策略
	switch marketCondition {
	case layers.MarketTrending:
		return od.detectTrendingOpportunity(marketData)
	case layers.MarketBreakout:
		return od.detectBreakoutOpportunity(marketData)
	case layers.MarketRanging:
		return od.detectRangingOpportunity(marketData)
	case layers.MarketConsolidate:
		return od.detectConsolidateOpportunity(marketData)
	case layers.MarketVolatile:
		return od.detectVolatileOpportunity(marketData)
	}

	return layers.OpportunityNone, "未知市场状态"
}

// detectTrendingOpportunity 趋势市场机会
func (od *OpportunityDetector) detectTrendingOpportunity(data *layers.CleanedMarketData) (layers.TradingOpportunity, string) {
	// 上升趋势
	if data.EMA20 > data.EMA50 {
		// 回调买入
		if data.CurrentPrice < data.EMA20 && data.RSI14 < 40 {
			return layers.OpportunityLongEntry, "上升趋势回调至EMA20，RSI超卖，做多机会"
		}

		// MACD金叉
		if data.MACD > data.MACDSignal && data.MACD > 0 {
			return layers.OpportunityLongEntry, "上升趋势中MACD金叉，做多机会"
		}

		// 趋势延续
		if data.CurrentPrice > data.EMA20 && data.RSI14 > 50 && data.RSI14 < 70 {
			return layers.OpportunityLongEntry, "上升趋势延续，价格在EMA20上方，RSI健康"
		}
	}

	// 下降趋势
	if data.EMA20 < data.EMA50 {
		// 反弹卖出
		if data.CurrentPrice > data.EMA20 && data.RSI14 > 60 {
			return layers.OpportunityShortEntry, "下降趋势反弹至EMA20，RSI超买，做空机会"
		}

		// MACD死叉
		if data.MACD < data.MACDSignal && data.MACD < 0 {
			return layers.OpportunityShortEntry, "下降趋势中MACD死叉，做空机会"
		}

		// 趋势延续
		if data.CurrentPrice < data.EMA20 && data.RSI14 < 50 && data.RSI14 > 30 {
			return layers.OpportunityShortEntry, "下降趋势延续，价格在EMA20下方"
		}
	}

	return layers.OpportunityNone, "趋势市场但无明确入场点"
}

// detectBreakoutOpportunity 突破市场机会
func (od *OpportunityDetector) detectBreakoutOpportunity(data *layers.CleanedMarketData) (layers.TradingOpportunity, string) {
	// 向上突破
	if data.CurrentPrice > data.EMA20 && data.CurrentPrice > data.EMA50 {
		if data.VolumeChange > 50 && data.RSI14 > 55 {
			return layers.OpportunityLongEntry, "向上突破，成交量放大，做多机会"
		}
	}

	// 向下突破
	if data.CurrentPrice < data.EMA20 && data.CurrentPrice < data.EMA50 {
		if data.VolumeChange > 50 && data.RSI14 < 45 {
			return layers.OpportunityShortEntry, "向下突破，成交量放大，做空机会"
		}
	}

	return layers.OpportunityNone, "突破尚未确认"
}

// detectRangingOpportunity 震荡市场机会
func (od *OpportunityDetector) detectRangingOpportunity(data *layers.CleanedMarketData) (layers.TradingOpportunity, string) {
	// 震荡市场：高抛低吸
	if data.RSI14 < 30 {
		return layers.OpportunityLongEntry, "震荡市场RSI超卖，低吸机会"
	}

	if data.RSI14 > 70 {
		return layers.OpportunityShortEntry, "震荡市场RSI超买，高抛机会"
	}

	// 支撑位附近
	if data.CurrentPrice < data.EMA50 && data.RSI14 < 40 {
		return layers.OpportunityLongEntry, "价格接近支撑位EMA50，超卖"
	}

	// 阻力位附近
	if data.CurrentPrice > data.EMA50 && data.RSI14 > 60 {
		return layers.OpportunityShortEntry, "价格接近阻力位EMA50，超买"
	}

	return layers.OpportunityNone, "震荡市场，等待更好入场点"
}

// detectConsolidateOpportunity 整理市场机会
func (od *OpportunityDetector) detectConsolidateOpportunity(data *layers.CleanedMarketData) (layers.TradingOpportunity, string) {
	// 整理市场：等待突破
	// 通常不建议在整理阶段交易
	return layers.OpportunityNone, "整理阶段，等待方向选择"
}

// detectVolatileOpportunity 高波动市场机会
func (od *OpportunityDetector) detectVolatileOpportunity(data *layers.CleanedMarketData) (layers.TradingOpportunity, string) {
	// 高波动市场：剥头皮机会
	if data.RSI7 < 25 || data.RSI7 > 75 {
		return layers.OpportunityScalp, "高波动市场，短期反转机会"
	}

	return layers.OpportunityNone, "高波动市场，风险过高，观望"
}

// parseOpportunityFromText 从文本解析机会
func (od *OpportunityDetector) parseOpportunityFromText(text string) (layers.TradingOpportunity, string, error) {
	if contains(text, "long_entry") || contains(text, "做多") || contains(text, "买入") {
		return layers.OpportunityLongEntry, "从文本解析：做多机会", nil
	}
	if contains(text, "short_entry") || contains(text, "做空") || contains(text, "卖出") {
		return layers.OpportunityShortEntry, "从文本解析：做空机会", nil
	}
	if contains(text, "long_exit") || contains(text, "平多") {
		return layers.OpportunityLongExit, "从文本解析：多单出场", nil
	}
	if contains(text, "short_exit") || contains(text, "平空") {
		return layers.OpportunityShortExit, "从文本解析：空单出场", nil
	}
	if contains(text, "scalp") || contains(text, "剥头皮") {
		return layers.OpportunityScalp, "从文本解析：剥头皮", nil
	}

	return layers.OpportunityNone, "无交易机会", nil
}

// isValidOpportunity 验证机会类型
func isValidOpportunity(opp layers.TradingOpportunity) bool {
	switch opp {
	case layers.OpportunityLongEntry, layers.OpportunityShortEntry,
		layers.OpportunityLongExit, layers.OpportunityShortExit,
		layers.OpportunityScalp, layers.OpportunityNone:
		return true
	}
	return false
}

// GetOpportunityDescription 获取机会描述
func GetOpportunityDescription(opp layers.TradingOpportunity) string {
	descriptions := map[layers.TradingOpportunity]string{
		layers.OpportunityLongEntry:  "做多入场 - 适合开多仓",
		layers.OpportunityShortEntry: "做空入场 - 适合开空仓",
		layers.OpportunityLongExit:   "多单出场 - 应平多仓",
		layers.OpportunityShortExit:  "空单出场 - 应平空仓",
		layers.OpportunityScalp:      "剥头皮机会 - 短期快进快出",
		layers.OpportunityNone:       "无交易机会 - 观望为主",
	}

	if desc, ok := descriptions[opp]; ok {
		return desc
	}
	return "未知机会类型"
}
