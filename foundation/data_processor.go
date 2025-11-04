package foundation

import (
	"fmt"
	"math"
	"sort"
)

// DataProcessor 底层数据处理器
// 职责：数据获取、清洗、验证、格式化
// 不涉及交易决策，只提供数据处理服务
type DataProcessor struct {
	// 数据质量检查配置
	minDataPoints    int     // 最少数据点数量
	maxPriceDeviation float64 // 最大价格偏差（用于异常值检测）
}

// NewDataProcessor 创建数据处理器实例
func NewDataProcessor() *DataProcessor {
	return &DataProcessor{
		minDataPoints:    20,   // 至少20个数据点
		maxPriceDeviation: 0.2, // 最大20%价格偏差
	}
}

// KlineData K线数据
type KlineData struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// DataQualityReport 数据质量报告
type DataQualityReport struct {
	IsValid           bool     // 数据是否有效
	TotalPoints       int      // 总数据点数
	ValidPoints       int      // 有效数据点数
	MissingPoints     int      // 缺失数据点数
	OutlierPoints     int      // 异常值数量
	Issues            []string // 问题列表
	Warnings          []string // 警告列表
}

// ValidateKlineData 验证K线数据质量
func (dp *DataProcessor) ValidateKlineData(klines []KlineData, symbol string) *DataQualityReport {
	report := &DataQualityReport{
		IsValid:       true,
		TotalPoints:   len(klines),
		ValidPoints:   0,
		MissingPoints: 0,
		OutlierPoints: 0,
		Issues:        []string{},
		Warnings:      []string{},
	}

	// 检查1：数据点数量
	if len(klines) < dp.minDataPoints {
		report.IsValid = false
		report.Issues = append(report.Issues,
			fmt.Sprintf("数据点不足: %d < %d", len(klines), dp.minDataPoints))
		return report
	}

	// 检查2：逐个验证K线数据
	var prevClose float64
	for i, kline := range klines {
		// 检查价格有效性
		if kline.Open <= 0 || kline.High <= 0 || kline.Low <= 0 || kline.Close <= 0 {
			report.MissingPoints++
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("第%d个K线价格无效: O=%.4f H=%.4f L=%.4f C=%.4f",
					i, kline.Open, kline.High, kline.Low, kline.Close))
			continue
		}

		// 检查价格逻辑
		if kline.High < kline.Low || kline.High < kline.Open || kline.High < kline.Close ||
			kline.Low > kline.Open || kline.Low > kline.Close {
			report.Issues = append(report.Issues,
				fmt.Sprintf("第%d个K线价格逻辑错误: O=%.4f H=%.4f L=%.4f C=%.4f",
					i, kline.Open, kline.High, kline.Low, kline.Close))
			continue
		}

		// 检查异常值（与前一个K线相比）
		if i > 0 && prevClose > 0 {
			priceChange := math.Abs(kline.Close-prevClose) / prevClose
			if priceChange > dp.maxPriceDeviation {
				report.OutlierPoints++
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("第%d个K线价格波动异常: %.2f%% (前收盘: %.4f, 当前收盘: %.4f)",
						i, priceChange*100, prevClose, kline.Close))
			}
		}

		prevClose = kline.Close
		report.ValidPoints++
	}

	// 最终判断
	if report.ValidPoints < dp.minDataPoints {
		report.IsValid = false
		report.Issues = append(report.Issues,
			fmt.Sprintf("有效数据点不足: %d < %d", report.ValidPoints, dp.minDataPoints))
	}

	if len(report.Issues) > 0 {
		report.IsValid = false
	}

	return report
}

// CleanKlineData 清洗K线数据（移除异常值、填充缺失值）
func (dp *DataProcessor) CleanKlineData(klines []KlineData) []KlineData {
	if len(klines) == 0 {
		return klines
	}

	cleaned := make([]KlineData, 0, len(klines))

	for i, kline := range klines {
		// 跳过无效数据
		if kline.Open <= 0 || kline.High <= 0 || kline.Low <= 0 || kline.Close <= 0 {
			// 如果有前一个有效数据，使用前一个的收盘价填充
			if i > 0 && len(cleaned) > 0 {
				lastValid := cleaned[len(cleaned)-1]
				kline.Open = lastValid.Close
				kline.High = lastValid.Close
				kline.Low = lastValid.Close
				kline.Close = lastValid.Close
				kline.Volume = 0 // 标记为填充数据
			} else {
				continue // 跳过第一个无效数据
			}
		}

		// 检查价格逻辑
		if kline.High < kline.Low {
			// 交换高低价
			kline.High, kline.Low = kline.Low, kline.High
		}
		if kline.High < kline.Close {
			kline.High = kline.Close
		}
		if kline.Low > kline.Close {
			kline.Low = kline.Close
		}

		cleaned = append(cleaned, kline)
	}

	return cleaned
}

// MarketDataSummary 市场数据摘要
type MarketDataSummary struct {
	Symbol         string
	CurrentPrice   float64
	PriceChange24h float64 // 24小时价格变化百分比
	Volume24h      float64 // 24小时交易量
	High24h        float64 // 24小时最高价
	Low24h         float64 // 24小时最低价
	Volatility     float64 // 波动率（标准差）
	Trend          string  // 趋势："up", "down", "sideways"
}

// CalculateMarketSummary 计算市场数据摘要
func (dp *DataProcessor) CalculateMarketSummary(klines []KlineData, symbol string) (*MarketDataSummary, error) {
	if len(klines) == 0 {
		return nil, fmt.Errorf("no kline data")
	}

	// 验证数据
	report := dp.ValidateKlineData(klines, symbol)
	if !report.IsValid {
		return nil, fmt.Errorf("invalid kline data: %v", report.Issues)
	}

	summary := &MarketDataSummary{
		Symbol: symbol,
	}

	// 当前价格（最后一个K线的收盘价）
	summary.CurrentPrice = klines[len(klines)-1].Close

	// 24小时最高价和最低价
	summary.High24h = klines[0].High
	summary.Low24h = klines[0].Low
	totalVolume := 0.0

	for _, kline := range klines {
		if kline.High > summary.High24h {
			summary.High24h = kline.High
		}
		if kline.Low < summary.Low24h {
			summary.Low24h = kline.Low
		}
		totalVolume += kline.Volume
	}
	summary.Volume24h = totalVolume

	// 24小时价格变化
	if len(klines) > 0 {
		firstPrice := klines[0].Open
		if firstPrice > 0 {
			summary.PriceChange24h = (summary.CurrentPrice - firstPrice) / firstPrice * 100
		}
	}

	// 计算波动率（价格的标准差）
	summary.Volatility = dp.calculateVolatility(klines)

	// 判断趋势
	summary.Trend = dp.detectTrend(klines)

	return summary, nil
}

// calculateVolatility 计算波动率（收盘价的标准差）
func (dp *DataProcessor) calculateVolatility(klines []KlineData) float64 {
	if len(klines) < 2 {
		return 0
	}

	// 计算收益率
	returns := make([]float64, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		if klines[i-1].Close > 0 {
			returns[i-1] = (klines[i].Close - klines[i-1].Close) / klines[i-1].Close
		}
	}

	// 计算均值
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	// 计算标准差
	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	variance /= float64(len(returns))

	return math.Sqrt(variance) * 100 // 转换为百分比
}

// detectTrend 检测趋势
func (dp *DataProcessor) detectTrend(klines []KlineData) string {
	if len(klines) < 10 {
		return "sideways"
	}

	// 使用线性回归检测趋势
	n := len(klines)
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, kline := range klines {
		x := float64(i)
		y := kline.Close
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 计算斜率
	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)

	// 归一化斜率（相对于平均价格）
	avgPrice := sumY / float64(n)
	normalizedSlope := slope / avgPrice * 100 // 转换为百分比

	// 判断趋势
	if normalizedSlope > 0.5 {
		return "up"
	} else if normalizedSlope < -0.5 {
		return "down"
	}
	return "sideways"
}

// OrderBookData 订单簿数据
type OrderBookData struct {
	Bids [][2]float64 // [[price, quantity], ...]
	Asks [][2]float64 // [[price, quantity], ...]
}

// OrderBookAnalysis 订单簿分析结果
type OrderBookAnalysis struct {
	BidAskSpread      float64 // 买卖价差
	BidAskSpreadPercent float64 // 买卖价差百分比
	BidVolume         float64 // 买单总量
	AskVolume         float64 // 卖单总量
	BidAskRatio       float64 // 买卖比
	Imbalance         string  // 订单簿不平衡："bid_heavy", "ask_heavy", "balanced"
	LiquidityScore    float64 // 流动性评分（0-100）
}

// AnalyzeOrderBook 分析订单簿
func (dp *DataProcessor) AnalyzeOrderBook(orderBook OrderBookData, currentPrice float64) (*OrderBookAnalysis, error) {
	if len(orderBook.Bids) == 0 || len(orderBook.Asks) == 0 {
		return nil, fmt.Errorf("empty order book")
	}

	analysis := &OrderBookAnalysis{}

	// 最优买价和卖价
	bestBid := orderBook.Bids[0][0]
	bestAsk := orderBook.Asks[0][0]

	// 买卖价差
	analysis.BidAskSpread = bestAsk - bestBid
	if currentPrice > 0 {
		analysis.BidAskSpreadPercent = analysis.BidAskSpread / currentPrice * 100
	}

	// 计算买卖单总量
	for _, bid := range orderBook.Bids {
		analysis.BidVolume += bid[1]
	}
	for _, ask := range orderBook.Asks {
		analysis.AskVolume += ask[1]
	}

	// 买卖比
	if analysis.AskVolume > 0 {
		analysis.BidAskRatio = analysis.BidVolume / analysis.AskVolume
	}

	// 判断订单簿不平衡
	if analysis.BidAskRatio > 1.5 {
		analysis.Imbalance = "bid_heavy" // 买单占优
	} else if analysis.BidAskRatio < 0.67 {
		analysis.Imbalance = "ask_heavy" // 卖单占优
	} else {
		analysis.Imbalance = "balanced" // 平衡
	}

	// 流动性评分（基于买卖价差和订单簿深度）
	totalVolume := analysis.BidVolume + analysis.AskVolume
	spreadScore := math.Max(0, 100-analysis.BidAskSpreadPercent*100) // 价差越小分数越高
	volumeScore := math.Min(100, totalVolume/10000*100)               // 量越大分数越高（归一化）
	analysis.LiquidityScore = (spreadScore + volumeScore) / 2

	return analysis, nil
}

// PriceLevel 价格层级
type PriceLevel struct {
	Price      float64
	Type       string  // "support" 或 "resistance"
	Strength   float64 // 强度（0-100）
	TouchCount int     // 触及次数
}

// FindSupportResistance 寻找支撑位和阻力位
func (dp *DataProcessor) FindSupportResistance(klines []KlineData, lookback int) []PriceLevel {
	if len(klines) < lookback {
		lookback = len(klines)
	}

	// 提取最近的K线
	recentKlines := klines[len(klines)-lookback:]

	// 收集所有高点和低点
	highs := make([]float64, 0)
	lows := make([]float64, 0)

	for _, kline := range recentKlines {
		highs = append(highs, kline.High)
		lows = append(lows, kline.Low)
	}

	// 聚类分析：将相近的价格归为一组
	tolerance := 0.02 // 2%容差

	resistanceClusters := dp.clusterPrices(highs, tolerance)
	supportClusters := dp.clusterPrices(lows, tolerance)

	levels := make([]PriceLevel, 0)

	// 添加阻力位
	for price, count := range resistanceClusters {
		strength := float64(count) / float64(lookback) * 100
		levels = append(levels, PriceLevel{
			Price:      price,
			Type:       "resistance",
			Strength:   strength,
			TouchCount: count,
		})
	}

	// 添加支撑位
	for price, count := range supportClusters {
		strength := float64(count) / float64(lookback) * 100
		levels = append(levels, PriceLevel{
			Price:      price,
			Type:       "support",
			Strength:   strength,
			TouchCount: count,
		})
	}

	// 按强度排序
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Strength > levels[j].Strength
	})

	return levels
}

// clusterPrices 聚类价格
func (dp *DataProcessor) clusterPrices(prices []float64, tolerance float64) map[float64]int {
	if len(prices) == 0 {
		return nil
	}

	clusters := make(map[float64]int)

	for _, price := range prices {
		found := false
		// 查找相近的聚类
		for clusterPrice := range clusters {
			if math.Abs(price-clusterPrice)/clusterPrice < tolerance {
				clusters[clusterPrice]++
				found = true
				break
			}
		}
		// 创建新聚类
		if !found {
			clusters[price] = 1
		}
	}

	return clusters
}

// NormalizePrice 归一化价格（用于比较不同价格区间的币种）
func (dp *DataProcessor) NormalizePrice(price, minPrice, maxPrice float64) float64 {
	if maxPrice == minPrice {
		return 0.5
	}
	return (price - minPrice) / (maxPrice - minPrice)
}

// CalculateReturns 计算收益率序列
func (dp *DataProcessor) CalculateReturns(klines []KlineData) []float64 {
	if len(klines) < 2 {
		return []float64{}
	}

	returns := make([]float64, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		if klines[i-1].Close > 0 {
			returns[i-1] = (klines[i].Close - klines[i-1].Close) / klines[i-1].Close
		}
	}
	return returns
}

// CalculateSharpeRatio 计算夏普比率
func (dp *DataProcessor) CalculateSharpeRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	// 计算平均收益
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	avgReturn := sum / float64(len(returns))

	// 计算标准差
	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-avgReturn, 2)
	}
	stdDev := math.Sqrt(variance / float64(len(returns)))

	if stdDev == 0 {
		return 0
	}

	// 夏普比率 = (平均收益 - 无风险利率) / 标准差
	return (avgReturn - riskFreeRate) / stdDev
}
