import { useParams } from 'react-router-dom'

export default function ProductDetail() {
  const { id } = useParams()
  
  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-3xl font-bold">商品详情: {id}</h1>
    </div>
  )
}
