package traits

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"fashion-recommend/ai"
	"fashion-recommend/database"
)

// Extractor 特质提取器
type Extractor struct {
	aiService *ai.Service
	db        *database.DB
}

// NewExtractor 创建特质提取器
func NewExtractor(aiService *ai.Service, db *database.DB) *Extractor {
	return &Extractor{
		aiService: aiService,
		db:        db,
	}
}

// 关键词映射表
var styleKeywords = map[string][]string{
	"minimalist": {"简约", "极简", "简单", "干净", "纯色", "基础款"},
	"casual":     {"休闲", "舒适", "日常", "轻松", "随意"},
	"formal":     {"正式", "商务", "职业", "优雅", "西装", "衬衫"},
	"streetwear": {"街头", "潮流", "嘻哈", "运动", "oversize"},
	"vintage":    {"复古", "怀旧", "古着", "经典"},
	"romantic":   {"浪漫", "甜美", "温柔", "淑女", "蕾丝"},
}

var colorKeywords = map[string][]string{
	"black":  {"黑色", "黑"},
	"white":  {"白色", "白"},
	"gray":   {"灰色", "灰"},
	"blue":   {"蓝色", "蓝", "深蓝", "浅蓝"},
	"red":    {"红色", "红", "酒红"},
	"pink":   {"粉色", "粉", "粉红"},
	"beige":  {"米色", "米白", "杏色", "卡其"},
	"brown":  {"棕色", "咖啡色", "褐色"},
	"green":  {"绿色", "绿", "墨绿"},
	"yellow": {"黄色", "黄"},
}

var occasionKeywords = map[string][]string{
	"work":    {"上班", "工作", "职场", "办公", "商务"},
	"casual":  {"日常", "平时", "休闲", "逛街"},
	"party":   {"派对", "聚会", "晚宴", "宴会"},
	"date":    {"约会", "见面", "相亲"},
	"sport":   {"运动", "健身", "跑步", "瑜伽"},
	"travel":  {"旅行", "度假", "出游"},
	"wedding": {"婚礼", "结婚"},
}

var priceKeywords = map[string][]string{
	"low":    {"便宜", "实惠", "性价比", "平价", "省钱"},
	"medium": {"适中", "中等", "合理"},
	"high":   {"贵", "高端", "奢侈", "品质", "高档"},
}

// ExtractFromKeywords 从关键词提取特质
func (e *Extractor) ExtractFromKeywords(content string) *database.TraitsData {
	traits := &database.TraitsData{
		StylePreferences: make(map[string]float64),
		ColorPreferences: make(map[string]float64),
		Occasions:        []string{},
		Keywords:         []string{},
		BrandPreferences: []string{},
		Interests:        []string{},
	}

	contentLower := strings.ToLower(content)

	// 提取风格偏好
	for style, keywords := range styleKeywords {
		for _, keyword := range keywords {
			if strings.Contains(contentLower, keyword) {
				traits.StylePreferences[style] = traits.StylePreferences[style] + 0.3
				traits.Keywords = append(traits.Keywords, keyword)
			}
		}
	}

	// 提取颜色偏好
	for color, keywords := range colorKeywords {
		for _, keyword := range keywords {
			if strings.Contains(contentLower, keyword) {
				traits.ColorPreferences[color] = traits.ColorPreferences[color] + 0.3
			}
		}
	}

	// 提取场景
	for occasion, keywords := range occasionKeywords {
		for _, keyword := range keywords {
			if strings.Contains(contentLower, keyword) {
				if !contains(traits.Occasions, occasion) {
					traits.Occasions = append(traits.Occasions, occasion)
				}
			}
		}
	}

	// 提取价格敏感度
	for price, keywords := range priceKeywords {
		for _, keyword := range keywords {
			if strings.Contains(contentLower, keyword) {
				traits.PriceSensitivity = price
				break
			}
		}
	}

	// 归一化分数
	normalizeScores(traits.StylePreferences)
	normalizeScores(traits.ColorPreferences)

	return traits
}

// ExtractFromAI 使用 AI 提取特质
func (e *Extractor) ExtractFromAI(ctx context.Context, conversationHistory string) (*database.TraitsData, error) {
	systemPrompt := `你是一个时尚偏好分析专家。分析用户对话，提取用户的时尚偏好特质。

请以 JSON 格式返回，包含以下字段：
{
  "style_preferences": {"minimalist": 0.8, "casual": 0.6},
  "color_preferences": {"black": 0.9, "white": 0.7},
  "price_sensitivity": "medium",
  "brand_preferences": ["ZARA", "UNIQLO"],
  "occasions": ["work", "casual"],
  "keywords": ["简约", "舒适", "高质量"],
  "interests": ["运动", "旅行"]
}

注意：
1. 分数范围 0.0-1.0
2. price_sensitivity 只能是 low/medium/high
3. 只返回 JSON，不要其他解释`

	userPrompt := fmt.Sprintf("分析以下对话，提取用户的时尚偏好：\n\n%s", conversationHistory)

	// 直接构建对话历史字符串，使用简化的 Chat 方法
	fullPrompt := systemPrompt + "\n\n" + userPrompt
	
	// 使用简化的对话接口，传入空的历史记录
	response, err := e.aiService.ChatWithAssistant(ctx, fullPrompt, nil)
	if err != nil {
		return nil, err
	}

	// 解析 JSON
	var traits database.TraitsData
	if err := json.Unmarshal([]byte(response), &traits); err != nil {
		return nil, fmt.Errorf("解析 AI 返回的特质失败: %w", err)
	}

	return &traits, nil
}

// MergeTraits 合并特质（加权平均）
func (e *Extractor) MergeTraits(keywordTraits, aiTraits *database.TraitsData) *database.TraitsData {
	merged := &database.TraitsData{
		StylePreferences: make(map[string]float64),
		ColorPreferences: make(map[string]float64),
		Occasions:        []string{},
		Keywords:         []string{},
		BrandPreferences: []string{},
		Interests:        []string{},
	}

	// 关键词权重 0.4，AI 权重 0.6
	keywordWeight := 0.4
	aiWeight := 0.6

	// 合并风格偏好
	for style, score := range keywordTraits.StylePreferences {
		merged.StylePreferences[style] = score * keywordWeight
	}
	if aiTraits != nil {
		for style, score := range aiTraits.StylePreferences {
			merged.StylePreferences[style] += score * aiWeight
		}
	}

	// 合并颜色偏好
	for color, score := range keywordTraits.ColorPreferences {
		merged.ColorPreferences[color] = score * keywordWeight
	}
	if aiTraits != nil {
		for color, score := range aiTraits.ColorPreferences {
			merged.ColorPreferences[color] += score * aiWeight
		}
	}

	// 合并其他字段
	merged.Occasions = keywordTraits.Occasions
	merged.Keywords = keywordTraits.Keywords
	merged.BrandPreferences = keywordTraits.BrandPreferences
	merged.Interests = keywordTraits.Interests
	
	if aiTraits != nil {
		merged.Occasions = uniqueStrings(append(merged.Occasions, aiTraits.Occasions...))
		merged.Keywords = uniqueStrings(append(merged.Keywords, aiTraits.Keywords...))
		merged.BrandPreferences = uniqueStrings(append(merged.BrandPreferences, aiTraits.BrandPreferences...))
		merged.Interests = uniqueStrings(append(merged.Interests, aiTraits.Interests...))
	}

	// 价格敏感度优先使用 AI 的结果
	if aiTraits != nil && aiTraits.PriceSensitivity != "" {
		merged.PriceSensitivity = aiTraits.PriceSensitivity
	} else {
		merged.PriceSensitivity = keywordTraits.PriceSensitivity
	}

	return merged
}

// AnalyzeAndSave 分析对话并保存特质
func (e *Extractor) AnalyzeAndSave(ctx context.Context, userID, sessionID string, messages []database.AIMessage) error {
	// 构建对话历史
	var conversationHistory strings.Builder
	for _, msg := range messages {
		conversationHistory.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	// 1. 关键词提取（快速）
	keywordTraits := e.ExtractFromKeywords(conversationHistory.String())

	// 2. AI 分析（如果消息足够多）
	var aiTraits *database.TraitsData
	var err error
	if len(messages) >= 4 { // 至少 2 轮对话
		aiTraits, err = e.ExtractFromAI(ctx, conversationHistory.String())
		if err != nil {
			// AI 分析失败不影响整体流程
			fmt.Printf("AI 特质分析失败: %v\n", err)
		}
	}

	// 3. 合并特质
	finalTraits := e.MergeTraits(keywordTraits, aiTraits)

	// 4. 计算置信度
	confidence := calculateConfidence(finalTraits, len(messages))

	// 5. 保存到数据库
	traitsJSON, err := json.Marshal(finalTraits)
	if err != nil {
		return err
	}

	if err := e.db.SaveUserTraits(userID, traitsJSON, confidence); err != nil {
		return err
	}

	// 6. 记录日志
	source := "keyword"
	if aiTraits != nil {
		source = "keyword+ai"
	}
	return e.db.LogTraitExtraction(userID, sessionID, traitsJSON, source, confidence)
}

// 辅助函数
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range slice {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func normalizeScores(scores map[string]float64) {
	max := 0.0
	for _, score := range scores {
		if score > max {
			max = score
		}
	}
	if max > 0 {
		for key := range scores {
			scores[key] = scores[key] / max
		}
	}
}

func calculateConfidence(traits *database.TraitsData, messageCount int) float64 {
	// 基础置信度基于消息数量
	baseConfidence := float64(messageCount) / 20.0
	if baseConfidence > 0.8 {
		baseConfidence = 0.8
	}

	// 根据提取到的特质数量调整
	traitCount := len(traits.StylePreferences) + len(traits.ColorPreferences) + len(traits.Occasions)
	if traitCount > 5 {
		baseConfidence += 0.1
	}
	if traitCount > 10 {
		baseConfidence += 0.1
	}

	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	return baseConfidence
}
