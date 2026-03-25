# AFM 模型数值特征支持开发计划

## 1. 设计方案

### 1.1 核心思路

**数值特征与类别特征共享嵌入层，通过 `values` 进行缩放：**
- 类别特征：`embedding(idx) * 1`
- 数值特征：`embedding(idx) * scaled_value`

**不需要修改 AFM 的网络结构**，只需：
1. 在训练时用 `AutoScaler` 对数值特征进行归一化
2. 将 `AutoScaler` 与模型一起保存和加载

### 1.2 索引方案

**复用现有 label 索引**：
```
| user | item | user_label | item_label | context_label |
```

数值特征作为一种特殊的 label，使用统一索引：
- 例如：`user.age` → `EncodeUserLabel("age")`

### 1.3 AutoScaler 实现

```go
type AutoScaler struct {
    UseLog    bool         // true = log1p + MinMax, false = Robust
    MinMax    MinMaxScaler // 非负数据使用
    Robust    RobustScaler // 含负数数据使用
    HasRobust bool         // 是否使用 RobustScaler
}
```

**自动选择逻辑：**
- **有负数** → `RobustScaler`（对异常值鲁棒）
- **全非负** → `log1p` + `MinMaxScaler`（压缩长尾分布）

### 1.4 数值特征检测

**自动检测规则：**
- 遍历所有样本，收集每个特征的值
- 如果 `value != 1`，则判定为数值特征
- 为每个数值特征创建并拟合 `AutoScaler`

## 2. 实现进度

### Phase 1: Scaler 实现 ✅

| 任务 | 文件 | 状态 |
|------|------|------|
| MinMaxScaler | model/ctr/transformer.go | ✅ 完成 |
| RobustScaler | model/ctr/transformer.go | ✅ 完成 |
| AutoScaler | model/ctr/transformer.go | ✅ 完成 |
| 单元测试 | model/ctr/transformer_test.go | ✅ 完成 |

### Phase 2: AFM 集成 ✅

| 任务 | 文件 | 状态 |
|------|------|------|
| 添加 Scalers 字段 | model/ctr/fm.go | ✅ 完成 |
| 自动检测数值特征 | model/ctr/fm.go | ✅ 完成 |
| 训练时应用 scaler | model/ctr/fm.go | ✅ 完成 |
| 推理时应用 scaler | model/ctr/fm.go | ✅ 完成 |
| 模型序列化/反序列化 | model/ctr/fm.go | ✅ 完成 |
| 单元测试 | model/ctr/model_test.go | ✅ 通过 |

### Phase 3: 数据加载扩展 ⏳

| 任务 | 状态 |
|------|------|
| 数据格式支持数值字段 | 待讨论 |
| 数值特征显式声明配置 | 待讨论 |

### Phase 4: 集成测试 ⏳

| 任务 | 状态 |
|------|------|
| 真实数据集测试 | 待完成 |

## 3. 已完成的功能

### 3.1 MinMaxScaler

```go
type MinMaxScaler struct {
    Min float32
    Max float32
}

// X_scaled = (X - Min) / (Max - Min)
// range == 0 时返回 1
```

### 3.2 RobustScaler

```go
type RobustScaler struct {
    Median float32  // 中位数
    Q1     float32  // 25th percentile
    Q3     float32  // 75th percentile
    IQR    float32  // Q3 - Q1
}

// X_scaled = (X - Median) / IQR
```

### 3.3 AutoScaler

自动选择最佳归一化方法：
- 有负数 → RobustScaler（对异常值鲁棒）
- 全非负 → log1p + MinMaxScaler（压缩长尾）

### 3.4 AFM 集成

```go
type AFM struct {
    // ...
    Scalers map[int32]*AutoScaler  // feature_index -> scaler
}
```

**流程：**
1. `Init()`: 自动检测数值特征并拟合 scaler
2. `Fit()`: 训练时用 scaler 归一化数值特征
3. `BatchInternalPredict()`: 推理时用 scaler 归一化
4. `Marshal/Unmarshal`: scaler 随模型保存和加载

## 4. 下一步讨论

### 4.1 数据加载

**问题：用户如何声明数值特征？**

**选项 A：自动推断**
- 根据特征值自动判断（当前实现）
- 优点：无需配置
- 缺点：可能误判

**选项 B：配置声明**
```toml
[recommend.collaborative.features.numerical]
user_features = ["age", "income"]
item_features = ["price", "rating"]
```

**选项 C：数据格式约定**
```json
{
  "user_labels": {
    "age": 25,           // 数值
    "gender": "male"     // 类别
  }
}
```

### 4.2 测试数据集

需要在真实数据集上验证：
- 包含数值特征的推荐数据集
- 对比有无 scaler 的效果差异

### 4.3 其他模型

当前仅 AFM 支持，是否需要扩展到：
- FM (Factorization Machines)
- DeepFM
- 其他 CTR 模型

## 5. 提交记录

| 提交 | 说明 |
|------|------|
| `95eb08a` | refactor: remove AutoScale parameter and simplify scaler logic |
| `90762b4` | refactor: remove Skip field from AutoScaler |
| `d42f7d7` | feat: skip scaling for constant values in AutoScaler |
| `727a645` | test: disable AutoScale for Criteo test |
| `fd4c88c` | refactor: move AutoScale parameter to model package |
| `f51b0ef` | feat: add AutoScale parameter to AFM |
| `de0cf21` | feat: integrate AutoScaler into AFM |
| `95d3132` | feat: implement AutoScaler |
| `28b90f7` | feat: implement RobustScaler |
| `cdaedd5` | feat: implement MinMaxScaler |
