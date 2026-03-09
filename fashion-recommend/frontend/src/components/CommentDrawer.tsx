import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, Send, Heart, MessageCircle } from 'lucide-react'
import apiService from '../services/api'

interface Comment {
  id: number
  item_id: string
  user_id: string
  username: string
  content: string
  parent_id?: number
  reply_to_user_id?: string
  reply_to_username?: string
  likes_count: number
  replies_count: number
  created_at: string
  replies?: Comment[]
  is_liked: boolean
}

interface CommentDrawerProps {
  isOpen: boolean
  onClose: () => void
  itemId: string
  itemName: string
}

export default function CommentDrawer({ isOpen, onClose, itemId, itemName }: CommentDrawerProps) {
  const [comments, setComments] = useState<Comment[]>([])
  const [newComment, setNewComment] = useState('')
  const [replyTo, setReplyTo] = useState<{ id: number; username: string } | null>(null)
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)

  useEffect(() => {
    if (isOpen) {
      loadComments()
    }
  }, [isOpen, itemId])

  const loadComments = async () => {
    try {
      const username = localStorage.getItem('username') || 'guest'
      const data = await apiService.getComments(itemId, username)
      setComments(data.comments || [])
      setTotal(data.total || 0)
    } catch (error) {
      console.error('加载评论失败:', error)
    }
  }

  const handleSubmit = async () => {
    if (!newComment.trim()) return

    setLoading(true)
    try {
      const username = localStorage.getItem('username') || 'guest'
      await apiService.createComment({
        item_id: itemId,
        content: newComment,
        parent_id: replyTo?.id,
        reply_to_user_id: replyTo?.username,
        reply_to_username: replyTo?.username,
      }, username)

      setNewComment('')
      setReplyTo(null)
      await loadComments()
    } catch (error) {
      console.error('发送评论失败:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleLike = async (commentId: number, isLiked: boolean) => {
    try {
      const username = localStorage.getItem('username') || 'guest'
      if (isLiked) {
        await apiService.unlikeComment(commentId, username)
      } else {
        await apiService.likeComment(commentId, username)
      }
      await loadComments()
    } catch (error) {
      console.error('点赞失败:', error)
    }
  }

  const formatTime = (dateString: string) => {
    const date = new Date(dateString)
    const now = new Date()
    const diff = Math.floor((now.getTime() - date.getTime()) / 1000)

    if (diff < 60) return '刚刚'
    if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
    if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
    if (diff < 604800) return `${Math.floor(diff / 86400)}天前`
    return date.toLocaleDateString()
  }

  const CommentItem = ({ comment, isReply = false }: { comment: Comment; isReply?: boolean }) => (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      className={`${isReply ? 'ml-12 mt-2' : 'mb-4'}`}
    >
      <div className="flex items-start space-x-3">
        {/* 头像 */}
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-primary-400 to-pink-400 flex items-center justify-center text-white font-bold flex-shrink-0">
          {comment.username[0].toUpperCase()}
        </div>

        {/* 评论内容 */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center space-x-2">
            <span className="font-medium text-gray-900">{comment.username}</span>
            <span className="text-xs text-gray-400">{formatTime(comment.created_at)}</span>
          </div>

          {/* 回复标识 */}
          {comment.reply_to_username && (
            <div className="text-sm text-gray-600 mt-1">
              回复 <span className="text-primary-600">@{comment.reply_to_username}</span>
            </div>
          )}

          <p className="text-gray-700 mt-1 break-words">{comment.content}</p>

          {/* 操作按钮 */}
          <div className="flex items-center space-x-4 mt-2">
            <button
              onClick={() => handleLike(comment.id, comment.is_liked)}
              className="flex items-center space-x-1 text-gray-500 hover:text-red-500 transition-colors"
            >
              <Heart
                className={`h-4 w-4 ${comment.is_liked ? 'fill-red-500 text-red-500' : ''}`}
              />
              <span className="text-xs">{comment.likes_count > 0 ? comment.likes_count : ''}</span>
            </button>

            {!isReply && (
              <button
                onClick={() => setReplyTo({ id: comment.id, username: comment.username })}
                className="flex items-center space-x-1 text-gray-500 hover:text-primary-600 transition-colors"
              >
                <MessageCircle className="h-4 w-4" />
                <span className="text-xs">回复</span>
              </button>
            )}
          </div>

          {/* 回复列表 */}
          {comment.replies && comment.replies.length > 0 && (
            <div className="mt-3 space-y-2">
              {comment.replies.map((reply) => (
                <CommentItem key={reply.id} comment={reply} isReply />
              ))}
            </div>
          )}
        </div>
      </div>
    </motion.div>
  )

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* 遮罩层 */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 bg-black bg-opacity-50 z-40"
          />

          {/* 抽屉 */}
          <motion.div
            initial={{ y: '100%' }}
            animate={{ y: 0 }}
            exit={{ y: '100%' }}
            transition={{ type: 'spring', damping: 30, stiffness: 300 }}
            className="fixed bottom-0 left-0 right-0 bg-white rounded-t-3xl z-50 max-h-[80vh] flex flex-col"
          >
            {/* 头部 */}
            <div className="flex items-center justify-between p-4 border-b">
              <div>
                <h3 className="text-lg font-bold">评论 {total > 0 && `(${total})`}</h3>
                <p className="text-sm text-gray-500">{itemName}</p>
              </div>
              <button
                onClick={onClose}
                className="p-2 hover:bg-gray-100 rounded-full transition-colors"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            {/* 评论列表 */}
            <div className="flex-1 overflow-y-auto p-4">
              {comments.length === 0 ? (
                <div className="text-center py-12 text-gray-400">
                  <MessageCircle className="h-12 w-12 mx-auto mb-2 opacity-50" />
                  <p>还没有评论，快来抢沙发吧~</p>
                </div>
              ) : (
                <div>
                  {comments.map((comment) => (
                    <CommentItem key={comment.id} comment={comment} />
                  ))}
                </div>
              )}
            </div>

            {/* 输入框 */}
            <div className="border-t p-4 bg-white">
              {replyTo && (
                <div className="mb-2 flex items-center justify-between bg-gray-50 px-3 py-2 rounded-lg">
                  <span className="text-sm text-gray-600">
                    回复 <span className="text-primary-600">@{replyTo.username}</span>
                  </span>
                  <button
                    onClick={() => setReplyTo(null)}
                    className="text-gray-400 hover:text-gray-600"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
              )}

              <div className="flex items-center space-x-2">
                <input
                  type="text"
                  value={newComment}
                  onChange={(e) => setNewComment(e.target.value)}
                  onKeyPress={(e) => e.key === 'Enter' && handleSubmit()}
                  placeholder={replyTo ? `回复 @${replyTo.username}` : '说点什么...'}
                  className="flex-1 px-4 py-2 border border-gray-300 rounded-full focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                  disabled={loading}
                />
                <button
                  onClick={handleSubmit}
                  disabled={!newComment.trim() || loading}
                  className="bg-gradient-to-r from-primary-600 to-pink-600 text-white p-2 rounded-full hover:from-primary-700 hover:to-pink-700 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <Send className="h-5 w-5" />
                </button>
              </div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  )
}
