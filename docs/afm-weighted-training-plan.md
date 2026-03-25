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

实现 AFM 模型的带权重训练，通过表达式配置样本权重，支持：
- 常量权重：`"1"`、`"2.5"`
- 基于 Value 的表达式：`"log(Value)"`、`"Value * 2"`、`"sqrt(Value)"`

## 2. 技术方案

### 2.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      配置层 (Config)                         │
│  FeedbackWeight map[string]string                           │
│  例: {"click": "1", "purchase": "5", "rating": "Value"}     │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    数据层 (Dataset)                          │
│  构造时:                                                     │
│  - 解析表达式字符串 (expr.Compile)                            │
│  - 遍历样本计算权重 (expr.Run)                                │
│  - 存储 SampleWeights []float32                              │
│  - Get() 返回 (indices, values, embeddings, target, weight)  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    模型层 (AFM)                              │
│  - 接收 weight 参数                                          │
│  - Fit() 使用 BCEWithLogits(target, output, weights)         │
└─────────────────────────────────────────────────────────────┘
```

**设计原则：**
- 无需 WeightExpression 抽象，直接 inline 解析
- 数据层在构造时一次性计算所有权重
- 模型层只接收数值型权重

### 2.2 配置格式

```toml
[recommend.collaborative.feedback_weight]
click = "1"           # 点击权重为常量 1
favorite = "2"        # 收藏权重为常量 2
purchase = "5"        # 购买权重为常量 5
rating = "Value"      # 评分权重等于评分值
view_time = "log(Value)"  # 观看时长用对数权重
score = "Value / 5"   # 归一化评分
```

### 2.3 数据结构

#### 2.3.1 配置扩展 ✅

```go
// config/config.go

type DataSourceConfig struct {
    // 现有字段...
    
    // 新增：feedback 权重配置
    FeedbackWeight map[string]string `mapstructure:"feedback_weight"`
}
```

#### 2.3.2 Dataset 扩展

```go
// model/ctr/data.go

import "github.com/expr-lang/expr"

type Dataset struct {
    // 现有字段...
    
    // 新增：每个样本的权重
    SampleWeights []float32
}

// 计算样本权重 (在数据集构造时调用)
func (dataset *Dataset) ComputeWeights(feedbackWeight map[string]string, 
                                        feedbackTypes []string, 
                                        feedbackValues []float64) {
    // 编译所有表达式
    programs := make(map[string]*vm.Program)
    for fbType, exprStr := range feedbackWeight {
        programs[fbType], _ = expr.Compile(exprStr, expr.Env(map[string]float64{"Value": 0}))
    }
    
    // 计算每个样本的权重
    dataset.SampleWeights = make([]float32, len(feedbackTypes))
    for i, fbType := range feedbackTypes {
        if program, ok := programs[fbType]; ok {
            env := map[string]float64{"Value": feedbackValues[i]}
            result, _ := expr.Run(program, env)
            dataset.SampleWeights[i] = float32(result.(float64))
        } else {
            dataset.SampleWeights[i] = 1.0  // 默认权重
        }
    }
}

// Get 返回样本数据，新增 weight 参数
func (dataset *Dataset) Get(i int) ([]int32, []float32, [][]float32, float32, float32) {
    // ...
    return indices, values, embedding, target, dataset.SampleWeights[i]
}
```

#### 2.3.3 AFM (模型层)

```go
// model/ctr/fm.go

// AFM 不需要添加任何字段
// 只需修改 Fit() 接收权重

func (fm *AFM) Fit(ctx context.Context, trainSet, testSet dataset.CTRSplit, config *FitConfig) Score {
    // ...
    for i := 0; i < trainSet.Count(); i++ {
        indices, values, embeddings, target, weight := trainSet.Get(i)
        weights = append(weights, weight)
        // ...
    }
    // 使用带权重的损失函数
    batchLoss := nn.BCEWithLogits(batchTarget, batchOutput, batchWeights)
}
```

### 2.4 数据流

```
1. 配置加载: FeedbackWeight = {"click": "1", "purchase": "5"}
                    ↓
2. Dataset 构造:
   - 编译表达式: expr.Compile("1"), expr.Compile("5")
   - 加载样本时记录 feedback_type 和 value
   - 计算权重: expr.Run(program, {"Value": value})
   - 存入 SampleWeights
                    ↓
3. 训练:
   - Get(i) 返回 weight
   - AFM 使用 BCEWithLogits(target, output, weights)
```

## 3. 实现计划

### Phase 1: 表达式解析 ✅

| 任务 | 文件 | 状态 |
|------|------|------|
| 添加 expr 依赖 | go.mod | ✅ |
| 在 Dataset 中集成表达式解析 | model/ctr/data.go | ✅ |
| 单元测试 | model/ctr/data_test.go | ✅ |

### Phase 2: 配置扩展 ✅

| 任务 | 文件 | 状态 |
|------|------|------|
| 添加 FeedbackWeight 配置项 | config/config.go | ✅ |
| 解析权重表达式配置 | config/config.go | ✅ (存为string) |

### Phase 3: 数据层扩展 ✅

| 任务 | 文件 | 状态 |
|------|------|------|
| Dataset 添加 SampleWeights | model/ctr/data.go | ✅ |
| 实现 ComputeWeights() | model/ctr/data.go | ✅ |
| 扩展 Get() 返回 weight | model/ctr/data.go | ✅ |
| 向后兼容处理 | model/ctr/data.go | ✅ |

### Phase 4: 模型层扩展

| 任务 | 文件 | 状态 |
|------|------|------|
| 修改 Fit() 接收权重 | model/ctr/fm.go | ⏳ |
| 修改 fm_xla.go | model/ctr/fm_xla.go | ⏳ |

### Phase 5: 测试

| 任务 | 状态 |
|------|------|
| 表达式解析测试 | ⏳ |
| 带权重训练测试 | ⏳ |
| 效果对比测试 | ⏳ |

## 4. expr 库使用示例

```go
package main

import (
    "fmt"
    "github.com/expr-lang/expr"
)

func main() {
    // 编译表达式
    program, _ := expr.Compile("log(Value) + 1", expr.Env(map[string]float64{"Value": 0}))
    
    // 执行表达式
    env := map[string]float64{"Value": 100.0}
    result, _ := expr.Run(program, env)
    fmt.Println(result) // 5.605...
}
```

## 5. 向后兼容性

1. **默认行为**：未配置 feedback_weight 时，所有样本权重为 1.0
2. **未知 feedback_type**：使用默认权重 1.0
3. **模型加载**：旧版模型无需修改

## 6. 示例配置

### 6.1 电商场景

```toml
[recommend.collaborative.feedback_weight]
click = "1"
cart = "3"
purchase = "10"
rating = "Value * 2"
```

### 6.2 视频场景

```toml
[recommend.collaborative.feedback_weight]
view = "log(Value + 1)"
like = "3"
share = "10"
comment = "5"
```

### 6.3 音乐场景

```toml
[recommend.collaborative.feedback_weight]
play = "1"
complete = "3"
skip = "0.1"
like = "5"
```

## 7. 提交记录

| 提交 | 说明 |
|------|------|
| `342c609` | docs: refactor architecture - model layer should not be aware of WeightExpression |
| `f7a0dd1` | feat: add feedback_weight config for weighted training |

## 8. 参考资料

- [expr-lang/expr](https://github.com/expr-lang/expr) - Go 表达式引擎
- [BCEWithLogits Loss](https://pytorch.org/docs/stable/generated/torch.nn.functional.binary_cross_entropy_with_logits.html)
