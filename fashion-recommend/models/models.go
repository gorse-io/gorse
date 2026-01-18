package models

import "time"

// User 用户模型
type User struct {
	UserId    string   `json:"user_id"`
	Labels    []string `json:"labels"`
	Comment   string   `json:"comment,omitempty"`
	Subscribe []string `json:"subscribe,omitempty"`
}

// Item 商品模型
type Item struct {
	ItemId     string    `json:"ItemId"`
	IsHidden   bool      `json:"IsHidden"`
	Categories []string  `json:"Categories"`
	Labels     []string  `json:"Labels"`
	Comment    string    `json:"Comment,omitempty"`
	Timestamp  time.Time `json:"Timestamp"`
}

// Feedback 反馈模型
type Feedback struct {
	FeedbackType string    `json:"feedback_type"`
	UserId       string    `json:"user_id"`
	ItemId       string    `json:"item_id"`
	Value        float64   `json:"value"`
	Timestamp    time.Time `json:"timestamp"`
	Comment      string    `json:"comment,omitempty"`
}

// RecommendRequest 推荐请求
type RecommendRequest struct {
	UserId   string   `json:"user_id"`
	N        int      `json:"n"`
	Category string   `json:"category,omitempty"`
	Offset   int      `json:"offset,omitempty"`
}

// RecommendResponse 推荐响应
type RecommendResponse struct {
	Items []RecommendItem `json:"items"`
	Total int             `json:"total"`
}

// RecommendItem 推荐商品项
type RecommendItem struct {
	ItemId     string   `json:"item_id"`
	Score      float64  `json:"score"`
	Categories []string `json:"categories,omitempty"`
}

// FashionUserBuilder 时尚用户构建器
type FashionUserBuilder struct {
	user User
}

func NewFashionUser(userId string) *FashionUserBuilder {
	return &FashionUserBuilder{
		user: User{
			UserId: userId,
			Labels: make([]string, 0),
		},
	}
}

func (b *FashionUserBuilder) WithGender(gender string) *FashionUserBuilder {
	b.user.Labels = append(b.user.Labels, "gender:"+gender)
	return b
}

func (b *FashionUserBuilder) WithAgeGroup(ageGroup string) *FashionUserBuilder {
	b.user.Labels = append(b.user.Labels, "age_group:"+ageGroup)
	return b
}

func (b *FashionUserBuilder) WithStyle(styles ...string) *FashionUserBuilder {
	for _, style := range styles {
		b.user.Labels = append(b.user.Labels, "style:"+style)
	}
	return b
}

func (b *FashionUserBuilder) WithPricePreference(priceRange string) *FashionUserBuilder {
	b.user.Labels = append(b.user.Labels, "price_preference:"+priceRange)
	return b
}

func (b *FashionUserBuilder) WithFavoriteBrands(brands ...string) *FashionUserBuilder {
	for _, brand := range brands {
		b.user.Labels = append(b.user.Labels, "favorite_brand:"+brand)
	}
	return b
}

func (b *FashionUserBuilder) WithComment(comment string) *FashionUserBuilder {
	b.user.Comment = comment
	return b
}

func (b *FashionUserBuilder) Build() User {
	return b.user
}

// FashionItemBuilder 时尚商品构建器
type FashionItemBuilder struct {
	item Item
}

func NewFashionItem(itemId string) *FashionItemBuilder {
	return &FashionItemBuilder{
		item: Item{
			ItemId:     itemId,
			IsHidden:   false,
			Categories: make([]string, 0),
			Labels:     make([]string, 0),
			Timestamp:  time.Now(),
		},
	}
}

func (b *FashionItemBuilder) WithCategories(categories ...string) *FashionItemBuilder {
	b.item.Categories = append(b.item.Categories, categories...)
	return b
}

func (b *FashionItemBuilder) WithBrand(brand string) *FashionItemBuilder {
	b.item.Labels = append(b.item.Labels, "brand:"+brand)
	return b
}

func (b *FashionItemBuilder) WithSeason(season string) *FashionItemBuilder {
	b.item.Labels = append(b.item.Labels, "season:"+season)
	return b
}

func (b *FashionItemBuilder) WithStyle(styles ...string) *FashionItemBuilder {
	for _, style := range styles {
		b.item.Labels = append(b.item.Labels, "style:"+style)
	}
	return b
}

func (b *FashionItemBuilder) WithColor(colors ...string) *FashionItemBuilder {
	for _, color := range colors {
		b.item.Labels = append(b.item.Labels, "color:"+color)
	}
	return b
}

func (b *FashionItemBuilder) WithMaterial(material string) *FashionItemBuilder {
	b.item.Labels = append(b.item.Labels, "material:"+material)
	return b
}

func (b *FashionItemBuilder) WithPriceRange(priceRange string) *FashionItemBuilder {
	b.item.Labels = append(b.item.Labels, "price_range:"+priceRange)
	return b
}

func (b *FashionItemBuilder) WithOccasion(occasions ...string) *FashionItemBuilder {
	for _, occasion := range occasions {
		b.item.Labels = append(b.item.Labels, "occasion:"+occasion)
	}
	return b
}

func (b *FashionItemBuilder) WithComment(comment string) *FashionItemBuilder {
	b.item.Comment = comment
	return b
}

func (b *FashionItemBuilder) WithTimestamp(timestamp time.Time) *FashionItemBuilder {
	b.item.Timestamp = timestamp
	return b
}

func (b *FashionItemBuilder) Hidden() *FashionItemBuilder {
	b.item.IsHidden = true
	return b
}

func (b *FashionItemBuilder) Build() Item {
	return b.item
}
