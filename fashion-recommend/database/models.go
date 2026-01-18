package database

import (
	"database/sql"
	"encoding/json"
	"time"
)

// AIConversation AI 对话会话
type AIConversation struct {
	ID           int       `json:"id"`
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`
	MessageCount int       `json:"message_count"`
}

// AIMessage AI 消息记录
type AIMessage struct {
	ID         int       `json:"id"`
	SessionID  string    `json:"session_id"`
	UserID     string    `json:"user_id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	TokensUsed int       `json:"tokens_used"`
}

// UserTraits 用户特质
type UserTraits struct {
	ID              int             `json:"id"`
	UserID          string          `json:"user_id"`
	Traits          json.RawMessage `json:"traits"`
	ConfidenceScore float64         `json:"confidence_score"`
	LastAnalyzedAt  *time.Time      `json:"last_analyzed_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// TraitExtractionLog 特质提取日志
type TraitExtractionLog struct {
	ID              int             `json:"id"`
	UserID          string          `json:"user_id"`
	SessionID       string          `json:"session_id"`
	ExtractedTraits json.RawMessage `json:"extracted_traits"`
	Source          string          `json:"source"`
	Confidence      float64         `json:"confidence"`
	CreatedAt       time.Time       `json:"created_at"`
}

// TraitsData 特质数据结构
type TraitsData struct {
	StylePreferences map[string]float64 `json:"style_preferences"`
	ColorPreferences map[string]float64 `json:"color_preferences"`
	PriceSensitivity string             `json:"price_sensitivity"`
	BrandPreferences []string           `json:"brand_preferences"`
	Occasions        []string           `json:"occasions"`
	Keywords         []string           `json:"keywords"`
	Interests        []string           `json:"interests"`
}

// DB 数据库连接
type DB struct {
	conn *sql.DB
}

// NewDB 创建数据库连接
func NewDB(connStr string) (*DB, error) {
	conn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.conn.Close()
}

// CreateConversation 创建对话会话
func (db *DB) CreateConversation(userID, sessionID, title string) error {
	query := `
		INSERT INTO ai_conversations (user_id, session_id, title)
		VALUES ($1, $2, $3)
		ON CONFLICT (session_id) DO NOTHING
	`
	_, err := db.conn.Exec(query, userID, sessionID, title)
	return err
}

// SaveMessage 保存消息
func (db *DB) SaveMessage(msg *AIMessage) error {
	query := `
		INSERT INTO ai_messages (session_id, user_id, role, content, tokens_used)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := db.conn.QueryRow(query, msg.SessionID, msg.UserID, msg.Role, msg.Content, msg.TokensUsed).
		Scan(&msg.ID, &msg.CreatedAt)
	
	if err != nil {
		return err
	}

	// 更新会话的消息计数和更新时间
	updateQuery := `
		UPDATE ai_conversations 
		SET message_count = message_count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE session_id = $1
	`
	_, err = db.conn.Exec(updateQuery, msg.SessionID)
	return err
}

// GetConversationMessages 获取会话消息
func (db *DB) GetConversationMessages(sessionID string, limit int) ([]AIMessage, error) {
	query := `
		SELECT id, session_id, user_id, role, content, created_at, tokens_used
		FROM ai_messages
		WHERE session_id = $1
		ORDER BY created_at ASC
		LIMIT $2
	`
	
	rows, err := db.conn.Query(query, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []AIMessage
	for rows.Next() {
		var msg AIMessage
		err := rows.Scan(&msg.ID, &msg.SessionID, &msg.UserID, &msg.Role, &msg.Content, &msg.CreatedAt, &msg.TokensUsed)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetUserConversations 获取用户的对话列表
func (db *DB) GetUserConversations(userID string, limit int) ([]AIConversation, error) {
	query := `
		SELECT id, user_id, session_id, created_at, updated_at, title, status, message_count
		FROM ai_conversations
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT $2
	`
	
	rows, err := db.conn.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []AIConversation
	for rows.Next() {
		var conv AIConversation
		err := rows.Scan(&conv.ID, &conv.UserID, &conv.SessionID, &conv.CreatedAt, 
			&conv.UpdatedAt, &conv.Title, &conv.Status, &conv.MessageCount)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// SaveUserTraits 保存用户特质
func (db *DB) SaveUserTraits(userID string, traits json.RawMessage, confidence float64) error {
	query := `
		INSERT INTO user_traits (user_id, traits, confidence_score, last_analyzed_at, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) 
		DO UPDATE SET 
			traits = $2,
			confidence_score = $3,
			last_analyzed_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := db.conn.Exec(query, userID, traits, confidence)
	return err
}

// GetUserTraits 获取用户特质
func (db *DB) GetUserTraits(userID string) (*UserTraits, error) {
	query := `
		SELECT id, user_id, traits, confidence_score, last_analyzed_at, created_at, updated_at
		FROM user_traits
		WHERE user_id = $1
	`
	
	var traits UserTraits
	err := db.conn.QueryRow(query, userID).Scan(
		&traits.ID, &traits.UserID, &traits.Traits, &traits.ConfidenceScore,
		&traits.LastAnalyzedAt, &traits.CreatedAt, &traits.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	return &traits, err
}

// LogTraitExtraction 记录特质提取日志
func (db *DB) LogTraitExtraction(userID, sessionID string, traits json.RawMessage, source string, confidence float64) error {
	query := `
		INSERT INTO trait_extraction_logs (user_id, session_id, extracted_traits, source, confidence)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := db.conn.Exec(query, userID, sessionID, traits, source, confidence)
	return err
}

// Comment 评论
type Comment struct {
	ID              int       `json:"id"`
	ItemID          string    `json:"item_id"`
	UserID          string    `json:"user_id"`
	Username        string    `json:"username"`
	Content         string    `json:"content"`
	ParentID        *int      `json:"parent_id"`
	ReplyToUserID   *string   `json:"reply_to_user_id"`
	ReplyToUsername *string   `json:"reply_to_username"`
	LikesCount      int       `json:"likes_count"`
	RepliesCount    int       `json:"replies_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Replies         []Comment `json:"replies,omitempty"`
	IsLiked         bool      `json:"is_liked"`
}

// CreateComment 创建评论
func (db *DB) CreateComment(comment *Comment) error {
	query := `
		INSERT INTO comments (item_id, user_id, username, content, parent_id, reply_to_user_id, reply_to_username)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`
	err := db.conn.QueryRow(query, comment.ItemID, comment.UserID, comment.Username, 
		comment.Content, comment.ParentID, comment.ReplyToUserID, comment.ReplyToUsername).
		Scan(&comment.ID, &comment.CreatedAt, &comment.UpdatedAt)
	
	if err != nil {
		return err
	}

	// 如果是回复，更新父评论的回复数
	if comment.ParentID != nil {
		updateQuery := `
			UPDATE comments 
			SET replies_count = replies_count + 1, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`
		_, err = db.conn.Exec(updateQuery, *comment.ParentID)
	}

	return err
}

// GetComments 获取商品的评论列表
func (db *DB) GetComments(itemID string, userID string, limit, offset int) ([]Comment, error) {
	query := `
		SELECT c.id, c.item_id, c.user_id, c.username, c.content, c.parent_id, 
		       c.reply_to_user_id, c.reply_to_username, c.likes_count, c.replies_count,
		       c.created_at, c.updated_at,
		       COALESCE((SELECT true FROM comment_likes WHERE comment_id = c.id AND user_id = $2), false) as is_liked
		FROM comments c
		WHERE c.item_id = $1 AND c.parent_id IS NULL
		ORDER BY c.created_at DESC
		LIMIT $3 OFFSET $4
	`
	
	rows, err := db.conn.Query(query, itemID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		err := rows.Scan(&c.ID, &c.ItemID, &c.UserID, &c.Username, &c.Content, &c.ParentID,
			&c.ReplyToUserID, &c.ReplyToUsername, &c.LikesCount, &c.RepliesCount,
			&c.CreatedAt, &c.UpdatedAt, &c.IsLiked)
		if err != nil {
			return nil, err
		}
		
		// 获取回复
		if c.RepliesCount > 0 {
			replies, err := db.GetReplies(c.ID, userID, 10)
			if err == nil {
				c.Replies = replies
			}
		}
		
		comments = append(comments, c)
	}

	return comments, nil
}

// GetReplies 获取评论的回复
func (db *DB) GetReplies(parentID int, userID string, limit int) ([]Comment, error) {
	query := `
		SELECT c.id, c.item_id, c.user_id, c.username, c.content, c.parent_id,
		       c.reply_to_user_id, c.reply_to_username, c.likes_count, c.replies_count,
		       c.created_at, c.updated_at,
		       COALESCE((SELECT true FROM comment_likes WHERE comment_id = c.id AND user_id = $2), false) as is_liked
		FROM comments c
		WHERE c.parent_id = $1
		ORDER BY c.created_at ASC
		LIMIT $3
	`
	
	rows, err := db.conn.Query(query, parentID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var replies []Comment
	for rows.Next() {
		var c Comment
		err := rows.Scan(&c.ID, &c.ItemID, &c.UserID, &c.Username, &c.Content, &c.ParentID,
			&c.ReplyToUserID, &c.ReplyToUsername, &c.LikesCount, &c.RepliesCount,
			&c.CreatedAt, &c.UpdatedAt, &c.IsLiked)
		if err != nil {
			return nil, err
		}
		replies = append(replies, c)
	}

	return replies, nil
}

// LikeComment 点赞评论
func (db *DB) LikeComment(commentID int, userID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 插入点赞记录
	insertQuery := `
		INSERT INTO comment_likes (comment_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (comment_id, user_id) DO NOTHING
	`
	result, err := tx.Exec(insertQuery, commentID, userID)
	if err != nil {
		return err
	}

	// 检查是否真的插入了新记录
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// 更新评论的点赞数
		updateQuery := `
			UPDATE comments 
			SET likes_count = likes_count + 1, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`
		_, err = tx.Exec(updateQuery, commentID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UnlikeComment 取消点赞评论
func (db *DB) UnlikeComment(commentID int, userID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除点赞记录
	deleteQuery := `
		DELETE FROM comment_likes
		WHERE comment_id = $1 AND user_id = $2
	`
	result, err := tx.Exec(deleteQuery, commentID, userID)
	if err != nil {
		return err
	}

	// 检查是否真的删除了记录
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// 更新评论的点赞数
		updateQuery := `
			UPDATE comments 
			SET likes_count = GREATEST(likes_count - 1, 0), updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`
		_, err = tx.Exec(updateQuery, commentID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetCommentCount 获取商品的评论总数
func (db *DB) GetCommentCount(itemID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM comments WHERE item_id = $1 AND parent_id IS NULL`
	err := db.conn.QueryRow(query, itemID).Scan(&count)
	return count, err
}

// ProductLike 商品点赞
type ProductLike struct {
	ID        int       `json:"id"`
	ItemID    string    `json:"item_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// LikeProduct 点赞商品
func (db *DB) LikeProduct(itemID, userID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 插入点赞记录
	insertQuery := `
		INSERT INTO product_likes (item_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (item_id, user_id) DO NOTHING
	`
	result, err := tx.Exec(insertQuery, itemID, userID)
	if err != nil {
		return err
	}

	// 检查是否真的插入了新记录
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// 更新统计表
		updateQuery := `
			INSERT INTO product_like_stats (item_id, like_count, updated_at)
			VALUES ($1, 1, CURRENT_TIMESTAMP)
			ON CONFLICT (item_id) 
			DO UPDATE SET 
				like_count = product_like_stats.like_count + 1,
				updated_at = CURRENT_TIMESTAMP
		`
		_, err = tx.Exec(updateQuery, itemID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UnlikeProduct 取消点赞商品
func (db *DB) UnlikeProduct(itemID, userID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除点赞记录
	deleteQuery := `
		DELETE FROM product_likes
		WHERE item_id = $1 AND user_id = $2
	`
	result, err := tx.Exec(deleteQuery, itemID, userID)
	if err != nil {
		return err
	}

	// 检查是否真的删除了记录
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// 更新统计表
		updateQuery := `
			UPDATE product_like_stats 
			SET like_count = GREATEST(like_count - 1, 0),
			    updated_at = CURRENT_TIMESTAMP
			WHERE item_id = $1
		`
		_, err = tx.Exec(updateQuery, itemID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetProductLikeCount 获取商品点赞数
func (db *DB) GetProductLikeCount(itemID string) (int, error) {
	var count int
	query := `
		SELECT COALESCE(like_count, 0) 
		FROM product_like_stats 
		WHERE item_id = $1
	`
	err := db.conn.QueryRow(query, itemID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// IsProductLiked 检查用户是否已点赞商品
func (db *DB) IsProductLiked(itemID, userID string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM product_likes 
			WHERE item_id = $1 AND user_id = $2
		)
	`
	err := db.conn.QueryRow(query, itemID, userID).Scan(&exists)
	return exists, err
}

// GetProductsLikeInfo 批量获取商品点赞信息
func (db *DB) GetProductsLikeInfo(itemIDs []string, userID string) (map[string]struct {
	LikeCount int
	IsLiked   bool
}, error) {
	result := make(map[string]struct {
		LikeCount int
		IsLiked   bool
	})

	if len(itemIDs) == 0 {
		return result, nil
	}

	// 构建查询
	query := `
		SELECT 
			s.item_id,
			COALESCE(s.like_count, 0) as like_count,
			EXISTS(SELECT 1 FROM product_likes WHERE item_id = s.item_id AND user_id = $1) as is_liked
		FROM (SELECT unnest($2::text[]) as item_id) s
		LEFT JOIN product_like_stats ps ON s.item_id = ps.item_id
	`

	rows, err := db.conn.Query(query, userID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var itemID string
		var likeCount int
		var isLiked bool
		
		if err := rows.Scan(&itemID, &likeCount, &isLiked); err != nil {
			return nil, err
		}
		
		result[itemID] = struct {
			LikeCount int
			IsLiked   bool
		}{
			LikeCount: likeCount,
			IsLiked:   isLiked,
		}
	}

	return result, nil
}
