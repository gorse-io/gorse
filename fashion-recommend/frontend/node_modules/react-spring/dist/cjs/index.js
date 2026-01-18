'use strict'
if (process.env.NODE_ENV === 'production') {
  module.exports = require('./react-spring.production.min.cjs')
} else {
  module.exports = require('./react-spring.development.cjs')
}