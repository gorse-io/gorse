# AFM 带权重训练开发计划

## 1. 背景

### 1.1 问题分析

在推荐系统中，用户行为有多种类型，例如：
- **点击 (click)** - 隐式反馈，数量多但价值较低
- **收藏 (favorite)** - 显式反馈，价值中等
- **购买 (purchase)** - 显式反馈，数量少但价值高
- **评分 (rating)** - 显式反馈，有具体数值

当前 AFM 模型对所有样本使用相同的权重训练，忽略了不同 feedback type 的价值差异。

### 1.2 目标

实现 AFM 模型的带权重训练，为每个 feedback type 设置不同的权重，使模型更关注高价值行为。

## 2. 技术方案

### 2.1 当前实现

**数据流：**
```
Feedback {FeedbackType, UserId, ItemId, Value, ...}
    ↓
Dataset {Users, Items, Target, ...}
    ↓
AFM.Fit() → BCEWithLogits(target, output, nil)  // weights = nil
```

**发现：**
- `nn.BCEWithLogits(target, prediction, weights)` 已支持 weights 参数
- 只需传递权重张量即可实现带权重训练

### 2.2 设计方案

#### 2.2.1 权重配置

```toml
[recommend.collaborative.weights]
click = 1.0      # 基准权重
favorite = 2.0   # 收藏权重为点击的2倍
purchase = 5.0   # 购买权重为点击的5倍
rating = 3.0     # 评分权重
```

#### 2.2.2 数据结构扩展

```go
// Dataset 扩展
type Dataset struct {
    // 现有字段...
    FeedbackTypes []string  // 每个样本对应的 feedback type
}

// AFM 扩展
type AFM struct {
    // 现有字段...
    Weights map[string]float32  // feedback_type -> weight
}
```

#### 2.2.3 训练流程

```
1. 加载数据时记录每个样本的 feedback_type
2. 根据配置将 feedback_type 映射为权重
3. 构建权重张量 weights
4. 训练时使用 BCEWithLogits(target, output, weights)
```

### 2.3 实现细节

#### 2.3.1 Dataset.Get() 扩展

```go
// 现有
func (dataset *Dataset) Get(i int) ([]int32, []float32, [][]float32, float32)

// 扩展
func (dataset *Dataset) Get(i int) ([]int32, []float32, [][]float32, float32, string)
//                                                            feedback_type ↑
```

#### 2.3.2 AFM.Fit() 修改

```go
// 构建权重张量
weightsData := make([]float32, len(batchTarget))
for i, feedbackType := range batchFeedbackTypes {
    weightsData[i] = fm.Weights[feedbackType]
}
weightsTensor := nn.NewTensor(weightsData, batchSize)

// 带权重损失
batchLoss := nn.BCEWithLogits(batchTarget, batchOutput, weightsTensor)
```

## 3. 实现计划

### Phase 1: 数据层扩展

| 任务 | 文件 | 优先级 |
|------|------|--------|
| Dataset 添加 FeedbackTypes 字段 | model/ctr/data.go | P0 |
| 修改数据加载流程记录 feedback_type | model/ctr/data.go | P0 |
| 扩展 Get() 返回 feedback_type | model/ctr/data.go | P0 |

### Phase 2: 模型层扩展

| 任务 | 文件 | 优先级 |
|------|------|--------|
| AFM 添加 Weights 字段 | model/ctr/fm.go | P0 |
| 修改 Fit() 支持带权重训练 | model/ctr/fm.go | P0 |
| 修改 fm_xla.go 支持带权重训练 | model/ctr/fm_xla.go | P0 |
| 模型序列化保存 Weights | model/ctr/fm.go | P0 |

### Phase 3: 配置支持

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 添加权重配置解析 | config/config.go | P0 |
| 支持从配置加载 Weights | model/ctr/fm.go | P0 |

### Phase 4: 测试

| 任务 | 优先级 |
|------|--------|
| 单元测试：带权重训练 | P0 |
| 对比测试：有无权重效果对比 | P1 |

## 4. 向后兼容性

1. **默认行为**：未配置权重时，所有样本权重为 1.0（等价于原行为）
2. **模型加载**：旧版模型无 Weights 字段时，使用默认权重
3. **API 兼容**：Get() 方法保持向后兼容

## 5. 待讨论问题

### 5.1 数据来源

**问题：训练数据如何关联 feedback_type？**

**选项 A：从数据库加载**
- 从 feedback 表读取，保留 feedback_type
- 需要修改数据加载流程

**选项 B：从配置文件加载**
- 训练数据格式扩展，增加 feedback_type 字段
- 兼容 libfm 格式扩展

### 5.2 权重归一化

**问题：是否需要对权重归一化？**

**选项 A：不归一化**
- 保持原始权重值
- 权重影响梯度大小

**选项 B：归一化**
- 权重归一化到 [0, 1] 或均值为 1
- 更稳定的训练过程

### 5.3 XLA/GoMLX 版本

**问题：fm_xla.go 是否同步支持？**

GoMLX 的损失函数：
```go
losses.BinaryCrossentropyLogits(labels, predictions)
```

需要检查是否支持 sample weights。

## 6. 参考资料

- [BCEWithLogits Loss](https://pytorch.org/docs/stable/generated/torch.nn.functional.binary_cross_entropy_with_logits.html)
- [Sample Weights in Deep Learning](https://scikit-learn.org/stable/modules/generated/sklearn.utils.class_weight.compute_sample_weight.html)
