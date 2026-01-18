package ai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// Config AI 配置
type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float32
}

// Service AI 服务
type Service struct {
	client *openai.Client
	config Config
}

// NewService 创建 AI 服务
func NewService(config Config) *Service {
	clientConfig := openai.DefaultConfig(config.APIKey)
	clientConfig.BaseURL = config.BaseURL
	
	return &Service{
		client: openai.NewClientWithConfig(clientConfig),
		config: config,
	}
}

// Chat 对话
func (s *Service) Chat(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	req := openai.ChatCompletionRequest{
		Model:       s.config.Model,
		Messages:    messages,
		MaxTokens:   s.config.MaxTokens,
		Temperature: s.config.Temperature,
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("AI 对话失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI 未返回响应")
	}

	return resp.Choices[0].Message.Content, nil
}

// ExplainRecommendation 解释推荐理由
func (s *Service) ExplainRecommendation(ctx context.Context, username string, itemID string, userPreferences []string) (string, error) {
	systemPrompt := "你是一个时尚品牌推荐系统的 AI 助手，擅长解释为什么推荐某个商品给用户。请用简洁、友好的语言解释推荐理由。"
	
	userPrompt := fmt.Sprintf(
		"用户 %s 的偏好是：%v\n我们推荐了商品 %s 给这位用户。请用1-2句话解释为什么推荐这个商品。",
		username,
		userPreferences,
		itemID,
	)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt,
		},
	}

	return s.Chat(ctx, messages)
}

// GenerateStyleAdvice 生成穿搭建议
func (s *Service) GenerateStyleAdvice(ctx context.Context, items []string, occasion string) (string, error) {
	systemPrompt := "你是一个专业的时尚造型师，擅长为用户提供穿搭建议。"
	
	userPrompt := fmt.Sprintf(
		"用户选择了这些商品：%v\n场合是：%s\n请提供简洁的穿搭建议和搭配技巧。",
		items,
		occasion,
	)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt,
		},
	}

	return s.Chat(ctx, messages)
}

// ChatWithAssistant 与 AI 助手对话
func (s *Service) ChatWithAssistant(ctx context.Context, userMessage string, conversationHistory []openai.ChatCompletionMessage) (string, error) {
	systemPrompt := "你是一个时尚品牌推荐系统的 AI 助手，名字叫「时尚小助手」。你可以帮助用户了解时尚趋势、推荐商品、提供穿搭建议。请用友好、专业的语气回答用户的问题。"
	
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}
	
	// 添加历史对话
	messages = append(messages, conversationHistory...)
	
	// 添加当前用户消息
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userMessage,
	})

	return s.Chat(ctx, messages)
}
