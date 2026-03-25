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

**复用现有 label 索引**（选项 A）：
```
| user | item | user_label | item_label | context_label |
```

数值特征作为一种特殊的 label，使用统一索引：
- 例如：`user.age` → `EncodeUserLabel("age")`

### 1.3 AutoScaler 存储

在 AFM 中添加：
```go
type AFM struct {
    // 现有字段...
    
    // 新增：数值特征的 Scaler 映射
    Scalers map[int32]*AutoScaler  // feature_index -> scaler
}
```

### 1.4 数据处理流程

**训练阶段：**
1. 遍历所有样本，收集每个数值特征的值
2. 为每个数值特征创建并拟合 `AutoScaler`
3. 用 `scaler.Transform(value)` 归一化数值特征
4. 将归一化后的值填入 `values`

**推理阶段：**
1. 根据特征名找到索引
2. 如果该索引在 `Scalers` 中存在，使用对应的 scaler 归一化
3. 如果不存在（类别特征），直接使用原始值（通常为 1）
4. 用户自行负责字段类型匹配：
   - 数值字段传成 label → 忽略 scaler，直接使用值
   - label 字段传成数值 → 忽略，值会被当作类别特征处理

## 2. 实现计划

### Phase 1: Scaler 实现 ✅ (已完成)

| 任务 | 状态 |
|------|------|
| 实现 MinMaxScaler | ✅ |
| 实现 RobustScaler | ✅ |
| 实现 AutoScaler | ✅ |
| 添加单元测试 | ✅ |

### Phase 2: AFM 扩展

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 在 AFM 中添加 Scalers 字段 | model/ctr/fm.go | P0 |
| 修改 AFM.Init() 收集数值特征并拟合 scaler | model/ctr/fm.go | P0 |
| 修改 AFM.Forward() 使用 scaler 归一化数值 | model/ctr/fm.go | P0 |
| 扩展 AFM.Marshal/Unmarshal 保存 scaler | model/ctr/fm.go | P0 |
| 添加单元测试 | model/ctr/model_test.go | P1 |

### Phase 3: 数据加载扩展

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 扩展 Dataset 支持数值特征声明 | model/ctr/data.go | P0 |
| 修改数据加载流程收集数值特征 | model/ctr/data.go | P0 |

### Phase 4: 集成测试

| 任务 | 优先级 |
|------|--------|
| 数值特征 + 类别特征混合训练测试 | P0 |
| 模型保存/加载测试 | P0 |
| 推理正确性测试 | P1 |

## 3. 接口设计

### 3.1 数值特征声明（配置方式）

```toml
[recommend.collaborative.features.numerical]
user_features = ["age", "income"]
item_features = ["price", "rating"]
```

### 3.2 数值特征声明（数据格式方式）

用户在数据中直接传入数值：
```json
{
  "user_labels": {
    "age": 25,           // 数值
    "gender": "male"     // 类别
  },
  "item_labels": {
    "price": 99.99,      // 数值
    "category": "book"   // 类别
  }
}
```

训练时自动识别数值类型的字段并创建 scaler。

## 4. 向后兼容性

1. **默认行为**：没有 scaler 的特征按类别特征处理
2. **模型加载**：旧版模型（无 Scalers 字段）可正常加载
3. **API 兼容**：现有预测接口保持不变

## 5. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 类型混淆 | 中 | 文档说明，用户自行负责 |
| 性能影响 | 低 | Scaler 计算开销小 |
| 向后兼容 | 低 | Scalers 为空时退化为原有行为 |

## 6. 已完成

- ✅ MinMaxScaler 实现
- ✅ RobustScaler 实现  
- ✅ AutoScaler 实现（自动选择：有负数→Robust，全非负→log1p+MinMax）
- ✅ 单元测试
- ✅ 序列化/反序列化
