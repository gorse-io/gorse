# 负反馈优先级设计方案

## Issue

[#848 是否支持反向反馈？](https://github.com/gorse-io/gorse/issues/848)

## 背景

当前 Gorse 的反馈系统如下：

- **正反馈 (Positive Feedback)**: 用户对物品的正面交互，如点赞、收藏、购买等
- **读反馈 (Read Feedback)**: 用户浏览但未交互的行为，用于隐式负样本

**问题**: 没有显式负反馈的支持。用户可能明确表示不喜欢某物品（如"不感兴趣"、"踩"等），这类反馈应该有最高优先级。

## 需求

**负样本的行为优先级最高**：
- 如果一个物品被标记为负样本，那它必须为负样本
- 不受是否有正样本 feedback 影响
- 即使用户之前有正反馈，一旦有负反馈，该用户-物品对应被视为负样本

## 设计方案

### 1. 配置扩展

在 `DataSourceConfig` 中添加新配置：

```go
// config/config.go
type DataSourceConfig struct {
    PositiveFeedbackTypes []expression.FeedbackTypeExpression `mapstructure:"positive_feedback_types"`
    ReadFeedbackTypes     []expression.FeedbackTypeExpression `mapstructure:"read_feedback_types"`
    NegativeFeedbackTypes []expression.FeedbackTypeExpression `mapstructure:"negative_feedback_types"` // 新增
    PositiveFeedbackTTL   uint                                `mapstructure:"positive_feedback_ttl"`
    ItemTTL               uint                                `mapstructure:"item_ttl"`
}
```

配置示例：
```yaml
recommend:
  data_source:
    positive_feedback_types: ["like", "purchase"]
    read_feedback_types: ["read"]
    negative_feedback_types: ["dislike", "not_interested"]  # 新增
```

### 2. 数据加载逻辑修改

修改 `master/tasks.go` 中的数据加载逻辑：

```
当前逻辑流程：
1. 加载正反馈 → positiveSet
2. 加载读反馈 → negativeSet  
3. 构建数据集：positiveSet → target=1, negativeSet → target=-1

新逻辑流程：
1. 加载负反馈 → negativeFeedbackSet (优先级最高)
2. 加载正反馈 → positiveSet (排除已在 negativeFeedbackSet 中的)
3. 加载读反馈 → readSet (排除已在 negativeFeedbackSet 或 positiveSet 中的)
4. 构建数据集：
   - negativeFeedbackSet → target=-1 (强制负样本)
   - positiveSet → target=1
   - readSet → target=-1
```

### 3. 核心代码修改

#### 3.1 数据加载 (master/tasks.go)

```go
func (m *Master) loadDataset(...) {
    // ... existing code ...
    
    // STEP 3: pull negative feedback (NEW - 最高优先级)
    negativeFeedbackSet := make([]mapset.Set[int32], dataSet.CountUsers())
    for i := range negativeFeedbackSet {
        negativeFeedbackSet[i] = mapset.NewSet[int32]()
    }
    
    if len(negFeedbackTypes) > 0 {
        err = parallel.Parallel(newCtx, len(itemGroups), m.Config.Master.NumJobs, func(_, i int) error {
            feedbackChan, errChan := database.GetFeedbackStream(newCtx, batchSize,
                data.WithBeginItemId(itemGroups[i][0].ItemId),
                data.WithEndItemId(itemGroups[i][len(itemGroups[i])-1].ItemId),
                feedbackTimeLimit,
                data.WithEndTime(*m.Config.Now()),
                data.WithFeedbackTypes(negFeedbackTypes...),
                data.WithOrderByItemId())
            // ... process negative feedback ...
            for _, f := range feedback {
                userIndex := dataSet.GetUserDict().Id(f.UserId)
                itemIndex := dataSet.GetItemDict().Id(f.ItemId)
                negativeFeedbackSet[userIndex].Add(itemIndex)
            }
            // ...
        })
    }
    
    // STEP 4: pull positive feedback (排除已在负反馈集合中的)
    for _, f := range feedback {
        userIndex := dataSet.GetUserDict().Id(f.UserId)
        itemIndex := dataSet.GetItemDict().Id(f.ItemId)
        // 如果已在负反馈集合中，跳过
        if !negativeFeedbackSet[userIndex].Contains(itemIndex) {
            positiveSet[userIndex].Add(itemIndex)
        }
    }
    
    // STEP 5: pull read feedback (排除已在负反馈或正反馈集合中的)
    for _, f := range feedback {
        userIndex := dataSet.GetUserDict().Id(f.UserId)
        itemIndex := dataSet.GetItemDict().Id(f.ItemId)
        // 如果已在负反馈或正反馈集合中，跳过
        if !negativeFeedbackSet[userIndex].Contains(itemIndex) && 
           !positiveSet[userIndex].Contains(itemIndex) {
            readSet[userIndex].Add(itemIndex)
        }
    }
    
    // STEP 6: create click-through rate dataset
    for userIndex := range positiveSet {
        // 负反馈样本 (优先级最高)
        for _, itemIndex := range negativeFeedbackSet[userIndex].ToSlice() {
            ctrDataset.Users = append(ctrDataset.Users, int32(userIndex))
            ctrDataset.Items = append(ctrDataset.Items, itemIndex)
            ctrDataset.Target = append(ctrDataset.Target, -1)
            ctrDataset.NegativeCount++
        }
        // 正反馈样本
        for _, itemIndex := range positiveSet[userIndex].ToSlice() {
            ctrDataset.Users = append(ctrDataset.Users, int32(userIndex))
            ctrDataset.Items = append(ctrDataset.Items, itemIndex)
            ctrDataset.Target = append(ctrDataset.Target, 1)
            ctrDataset.PositiveCount++
        }
        // 读反馈样本 (隐式负样本)
        for _, itemIndex := range readSet[userIndex].ToSlice() {
            ctrDataset.Users = append(ctrDataset.Users, int32(userIndex))
            ctrDataset.Items = append(ctrDataset.Items, itemIndex)
            ctrDataset.Target = append(ctrDataset.Target, -1)
            ctrDataset.NegativeCount++
        }
    }
}
```

### 4. 推荐时排除负反馈物品

在生成推荐时，应排除用户明确标记为负反馈的物品：

```go
// 在推荐生成时过滤
func (m *Master) getItemCandidates(userId string) []string {
    // 获取用户的负反馈物品
    negativeItems := m.getNegativeFeedbackItems(userId)
    
    // 从候选集中排除
    candidates := filterCandidates(allCandidates, negativeItems)
    
    return candidates
}
```

### 5. 反馈时间戳考虑

负反馈的时间戳行为：

| 场景 | 行为 |
|------|------|
| 先正反馈，后负反馈 | 负样本（负反馈优先） |
| 先负反馈，后正反馈 | 负样本（负反馈优先） |
| 同时存在 | 负样本（负反馈优先） |

**理由**: 用户明确表示不感兴趣的行为应该被尊重，即使用户后来改变了主意（可能是误操作），系统应该保守处理，避免推荐用户不喜欢的物品。

如果需要支持"负反馈撤销"场景，可以：
1. 添加配置项 `negative_feedback_revocable: true`
2. 比较 timestamp，最新反馈优先

### 6. API 变更

#### 6.1 插入负反馈

```http
POST /api/feedback
Content-Type: application/json

[
  {
    "FeedbackType": "dislike",
    "UserId": "user1",
    "ItemId": "item1",
    "Timestamp": "2026-03-26T10:00:00Z"
  }
]
```

#### 6.2 查询用户负反馈

```http
GET /api/user/{user_id}/feedback?feedback_type=dislike
```

### 7. 兼容性考虑

- **向后兼容**: 不配置 `negative_feedback_types` 时，行为与当前版本一致
- **默认值**: `negative_feedback_types` 默认为空数组

### 8. 测试计划

1. **单元测试**
   - 配置解析测试
   - 数据集构建测试：验证负反馈优先级
   - 边界情况：空配置、多种反馈类型组合

2. **集成测试**
   - 端到端推荐流程测试
   - 验证负反馈物品不在推荐结果中

3. **性能测试**
   - 额外的负反馈集合对内存的影响
   - 数据加载时间的增加

## 实现步骤

1. **Phase 1**: 配置扩展
   - 修改 `config/config.go`，添加 `NegativeFeedbackTypes` 配置
   - 更新配置验证逻辑

2. **Phase 2**: 数据加载逻辑修改
   - 修改 `master/tasks.go` 中的 `loadDataset` 函数
   - 添加负反馈加载步骤
   - 修改正反馈和读反馈的过滤逻辑

3. **Phase 3**: 推荐过滤
   - 在推荐生成时排除负反馈物品

4. **Phase 4**: 测试与文档
   - 添加单元测试和集成测试
   - 更新用户文档

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| 内存增加（额外的集合） | 使用位图或压缩数据结构 |
| 配置复杂性增加 | 提供合理的默认值和文档 |
| 与现有行为不一致 | 明确文档说明，确保向后兼容 |

## 总结

本方案通过引入 `negative_feedback_types` 配置，在数据加载和推荐生成阶段实现负反馈的最高优先级。设计遵循以下原则：

1. **优先级明确**: 负反馈 > 正反馈 > 读反馈
2. **向后兼容**: 不配置新选项时行为不变
3. **可扩展**: 支持多种负反馈类型，支持表达式配置

---

## 实现状态

✅ **已完成** (2026-03-26)

### 提交记录

| Phase | 提交 | 描述 |
|-------|------|------|
| Phase 1 | `e4d3a33` | 配置扩展 - 添加 `NegativeFeedbackTypes` |
| Phase 2 | `c1666d9` | 数据加载逻辑 - 负反馈优先级 |
| Phase 3 | `c98920a` | 推荐过滤 - 负反馈物品始终排除 |
| Phase 4 | - | 测试与文档 |

### 使用示例

```yaml
# config.toml
[recommend.data_source]
positive_feedback_types = ["like", "purchase"]
negative_feedback_types = ["dislike", "not_interested"]
read_feedback_types = ["read"]
```

### API 使用

```bash
# 插入负反馈
curl -X POST http://localhost:8087/api/feedback \
  -H "Content-Type: application/json" \
  -d '[{
    "FeedbackType": "dislike",
    "UserId": "user1",
    "ItemId": "item1",
    "Timestamp": "2026-03-26T10:00:00Z"
  }]'
```

### 测试覆盖

- `TestNegativeFeedbackPriority`: 验证负反馈优先级逻辑
  - 用户对同一物品同时有正反馈和负反馈时，负反馈优先
  - 负反馈物品从推荐候选中排除
