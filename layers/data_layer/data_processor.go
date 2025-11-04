package data_layer

import (
	"fmt"
	"nofx/layers"
	"nofx/market"
	"time"
)

// DataProcessor 数据处理器（底层）
// 职责：数据获取和清洗
type DataProcessor struct {
	config layers.DataLayerConfig
}

// NewDataProcessor 创建数据处理器
func NewDataProcessor(config layers.DataLayerConfig) *DataProcessor {
	return &DataProcessor{
		config: config,
	}
}

// ProcessMarketData 处理市场数据
// 输入：原始市场数据
// 输出：清洗后的市场数据（650字符压缩摘要）
func (dp *DataProcessor) ProcessMarketData(rawData *market.Data) (*layers.CleanedMarketData, error) {
	if rawData == nil {
		return nil, fmt.Errorf("raw data is nil")
	}

	cleaned := &layers.CleanedMarketData{
		Symbol:         rawData.Symbol,
		Timestamp:      time.Now(),
		CurrentPrice:   rawData.CurrentPrice,
		PriceChange1h:  rawData.PriceChange1h,
		PriceChange4h:  rawData.PriceChange4h,
		PriceChange24h: dp.calculatePriceChange24h(rawData),
		EMA20:          rawData.CurrentEMA20,
		MACD:           rawData.CurrentMACD,
		RSI7:           rawData.CurrentRSI7,
		FundingRate:    rawData.FundingRate,
	}

	// 提取长期数据
	if rawData.LongerTermContext != nil {
		cleaned.EMA50 = rawData.LongerTermContext.EMA50
		cleaned.ATR = rawData.LongerTermContext.ATR14
		cleaned.Volume24h = rawData.LongerTermContext.CurrentVolume
		cleaned.VolumeChange = dp.calculateVolumeChange(rawData)
	}

	// 提取持仓量数据
	if rawData.OpenInterest != nil {
		cleaned.OpenInterest = rawData.OpenInterest.Latest
		cleaned.OIChange = dp.calculateOIChange(rawData)
	}

	// 计算技术指标
	cleaned.MACDSignal = dp.calculateMACDSignal(rawData)
	cleaned.RSI14 = dp.calculateRSI14(rawData)

	// 数据质量评估
	cleaned.DataQuality = dp.assessDataQuality(rawData)
	cleaned.IsValid = cleaned.DataQuality >= dp.config.MinDataQuality

	// 生成压缩摘要（650字符以内）
	cleaned.CompressedSummary = dp.generateCompressedSummary(cleaned)

	return cleaned, nil
}

// calculatePriceChange24h 计算24小时价格变化
func (dp *DataProcessor) calculatePriceChange24h(data *market.Data) float64 {
	// 从日内数据中计算
	if data.IntradaySeries != nil && len(data.IntradaySeries.MidPrices) > 0 {
		first := data.IntradaySeries.MidPrices[0]
		if first > 0 {
			return ((data.CurrentPrice - first) / first) * 100
		}
	}
	return 0
}

// calculateVolumeChange 计算成交量变化
func (dp *DataProcessor) calculateVolumeChange(data *market.Data) float64 {
	if data.LongerTermContext != nil {
		current := data.LongerTermContext.CurrentVolume
		avg := data.LongerTermContext.AverageVolume
		if avg > 0 {
			return ((current - avg) / avg) * 100
		}
	}
	return 0
}

// calculateOIChange 计算持仓量变化
func (dp *DataProcessor) calculateOIChange(data *market.Data) float64 {
	if data.OpenInterest != nil {
		latest := data.OpenInterest.Latest
		avg := data.OpenInterest.Average
		if avg > 0 {
			return ((latest - avg) / avg) * 100
		}
	}
	return 0
}

// calculateMACDSignal 计算MACD信号线
func (dp *DataProcessor) calculateMACDSignal(data *market.Data) float64 {
	// 简化版：使用MACD值的9周期EMA
	// 实际项目可以从IntradaySeries中计算
	if data.IntradaySeries != nil && len(data.IntradaySeries.MACDValues) >= 9 {
		macdValues := data.IntradaySeries.MACDValues
		sum := 0.0
		for i := len(macdValues) - 9; i < len(macdValues); i++ {
			sum += macdValues[i]
		}
		return sum / 9.0
	}
	return data.CurrentMACD * 0.9 // 近似值
}

// calculateRSI14 计算RSI14
func (dp *DataProcessor) calculateRSI14(data *market.Data) float64 {
	if data.IntradaySeries != nil && len(data.IntradaySeries.RSI14Values) > 0 {
		values := data.IntradaySeries.RSI14Values
		return values[len(values)-1]
	}
	return data.CurrentRSI7 // 回退到RSI7
}

// assessDataQuality 评估数据质量
func (dp *DataProcessor) assessDataQuality(data *market.Data) float64 {
	quality := 1.0

	// 检查必要字段
	if data.CurrentPrice <= 0 {
		quality -= 0.5
	}
	if data.CurrentEMA20 <= 0 {
		quality -= 0.1
	}
	if data.CurrentRSI7 <= 0 || data.CurrentRSI7 > 100 {
		quality -= 0.1
	}

	// 检查数据新鲜度
	if data.IntradaySeries == nil || len(data.IntradaySeries.MidPrices) < 10 {
		quality -= 0.2
	}

	if quality < 0 {
		quality = 0
	}

	return quality
}

// generateCompressedSummary 生成压缩摘要（650字符以内）
func (dp *DataProcessor) generateCompressedSummary(data *layers.CleanedMarketData) string {
	summary := fmt.Sprintf(
		"%s|P:%.2f|1h:%.2f%%|4h:%.2f%%|24h:%.2f%%|EMA20:%.2f|EMA50:%.2f|"+
		"MACD:%.4f|Sig:%.4f|RSI7:%.1f|RSI14:%.1f|ATR:%.2f|"+
		"Vol24h:%.0f|VolChg:%.1f%%|OI:%.0f|OIChg:%.1f%%|FR:%.4f%%|Q:%.2f",
		data.Symbol,
		data.CurrentPrice,
		data.PriceChange1h,
		data.PriceChange4h,
		data.PriceChange24h,
		data.EMA20,
		data.EMA50,
		data.MACD,
		data.MACDSignal,
		data.RSI7,
		data.RSI14,
		data.ATR,
		data.Volume24h,
		data.VolumeChange,
		data.OpenInterest,
		data.OIChange,
		data.FundingRate*100,
		data.DataQuality,
	)

	// 限制长度
	if len(summary) > 650 {
		summary = summary[:647] + "..."
	}

	return summary
}

// BatchProcessMarketData 批量处理市场数据
func (dp *DataProcessor) BatchProcessMarketData(rawDataList []*market.Data) ([]*layers.CleanedMarketData, error) {
	cleaned := make([]*layers.CleanedMarketData, 0, len(rawDataList))

	for _, rawData := range rawDataList {
		if rawData == nil {
			continue
		}

		cleanedData, err := dp.ProcessMarketData(rawData)
		if err != nil {
			// 记录错误但继续处理其他数据
			continue
		}

		// 只保留高质量数据
		if cleanedData.IsValid {
			cleaned = append(cleaned, cleanedData)
		}
	}

	if len(cleaned) == 0 {
		return nil, fmt.Errorf("no valid data after cleaning")
	}

	return cleaned, nil
}

// GetDataQualityReport 获取数据质量报告
func (dp *DataProcessor) GetDataQualityReport(data *layers.CleanedMarketData) map[string]interface{} {
	report := map[string]interface{}{
		"symbol":       data.Symbol,
		"quality":      data.DataQuality,
		"is_valid":     data.IsValid,
		"timestamp":    data.Timestamp,
		"summary_len":  len(data.CompressedSummary),
	}

	// 数据完整性检查
	completeness := 0.0
	totalFields := 15.0

	if data.CurrentPrice > 0 {
		completeness++
	}
	if data.EMA20 > 0 {
		completeness++
	}
	if data.EMA50 > 0 {
		completeness++
	}
	if data.MACD != 0 {
		completeness++
	}
	if data.MACDSignal != 0 {
		completeness++
	}
	if data.RSI7 > 0 {
		completeness++
	}
	if data.RSI14 > 0 {
		completeness++
	}
	if data.ATR > 0 {
		completeness++
	}
	if data.Volume24h > 0 {
		completeness++
	}
	if data.OpenInterest > 0 {
		completeness++
	}
	if data.PriceChange1h != 0 {
		completeness++
	}
	if data.PriceChange4h != 0 {
		completeness++
	}
	if data.PriceChange24h != 0 {
		completeness++
	}
	if data.VolumeChange != 0 {
		completeness++
	}
	if data.OIChange != 0 {
		completeness++
	}

	report["completeness"] = completeness / totalFields

	return report
}
