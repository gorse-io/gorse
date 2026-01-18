package main

import (
	"fmt"
	"log"
	"time"

	"fashion-recommend/client"
	"fashion-recommend/models"
)

func main() {
	// 创建 Gorse 客户端
	gorseClient := client.NewGorseClient("http://localhost:8088", "")

	fmt.Println("开始初始化数据...")

	// 1. 创建用户数据
	users := createUsers()
	if err := gorseClient.InsertUsers(users); err != nil {
		log.Fatalf("插入用户失败: %v", err)
	}
	fmt.Printf("✓ 成功插入 %d 个用户\n", len(users))

	// 2. 创建商品数据（逐个插入避免批量插入问题）
	items := createItems()
	for i, item := range items {
		if err := gorseClient.InsertItem(item); err != nil {
			log.Fatalf("插入商品 %d (%s) 失败: %v", i+1, item.ItemId, err)
		}
		fmt.Printf("  - 插入商品 %s\n", item.ItemId)
	}
	fmt.Printf("✓ 成功插入 %d 个商品\n", len(items))

	// 3. 创建反馈数据
	feedbacks := createFeedbacks()
	if err := gorseClient.InsertFeedback(feedbacks); err != nil {
		log.Fatalf("插入反馈失败: %v", err)
	}
	fmt.Printf("✓ 成功插入 %d 条反馈\n", len(feedbacks))

	fmt.Println("\n数据初始化完成！")
	fmt.Println("访问 http://localhost:8088 查看 Gorse Dashboard")
}

// createUsers 创建示例用户
func createUsers() []models.User {
	users := []models.User{
		models.NewFashionUser("user_001").
			WithGender("female").
			WithAgeGroup("25-34").
			WithStyle("minimalist", "casual").
			WithPricePreference("mid-range").
			WithFavoriteBrands("zara", "uniqlo").
			WithComment("喜欢简约风格的职场女性").
			Build(),

		models.NewFashionUser("user_002").
			WithGender("female").
			WithAgeGroup("18-24").
			WithStyle("street", "trendy").
			WithPricePreference("budget").
			WithFavoriteBrands("h&m", "pull&bear").
			WithComment("追求潮流的年轻女性").
			Build(),

		models.NewFashionUser("user_003").
			WithGender("male").
			WithAgeGroup("25-34").
			WithStyle("business", "classic").
			WithPricePreference("high-end").
			WithFavoriteBrands("hugo_boss", "armani").
			WithComment("商务人士").
			Build(),

		models.NewFashionUser("user_004").
			WithGender("female").
			WithAgeGroup("35-44").
			WithStyle("elegant", "classic").
			WithPricePreference("luxury").
			WithFavoriteBrands("chanel", "dior").
			WithComment("追求品质的成熟女性").
			Build(),

		models.NewFashionUser("user_005").
			WithGender("male").
			WithAgeGroup("18-24").
			WithStyle("casual", "sporty").
			WithPricePreference("budget").
			WithFavoriteBrands("nike", "adidas").
			WithComment("运动休闲风格的年轻男性").
			Build(),
	}

	return users
}

// createItems 创建示例商品
func createItems() []models.Item {
	now := time.Now()
	items := []models.Item{
		// 女装上衣
		models.NewFashionItem("product_001").
			WithCategories("women", "tops", "blouse").
			WithBrand("zara").
			WithSeason("spring_2024").
			WithStyle("minimalist", "office").
			WithColor("white").
			WithMaterial("cotton").
			WithPriceRange("50-100").
			WithOccasion("work", "daily").
			WithComment("经典白色衬衫").
			WithTimestamp(now.AddDate(0, 0, -10)).
			Build(),

		models.NewFashionItem("product_002").
			WithCategories("women", "tops", "tshirt").
			WithBrand("uniqlo").
			WithSeason("spring_2024").
			WithStyle("casual", "minimalist").
			WithColor("black", "white", "gray").
			WithMaterial("cotton").
			WithPriceRange("20-50").
			WithOccasion("daily", "weekend").
			WithComment("基础款T恤").
			WithTimestamp(now.AddDate(0, 0, -5)).
			Build(),

		models.NewFashionItem("product_003").
			WithCategories("women", "dresses", "casual").
			WithBrand("h&m").
			WithSeason("spring_2024").
			WithStyle("trendy", "casual").
			WithColor("floral").
			WithMaterial("polyester").
			WithPriceRange("50-100").
			WithOccasion("date", "party").
			WithComment("碎花连衣裙").
			WithTimestamp(now.AddDate(0, 0, -3)).
			Build(),

		// 女装裤装
		models.NewFashionItem("product_004").
			WithCategories("women", "bottoms", "jeans").
			WithBrand("zara").
			WithSeason("all_season").
			WithStyle("casual", "minimalist").
			WithColor("blue").
			WithMaterial("denim").
			WithPriceRange("50-100").
			WithOccasion("daily", "weekend").
			WithComment("经典牛仔裤").
			WithTimestamp(now.AddDate(0, 0, -15)).
			Build(),

		models.NewFashionItem("product_005").
			WithCategories("women", "bottoms", "trousers").
			WithBrand("uniqlo").
			WithSeason("all_season").
			WithStyle("office", "minimalist").
			WithColor("black", "navy").
			WithMaterial("polyester").
			WithPriceRange("50-100").
			WithOccasion("work", "formal").
			WithComment("西装裤").
			WithTimestamp(now.AddDate(0, 0, -20)).
			Build(),

		// 男装
		models.NewFashionItem("product_006").
			WithCategories("men", "tops", "shirt").
			WithBrand("hugo_boss").
			WithSeason("all_season").
			WithStyle("business", "classic").
			WithColor("white", "blue").
			WithMaterial("cotton").
			WithPriceRange("100-200").
			WithOccasion("work", "formal").
			WithComment("商务衬衫").
			WithTimestamp(now.AddDate(0, 0, -12)).
			Build(),

		models.NewFashionItem("product_007").
			WithCategories("men", "tops", "tshirt").
			WithBrand("nike").
			WithSeason("spring_2024").
			WithStyle("sporty", "casual").
			WithColor("black", "white", "gray").
			WithMaterial("polyester").
			WithPriceRange("50-100").
			WithOccasion("sports", "daily").
			WithComment("运动T恤").
			WithTimestamp(now.AddDate(0, 0, -7)).
			Build(),

		models.NewFashionItem("product_008").
			WithCategories("men", "bottoms", "jeans").
			WithBrand("levi's").
			WithSeason("all_season").
			WithStyle("casual", "classic").
			WithColor("blue", "black").
			WithMaterial("denim").
			WithPriceRange("100-200").
			WithOccasion("daily", "weekend").
			WithComment("经典牛仔裤").
			WithTimestamp(now.AddDate(0, 0, -18)).
			Build(),

		// 配饰
		models.NewFashionItem("product_009").
			WithCategories("accessories", "bags", "handbag").
			WithBrand("chanel").
			WithSeason("all_season").
			WithStyle("elegant", "luxury").
			WithColor("black").
			WithMaterial("leather").
			WithPriceRange("2000+").
			WithOccasion("formal", "party").
			WithComment("经典手提包").
			WithTimestamp(now.AddDate(0, 0, -30)).
			Build(),

		models.NewFashionItem("product_010").
			WithCategories("accessories", "shoes", "sneakers").
			WithBrand("adidas").
			WithSeason("all_season").
			WithStyle("sporty", "casual").
			WithColor("white", "black").
			WithMaterial("synthetic").
			WithPriceRange("100-200").
			WithOccasion("sports", "daily").
			WithComment("运动鞋").
			WithTimestamp(now.AddDate(0, 0, -8)).
			Build(),
	}

	return items
}

// createFeedbacks 创建示例反馈
func createFeedbacks() []models.Feedback {
	now := time.Now()
	feedbacks := []models.Feedback{
		// user_001 的行为（喜欢简约风格）
		{FeedbackType: "view", UserId: "user_001", ItemId: "product_001", Value: 1.0, Timestamp: now.AddDate(0, 0, -5)},
		{FeedbackType: "favorite", UserId: "user_001", ItemId: "product_001", Value: 1.0, Timestamp: now.AddDate(0, 0, -5)},
		{FeedbackType: "purchase", UserId: "user_001", ItemId: "product_001", Value: 1.0, Timestamp: now.AddDate(0, 0, -4)},
		{FeedbackType: "view", UserId: "user_001", ItemId: "product_002", Value: 1.0, Timestamp: now.AddDate(0, 0, -3)},
		{FeedbackType: "purchase", UserId: "user_001", ItemId: "product_002", Value: 1.0, Timestamp: now.AddDate(0, 0, -2)},
		{FeedbackType: "view", UserId: "user_001", ItemId: "product_004", Value: 1.0, Timestamp: now.AddDate(0, 0, -1)},

		// user_002 的行为（追求潮流）
		{FeedbackType: "view", UserId: "user_002", ItemId: "product_003", Value: 1.0, Timestamp: now.AddDate(0, 0, -4)},
		{FeedbackType: "add_to_cart", UserId: "user_002", ItemId: "product_003", Value: 1.0, Timestamp: now.AddDate(0, 0, -4)},
		{FeedbackType: "purchase", UserId: "user_002", ItemId: "product_003", Value: 1.0, Timestamp: now.AddDate(0, 0, -3)},
		{FeedbackType: "view", UserId: "user_002", ItemId: "product_010", Value: 1.0, Timestamp: now.AddDate(0, 0, -2)},
		{FeedbackType: "favorite", UserId: "user_002", ItemId: "product_010", Value: 1.0, Timestamp: now.AddDate(0, 0, -2)},

		// user_003 的行为（商务人士）
		{FeedbackType: "view", UserId: "user_003", ItemId: "product_006", Value: 1.0, Timestamp: now.AddDate(0, 0, -6)},
		{FeedbackType: "purchase", UserId: "user_003", ItemId: "product_006", Value: 1.0, Timestamp: now.AddDate(0, 0, -5)},
		{FeedbackType: "view", UserId: "user_003", ItemId: "product_008", Value: 1.0, Timestamp: now.AddDate(0, 0, -3)},
		{FeedbackType: "add_to_cart", UserId: "user_003", ItemId: "product_008", Value: 1.0, Timestamp: now.AddDate(0, 0, -3)},

		// user_004 的行为（追求品质）
		{FeedbackType: "view", UserId: "user_004", ItemId: "product_009", Value: 1.0, Timestamp: now.AddDate(0, 0, -7)},
		{FeedbackType: "favorite", UserId: "user_004", ItemId: "product_009", Value: 1.0, Timestamp: now.AddDate(0, 0, -7)},
		{FeedbackType: "purchase", UserId: "user_004", ItemId: "product_009", Value: 1.0, Timestamp: now.AddDate(0, 0, -6)},
		{FeedbackType: "view", UserId: "user_004", ItemId: "product_001", Value: 1.0, Timestamp: now.AddDate(0, 0, -2)},

		// user_005 的行为（运动休闲）
		{FeedbackType: "view", UserId: "user_005", ItemId: "product_007", Value: 1.0, Timestamp: now.AddDate(0, 0, -5)},
		{FeedbackType: "purchase", UserId: "user_005", ItemId: "product_007", Value: 1.0, Timestamp: now.AddDate(0, 0, -4)},
		{FeedbackType: "view", UserId: "user_005", ItemId: "product_010", Value: 1.0, Timestamp: now.AddDate(0, 0, -3)},
		{FeedbackType: "add_to_cart", UserId: "user_005", ItemId: "product_010", Value: 1.0, Timestamp: now.AddDate(0, 0, -3)},
		{FeedbackType: "purchase", UserId: "user_005", ItemId: "product_010", Value: 1.0, Timestamp: now.AddDate(0, 0, -2)},
	}

	return feedbacks
}
