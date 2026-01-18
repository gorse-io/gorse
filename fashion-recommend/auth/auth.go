package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 认证用户
type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Email        string    `json:"email"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session 会话
type Session struct {
	Token     string
	Username  string
	ExpiresAt time.Time
}

// AuthService 认证服务
type AuthService struct {
	users    map[string]*User
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewAuthService 创建认证服务
func NewAuthService() *AuthService {
	return &AuthService{
		users:    make(map[string]*User),
		sessions: make(map[string]*Session),
	}
}

// Register 注册用户
func (s *AuthService) Register(username, password, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; exists {
		return errors.New("用户名已存在")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	s.users[username] = &User{
		Username:     username,
		PasswordHash: string(hash),
		Email:        email,
		CreatedAt:    time.Now(),
	}

	return nil
}

// Login 登录
func (s *AuthService) Login(username, password string) (string, error) {
	s.mu.RLock()
	user, exists := s.users[username]
	s.mu.RUnlock()

	if !exists {
		return "", errors.New("用户名或密码错误")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("用户名或密码错误")
	}

	// 生成 token
	token := generateToken()
	
	s.mu.Lock()
	s.sessions[token] = &Session{
		Token:     token,
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	s.mu.Unlock()

	return token, nil
}

// ValidateToken 验证 token
func (s *AuthService) ValidateToken(token string) (*User, error) {
	s.mu.RLock()
	session, exists := s.sessions[token]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("无效的 token")
	}

	if time.Now().After(session.ExpiresAt) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return nil, errors.New("token 已过期")
	}

	s.mu.RLock()
	user := s.users[session.Username]
	s.mu.RUnlock()

	return user, nil
}

// Logout 登出
func (s *AuthService) Logout(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// GetUser 获取用户信息
func (s *AuthService) GetUser(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[username]
	if !exists {
		return nil, errors.New("用户不存在")
	}

	return user, nil
}

// generateToken 生成随机 token
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
