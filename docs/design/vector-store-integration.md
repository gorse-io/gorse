# Vector Store 集成设计文档

## 背景

Gorse 目前使用两种存储：
- **Data Store**: 存储用户、物品、反馈等持久化数据
- **Cache Store**: 存储推荐结果缓存

Vector Store 已在 protocol 和 storage 层实现，但尚未集成到配置系统和分发链路中。

## 目标

1. 在配置中添加 Vector Store 支持
2. 在 Master 中初始化 Vector Store 并通过 gRPC 分发
3. 在 Worker 和 Server 中使用 Vector Store

## 当前架构

### 配置结构

```go
type DatabaseConfig struct {
    DataStore        string      `mapstructure:"data_store"`
    CacheStore       string      `mapstructure:"cache_store"`
    // ...
}
```

### 存储初始化流程 (master.go)

```go
// 1. 打开 Data Store
m.DataClient, err = data.Open(m.Config.Database.DataStore, ...)

// 2. 打开 Cache Store  
m.CacheClient, err = cache.Open(m.Config.Database.CacheStore, ...)

// 3. 注册 gRPC 服务
protocol.RegisterCacheStoreServer(m.grpcServer, cache.NewProxyServer(m.CacheClient))
protocol.RegisterDataStoreServer(m.grpcServer, data.NewProxyServer(m.DataClient))
```

## 设计方案

### 1. 配置扩展

在 `config/config.go` 的 `DatabaseConfig` 中添加：

```go
type DatabaseConfig struct {
    DataStore        string      `mapstructure:"data_store" validate:"required,data_store"`
    CacheStore       string      `mapstructure:"cache_store" validate:"required,cache_store"`
    VectorStore      string      `mapstructure:"vector_store"`  // 新增：向量数据库连接字符串
    TablePrefix      string      `mapstructure:"table_prefix"`
    DataTablePrefix  string      `mapstructure:"data_table_prefix"`
    CacheTablePrefix string      `mapstructure:"cache_table_prefix"`
    VectorTablePrefix string     `mapstructure:"vector_table_prefix"`  // 新增：向量表前缀
    MySQL            MySQLConfig `mapstructure:"mysql"`
    Postgres         SQLConfig   `mapstructure:"postgres"`
    Redis            RedisConfig `mapstructure:"redis"`
}
```

### 2. 支持的向量数据库

| 数据库 | 连接字符串格式 | 特点 |
|--------|---------------|------|
| **Qdrant** | `qdrant://host:port` | 高性能，支持过滤 |
| **Milvus** | `milvus://host:port` | 可扩展，云原生 |
| **Elasticsearch** | `elasticsearch://host:port` | 成熟生态，全文搜索 |
| **SQLite-Vec** | `sqlite://path/to/db.db` | 轻量级，嵌入式 |
| **Redis** | `redis://host:port` | 复用现有 Redis |

### 3. Master 初始化流程修改

```go
// master.go

type Master struct {
    // ... existing fields ...
    
    // 新增：Vector Store 客户端
    VectorClient vectors.Database
}

func (m *Master) Serve() {
    // ... existing code ...
    
    // 1. 打开 Data Store
    m.DataClient, err = data.Open(m.Config.Database.DataStore, ...)
    
    // 2. 打开 Cache Store
    m.CacheClient, err = cache.Open(m.Config.Database.CacheStore, ...)
    
    // 3. 打开 Vector Store（新增）
    if m.Config.Database.VectorStore != "" {
        m.VectorClient, err = vectors.Open(
            m.Config.Database.VectorStore,
            m.Config.Database.VectorTablePrefix,
        )
        if err != nil {
            log.Logger().Fatal("failed to open vector store", zap.Error(err))
        }
        if err = m.VectorClient.Init(); err != nil {
            log.Logger().Fatal("failed to initialize vector store", zap.Error(err))
        }
    } else {
        m.VectorClient = vectors.NoDatabase{}
    }
    
    // 4. 注册 gRPC 服务
    protocol.RegisterCacheStoreServer(m.grpcServer, cache.NewProxyServer(m.CacheClient))
    protocol.RegisterDataStoreServer(m.grpcServer, data.NewProxyServer(m.DataClient))
    protocol.RegisterVectorStoreServer(m.grpcServer, vectors.NewProxyServer(m.VectorClient))  // 新增
}
```

### 4. Worker 使用 Vector Store

```go
// worker/worker.go

type Worker struct {
    // ... existing fields ...
    VectorClient protocol.VectorStoreClient  // 新增
}

func (w *Worker) Connect(...) {
    // ... existing code ...
    
    // 连接 Vector Store（新增）
    w.VectorClient = protocol.NewVectorStoreClient(conn)
}
```

### 5. Server 使用 Vector Store

```go
// server/server.go

type RestServer struct {
    // ... existing fields ...
    VectorClient vectors.Database  // 新增（本地或远程）
}

// API 扩展：向量搜索
func (s *RestServer) SearchSimilarItems(ctx *restful.Context) {
    // 使用 VectorClient 进行向量相似搜索
}
```

### 6. 配置文件示例

```toml
[database]
data_store = "mysql://user:pass@localhost/gorse"
cache_store = "redis://localhost:6379"
vector_store = "qdrant://localhost:6333"  # 新增

[database.redis]
max_search_results = 10000

# 向量搜索配置（新增）
[database.vector]
collection_prefix = "gorse_"
```

## 存储接口设计

### Vector Store 接口 (已存在于 protocol/vector_store.proto)

```protobuf
service VectorStore {
    rpc ListCollections(ListCollectionsRequest) returns (ListCollectionsResponse);
    rpc AddCollection(AddCollectionRequest) returns (google.protobuf.Empty);
    rpc DeleteCollection(DeleteCollectionRequest) returns (google.protobuf.Empty);
    rpc AddVectors(AddVectorsRequest) returns (google.protobuf.Empty);
    rpc DeleteVectors(DeleteVectorsRequest) returns (google.protobuf.Empty);
    rpc QueryVectors(QueryVectorsRequest) returns (QueryVectorsResponse);
}
```

### 数据库实现接口

```go
// storage/vectors/proxy.go (已存在)

type Database interface {
    Init() error
    Close() error
    
    // Collection 管理
    ListCollections(ctx context.Context) ([]string, error)
    AddCollection(ctx context.Context, name string, dimension int) error
    DeleteCollection(ctx context.Context, name string) error
    
    // 向量操作
    AddVectors(ctx context.Context, name string, vectors []Vector) error
    DeleteVectors(ctx context.Context, name string, ids []string) error
    QueryVectors(ctx context.Context, name string, query Vector, k int) ([]SearchResult, error)
}
```

## 使用场景

### 1. 物品相似度推荐 (Item-to-Item)

```go
// 获取物品向量
vector := item.Labels["embedding"]

// 搜索相似物品
results, _ := worker.VectorClient.QueryVectors(ctx, &protocol.QueryVectorsRequest{
    Collection: "items",
    QueryVector: vector,
    K: 10,
})
```

### 2. 用户个性化推荐

```go
// 基于用户向量搜索相似物品
userVector := computeUserEmbedding(user)
results, _ := worker.VectorClient.QueryVectors(ctx, &protocol.QueryVectorsRequest{
    Collection: "items",
    QueryVector: userVector,
    K: 20,
    Filter: "category == 'electronics'",
})
```

### 3. 语义搜索

```go
// 文本转向量（使用 OpenAI embedding）
queryVector := openai.Embedding(query)

// 向量搜索
results, _ := server.VectorClient.QueryVectors(ctx, &protocol.QueryVectorsRequest{
    Collection: "items",
    QueryVector: queryVector,
    K: 10,
})
```

## 部署架构

### Standalone 模式

```
┌─────────────────────────────────────┐
│         gorse-in-one                │
│                                     │
│  ┌──────────┐  ┌──────────────────┐ │
│  │  Master  │  │ Data/Cache/Vector│ │
│  │  Worker  │  │     Store        │ │
│  │  Server  │  │  (Embedded)      │ │
│  └──────────┘  └──────────────────┘ │
└─────────────────────────────────────┘
```

### Distributed 模式

```
                    ┌─────────────┐
                    │   Master    │
                    │  (gRPC:8080)│
                    └──────┬──────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
    ┌─────▼─────┐    ┌─────▼─────┐    ┌─────▼─────┐
    │  Worker 1 │    │  Worker 2 │    │  Server   │
    │           │    │           │    │           │
    └─────┬─────┘    └─────┬─────┘    └─────┬─────┘
          │                │                │
          └────────────────┼────────────────┘
                           │
          ┌────────────────┴────────────────┐
          │                                 │
    ┌─────▼─────┐  ┌─────────────┐  ┌──────▼──────┐
    │ Data Store│  │Cache Store  │  │Vector Store │
    │  (MySQL)  │  │  (Redis)    │  │  (Qdrant)   │
    └───────────┘  └─────────────┘  └─────────────┘
```

## 迁移计划

### Phase 1: 配置支持
- [ ] 扩展 `DatabaseConfig` 添加 `VectorStore` 字段
- [ ] 添加配置验证器 `validate:"vector_store"`
- [ ] 更新配置文档

### Phase 2: Master 集成
- [ ] Master 初始化 Vector Store
- [ ] 注册 Vector Store gRPC 服务
- [ ] 添加健康检查

### Phase 3: Worker 集成
- [ ] Worker 连接 Vector Store
- [ ] 实现基于向量的推荐器

### Phase 4: Server 集成
- [ ] Server 连接 Vector Store
- [ ] 添加向量搜索 API
- [ ] 实现语义搜索功能

### Phase 5: Helm Chart 更新
- [ ] 添加 Vector Store 连接配置
- [ ] 更新文档和示例

## 兼容性

- **向后兼容**: `vector_store` 配置可选，默认为空（NoDatabase）
- **无缝迁移**: 现有部署无需修改配置
- **渐进式采用**: 可以按需启用向量功能

## 测试计划

1. **单元测试**: 各向量数据库实现
2. **集成测试**: Master-Worker-Server 链路
3. **性能测试**: 向量搜索延迟和吞吐量
4. **故障测试**: Vector Store 不可用时的降级

## 参考文档

- [Qdrant Documentation](https://qdrant.tech/documentation/)
- [Milvus Documentation](https://milvus.io/docs)
- [SQLite-Vec](https://github.com/asg017/sqlite-vec)
- [Gorse Protocol Buffer Definitions](../../protocol/)
