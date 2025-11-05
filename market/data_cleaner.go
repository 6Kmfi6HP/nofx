package market

import (
	"fmt"
	"math"
)

// DataCleaner 数据清洗器 - 三层架构中的底层组件
// 职责：验证和清洗市场数据，确保数据质量
type DataCleaner struct{}

// NewDataCleaner 创建数据清洗器实例
func NewDataCleaner() *DataCleaner {
	return &DataCleaner{}
}

// ValidationResult 数据验证结果
type ValidationResult struct {
	IsValid bool
	Errors  []string
	Warnings []string
}

// ValidateMarketData 验证市场数据完整性和合理性
// 这是底层数据处理的第一步，确保传递给上层的数据是可靠的
func (dc *DataCleaner) ValidateMarketData(data *Data) *ValidationResult {
	result := &ValidationResult{
		IsValid: true,
		Errors:  []string{},
		Warnings: []string{},
	}

	if data == nil {
		result.IsValid = false
		result.Errors = append(result.Errors, "市场数据为空")
		return result
	}

	// 验证基础价格数据
	if data.CurrentPrice <= 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("当前价格无效: %.4f", data.CurrentPrice))
	}

	// 验证价格变化百分比的合理性（防止异常数据）
	if math.Abs(data.PriceChange1h) > 50 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("1小时价格变化异常: %.2f%%", data.PriceChange1h))
	}
	if math.Abs(data.PriceChange4h) > 100 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("4小时价格变化异常: %.2f%%", data.PriceChange4h))
	}

	// 验证技术指标的有效性
	if data.CurrentEMA20 <= 0 {
		result.Warnings = append(result.Warnings, "EMA20指标为零或负值")
	}

	// 验证RSI范围（标准范围0-100）
	if data.CurrentRSI7 < 0 || data.CurrentRSI7 > 100 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("RSI7超出正常范围: %.2f", data.CurrentRSI7))
	}

	// 验证持仓量数据
	if data.OpenInterest != nil {
		if data.OpenInterest.Latest < 0 {
			result.Warnings = append(result.Warnings, "持仓量为负值")
		}
	}

	// 验证时间序列数据完整性
	if data.IntradaySeries != nil {
		if len(data.IntradaySeries.MidPrices) == 0 {
			result.Warnings = append(result.Warnings, "日内价格序列为空")
		}
	}

	if data.LongerTermContext != nil {
		if data.LongerTermContext.EMA20 <= 0 || data.LongerTermContext.EMA50 <= 0 {
			result.Warnings = append(result.Warnings, "长期EMA数据不完整")
		}
	}

	return result
}

// CleanMarketData 清洗市场数据，修正异常值
// 对于某些可以修正的数据问题，进行自动修正
func (dc *DataCleaner) CleanMarketData(data *Data) *Data {
	if data == nil {
		return nil
	}

	cleaned := *data

	// 修正RSI超出范围的情况
	if cleaned.CurrentRSI7 < 0 {
		cleaned.CurrentRSI7 = 0
	}
	if cleaned.CurrentRSI7 > 100 {
		cleaned.CurrentRSI7 = 100
	}

	// 修正持仓量负值
	if cleaned.OpenInterest != nil {
		if cleaned.OpenInterest.Latest < 0 {
			cleaned.OpenInterest.Latest = 0
		}
		if cleaned.OpenInterest.Average < 0 {
			cleaned.OpenInterest.Average = 0
		}
	}

	// 修正极端价格变化（可能是数据错误）
	if math.Abs(cleaned.PriceChange1h) > 50 {
		cleaned.PriceChange1h = math.Copysign(50, cleaned.PriceChange1h)
	}
	if math.Abs(cleaned.PriceChange4h) > 100 {
		cleaned.PriceChange4h = math.Copysign(100, cleaned.PriceChange4h)
	}

	return &cleaned
}

// ValidateAndClean 组合验证和清洗操作
// 这是底层数据处理的标准入口点
func (dc *DataCleaner) ValidateAndClean(data *Data) (*Data, *ValidationResult, error) {
	// 先验证
	validationResult := dc.ValidateMarketData(data)

	// 如果有致命错误，直接返回
	if !validationResult.IsValid {
		return nil, validationResult, fmt.Errorf("市场数据验证失败: %v", validationResult.Errors)
	}

	// 清洗数据
	cleanedData := dc.CleanMarketData(data)

	return cleanedData, validationResult, nil
}

// CheckLiquidity 检查流动性是否满足交易要求
// 持仓价值 = 持仓量 × 当前价格
// 返回值：是否满足流动性要求，持仓价值（百万美元）
func (dc *DataCleaner) CheckLiquidity(data *Data, minOIValueMillions float64) (bool, float64) {
	if data == nil || data.OpenInterest == nil {
		return false, 0
	}

	if data.CurrentPrice <= 0 {
		return false, 0
	}

	// 计算持仓价值（USD）
	oiValue := data.OpenInterest.Latest * data.CurrentPrice
	oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位

	return oiValueInMillions >= minOIValueMillions, oiValueInMillions
}
