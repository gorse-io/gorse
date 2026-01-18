package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fashion-recommend/models"

	"github.com/stretchr/testify/assert"
)

// MockGorseServer 模拟 Gorse 服务器
func MockGorseServer() *httptest.Server {
	handler := http.NewServeMux()

	// 模拟插入用户
	handler.HandleFunc("/api/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "success"})
		}
	})

	// 模拟获取用户
	handler.HandleFunc("/api/user/", func(w http.ResponseWriter, r *http.Request) {
		user := models.User{
			UserId: "test_user",
			Labels: []string{"gender:female", "style:casual"},
		}
		json.NewEncoder(w).Encode(user)
	})

	// 模拟批量插入用户
	handler.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	})

	// 模拟插入商品
	handler.HandleFunc("/api/item", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "success"})
		}
	})

	// 模拟获取商品
	handler.HandleFunc("/api/item/", func(w http.ResponseWriter, r *http.Request) {
		item := models.Item{
			ItemId:     "test_item",
			Categories: []string{"women", "tops"},
			Labels:     []string{"brand:zara", "style:casual"},
			Timestamp:  time.Now(),
		}
		json.NewEncoder(w).Encode(item)
	})

	// 模拟批量插入商品
	handler.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	})

	// 模拟插入反馈
	handler.HandleFunc("/api/feedback", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	})

	// 模拟获取推荐
	handler.HandleFunc("/api/recommend/", func(w http.ResponseWriter, r *http.Request) {
		items := []models.RecommendItem{
			{ItemId: "item_001", Score: 0.95},
			{ItemId: "item_002", Score: 0.88},
		}
		json.NewEncoder(w).Encode(items)
	})

	// 模拟获取相似商品
	handler.HandleFunc("/api/item/test_item/neighbors", func(w http.ResponseWriter, r *http.Request) {
		items := []models.RecommendItem{
			{ItemId: "similar_001", Score: 0.92},
			{ItemId: "similar_002", Score: 0.85},
		}
		json.NewEncoder(w).Encode(items)
	})

	return httptest.NewServer(handler)
}

func TestHealthCheck(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "fashion-recommend-api", response["service"])
}

func TestCreateUser(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	user := models.NewFashionUser("test_user_001").
		WithGender("female").
		WithStyle("casual").
		Build()

	jsonData, _ := json.Marshal(user)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/user", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "user created successfully", response["message"])
	assert.Equal(t, "test_user_001", response["user_id"])
}

func TestCreateItem(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	item := models.NewFashionItem("test_item_001").
		WithCategories("women", "tops").
		WithBrand("zara").
		WithStyle("casual").
		Build()

	jsonData, _ := json.Marshal(item)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/item", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "item created successfully", response["message"])
	assert.Equal(t, "test_item_001", response["item_id"])
}

func TestCreateFeedback(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	feedbacks := []models.Feedback{
		{
			FeedbackType: "purchase",
			UserId:       "user_001",
			ItemId:       "item_001",
			Value:        1.0,
			Timestamp:    time.Now(),
		},
	}

	jsonData, _ := json.Marshal(feedbacks)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/feedback", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "feedback created successfully", response["message"])
}

func TestGetRecommend(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/recommend/user_001?n=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "user_001", response["user_id"])
	assert.NotNil(t, response["items"])
}

func TestGetSimilarItems(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/similar/test_item?n=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "test_item", response["item_id"])
	assert.NotNil(t, response["items"])
}

func TestCreateUsers_Batch(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	users := []models.User{
		models.NewFashionUser("user_001").WithGender("female").Build(),
		models.NewFashionUser("user_002").WithGender("male").Build(),
	}

	jsonData, _ := json.Marshal(users)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "users created successfully", response["message"])
	assert.Equal(t, float64(2), response["count"])
}

func TestCreateItems_Batch(t *testing.T) {
	mockServer := MockGorseServer()
	defer mockServer.Close()

	server := NewServer(mockServer.URL, "")
	router := server.GetRouter()

	items := []models.Item{
		models.NewFashionItem("item_001").WithBrand("zara").Build(),
		models.NewFashionItem("item_002").WithBrand("uniqlo").Build(),
	}

	jsonData, _ := json.Marshal(items)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/items", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "items created successfully", response["message"])
	assert.Equal(t, float64(2), response["count"])
}
