package traits

import (
	"encoding/json"
	"fmt"

	"fashion-recommend/client"
	"fashion-recommend/database"
	"fashion-recommend/models"
)

// GorseSync Gorse 同步服务
type GorseSync struct {
	gorseClient *client.GorseClient
	db          *database.DB
}

// NewGorseSync 创建 Gorse 同步服务
func NewGorseSync(gorseClient *client.GorseClient, db *database.DB) *GorseSync {
	return &GorseSync{
		gorseClient: gorseClient,
		db:          db,
	}
}

// SyncUserTraitsToGorse 同步用户特质到 Gorse
func (g *GorseSync) SyncUserTraitsToGorse(userID string) error {
	// 1. 从数据库获取用户特质
	userTraits, err := g.db.GetUserTraits(userID)
	if err != nil {
		return fmt.Errorf("获取用户特质失败: %w", err)
	}

	if userTraits == nil {
		// 用户还没有特质数据
		return nil
	}

	// 2. 解析特质数据
	var traits database.TraitsData
	if err := json.Unmarshal(userTraits.Traits, &traits); err != nil {
		return fmt.Errorf("解析特质数据失败: %w", err)
	}

	// 3. 转换为 Gorse Labels
	labels := g.convertTraitsToLabels(&traits)

	// 4. 更新 Gorse 用户
	user := models.User{
		UserId:  userID,
		Labels:  labels,
		Comment: fmt.Sprintf("特质置信度: %.2f", userTraits.ConfidenceScore),
	}

	if err := g.gorseClient.InsertUser(user); err != nil {
		return fmt.Errorf("更新 Gorse 用户失败: %w", err)
	}

	return nil
}

// convertTraitsToLabels 将特质转换为 Gorse Labels
func (g *GorseSync) convertTraitsToLabels(traits *database.TraitsData) []string {
	labels := []string{}

	// 风格偏好（只添加分数 > 0.5 的）
	for style, score := range traits.StylePreferences {
		if score > 0.5 {
			labels = append(labels, fmt.Sprintf("style:%s:%.1f", style, score))
		}
	}

	// 颜色偏好（只添加分数 > 0.5 的）
	for color, score := range traits.ColorPreferences {
		if score > 0.5 {
			labels = append(labels, fmt.Sprintf("color:%s:%.1f", color, score))
		}
	}

	// 价格敏感度
	if traits.PriceSensitivity != "" {
		labels = append(labels, fmt.Sprintf("price:%s", traits.PriceSensitivity))
	}

	// 品牌偏好
	for _, brand := range traits.BrandPreferences {
		labels = append(labels, fmt.Sprintf("brand:%s", brand))
	}

	// 使用场景
	for _, occasion := range traits.Occasions {
		labels = append(labels, fmt.Sprintf("occasion:%s", occasion))
	}

	// 兴趣标签
	for _, interest := range traits.Interests {
		labels = append(labels, fmt.Sprintf("interest:%s", interest))
	}

	// 关键词（最多取前 5 个）
	keywordCount := len(traits.Keywords)
	if keywordCount > 5 {
		keywordCount = 5
	}
	for i := 0; i < keywordCount; i++ {
		labels = append(labels, fmt.Sprintf("keyword:%s", traits.Keywords[i]))
	}

	return labels
}

// GenerateImplicitFeedback 根据对话生成隐式反馈
func (g *GorseSync) GenerateImplicitFeedback(userID string, traits *database.TraitsData) error {
	// 这里可以根据用户特质生成一些隐式的"喜欢"反馈
	// 例如：如果用户喜欢简约风格，可以给简约风格的商品添加隐式反馈
	
	// 暂时不实现，留作后续优化
	return nil
}
