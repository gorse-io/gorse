import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import ProductCard from '../components/ProductCard'
import apiService, { RecommendItem } from '../services/api'
import { Loader2 } from 'lucide-react'

export default function HomePage() {
  const [items, setItems] = useState<RecommendItem[]>([])
  const [loading, setLoading] = useState(true)
  const [scrollY, setScrollY] = useState(0)
  const navigate = useNavigate()

  useEffect(() => {
    loadRecommendations()
  }, [])

  useEffect(() => {
    const handleScroll = () => setScrollY(window.scrollY)
    window.addEventListener('scroll', handleScroll)
    return () => window.removeEventListener('scroll', handleScroll)
  }, [])

  const loadRecommendations = async () => {
    try {
      setLoading(true)
      const data = await apiService.getRecommendations('user_001', 20)
      setItems(data.items)
    } catch (error) {
      console.error('Failed to load recommendations:', error)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50">
        <motion.div
          animate={{ rotate: 360 }}
          transition={{ duration: 2, repeat: Infinity, ease: 'linear' }}
        >
          <Loader2 className="h-8 w-8 text-black" />
        </motion.div>
      </div>
    )
  }

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: {
        staggerChildren: 0.1,
        delayChildren: 0.2
      }
    }
  }

  const itemVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: {
      opacity: 1,
      y: 0,
      transition: { duration: 0.6, ease: 'easeOut' }
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-b from-amber-50 via-orange-50 to-red-50 overflow-hidden">
      {/* Decorative Background Elements */}
      <div className="fixed inset-0 pointer-events-none opacity-30">
        <div className="absolute top-0 right-0 w-96 h-96 bg-orange-200 rounded-full blur-3xl" />
        <div className="absolute bottom-0 left-0 w-96 h-96 bg-red-200 rounded-full blur-3xl" />
      </div>

      {/* Hero Section - Artistic Elegant Style */}
      <section className="relative z-10 min-h-screen flex items-center justify-center pt-20">
        <div className="max-w-4xl mx-auto px-8 text-center relative z-20">
          <motion.div
            className="mb-8"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
          >
            <span className="inline-block px-6 py-3 bg-gradient-to-r from-orange-200 to-red-200 rounded-full text-amber-900 text-sm font-serif italic">
              ✨ Discover Fashion Through Artistry
            </span>
          </motion.div>

          <motion.h1
            className="text-6xl md:text-7xl font-serif font-bold text-amber-950 mb-6 leading-tight"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1, duration: 0.8 }}
            style={{ textShadow: '2px 2px 4px rgba(0,0,0,0.1)' }}
          >
            Curated Fashion
            <span className="block bg-gradient-to-r from-orange-600 via-red-600 to-amber-600 bg-clip-text text-transparent">
              A Journey of Style
            </span>
          </motion.h1>

          <motion.p
            className="text-xl text-amber-900 mb-8 max-w-2xl mx-auto leading-relaxed font-serif italic"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2, duration: 0.8 }}
          >
            Experience the art of fashion curation. Each piece tells a story, carefully selected to reflect your unique aesthetic and personal narrative.
          </motion.p>

          <motion.div
            className="flex flex-col sm:flex-row gap-6 justify-center mb-12"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3, duration: 0.8 }}
          >
            <motion.button
              whileHover={{ scale: 1.05, boxShadow: '0 10px 30px rgba(217, 119, 6, 0.3)' }}
              whileTap={{ scale: 0.95 }}
              className="px-8 py-4 bg-gradient-to-r from-orange-500 to-red-600 text-white font-serif font-bold rounded-lg text-lg transition-all"
            >
              Explore Collection
            </motion.button>

            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              className="px-8 py-4 border-2 border-orange-600 text-orange-600 font-serif font-bold rounded-lg text-lg hover:bg-orange-50 transition-all"
            >
              Learn Our Story
            </motion.button>
          </motion.div>

          <motion.div
            className="grid grid-cols-3 gap-8 text-center"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.4, duration: 0.8 }}
          >
            {[
              { number: '500+', label: 'Curated Pieces' },
              { number: '50K+', label: 'Style Seekers' },
              { number: '100%', label: 'Authentic' }
            ].map((stat, i) => (
              <div key={i}>
                <div className="text-3xl md:text-4xl font-serif font-bold bg-gradient-to-r from-orange-600 to-red-600 bg-clip-text text-transparent">
                  {stat.number}
                </div>
                <div className="text-amber-900 text-sm mt-2 font-serif">{stat.label}</div>
              </div>
            ))}
          </motion.div>
        </div>

        <motion.div
          animate={{ y: [0, 10, 0] }}
          transition={{ duration: 2, repeat: Infinity }}
          className="absolute bottom-8 left-1/2 transform -translate-x-1/2 z-20"
        >
          <svg
            className="w-6 h-6 text-orange-600"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 14l-7 7m0 0l-7-7m7 7V3"
            />
          </svg>
        </motion.div>
      </section>

      {/* Features Section */}
      <section className="relative z-10 py-24 px-8 bg-gradient-to-r from-orange-100/50 to-red-100/50 border-b-4 border-orange-300">
        <div className="max-w-7xl mx-auto">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
            viewport={{ once: true }}
            className="text-center mb-16"
          >
            <h2 className="text-4xl md:text-5xl font-serif font-bold text-amber-950 mb-4">
              Why Choose Our Collection
            </h2>
            <p className="text-xl text-amber-900 font-serif italic">
              Discover the artistry behind every curation
            </p>
          </motion.div>

          <motion.div
            initial="hidden"
            whileInView="visible"
            viewport={{ once: true }}
            variants={containerVariants}
            className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8"
          >
            {[
              { icon: '🎨', title: 'Artistic Curation', desc: 'Each piece selected with aesthetic vision' },
              { icon: '✨', title: 'Quality First', desc: 'Premium materials and craftsmanship' },
              { icon: '💎', title: 'Timeless Elegance', desc: 'Pieces that transcend trends' },
              { icon: '🌟', title: 'Personal Stories', desc: 'Every item has a narrative' },
              { icon: '🎭', title: 'Style Expression', desc: 'Reflect your unique personality' },
              { icon: '🌍', title: 'Global Discovery', desc: 'Curated from around the world' }
            ].map((feature, i) => (
              <motion.div
                key={i}
                variants={itemVariants}
                whileHover={{ y: -10 }}
                className="group p-8 rounded-lg border-l-4 border-orange-500 bg-white shadow-lg hover:shadow-xl transition-all duration-300"
              >
                <div className="text-5xl mb-4 group-hover:scale-110 transition-transform duration-300">
                  {feature.icon}
                </div>
                <h3 className="text-xl font-serif font-bold text-amber-950 mb-3">
                  {feature.title}
                </h3>
                <p className="text-amber-900 leading-relaxed font-serif">
                  {feature.desc}
                </p>
              </motion.div>
            ))}
          </motion.div>
        </div>
      </section>

      {/* Products Section */}
      <section className="relative z-10 py-24 px-8 bg-gradient-to-b from-amber-50 via-orange-50 to-red-50 border-b-4 border-orange-300">
        <div className="max-w-7xl mx-auto">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8 }}
            viewport={{ once: true }}
            className="text-center mb-16"
          >
            <h2 className="text-4xl md:text-5xl font-serif font-bold text-amber-950 mb-4">
              Curated for You
            </h2>
            <p className="text-xl text-amber-900 font-serif italic">
              Personalized selections that tell your story
            </p>
          </motion.div>

          {loading ? (
            <div className="flex items-center justify-center py-24">
              <motion.div
                animate={{ rotate: 360 }}
                transition={{ duration: 2, repeat: Infinity, ease: 'linear' }}
              >
                <Loader2 className="h-8 w-8 text-orange-600" />
              </motion.div>
            </div>
          ) : items.length > 0 ? (
            <motion.div
              className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-8"
              variants={containerVariants}
              initial="hidden"
              animate="visible"
            >
              {items.map((item) => (
                <motion.div
                  key={item.item_id}
                  variants={itemVariants}
                  whileInView={{ opacity: 1, y: 0 }}
                  initial={{ opacity: 0, y: 20 }}
                  viewport={{ once: true, margin: '-100px' }}
                >
                  <ProductCard
                    id={item.item_id}
                    name={item.item_id}
                    score={item.score}
                  />
                </motion.div>
              ))}
            </motion.div>
          ) : (
            <motion.div
              className="text-center py-24"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 0.3 }}
            >
              <p className="text-amber-900 text-base font-serif">
                No recommendations available at this time
              </p>
            </motion.div>
          )}
        </div>
      </section>

      {/* CTA Section */}
      <section className="relative z-10 py-24 px-8 bg-gradient-to-r from-orange-200 via-red-200 to-amber-200 border-b-4 border-orange-400">
        <div className="max-w-4xl mx-auto text-center">
          <motion.h2
            className="text-4xl md:text-5xl font-serif font-bold text-amber-950 mb-6"
            initial={{ opacity: 0 }}
            whileInView={{ opacity: 1 }}
            viewport={{ once: true }}
          >
            Begin Your Fashion Journey
          </motion.h2>
          <motion.p
            className="text-xl text-amber-900 mb-8 max-w-2xl mx-auto font-serif italic"
            initial={{ opacity: 0 }}
            whileInView={{ opacity: 1 }}
            viewport={{ once: true }}
          >
            Join our community of style enthusiasts discovering authentic fashion through artistry and personal expression
          </motion.p>
          <motion.div
            className="flex flex-col sm:flex-row gap-6 justify-center"
            initial={{ opacity: 0 }}
            whileInView={{ opacity: 1 }}
            viewport={{ once: true }}
          >
            <motion.button
              whileHover={{ scale: 1.05, boxShadow: '0 10px 30px rgba(217, 119, 6, 0.3)' }}
              whileTap={{ scale: 0.95 }}
              className="px-8 py-4 bg-gradient-to-r from-orange-500 to-red-600 text-white font-serif font-bold rounded-lg text-lg transition-all"
            >
              Start Exploring
            </motion.button>

            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              className="px-8 py-4 border-2 border-orange-600 text-orange-600 font-serif font-bold rounded-lg text-lg hover:bg-orange-100 transition-all"
            >
              Learn More
            </motion.button>
          </motion.div>
        </div>
      </section>
    </div>
  )
}
