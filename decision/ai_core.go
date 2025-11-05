package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/mcp"
	"strings"
	"time"
)

// AIDecisionCore AI 决策核心 - 三层架构中的中间 AI 层
// 职责：纯粹的 AI 决策引擎，与参数计算和执行完全解耦
// 只做三件事：1. 市场状态判断  2. 交易机会识别  3. 输出结构化决策信号
type AIDecisionCore struct {
	mcpClient *mcp.Client
}

// NewAIDecisionCore 创建 AI 决策核心实例
func NewAIDecisionCore(mcpClient *mcp.Client) *AIDecisionCore {
	return &AIDecisionCore{
		mcpClient: mcpClient,
	}
}

// AnalyzeRequest AI 分析请求
type AnalyzeRequest struct {
	Context         *TradingContext // 交易上下文
	SystemPrompt    string          // 系统提示词（策略规则）
	CustomPrompt    string          // 自定义提示词
	OverrideBase    bool            // 是否覆盖基础提示词
	TemplateName    string          // 模板名称
}

// Analyze 执行 AI 分析
// 这是 AI 层的主入口，接收交易上下文，返回 AI 分析结果
func (core *AIDecisionCore) Analyze(req *AnalyzeRequest) (*AIAnalysisResult, error) {
	if req.Context == nil {
		return nil, fmt.Errorf("交易上下文不能为空")
	}

	// 1. 构建 AI 输入 Prompt
	userPrompt := core.buildAIInputPrompt(req.Context)

	// 2. 调用 AI 模型
	log.Printf("🤖 [AI核心] 调用 AI 模型进行分析...")
	aiResponse, err := core.mcpClient.CallWithMessages(req.SystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI 模型调用失败: %w", err)
	}

	// 3. 解析 AI 响应
	result, err := core.parseAIResponse(aiResponse)
	if err != nil {
		return nil, fmt.Errorf("AI 响应解析失败: %w", err)
	}

	result.Timestamp = time.Now()

	log.Printf("✓ [AI核心] AI 分析完成，识别到 %d 个交易机会", len(result.TradingOpportunities))
	return result, nil
}

// buildAIInputPrompt 构建 AI 输入 Prompt
// 将交易上下文转换为 AI 可理解的文本格式
func (core *AIDecisionCore) buildAIInputPrompt(ctx *TradingContext) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("时间: %s | 周期: #%d | 运行: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场基准
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// 账户状态
	sb.WriteString(fmt.Sprintf("账户: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 当前持仓（简化版，只给 AI 关键信息）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60)
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | 持仓%d分钟", durationMin)
				} else {
					holdingDuration = fmt.Sprintf(" | 持仓%d小时", durationMin/60)
				}
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场%.4f 当前%.4f | 盈亏%+.2f%%%s\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct, holdingDuration))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("当前持仓: 无\n\n")
	}

	// 候选币种及市场数据
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.CandidateCoins)))
	for _, coin := range ctx.CandidateCoins {
		if marketData, ok := ctx.MarketDataMap[coin.Symbol]; ok {
			// 简化的市场数据（AI 不需要所有细节）
			sb.WriteString(fmt.Sprintf("### %s\n", coin.Symbol))
			sb.WriteString(fmt.Sprintf("价格: %.4f | 1h: %+.2f%% | 4h: %+.2f%%\n",
				marketData.CurrentPrice, marketData.PriceChange1h, marketData.PriceChange4h))
			sb.WriteString(fmt.Sprintf("MACD: %.4f | RSI: %.2f | EMA20: %.4f\n",
				marketData.CurrentMACD, marketData.CurrentRSI7, marketData.CurrentEMA20))
			if marketData.OpenInterest != nil {
				sb.WriteString(fmt.Sprintf("持仓量: %.0f | 资金费率: %.4e\n",
					marketData.OpenInterest.Latest, marketData.FundingRate))
			}
			sb.WriteString("\n")
		}
	}

	// 添加简化的性能反馈（如果有）
	if ctx.Performance != nil {
		type PerformanceData struct {
			SharpeRatio float64 `json:"sharpe_ratio"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("## 📊 夏普比率: %.2f\n\n", perfData.SharpeRatio))
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("请分析市场状态并输出交易决策（思维链 + JSON数组）\n")

	return sb.String()
}

// parseAIResponse 解析 AI 响应
// 从 AI 的文本输出中提取结构化的决策信号
func (core *AIDecisionCore) parseAIResponse(response string) (*AIAnalysisResult, error) {
	result := &AIAnalysisResult{
		TradingOpportunities: []AIDecisionSignal{},
	}

	// 1. 提取思维链（JSON 之前的部分）
	result.CoTTrace = extractCoTTrace(response)

	// 2. 提取 JSON 决策数组
	decisions, err := extractDecisions(response)
	if err != nil {
		return result, fmt.Errorf("提取决策失败: %w", err)
	}

	// 3. 转换为 AI 决策信号格式
	for _, d := range decisions {
		signal := AIDecisionSignal{
			Symbol:     d.Symbol,
			Action:     core.normalizeAction(d.Action),
			Confidence: float64(d.Confidence) / 100.0, // 转换为 0-1 范围
			Reasoning:  d.Reasoning,
		}
		result.TradingOpportunities = append(result.TradingOpportunities, signal)
	}

	// 4. 简单的市场状态判断（基于AI输出推断）
	result.MarketState = core.inferMarketState(result.CoTTrace)
	result.MarketConfidence = 0.7 // 默认信心度

	return result, nil
}

// normalizeAction 标准化动作名称
// 将 AI 输出的动作转换为标准格式
func (core *AIDecisionCore) normalizeAction(action string) string {
	switch action {
	case "open_long":
		return "BUY"
	case "open_short":
		return "SELL"
	case "close_long", "close_short":
		return "CLOSE"
	case "hold", "wait":
		return "HOLD"
	default:
		return "HOLD"
	}
}

// inferMarketState 从思维链推断市场状态
// 这是一个简单的启发式方法，可以在未来改进为更复杂的分析
func (core *AIDecisionCore) inferMarketState(cotTrace string) string {
	cotLower := strings.ToLower(cotTrace)

	// 简单的关键词匹配
	if strings.Contains(cotLower, "上升趋势") || strings.Contains(cotLower, "上涨") {
		return "UPTREND"
	}
	if strings.Contains(cotLower, "下降趋势") || strings.Contains(cotLower, "下跌") {
		return "DOWNTREND"
	}
	if strings.Contains(cotLower, "震荡") || strings.Contains(cotLower, "盘整") {
		return "CONSOLIDATION"
	}
	if strings.Contains(cotLower, "突破") {
		return "BREAKOUT"
	}

	return "UNCERTAIN"
}

// AnalyzeMarketState 单独分析市场状态
// 提供给上层调用的便捷方法
func (core *AIDecisionCore) AnalyzeMarketState(ctx *TradingContext) (string, float64, error) {
	// 构建简化的市场状态查询 Prompt
	prompt := fmt.Sprintf("分析当前市场状态（BTC: %.2f, 1h变化: %.2f%%）。请简短回答：上升趋势/下降趋势/震荡/突破",
		ctx.MarketDataMap["BTCUSDT"].CurrentPrice,
		ctx.MarketDataMap["BTCUSDT"].PriceChange1h)

	response, err := core.mcpClient.CallWithMessages("你是市场分析专家", prompt)
	if err != nil {
		return "UNCERTAIN", 0, err
	}

	state := core.inferMarketState(response)
	confidence := 0.7 // 默认信心度

	return state, confidence, nil
}
