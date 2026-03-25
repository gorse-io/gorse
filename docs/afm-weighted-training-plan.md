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

### 2.1 配置格式

```toml
[recommend.collaborative.feedback_weight]
click = "1"           # 点击权重为常量 1
favorite = "2"        # 收藏权重为常量 2
purchase = "5"        # 购买权重为常量 5
rating = "Value"      # 评分权重等于评分值
view_time = "log(Value)"  # 观看时长用对数权重
score = "Value / 5"   # 归一化评分
```

### 2.2 表达式设计

**支持的语法：**

| 类型 | 示例 | 说明 |
|------|------|------|
| 常量 | `1`, `2.5` | 固定权重值 |
| 变量 | `Value` | 使用 feedback 的 value 字段 |
| 数学函数 | `log(Value)`, `sqrt(Value)`, `abs(Value)` | 常用数学函数 |
| 算术运算 | `Value * 2`, `Value / 10`, `Value + 1` | 四则运算 |
| 复合表达式 | `log(Value + 1)`, `sqrt(Value) * 2` | 组合使用 |

**实现方式：**
- 使用 `github.com/expr-lang/expr` 表达式引擎

### 2.3 数据结构

#### 2.3.1 WeightExpression

```go
// common/expression/weight.go

import "github.com/expr-lang/expr"

type WeightExpression struct {
    expr string
    program *vm.Program
}

func ParseWeightExpression(s string) (*WeightExpression, error) {
    program, err := expr.Compile(s, expr.Env(map[string]float64{"Value": 0}))
    if err != nil {
        return nil, err
    }
    return &WeightExpression{expr: s, program: program}, nil
}

func (e *WeightExpression) Evaluate(value float64) float32 {
    env := map[string]float64{"Value": value}
    result, _ := expr.Run(e.program, env)
    return float32(result.(float64))
}
```

#### 2.3.2 配置扩展

```go
// config/config.go

type Config struct {
    // 现有字段...
    
    // 新增：feedback 权重配置
    FeedbackWeight map[string]string `mapstructure:"feedback_weight"`
}
```

#### 2.3.3 AFM 扩展

```go
// model/ctr/fm.go

type AFM struct {
    // 现有字段...
    
    // 新增：feedback 权重表达式
    FeedbackWeight map[string]*WeightExpression
}
```

#### 2.3.4 Dataset 扩展

```go
// model/ctr/data.go

type Dataset struct {
    // 现有字段...
    
    // 新增：每个样本的 feedback type 和 value
    FeedbackTypes []string
    FeedbackValues []float64
}
```

### 2.4 训练流程

```
1. 解析配置中的 feedback_weight 表达式
2. 加载数据时记录每个样本的 feedback_type 和 value
3. 训练时：
   a. 根据 feedback_type 找到对应的权重表达式
   b. 用 value 计算实际权重
   c. 构建权重张量
   d. 使用 BCEWithLogits(target, output, weights) 计算损失
```

## 3. 实现计划

### Phase 1: 表达式解析

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 添加 expr 依赖 | go.mod | P0 |
| 实现 WeightExpression 类型 | common/expression/weight.go | P0 |
| 支持常量和 Value 变量 | common/expression/weight.go | P0 |
| 支持数学函数 (log, sqrt, abs) | common/expression/weight.go | P1 |
| 单元测试 | common/expression/weight_test.go | P0 |

### Phase 2: 配置扩展

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 添加 FeedbackWeight 配置项 | config/config.go | P0 |
| 解析权重表达式配置 | config/config.go | P0 |
| 配置验证 | config/config.go | P0 |

### Phase 3: 数据层扩展

| 任务 | 文件 | 优先级 |
|------|------|--------|
| Dataset 添加 FeedbackTypes/FeedbackValues | model/ctr/data.go | P0 |
| 修改数据加载流程 | model/ctr/data.go | P0 |
| 向后兼容处理 | model/ctr/data.go | P0 |

### Phase 4: 模型层扩展

| 任务 | 文件 | 优先级 |
|------|------|--------|
| AFM 添加 FeedbackWeight 字段 | model/ctr/fm.go | P0 |
| 修改 Fit() 支持带权重训练 | model/ctr/fm.go | P0 |
| 修改 fm_xla.go | model/ctr/fm_xla.go | P0 |
| 模型序列化 | model/ctr/fm.go | P0 |

### Phase 5: 测试

| 任务 | 优先级 |
|------|--------|
| 表达式解析测试 | P0 |
| 带权重训练测试 | P0 |
| 效果对比测试 | P1 |

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

**expr 特点：**
- 安全的表达式执行
- 支持丰富的运算符和函数
- 编译后可重复执行，性能好
- 活跃维护

## 5. 向后兼容性

1. **默认行为**：未配置 feedback_weight 时，所有样本权重为 1.0
2. **未知 feedback_type**：使用默认权重 1.0
3. **模型加载**：旧版模型无 FeedbackWeight 字段时，使用默认权重

## 6. 示例配置

### 6.1 电商场景

```toml
[recommend.collaborative.feedback_weight]
click = "1"
cart = "3"
purchase = "10"
rating = "Value * 2"  # 1-5星映射到 2-10
```

### 6.2 视频场景

```toml
[recommend.collaborative.feedback_weight]
view = "log(Value + 1)"  # 观看时长对数权重
like = "3"
share = "10"
comment = "5"
```

### 6.3 音乐场景

```toml
[recommend.collaborative.feedback_weight]
play = "1"
complete = "3"           # 完整播放
skip = "0.1"             # 跳过（低权重负样本）
like = "5"
```

## 7. 参考资料

- [expr-lang/expr](https://github.com/expr-lang/expr) - Go 表达式引擎
- [BCEWithLogits Loss](https://pytorch.org/docs/stable/generated/torch.nn.functional.binary_cross_entropy_with_logits.html)
- [Sample Weights in sklearn](https://scikit-learn.org/stable/modules/generated/sklearn.utils.class_weight.compute_sample_weight.html)
