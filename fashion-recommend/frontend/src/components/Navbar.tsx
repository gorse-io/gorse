import { Link, useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { User, LogOut, Palette } from 'lucide-react'
import apiService from '../services/api'

export default function Navbar() {
  const navigate = useNavigate()
  const username = localStorage.getItem('username') || 'Guest'

  const handleLogout = async () => {
    const token = localStorage.getItem('token')
    if (token) {
      try {
        await apiService.logout(token)
      } catch (error) {
        console.error('Logout failed:', error)
      }
    }
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    navigate('/login')
  }

  return (
    <motion.nav
      className="sticky top-0 z-50 bg-gradient-to-r from-orange-100 to-red-100 backdrop-blur-md border-b-4 border-orange-400 shadow-lg"
      initial={{ y: -100 }}
      animate={{ y: 0 }}
      transition={{ duration: 0.5 }}
    >
      <div className="max-w-7xl mx-auto px-8">
        <div className="flex justify-between items-center h-16">
          {/* Logo */}
          <motion.div
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
          >
            <Link to="/" className="flex items-center space-x-2 group">
              <motion.div
                animate={{ rotate: 360 }}
                transition={{ duration: 20, repeat: Infinity, ease: 'linear' }}
              >
                <Palette className="h-5 w-5 text-orange-600" />
              </motion.div>
              <span className="text-lg font-serif font-bold bg-gradient-to-r from-orange-600 to-red-600 bg-clip-text text-transparent tracking-tight">
                Fashion Gallery
              </span>
            </Link>
          </motion.div>

          {/* Navigation Links */}
          <div className="flex items-center space-x-8">
            <motion.div
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.1 }}
            >
              <Link
                to="/"
                className="text-sm font-serif font-medium text-amber-900 hover:text-orange-600 transition-colors relative group"
              >
                Discover
                <span className="absolute bottom-0 left-0 w-0 h-0.5 bg-gradient-to-r from-orange-500 to-red-600 group-hover:w-full transition-all duration-300" />
              </Link>
            </motion.div>

            <motion.div
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.15 }}
            >
              <Link
                to="/ai-chat"
                className="text-sm font-serif font-medium text-amber-900 hover:text-orange-600 transition-colors relative group"
              >
                Curator
                <span className="absolute bottom-0 left-0 w-0 h-0.5 bg-gradient-to-r from-orange-500 to-red-600 group-hover:w-full transition-all duration-300" />
              </Link>
            </motion.div>

            <motion.div
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.2 }}
            >
              <Link
                to={`/user/${username}`}
                className="flex items-center space-x-2 text-sm font-serif font-medium text-amber-900 hover:text-orange-600 transition-colors"
              >
                <User className="h-4 w-4" />
                <span className="hidden sm:inline">{username}</span>
              </Link>
            </motion.div>

            <motion.button
              onClick={handleLogout}
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.25 }}
              className="flex items-center space-x-2 text-sm font-serif font-medium text-amber-900 hover:text-red-600 transition-colors"
            >
              <LogOut className="h-4 w-4" />
              <span className="hidden sm:inline">Sign Out</span>
            </motion.button>
          </div>
        </div>
      </div>
    </motion.nav>
  )
}
