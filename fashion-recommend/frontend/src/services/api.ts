import axios from 'axios'

const API_BASE_URL = '/api'

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

export interface RecommendItem {
  item_id: string
  score: number
  categories?: string[]
}

export interface RecommendResponse {
  items: RecommendItem[]
  total: number
  user_id: string
}

export const apiService = {
  // 认证相关
  register: async (username: string, password: string, email: string) => {
    const response = await api.post('/auth/register', { username, password, email })
    return response.data
  },

  login: async (username: string, password: string) => {
    const response = await api.post('/auth/login', { username, password })
    return response.data
  },

  logout: async (token: string) => {
    const response = await api.post('/auth/logout', {}, {
      headers: { Authorization: token }
    })
    return response.data
  },

  getCurrentUser: async (token: string) => {
    const response = await api.get('/auth/me', {
      headers: { Authorization: token }
    })
    return response.data
  },

  // AI 相关
  chatWithAI: async (message: string, history: any[], sessionId?: string, userId?: string) => {
    const headers: any = {}
    if (sessionId) {
      headers['X-Session-ID'] = sessionId
    }
    if (userId) {
      headers['X-User-ID'] = userId
    }
    const response = await api.post('/ai/chat', { message, history }, { headers })
    return response.data
  },

  explainRecommendation: async (username: string, itemId: string, userPreferences: string[]) => {
    const response = await api.post('/ai/explain', {
      username,
      item_id: itemId,
      user_preferences: userPreferences
    })
    return response.data
  },

  getStyleAdvice: async (items: string[], occasion: string) => {
    const response = await api.post('/ai/style-advice', { items, occasion })
    return response.data
  },

  // 获取推荐
  getRecommendations: async (userId: string, n: number = 20, category?: string) => {
    const params: any = { n }
    if (category) params.category = category
    const response = await api.get<RecommendResponse>(`/recommend/${userId}`, { params })
    return response.data
  },

  // 获取相似商品
  getSimilarItems: async (itemId: string, n: number = 10) => {
    const response = await api.get(`/similar/${itemId}`, { params: { n } })
    return response.data
  },

  // 获取商品详情
  getItem: async (itemId: string) => {
    const response = await api.get(`/item/${itemId}`)
    return response.data
  },

  // 获取用户信息
  getUser: async (userId: string) => {
    const response = await api.get(`/user/${userId}`)
    return response.data
  },

  // 添加反馈
  addFeedback: async (feedback: any[]) => {
    const response = await api.post('/feedback', feedback)
    return response.data
  },

  // 评论相关
  getComments: async (itemId: string, userId: string) => {
    const response = await api.get(`/comments/${itemId}`, {
      headers: { 'X-User-ID': userId }
    })
    return response.data
  },

  createComment: async (comment: any, userId: string) => {
    const response = await api.post('/comments', comment, {
      headers: { 'X-User-ID': userId }
    })
    return response.data
  },

  likeComment: async (commentId: number, userId: string) => {
    const response = await api.post(`/comments/${commentId}/like`, {}, {
      headers: { 'X-User-ID': userId }
    })
    return response.data
  },

  unlikeComment: async (commentId: number, userId: string) => {
    const response = await api.delete(`/comments/${commentId}/like`, {
      headers: { 'X-User-ID': userId }
    })
    return response.data
  },

  // 商品点赞相关
  likeProduct: async (itemId: string, userId: string) => {
    const response = await api.post(`/products/${itemId}/like`, {}, {
      headers: { 'X-User-ID': userId }
    })
    return response.data
  },

  unlikeProduct: async (itemId: string, userId: string) => {
    const response = await api.delete(`/products/${itemId}/like`, {
      headers: { 'X-User-ID': userId }
    })
    return response.data
  },

  getProductLikes: async (itemId: string, userId: string) => {
    const response = await api.get(`/products/${itemId}/likes`, {
      headers: { 'X-User-ID': userId }
    })
    return response.data
  },

  getBatchProductLikes: async (itemIds: string[], userId: string) => {
    const response = await api.post('/products/likes/batch', 
      { item_ids: itemIds },
      { headers: { 'X-User-ID': userId } }
    )
    return response.data
  },
}

export default apiService
