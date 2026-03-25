# AFM 模型数值特征支持开发计划

## 1. 背景分析

### 1.1 当前实现现状

#### 数据层面
- **Label 结构体**：已支持 `Name` 和 `Value` 字段
  ```go
  type Label struct {
      Name  string
      Value float32
  }
  ```
- **ConvertLabels 函数**：已支持数值特征转换（`json.Number` → `Label{Value: float32(value)}`）
- **Dataset.Get()**：返回 `indices, values` 支持特征值

#### 模型层面
- **AFM.Forward()** 当前处理方式：
  ```go
  v := fm.V.Forward(indices)      // 获取嵌入向量
  x := nn.Reshape(values, ...)    // 特征值 reshape
  vx := nn.BMM(v, x, ...)         // 嵌入向量 × 特征值
  ```
- 所有特征统一通过嵌入层处理，数值作为嵌入向量的缩放因子

### 1.2 问题分析

当前实现对数值特征的处理存在以下问题：

1. **缺乏特征类型区分**：无法区分类别特征和数值特征
2. **数值特征处理不优化**：
   - 类别特征：嵌入向量学习离散值的表示
   - 数值特征：直接缩放嵌入向量可能不是最优方案
3. **特征交互计算**：AFM 的注意力机制针对特征交互设计，对纯数值特征的处理需要优化

## 2. 目标设计

### 2.1 功能目标

1. 支持显式声明特征类型（类别/数值）
2. 为数值特征提供专门的编码方式
3. 优化数值特征与类别特征的交互计算
4. 保持向后兼容性

### 2.2 技术方案选项

#### 方案 A：Field-aware 数值嵌入
- 为数值特征设计专门的嵌入层
- 数值特征通过线性变换映射到嵌入空间
- 保留原始数值信息

#### 方案 B：离散化 + 嵌入
- 将数值特征离散化为桶
- 每个桶学习独立的嵌入向量
- 简单但可能损失精度

#### 方案 C：混合方案（推荐）
- 数值特征使用线性投影 + 激活函数
- 类别特征使用传统嵌入
- 两者在嵌入空间进行交互

## 3. 详细设计

### 3.1 特征元数据扩展

#### 3.1.1 特征类型定义

```go
// model/ctr/data.go

type FeatureType int

const (
    FeatureTypeCategorical FeatureType = iota  // 类别特征
    FeatureTypeNumerical                        // 数值特征
)

type FeatureMeta struct {
    Name     string
    Type     FeatureType
    Dim      int       // 嵌入维度（数值特征为投影维度）
}

type FeatureSchema struct {
    UserFeatures   []FeatureMeta
    ItemFeatures   []FeatureMeta
    ContextFeatures []FeatureMeta
}
```

#### 3.1.2 UnifiedIndex 扩展

```go
// dataset/unified_index.go

type UnifiedIndex interface {
    // 现有方法...
    
    // 新增方法
    GetFeatureType(index int32) FeatureType
    IsNumerical(index int32) bool
    CountNumericalFeatures() int32
    CountCategoricalFeatures() int32
}
```

### 3.2 数据处理扩展

#### 3.2.1 数值特征处理

```go
// model/ctr/data.go

type NumericalEncoder struct {
    // 归一化参数
    Mean   []float32
    Std    []float32
    Min    []float32
    Max    []float32
}

func (e *NumericalEncoder) Normalize(value float32, index int) float32 {
    return (value - e.Mean[index]) / e.Std[index]
}
```

#### 3.2.2 数据加载支持

```go
// 扩展配置格式支持数值特征声明
type FeatureConfig struct {
    Name string `yaml:"name"`
    Type string `yaml:"type"` // "categorical" or "numerical"
}
```

### 3.3 AFM 模型扩展

#### 3.3.1 新增数值特征处理层

```go
// model/ctr/fm.go

type AFM struct {
    // 现有字段...
    
    // 新增：数值特征处理
    NumericalProjection nn.Layer  // 数值特征投影层
    NumericalBias       *nn.Tensor
}

func (fm *AFM) Init(trainSet dataset.CTRSplit) {
    // 现有初始化...
    
    // 初始化数值特征投影
    numNumerical := trainSet.GetIndex().CountNumericalFeatures()
    if numNumerical > 0 {
        fm.NumericalProjection = nn.NewLinear(1, fm.nFactors)
        fm.NumericalBias = nn.Zeros(fm.nFactors)
    }
}
```

#### 3.3.2 Forward 方法改进

```go
func (fm *AFM) Forward(indices, values *nn.Tensor, embeddings []*nn.Tensor, jobs int) *nn.Tensor {
    batchSize := indices.Shape()[0]
    
    // 分离类别特征和数值特征
    categoricalMask := fm.getCategoricalMask(indices)
    numericalMask := fm.getNumericalMask(indices)
    
    // 类别特征处理（原有逻辑）
    v := fm.V.Forward(indices)
    x := nn.Reshape(values, batchSize, fm.numDimension, 1)
    vx := nn.BMM(v, x, true, false, jobs)
    
    // 数值特征处理（新增）
    var numericalEmbedding *nn.Tensor
    if fm.NumericalProjection != nil {
        numericalEmbedding = fm.processNumericalFeatures(indices, values, numericalMask)
    }
    
    // 合并处理
    // ... 后续计算
}
```

### 3.4 序列化/反序列化扩展

```go
// model/ctr/fm.go

func (fm *AFM) Marshal(w io.Writer) error {
    // 现有序列化...
    
    // 序列化数值特征处理层
    if fm.NumericalProjection != nil {
        if err := encoding.WriteBool(w, true); err != nil {
            return err
        }
        if err := nn.Save(fm.NumericalProjection.Parameters(), w); err != nil {
            return err
        }
    } else {
        if err := encoding.WriteBool(w, false); err != nil {
            return err
        }
    }
    return nil
}
```

## 4. 实现计划

### Phase 1: 基础设施 (预计 2-3 天)

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 定义 FeatureType 和 FeatureMeta | model/ctr/data.go | P0 |
| 扩展 UnifiedIndex 接口 | dataset/unified_index.go | P0 |
| 实现特征类型判断方法 | dataset/unified_index.go | P0 |
| 添加单元测试 | dataset/unified_index_test.go | P1 |

### Phase 2: 数据处理 (预计 2-3 天)

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 实现 NumericalEncoder | model/ctr/data.go | P0 |
| 扩展 Dataset 支持特征类型 | model/ctr/data.go | P0 |
| 修改 ConvertLabels 支持类型声明 | model/ctr/data.go | P0 |
| 添加单元测试 | model/ctr/data_test.go | P1 |

### Phase 3: 模型扩展 (预计 3-4 天)

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 添加数值特征投影层 | model/ctr/fm.go | P0 |
| 修改 AFM.Forward() | model/ctr/fm.go | P0 |
| 扩展 AFM.Init() | model/ctr/fm.go | P0 |
| 扩展序列化/反序列化 | model/ctr/fm.go | P0 |
| 添加单元测试 | model/ctr/model_test.go | P1 |

### Phase 4: 集成与测试 (预计 2-3 天)

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 更新配置格式 | config/config.go | P0 |
| 集成测试 | model/ctr/model_test.go | P0 |
| 性能基准测试 | model/ctr/benchmark_test.go | P2 |
| 文档更新 | docs/ | P2 |

## 5. 配置示例

```toml
[recommend.collaborative]
model = "afm"

[recommend.collaborative.features]
# 用户特征
user_features = [
    { name = "age", type = "numerical" },
    { name = "gender", type = "categorical" },
    { name = "city", type = "categorical" }
]

# 物品特征  
item_features = [
    { name = "price", type = "numerical" },
    { name = "category", type = "categorical" },
    { name = "brand", type = "categorical" }
]
```

## 6. 向后兼容性

1. **默认行为**：未声明类型的特征视为类别特征
2. **模型加载**：旧版模型文件可正常加载
3. **API 兼容**：现有预测接口保持不变

## 7. 测试计划

### 7.1 单元测试

- [ ] FeatureType 判断正确性
- [ ] NumericalEncoder 归一化计算
- [ ] 数值特征前向传播
- [ ] 模型序列化/反序列化

### 7.2 集成测试

- [ ] 数值特征 + 类别特征混合训练
- [ ] 纯数值特征场景
- [ ] 与旧版模型对比测试

### 7.3 性能测试

- [ ] 训练时间对比
- [ ] 预测延迟对比
- [ ] 内存占用对比

## 8. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 性能下降 | 中 | 优化张量操作，缓存中间结果 |
| 数值不稳定 | 中 | 添加归一化，梯度裁剪 |
| 向后兼容破坏 | 高 | 完善迁移测试，提供迁移脚本 |

## 9. 参考资料

- [Attentional Factorization Machines: Learning the Weight of Feature Interactions via Attention Networks](https://arxiv.org/abs/1708.04617)
- [Field-aware Factorization Machines for CTR Prediction](https://www.csie.ntu.edu.tw/~cjlin/papers/ffm.pdf)
- [Deep & Cross Network for Ad Click Predictions](https://arxiv.org/abs/1708.05123)
