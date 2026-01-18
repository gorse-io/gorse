package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"fashion-recommend/models"
)

// GorseClient Gorse 客户端
type GorseClient struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

// NewGorseClient 创建 Gorse 客户端
func NewGorseClient(endpoint, apiKey string) *GorseClient {
	return &GorseClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// InsertUser 插入用户
func (c *GorseClient) InsertUser(user models.User) error {
	url := fmt.Sprintf("%s/api/user", c.endpoint)
	return c.doRequest("POST", url, user, nil)
}

// InsertUsers 批量插入用户
func (c *GorseClient) InsertUsers(users []models.User) error {
	url := fmt.Sprintf("%s/api/users", c.endpoint)
	return c.doRequest("POST", url, users, nil)
}

// GetUser 获取用户
func (c *GorseClient) GetUser(userId string) (*models.User, error) {
	url := fmt.Sprintf("%s/api/user/%s", c.endpoint, userId)
	var user models.User
	err := c.doRequest("GET", url, nil, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// InsertItem 插入商品
func (c *GorseClient) InsertItem(item models.Item) error {
	url := fmt.Sprintf("%s/api/item", c.endpoint)
	return c.doRequest("POST", url, item, nil)
}

// InsertItems 批量插入商品
func (c *GorseClient) InsertItems(items []models.Item) error {
	url := fmt.Sprintf("%s/api/items", c.endpoint)
	return c.doRequest("POST", url, items, nil)
}

// GetItem 获取商品
func (c *GorseClient) GetItem(itemId string) (*models.Item, error) {
	url := fmt.Sprintf("%s/api/item/%s", c.endpoint, itemId)
	var item models.Item
	err := c.doRequest("GET", url, nil, &item)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// InsertFeedback 插入反馈
func (c *GorseClient) InsertFeedback(feedback []models.Feedback) error {
	url := fmt.Sprintf("%s/api/feedback", c.endpoint)
	return c.doRequest("POST", url, feedback, nil)
}

// GetRecommend 获取推荐
func (c *GorseClient) GetRecommend(userId string, category string, n int) ([]models.RecommendItem, error) {
	url := fmt.Sprintf("%s/api/recommend/%s?n=%d", c.endpoint, userId, n)
	if category != "" {
		url += fmt.Sprintf("&category=%s", category)
	}

	var itemIds []string
	err := c.doRequest("GET", url, nil, &itemIds)
	if err != nil {
		return nil, err
	}
	
	// 转换为 RecommendItem 格式
	items := make([]models.RecommendItem, len(itemIds))
	for i, id := range itemIds {
		items[i] = models.RecommendItem{
			ItemId: id,
			Score:  float64(len(itemIds) - i), // 简单的分数，按顺序递减
		}
	}
	return items, nil
}

// GetItemNeighbors 获取相似商品
func (c *GorseClient) GetItemNeighbors(itemId string, n int) ([]models.RecommendItem, error) {
	url := fmt.Sprintf("%s/api/item/%s/neighbors?n=%d", c.endpoint, itemId, n)
	var itemIds []string
	err := c.doRequest("GET", url, nil, &itemIds)
	if err != nil {
		return nil, err
	}
	
	// 转换为 RecommendItem 格式
	items := make([]models.RecommendItem, len(itemIds))
	for i, id := range itemIds {
		items[i] = models.RecommendItem{
			ItemId: id,
			Score:  float64(len(itemIds) - i), // 简单的分数，按顺序递减
		}
	}
	return items, nil
}

// doRequest 执行 HTTP 请求
func (c *GorseClient) doRequest(method, url string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil && method == "GET" {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
