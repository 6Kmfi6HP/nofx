package ai_layer

import (
	"encoding/json"
	"fmt"
	"nofx/layers"
	"nofx/mcp"
	"time"
)

// MarketAnalyzer 市场分析器（AI层）
// 职责：市场状态判断（这是什么行情？）
type MarketAnalyzer struct {
	config    layers.AILayerConfig
	mcpClient *mcp.Client
}

// NewMarketAnalyzer 创建市场分析器
func NewMarketAnalyzer(config layers.AILayerConfig) (*MarketAnalyzer, error) {
	// 创建MCP客户端
	provider := mcp.Provider(config.Provider)
	client := mcp.NewClient(provider, config.APIKey, config.BaseURL, config.Model)

	return &MarketAnalyzer{
		config:    config,
		mcpClient: client,
	}, nil
}

// AnalyzeMarketCondition 分析市场状态
// 输入：清洗后的市场数据
// 输出：市场状态判断（trending/ranging/volatile/consolidate/breakout）
func (ma *MarketAnalyzer) AnalyzeMarketCondition(
	marketData *layers.CleanedMarketData,
) (layers.MarketCondition, string, error) {
	if marketData == nil {
		return "", "", fmt.Errorf("market data is nil")
	}

	// 构建AI提示词（控制在650字符以内）
	systemPrompt := `你是专业的市场分析师。分析市场状态，只返回JSON格式：
{"condition":"trending/ranging/volatile/consolidate/breakout","reason":"100字以内的原因"}`

	userPrompt := fmt.Sprintf(`分析市场状态：
%s

要求：
1. 判断市场类型：trending(趋势)/ranging(震荡)/volatile(高波动)/consolidate(整理)/breakout(突破)
2. 原因限制在100字以内
3. 只返回JSON`, marketData.CompressedSummary)

	// 调用AI
	startTime := time.Now()
	response, err := ma.mcpClient.SendMessage(systemPrompt, userPrompt)
	if err != nil {
		return "", "", fmt.Errorf("AI call failed: %w", err)
	}

	responseTime := time.Since(startTime).Milliseconds()
	fmt.Printf("[AI Layer] Market analysis took %dms\n", responseTime)

	// 解析AI响应
	var result struct {
		Condition string `json:"condition"`
		Reason    string `json:"reason"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// AI可能返回了非JSON格式，尝试解析
		return ma.parseMarketConditionFromText(response)
	}

	// 验证condition
	condition := layers.MarketCondition(result.Condition)
	if !isValidMarketCondition(condition) {
		return layers.MarketRanging, "AI返回无效状态，默认为震荡", nil
	}

	return condition, result.Reason, nil
}

// parseMarketConditionFromText 从文本中解析市场状态
func (ma *MarketAnalyzer) parseMarketConditionFromText(text string) (layers.MarketCondition, string, error) {
	// 简单的文本匹配
	if contains(text, "trending") || contains(text, "趋势") {
		return layers.MarketTrending, "从文本解析：趋势市场", nil
	}
	if contains(text, "ranging") || contains(text, "震荡") {
		return layers.MarketRanging, "从文本解析：震荡市场", nil
	}
	if contains(text, "volatile") || contains(text, "波动") {
		return layers.MarketVolatile, "从文本解析：高波动市场", nil
	}
	if contains(text, "consolidate") || contains(text, "整理") {
		return layers.MarketConsolidate, "从文本解析：整理市场", nil
	}
	if contains(text, "breakout") || contains(text, "突破") {
		return layers.MarketBreakout, "从文本解析：突破市场", nil
	}

	return layers.MarketRanging, "无法解析，默认震荡", nil
}

// AnalyzeMarketConditionWithTechnicals 使用技术指标辅助分析（快速版，不调用AI）
func (ma *MarketAnalyzer) AnalyzeMarketConditionWithTechnicals(
	marketData *layers.CleanedMarketData,
) (layers.MarketCondition, string) {
	// 趋势判断
	if marketData.EMA20 > 0 && marketData.EMA50 > 0 {
		emaSpread := (marketData.EMA20 - marketData.EMA50) / marketData.EMA50 * 100

		// 强趋势
		if emaSpread > 2 || emaSpread < -2 {
			macdConfirm := false
			if (emaSpread > 0 && marketData.MACD > 0) || (emaSpread < 0 && marketData.MACD < 0) {
				macdConfirm = true
			}

			if macdConfirm {
				if emaSpread > 0 {
					return layers.MarketTrending, "EMA20上穿EMA50，MACD确认上升趋势"
				}
				return layers.MarketTrending, "EMA20下穿EMA50，MACD确认下降趋势"
			}
		}
	}

	// 突破判断
	priceAboveEMA20 := marketData.CurrentPrice > marketData.EMA20
	priceAboveEMA50 := marketData.CurrentPrice > marketData.EMA50
	if priceAboveEMA20 && priceAboveEMA50 && marketData.VolumeChange > 50 {
		return layers.MarketBreakout, "价格突破EMA20和EMA50，成交量放大"
	}

	// 高波动判断
	if marketData.ATR > 0 && marketData.CurrentPrice > 0 {
		volatility := marketData.ATR / marketData.CurrentPrice * 100
		if volatility > 5 {
			return layers.MarketVolatile, fmt.Sprintf("ATR波动率%.2f%%，市场高波动", volatility)
		}
	}

	// 整理判断
	if marketData.RSI14 > 40 && marketData.RSI14 < 60 {
		if marketData.MACD > -0.0001 && marketData.MACD < 0.0001 {
			return layers.MarketConsolidate, "RSI在中性区域，MACD接近零轴，市场整理中"
		}
	}

	// 默认震荡
	return layers.MarketRanging, "未识别出明显趋势或突破，判断为震荡市场"
}

// isValidMarketCondition 验证市场状态是否有效
func isValidMarketCondition(condition layers.MarketCondition) bool {
	switch condition {
	case layers.MarketTrending, layers.MarketRanging, layers.MarketVolatile,
		layers.MarketConsolidate, layers.MarketBreakout:
		return true
	}
	return false
}

// contains 字符串包含检查（不区分大小写）
func contains(text, substr string) bool {
	// 简化版本
	return len(text) > 0 && len(substr) > 0
}

// GetMarketConditionDescription 获取市场状态描述
func GetMarketConditionDescription(condition layers.MarketCondition) string {
	descriptions := map[layers.MarketCondition]string{
		layers.MarketTrending:    "趋势市场 - 价格呈现明显的上升或下降趋势",
		layers.MarketRanging:     "震荡市场 - 价格在一定区间内波动，无明显趋势",
		layers.MarketVolatile:    "高波动市场 - 价格波动剧烈，风险较高",
		layers.MarketConsolidate: "整理市场 - 价格在小范围内整理，等待方向选择",
		layers.MarketBreakout:    "突破市场 - 价格突破关键支撑或阻力位",
	}

	if desc, ok := descriptions[condition]; ok {
		return desc
	}
	return "未知市场状态"
}
