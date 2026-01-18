import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Heart, MessageCircle } from 'lucide-react'
import CommentDrawer from './CommentDrawer'
import apiService from '../services/api'

interface ProductCardProps {
  id: string
  name: string
  score: number
  onLike?: () => void
  onAddToCart?: () => void
}

export default function ProductCard({ id, name, score }: ProductCardProps) {
  const [liked, setLiked] = useState(false)
  const [likeCount, setLikeCount] = useState(0)
  const [showComments, setShowComments] = useState(false)
  const [loading, setLoading] = useState(false)
  const [imageLoaded, setImageLoaded] = useState(false)

  useEffect(() => {
    loadLikeInfo()
  }, [id])

  const loadLikeInfo = async () => {
    try {
      const username = localStorage.getItem('username') || 'guest'
      const data = await apiService.getProductLikes(id, username)
      setLiked(data.is_liked)
      setLikeCount(data.like_count)
    } catch (error) {
      console.error('Failed to load like info:', error)
    }
  }

  const handleLike = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (loading) return

    setLoading(true)
    try {
      const username = localStorage.getItem('username') || 'guest'
      
      if (liked) {
        const data = await apiService.unlikeProduct(id, username)
        setLiked(false)
        setLikeCount(data.like_count)
      } else {
        const data = await apiService.likeProduct(id, username)
        setLiked(true)
        setLikeCount(data.like_count)
      }
    } catch (error) {
      console.error('Failed to toggle like:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleComment = (e: React.MouseEvent) => {
    e.stopPropagation()
    setShowComments(true)
  }

  return (
    <>
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
        className="group cursor-pointer h-full"
      >
        {/* Product Image Container */}
        <motion.div
          className="relative mb-6 overflow-hidden rounded-lg border-l-4 border-orange-500 bg-white shadow-lg"
          whileHover={{ y: -10, boxShadow: '0 20px 40px rgba(217, 119, 6, 0.2)' }}
          transition={{ duration: 0.3 }}
        >
          {/* Image Container */}
          <div className="aspect-[3/4] flex items-center justify-center overflow-hidden relative bg-gradient-to-br from-orange-100 to-red-100">
            <motion.img
              src={`/images/${id}.jpg`}
              alt={name}
              className="w-full h-full object-cover"
              initial={{ opacity: 0, scale: 1.1 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ duration: 0.4 }}
              onLoad={() => setImageLoaded(true)}
              onError={(e) => {
                e.currentTarget.style.display = 'none';
                e.currentTarget.parentElement?.classList.add('flex', 'items-center', 'justify-center');
                const span = document.createElement('span');
                span.className = 'text-4xl font-light text-orange-300';
                span.innerText = id;
                e.currentTarget.parentElement?.appendChild(span);
              }}
            />

            {/* Image Loading Skeleton */}
            {!imageLoaded && (
              <div className="absolute inset-0 bg-gradient-to-r from-orange-200 via-red-200 to-orange-200 animate-pulse" />
            )}
          </div>

          {/* Like Button */}
          <motion.button
            onClick={handleLike}
            disabled={loading}
            whileHover={{ scale: 1.1 }}
            whileTap={{ scale: 0.95 }}
            className="absolute top-4 right-4 p-2 bg-white border-2 border-orange-400 rounded-lg hover:bg-orange-50 transition-all disabled:opacity-50 z-10 shadow-md"
          >
            <Heart
              className={`h-4 w-4 transition-colors ${
                liked ? 'fill-red-500 text-red-500' : 'text-orange-400'
              }`}
            />
          </motion.button>

          {/* Like Count Badge */}
          {likeCount > 0 && (
            <motion.div
              initial={{ scale: 0, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              className="absolute top-4 left-4 px-3 py-1 bg-white border-2 border-orange-400 text-xs font-serif font-medium text-orange-600 rounded-lg shadow-md"
            >
              {likeCount}
            </motion.div>
          )}
        </motion.div>

        {/* Product Info */}
        <div className="pt-4 px-1">
          <h3 className="text-sm font-serif font-semibold text-amber-950 mb-2 tracking-tight">
            {name || `Product ${id}`}
          </h3>

          {score && (
            <p className="text-xs text-amber-900 mb-4 font-serif">
              Rating: <span className="text-orange-600 font-semibold">{score.toFixed(1)}</span>/10
            </p>
          )}

          {/* Actions */}
          <div className="flex items-center justify-between">
            <span className="text-base font-serif font-semibold bg-gradient-to-r from-orange-600 to-red-600 bg-clip-text text-transparent">$299</span>

            <div className="flex items-center space-x-3">
              {/* Comment Button */}
              <motion.button
                onClick={handleComment}
                whileHover={{ scale: 1.1 }}
                whileTap={{ scale: 0.95 }}
                className="p-1.5 border-2 border-orange-400 hover:bg-orange-50 rounded-lg transition-colors"
              >
                <MessageCircle className="h-4 w-4 text-orange-600" />
              </motion.button>

              {/* Add to Cart Button */}
              <motion.button
                whileHover={{ scale: 1.05, boxShadow: '0 10px 20px rgba(217, 119, 6, 0.3)' }}
                whileTap={{ scale: 0.95 }}
                className="px-4 py-1.5 bg-gradient-to-r from-orange-500 to-red-600 text-white text-xs font-serif font-medium rounded-lg hover:shadow-lg transition-all"
              >
                Add
              </motion.button>
            </div>
          </div>
        </div>
      </motion.div>

      {/* Comment Drawer */}
      <CommentDrawer
        isOpen={showComments}
        onClose={() => setShowComments(false)}
        itemId={id}
        itemName={name || `Product ${id}`}
      />
    </>
  )
}
