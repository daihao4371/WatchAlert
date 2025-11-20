# 需求文档:告警AI自动分析功能

## 一、需求概述

### 1.1 需求背景
**现状问题**:
- 当前AI分析功能需要用户手动触发(调用 `POST /api/w8t/ai/chat`)
- 值班人员收到告警后,需要额外点击"AI分析"按钮才能获得排查建议
- 紧急故障时,多一步操作会延缓响应速度
- AI分析结果与告警通知分离,体验不流畅

**改进目标**:
- 告警触发时自动调用AI分析(异步模式)
- 将AI分析结果直接注入到告警通知消息中
- 值班人员收到告警时,即可看到AI建议,无需额外操作
- 通过智能触发策略,避免AI调用成本过高

### 1.2 需求价值
- **提升响应效率**: 减少1-2分钟的手动操作时间,紧急故障时尤为关键
- **降低认知负担**: 值班人员无需记住"需要手动点AI分析"
- **提高AI使用率**: 从"需要主动使用"变为"默认使用",使用率预计提升300%+
- **改善用户体验**: 告警通知消息更智能,更有价值

---

## 二、功能详细设计

### 2.1 核心功能点

#### 功能1: 告警规则AI分析配置
**位置**: 告警规则配置页面

**新增配置项**:
```go
type AlertRule struct {
    // ... 现有字段

    // AI自动分析配置
    AutoAiAnalysis     *bool  `json:"autoAiAnalysis"`     // 是否启用AI自动分析
    AiAnalysisMode     string `json:"aiAnalysisMode"`     // 分析模式: realtime/async/manual
    AiAnalysisPriority string `json:"aiAnalysisPriority"` // 触发优先级: all/p0p1/p0/none
}
```

**配置说明**:

| 字段 | 类型 | 说明 | 默认值 | 可选值 |
|------|------|------|--------|--------|
| autoAiAnalysis | *bool | 是否启用自动AI分析 | false | true/false |
| aiAnalysisMode | string | 分析模式 | "async" | "realtime"(同步)/"async"(异步)/"manual"(手动) |
| aiAnalysisPriority | string | 仅对指定级别告警分析 | "all" | "all"(所有)/"p0p1"(仅P0/P1)/"p0"(仅P0)/"none"(禁用) |

**前端UI示例**:
```
┌─────────────────────────────────────────┐
│ 告警规则配置                              │
├─────────────────────────────────────────┤
│ 规则名称: [CPU使用率过高            ]    │
│ 数据源:   [Prometheus ▼]                 │
│ ...                                      │
├─────────────────────────────────────────┤
│ ☑️ 启用AI自动分析                        │
│                                          │
│   分析模式:                               │
│   ○ 实时分析(告警时立即分析,可能影响性能) │
│   ● 异步分析(告警入库后异步调用,推荐)     │
│   ○ 手动触发(保持现有行为)               │
│                                          │
│   触发策略:                               │
│   ☑️ 仅对P0/P1告警自动分析               │
│   ☐ 仅对新指纹告警分析(避免重复调用AI)    │
│                                          │
│   AI分析结果将自动附加到告警通知中        │
└─────────────────────────────────────────┘
```

---

#### 功能2: 告警事件AI分析结果存储
**位置**: `internal/models/alert_current_event.go`

**新增字段**:
```go
type AlertCurEvent struct {
    // ... 现有字段

    // AI分析相关
    AiAnalysisResult   string `json:"aiAnalysisResult" gorm:"-"`   // AI分析结果(不存数据库,仅缓存)
    AiAnalyzedAt       int64  `json:"aiAnalyzedAt" gorm:"-"`       // AI分析时间
    AiAnalysisStatus   string `json:"aiAnalysisStatus" gorm:"-"`   // AI分析状态: pending/analyzing/completed/failed
}
```

**存储方式**:
- **不存MySQL**: AI分析结果存储在Redis缓存中,随告警事件一起缓存
- **缓存键**: `w8t:{tenantId}:faultCenter:{faultCenterId}.events` (现有缓存结构)
- **TTL**: 跟随告警事件生命周期,告警恢复后保留24小时

---

#### 功能3: AI自动分析触发逻辑
**位置**: `alert/process/eval.go` 或新建 `alert/process/ai_analysis.go`

**触发时机**: 告警评估完成,准备发送通知前

**核心逻辑**:
```go
// 伪代码
func processAlertWithAI(ctx *ctx.Context, alert *models.AlertCurEvent, rule *models.AlertRule) error {
    // 1. 检查是否启用AI自动分析
    if rule.AutoAiAnalysis == nil || !*rule.AutoAiAnalysis {
        return nil // 未启用,跳过
    }

    // 2. 检查分析模式
    if rule.AiAnalysisMode == "manual" {
        return nil // 手动模式,跳过
    }

    // 3. 检查触发策略
    if !shouldTriggerAI(alert.Severity, rule.AiAnalysisPriority) {
        return nil // 不满足触发条件
    }

    // 4. 检查是否已分析(避免重复调用)
    if alert.AiAnalysisResult != "" {
        return nil // 已有分析结果,跳过
    }

    // 5. 根据模式调用AI
    if rule.AiAnalysisMode == "realtime" {
        // 同步调用
        result, err := callAiAnalysis(ctx, alert, rule)
        if err == nil {
            alert.AiAnalysisResult = result
            alert.AiAnalyzedAt = time.Now().Unix()
            alert.AiAnalysisStatus = "completed"
        }
    } else if rule.AiAnalysisMode == "async" {
        // 异步调用(推荐)
        alert.AiAnalysisStatus = "analyzing"
        go asyncCallAI(ctx, alert, rule)
    }

    return nil
}

// 判断是否应该触发AI
func shouldTriggerAI(severity, priority string) bool {
    switch priority {
    case "all":
        return true
    case "p0p1":
        return severity == "P0" || severity == "P1"
    case "p0":
        return severity == "P0"
    default:
        return false
    }
}

// 异步AI分析
func asyncCallAI(ctx *ctx.Context, alert *models.AlertCurEvent, rule *models.AlertRule) {
    // 调用AI服务
    result, err := callAiAnalysis(ctx, alert, rule)

    // 更新缓存
    cache := ctx.Redis.Alert()
    event, _ := cache.GetEventFromCache(alert.TenantId, alert.FaultCenterId, alert.Fingerprint)

    if err == nil {
        event.AiAnalysisResult = result
        event.AiAnalyzedAt = time.Now().Unix()
        event.AiAnalysisStatus = "completed"
    } else {
        event.AiAnalysisStatus = "failed"
    }

    cache.PushAlertEvent(&event)
}

// 调用AI分析(复用现有逻辑)
func callAiAnalysis(ctx *ctx.Context, alert *models.AlertCurEvent, rule *models.AlertRule) (string, error) {
    // 构造请求参数
    req := &types.RequestAiChatContent{
        RuleName: alert.RuleName,
        RuleId:   alert.RuleId,
        Content:  alert.Annotations,
        SearchQL: alert.SearchQL,
        Deep:     "false", // 使用缓存
    }

    // 调用现有AI服务
    result, err := services.AiService.Chat(req)
    if err != nil {
        return "", err
    }

    return result.(string), nil
}
```

**集成位置**:
- 在 `alert/process/process.go` 的 `handleAlertEvent` 函数中
- 在发送通知前调用 `processAlertWithAI`

---

#### 功能4: AI结果注入告警通知
**位置**: `alert/process/handle.go:159`

**修改函数**: `generateAlertContent`

**改造方案**:
```go
// 改造前
func generateAlertContent(ctx *ctx.Context, alert *models.AlertCurEvent, noticeData models.AlertNotice) string {
    if noticeData.NoticeType == "CustomHook" {
        return tools.JsonMarshalToString(alert)
    }
    return templates.NewTemplate(ctx, *alert, noticeData).CardContentMsg
}

// 改造后
func generateAlertContent(ctx *ctx.Context, alert *models.AlertCurEvent, noticeData models.AlertNotice) string {
    if noticeData.NoticeType == "CustomHook" {
        return tools.JsonMarshalToString(alert)
    }

    // 生成基础通知内容
    baseContent := templates.NewTemplate(ctx, *alert, noticeData).CardContentMsg

    // 如果存在AI分析结果,注入到通知内容
    if alert.AiAnalysisResult != "" {
        baseContent = injectAiAnalysis(baseContent, alert, noticeData.NoticeType)
    } else if alert.AiAnalysisStatus == "analyzing" {
        // AI正在分析中,可选:提示用户稍后刷新查看
        baseContent = appendAnalyzingTip(baseContent, noticeData.NoticeType)
    }

    return baseContent
}

// 注入AI分析结果
func injectAiAnalysis(content string, alert *models.AlertCurEvent, noticeType string) string {
    aiSection := formatAiAnalysis(alert.AiAnalysisResult, noticeType)

    switch noticeType {
    case "FeiShu":
        // 飞书卡片需要解析JSON,追加元素
        return injectAiToFeishuCard(content, aiSection)
    case "DingDing":
        // 钉钉Markdown格式,直接追加
        return content + "\n\n" + aiSection
    case "Email":
        return content + "\n\n" + aiSection
    default:
        return content + "\n\n" + aiSection
    }
}

// 格式化AI分析结果
func formatAiAnalysis(aiResult, noticeType string) string {
    switch noticeType {
    case "FeiShu", "DingDing":
        return fmt.Sprintf("🤖 **AI分析建议**:\n%s", aiResult)
    case "Email":
        return fmt.Sprintf("<h3>🤖 AI分析建议</h3><p>%s</p>", aiResult)
    default:
        return fmt.Sprintf("AI分析建议:\n%s", aiResult)
    }
}

// 注入AI到飞书卡片
func injectAiToFeishuCard(jsonContent, aiSection string) string {
    // 1. 反序列化JSON
    var card models.FeiShuJsonCardMsg
    json.Unmarshal([]byte(jsonContent), &card)

    // 2. 添加AI分析元素(在分隔线前插入)
    aiElement := map[string]interface{}{
        "tag": "div",
        "text": map[string]interface{}{
            "tag":     "lark_md",
            "content": aiSection,
        },
    }

    // 在倒数第二个位置插入(分隔线和Footer之前)
    elements := card.Card.Elements
    if len(elements) >= 2 {
        card.Card.Elements = append(
            elements[:len(elements)-2],
            aiElement,
            elements[len(elements)-2:]...,
        )
    }

    // 3. 序列化回JSON
    result, _ := json.Marshal(card)
    return string(result)
}
```

---

### 2.2 通知效果示例

#### 飞书卡片效果
```
┌──────────────────────────────────┐
│ 🔴 告警: CPU使用率过高             │
├──────────────────────────────────┤
│ **告警详情**                      │
│ • 主机: 192.168.1.100             │
│ • 当前值: 95%                     │
│ • 阈值: 80%                       │
│ • 触发时间: 2024-01-15 14:30:00  │
├──────────────────────────────────┤
│ 🤖 **AI分析建议**                 │
│                                   │
│ **可能原因**:                     │
│ 1. Java进程占用CPU过高            │
│ 2. 可能存在死循环或性能问题        │
│                                   │
│ **排查建议**:                     │
│ 1. 执行 `top` 查看进程CPU占用     │
│ 2. 检查应用日志是否有异常          │
│ 3. 使用 jstack 分析线程堆栈        │
│                                   │
│ **紧急处理**:                     │
│ 如持续告警,建议重启应用服务         │
├──────────────────────────────────┤
│ 📌 值班人: @张三                  │
│ 🔗 查看详情 | 📊 查看监控          │
└──────────────────────────────────┘
```

#### 钉钉Markdown效果
```markdown
**🔴 告警: CPU使用率过高**

**告警详情**
- 主机: 192.168.1.100
- 当前值: 95%
- 阈值: 80%
- 触发时间: 2024-01-15 14:30:00

---

🤖 **AI分析建议**:

**可能原因**:
1. Java进程占用CPU过高
2. 可能存在死循环或性能问题

**排查建议**:
1. 执行 `top` 查看进程CPU占用
2. 检查应用日志是否有异常
3. 使用 jstack 分析线程堆栈

**紧急处理**:
如持续告警,建议重启应用服务

---
📌 @张三 请及时处理
```

---

### 2.3 性能优化方案

#### 优化1: AI调用优先级队列
**问题**: 同时触发大量告警时,AI调用可能堵塞

**方案**: 使用优先级队列
```go
type AiAnalysisTask struct {
    Priority    int                    // P0=100, P1=80, P2=60...
    Alert       *models.AlertCurEvent
    Rule        *models.AlertRule
    SubmitTime  int64
}

// 使用 Redis ZSet 实现优先级队列
// Key: w8t:ai:analysis:queue
// Score: Priority
// Member: JSON(AiAnalysisTask)

// 后台Worker定期消费
func aiAnalysisWorker(ctx *ctx.Context) {
    for {
        // 从队列取出优先级最高的任务
        task := popHighestPriorityTask(ctx)
        if task == nil {
            time.Sleep(1 * time.Second)
            continue
        }

        // 执行AI分析
        result, _ := callAiAnalysis(ctx, task.Alert, task.Rule)

        // 更新缓存
        updateAlertAiResult(ctx, task.Alert, result)
    }
}
```

#### 优化2: AI调用去重
**问题**: 相同告警重复触发,重复调用AI浪费成本

**方案**: 基于RuleId + Fingerprint缓存
```go
// 生成AI缓存Key
func buildAiCacheKey(ruleId, fingerprint string) string {
    return fmt.Sprintf("w8t:ai:cache:%s:%s", ruleId, fingerprint)
}

// 检查缓存
func getAiResultFromCache(ctx *ctx.Context, ruleId, fingerprint string) (string, bool) {
    key := buildAiCacheKey(ruleId, fingerprint)
    result, err := ctx.Redis.Client().Get(ctx.Ctx, key).Result()
    if err == nil && result != "" {
        return result, true
    }
    return "", false
}

// 写入缓存(TTL: 1小时)
func setAiResultToCache(ctx *ctx.Context, ruleId, fingerprint, result string) {
    key := buildAiCacheKey(ruleId, fingerprint)
    ctx.Redis.Client().Set(ctx.Ctx, key, result, 1*time.Hour)
}
```

#### 优化3: AI调用超时控制
**方案**: 设置超时时间,避免长时间等待
```go
func callAiAnalysisWithTimeout(ctx *ctx.Context, alert *models.AlertCurEvent, rule *models.AlertRule) (string, error) {
    // 创建超时上下文
    timeoutCtx, cancel := context.WithTimeout(ctx.Ctx, 10*time.Second)
    defer cancel()

    resultChan := make(chan string, 1)
    errChan := make(chan error, 1)

    // 异步调用AI
    go func() {
        result, err := callAiAnalysis(ctx, alert, rule)
        if err != nil {
            errChan <- err
            return
        }
        resultChan <- result
    }()

    // 等待结果或超时
    select {
    case result := <-resultChan:
        return result, nil
    case err := <-errChan:
        return "", err
    case <-timeoutCtx.Done():
        return "", fmt.Errorf("AI分析超时")
    }
}
```

---

## 三、数据库/缓存设计

### 3.1 MySQL表结构变更

#### 表: `w8t_alert_rule`
**新增字段**:
```sql
ALTER TABLE `w8t_alert_rule`
ADD COLUMN `auto_ai_analysis` TINYINT(1) DEFAULT 0 COMMENT '是否启用AI自动分析',
ADD COLUMN `ai_analysis_mode` VARCHAR(20) DEFAULT 'async' COMMENT 'AI分析模式: realtime/async/manual',
ADD COLUMN `ai_analysis_priority` VARCHAR(20) DEFAULT 'all' COMMENT 'AI触发优先级: all/p0p1/p0/none';
```

### 3.2 Redis缓存设计

#### 缓存1: 告警事件缓存(现有缓存扩展)
**Key**: `w8t:{tenantId}:faultCenter:{faultCenterId}.events`
**Type**: Hash
**Field**: `{fingerprint}`
**Value**: JSON(AlertCurEvent) - 包含新增的AI字段

#### 缓存2: AI分析结果缓存
**Key**: `w8t:ai:cache:{ruleId}:{fingerprint}`
**Type**: String
**Value**: AI分析结果文本
**TTL**: 1小时

#### 缓存3: AI任务队列
**Key**: `w8t:ai:analysis:queue`
**Type**: ZSet
**Score**: 优先级(P0=100, P1=80...)
**Member**: JSON(AiAnalysisTask)

---

## 四、接口设计

### 4.1 新增配置接口(复用现有接口)
- 规则创建/更新接口已支持,无需新增
- `POST /api/w8t/rule/ruleCreate`
- `POST /api/w8t/rule/ruleUpdate`

**请求示例**:
```json
{
  "ruleName": "CPU使用率过高",
  "datasourceType": "Prometheus",
  "autoAiAnalysis": true,
  "aiAnalysisMode": "async",
  "aiAnalysisPriority": "p0p1",
  ...
}
```

### 4.2 查询告警事件接口(现有接口扩展)
- `GET /api/w8t/event/curEvent`

**响应示例(新增AI字段)**:
```json
{
  "code": 200,
  "data": {
    "list": [
      {
        "fingerprint": "abc123",
        "ruleName": "CPU使用率过高",
        "severity": "P1",
        "annotations": "CPU使用率: 95%",
        "aiAnalysisResult": "可能原因:\n1. Java进程占用CPU过高...",
        "aiAnalyzedAt": 1705305600,
        "aiAnalysisStatus": "completed"
      }
    ]
  }
}
```

---

## 五、实施计划

### 5.1 开发任务拆分

| 任务编号 | 任务名称 | 工作量 | 优先级 | 依赖 |
|---------|---------|--------|--------|------|
| AI-01 | 数据库表结构变更 | 0.5天 | P0 | - |
| AI-02 | AlertRule模型扩展 | 0.5天 | P0 | AI-01 |
| AI-03 | 前端规则配置页增加AI开关 | 1天 | P0 | AI-02 |
| AI-04 | AI自动触发逻辑开发 | 2天 | P0 | AI-02 |
| AI-05 | AI结果注入通知内容 | 1.5天 | P0 | AI-04 |
| AI-06 | AI调用优先级队列 | 1天 | P1 | AI-04 |
| AI-07 | AI结果缓存优化 | 0.5天 | P1 | AI-04 |
| AI-08 | 单元测试编写 | 1天 | P1 | AI-05 |
| AI-09 | 集成测试 | 1天 | P1 | AI-08 |

**总计**: 约9-10个工作日

### 5.2 开发里程碑

**Week 1**:
- ✅ 完成数据库变更
- ✅ 完成后端模型扩展
- ✅ 完成AI自动触发逻辑
- ✅ 完成AI结果注入

**Week 2**:
- ✅ 完成前端配置页
- ✅ 完成性能优化
- ✅ 完成测试
- ✅ 灰度发布

---

## 六、测试方案

### 6.1 功能测试

#### 测试用例1: AI自动分析-异步模式
**前置条件**:
- 规则配置: `autoAiAnalysis=true`, `aiAnalysisMode=async`, `aiAnalysisPriority=all`
- AI服务正常

**测试步骤**:
1. 触发告警
2. 等待3-5秒(异步分析时间)
3. 查询告警事件详情

**预期结果**:
- `aiAnalysisStatus = "completed"`
- `aiAnalysisResult` 有内容
- 告警通知消息包含AI分析结果

#### 测试用例2: AI分析优先级策略
**前置条件**:
- 规则配置: `aiAnalysisPriority=p0p1`

**测试步骤**:
1. 触发P0告警 → 应触发AI
2. 触发P2告警 → 不应触发AI

**预期结果**:
- P0告警有AI分析结果
- P2告警 `aiAnalysisResult` 为空

#### 测试用例3: AI分析缓存
**测试步骤**:
1. 触发告警A(首次)
2. 记录AI调用时间戳
3. 告警恢复后再次触发(相同规则+Fingerprint)
4. 验证是否使用缓存

**预期结果**:
- 第二次告警应使用缓存,无需重新调用AI
- 响应时间<100ms

### 6.2 性能测试

#### 测试场景: 告警风暴
**测试参数**:
- 并发告警: 100条/分钟
- AI分析模式: async
- AI响应时间: 2-5秒

**性能指标**:
- 告警通知延迟: <3秒(不受AI影响)
- AI分析完成率: >95%
- CPU使用率: <70%
- 内存使用率: <80%

---

## 七、风险与应对

### 7.1 风险识别

| 风险项 | 风险等级 | 影响 | 应对措施 |
|-------|---------|------|---------|
| AI服务不稳定 | 高 | 分析失败率高 | 增加重试机制+降级策略 |
| AI调用成本过高 | 中 | 费用超预算 | 严格触发策略+每日配额限制 |
| 告警通知延迟 | 中 | 影响用户体验 | 异步模式+超时控制 |
| AI结果注入失败 | 低 | 部分通知无AI内容 | 容错处理,不影响原有通知 |

### 7.2 降级方案

**降级触发条件**:
- AI服务连续失败>10次
- AI响应时间>10秒
- AI调用量超过每日配额

**降级策略**:
1. 自动切换为手动模式
2. 通知管理员
3. 记录降级日志
4. 服务恢复后自动恢复

---

## 八、上线计划

### 8.1 灰度发布策略

**阶段1: 内部测试(1-2天)**
- 仅对测试租户开放
- 验证功能正确性

**阶段2: 小范围灰度(3-5天)**
- 对10%用户开放
- 监控AI调用量、成本、失败率

**阶段3: 全量发布**
- 灰度无问题后全量开放
- 持续监控关键指标

### 8.2 监控指标

| 指标 | 阈值 | 告警级别 |
|------|------|---------|
| AI分析成功率 | <90% | P1 |
| AI平均响应时间 | >10秒 | P2 |
| AI每日调用量 | >10000 | P2 |
| AI分析队列堆积 | >100 | P1 |

---

## 九、FAQ

### Q1: AI分析失败会影响告警通知吗?
**A**: 不会。AI分析与告警通知完全解耦,AI失败只会导致通知中没有AI分析结果,不影响告警正常发送。

### Q2: 异步模式下,什么时候能看到AI分析结果?
**A**: 通常3-5秒内。如果告警通知发送时AI还在分析,用户可以稍后在告警详情页刷新查看。

### Q3: 如何控制AI调用成本?
**A**:
1. 使用 `aiAnalysisPriority` 仅对重要告警分析
2. 启用AI结果缓存(1小时)
3. 配置每日调用配额
4. 监控每日调用量

### Q4: 支持自定义AI Prompt吗?
**A**: 支持。在系统设置中配置AI Prompt模板,已支持变量: `{{ RuleName }}`, `{{ Content }}`, `{{ SearchQL }}`

---

## 十、附录

### 附录A: 相关代码文件清单
```
修改:
- internal/models/rule.go (AlertRule模型)
- alert/process/eval.go (触发逻辑)
- alert/process/handle.go (注入AI结果)
- pkg/templates/feishuCard.go (飞书卡片)
- pkg/templates/dingCard.go (钉钉卡片)

新增:
- alert/process/ai_analysis.go (AI分析核心逻辑)
- internal/services/ai_auto.go (AI自动分析服务)
```

### 附录B: 数据库迁移SQL
```sql
-- 迁移脚本: 20240115_add_ai_auto_analysis.sql
ALTER TABLE `w8t_alert_rule`
ADD COLUMN `auto_ai_analysis` TINYINT(1) DEFAULT 0 COMMENT '是否启用AI自动分析',
ADD COLUMN `ai_analysis_mode` VARCHAR(20) DEFAULT 'async' COMMENT 'AI分析模式',
ADD COLUMN `ai_analysis_priority` VARCHAR(20) DEFAULT 'all' COMMENT 'AI触发优先级';

-- 回滚脚本
ALTER TABLE `w8t_alert_rule`
DROP COLUMN `auto_ai_analysis`,
DROP COLUMN `ai_analysis_mode`,
DROP COLUMN `ai_analysis_priority`;
```

---

**文档版本**: v1.0
**编写日期**: 2024-01-15
**编写人**: AI Assistant
**审核人**: [待填写]
**批准人**: [待填写]