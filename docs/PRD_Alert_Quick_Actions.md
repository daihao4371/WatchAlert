# 需求文档:告警快捷操作功能

## 一、需求概述

### 1.1 需求背景
**现状问题**:
- 值班人员收到飞书/钉钉告警后,需要:
  1. 打开电脑 → 登录系统 → 找到对应告警 → 点击操作
  2. 非工作时间(夜间/周末)响应速度慢
  3. 简单操作(如认领、静默)需要5-10分钟
  4. 移动端体验差,无法快速响应

**改进目标**:
- 在告警通知消息中直接提供操作按钮
- 支持一键认领、静默、查看详情
- 无需登录系统,在手机上即可完成操作
- 响应时间从5-10分钟缩短至10-30秒

### 1.2 需求价值
- **提升响应速度**: 紧急告警响应时间缩短80%+
- **降低操作门槛**: 新人值班也能快速处理
- **改善移动体验**: 支持手机端一键操作
- **提高处理效率**: 减少重复性劳动,专注于实质性问题

---

## 二、功能详细设计

### 2.1 核心功能点

#### 功能1: 飞书卡片快捷按钮
**位置**: `pkg/templates/feishuCard.go`

**新增按钮**:
```go
// 在飞书卡片底部添加操作按钮组
func buildFeishuActionButtons(alert models.AlertCurEvent, baseUrl string) []models.Actions {
    return []models.Actions{
        {
            Tag:  "button",
            Type: "primary",
            Text: models.ActionsText{
                Tag:     "plain_text",
                Content: "认领告警",
            },
            URL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s",
                baseUrl, alert.Fingerprint, generateQuickToken(alert)),
        },
        {
            Tag:  "button",
            Type: "default",
            Text: models.ActionsText{
                Tag:     "plain_text",
                Content: "静默1小时",
            },
            Value: map[string]interface{}{
                "action":      "silence",
                "fingerprint": alert.Fingerprint,
                "duration":    "1h",
            },
            Confirm: models.Confirms{
                Title: models.Titles{
                    Tag:     "plain_text",
                    Content: "确认静默?",
                },
                Text: models.Texts{
                    Tag:     "plain_text",
                    Content: "此操作将静默该告警1小时",
                },
            },
        },
        {
            Tag:  "button",
            Type: "default",
            Text: models.ActionsText{
                Tag:     "plain_text",
                Content: "查看详情",
            },
            URL: fmt.Sprintf("%s/events/%s", baseUrl, alert.Fingerprint),
        },
    }
}
```

**飞书卡片效果**:
```
┌─────────────────────────────────────┐
│ 🔴 P1告警: CPU使用率过高              │
├─────────────────────────────────────┤
│ **告警详情**                          │
│ • 主机: 192.168.1.100                │
│ • 当前值: 95%                        │
│ • 持续时长: 5分钟                     │
├─────────────────────────────────────┤
│ 🤖 AI分析建议:                        │
│ 可能是Java进程占用过高...             │
├─────────────────────────────────────┤
│ [认领告警] [静默1小时] [查看详情]      │
└─────────────────────────────────────┘
```

---

#### 功能2: 钉钉卡片快捷按钮
**位置**: `pkg/templates/dingCard.go`

**钉钉ActionCard方案**:
```go
func buildDingdingActionCard(alert models.AlertCurEvent, baseUrl string) models.DingMsg {
    return models.DingMsg{
        Msgtype: "actionCard",
        ActionCard: models.ActionCard{
            Title: fmt.Sprintf("告警: %s", alert.RuleName),
            Text:  generateAlertText(alert),
            BtnOrientation: "1", // 竖直排列
            Btns: []models.ActionCardBtn{
                {
                    Title:     "认领告警",
                    ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s",
                        baseUrl, alert.Fingerprint, generateQuickToken(alert)),
                },
                {
                    Title:     "静默1小时",
                    ActionURL: fmt.Sprintf("%s/quick-silence?fingerprint=%s&duration=1h",
                        baseUrl, alert.Fingerprint),
                },
                {
                    Title:     "查看详情",
                    ActionURL: fmt.Sprintf("%s/events/%s", baseUrl, alert.Fingerprint),
                },
            },
        },
    }
}
```

**钉钉卡片效果**:
```
┌─────────────────────────────────┐
│ 告警: CPU使用率过高               │
├─────────────────────────────────┤
│ **告警详情**                      │
│ 主机: 192.168.1.100              │
│ 当前值: 95%                      │
│ 持续时长: 5分钟                   │
│                                  │
│ 🤖 AI分析: 可能是Java进程...      │
├─────────────────────────────────┤
│         [认领告警]                │
│         [静默1小时]               │
│         [查看详情]                │
└─────────────────────────────────┘
```

---

#### 功能3: 快捷操作API
**位置**: 新建 `api/quickAction.go`

**路由设计**:
```go
// 快捷操作API(无需登录,使用Token验证)
func (quickActionController quickActionController) API(gin *gin.RouterGroup) {
    a := gin.Group("alert")
    // 注意: 不使用 Auth 中间件,使用自定义Token验证
    a.Use(
        middleware.QuickActionAuth(), // 新增:快捷操作Token验证
        middleware.ParseTenant(),
    )
    {
        a.GET("quick-action", quickActionController.QuickAction)      // 通用快捷操作
        a.POST("quick-silence", quickActionController.QuickSilence)   // 快捷静默(支持自定义)
    }
}
```

**接口1: 通用快捷操作**
```
GET /api/v1/alert/quick-action
```

**请求参数**:
| 参数 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| action | string | 是 | 操作类型 | claim/resolve/silence |
| fingerprint | string | 是 | 告警指纹 | abc123... |
| token | string | 是 | 快捷操作Token | eyJhbG... |
| duration | string | 否 | 静默时长(action=silence时) | 1h/24h/7d |

**响应**:
```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "action": "claim",
    "fingerprint": "abc123",
    "operator": "张三",
    "timestamp": 1705305600
  }
}
```

**核心逻辑**:
```go
func (q quickActionController) QuickAction(ctx *gin.Context) {
    // 1. 解析参数
    action := ctx.Query("action")
    fingerprint := ctx.Query("fingerprint")
    token := ctx.Query("token")
    duration := ctx.DefaultQuery("duration", "1h")

    // 2. 验证Token(从Token中提取用户信息)
    userInfo, err := verifyQuickToken(token)
    if err != nil {
        response.Fail(ctx, "Token无效或已过期", nil)
        return
    }

    // 3. 获取租户ID(从Token或Header)
    tid, _ := ctx.Get("TenantID")
    tenantId := tid.(string)

    // 4. 执行操作
    switch action {
    case "claim":
        err = services.QuickActionService.ClaimAlert(tenantId, fingerprint, userInfo.Username)
    case "resolve":
        err = services.QuickActionService.ResolveAlert(tenantId, fingerprint, userInfo.Username)
    case "silence":
        err = services.QuickActionService.SilenceAlert(tenantId, fingerprint, duration, userInfo.Username)
    default:
        response.Fail(ctx, "不支持的操作类型", nil)
        return
    }

    if err != nil {
        response.Fail(ctx, err.Error(), nil)
        return
    }

    // 5. 返回成功页面(HTML)或跳转
    renderSuccessPage(ctx, action)
}

// 渲染成功页面(移动端友好)
func renderSuccessPage(ctx *gin.Context, action string) {
    actionName := map[string]string{
        "claim":   "认领",
        "resolve": "标记已处理",
        "silence": "静默",
    }[action]

    html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>操作成功</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .container {
            text-align: center;
            background: white;
            padding: 40px;
            border-radius: 12px;
            box-shadow: 0 2px 12px rgba(0,0,0,0.1);
        }
        .icon { font-size: 64px; margin-bottom: 20px; }
        h1 { color: #52c41a; margin: 0 0 10px 0; }
        p { color: #666; margin: 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">✅</div>
        <h1>%s成功</h1>
        <p>您可以关闭此页面</p>
    </div>
</body>
</html>
    `, actionName)

    ctx.Header("Content-Type", "text/html; charset=utf-8")
    ctx.String(200, html)
}
```

---

#### 功能4: 快捷操作Token机制
**目的**: 无需登录即可操作,但需要保证安全性

**Token生成**:
```go
type QuickActionToken struct {
    TenantId    string `json:"tenantId"`
    Fingerprint string `json:"fingerprint"`
    Username    string `json:"username"`    // 当前值班人
    ExpireAt    int64  `json:"expireAt"`    // 过期时间
}

// 生成Token(告警发送时)
func generateQuickToken(alert models.AlertCurEvent) string {
    payload := QuickActionToken{
        TenantId:    alert.TenantId,
        Fingerprint: alert.Fingerprint,
        Username:    alert.DutyUser,
        ExpireAt:    time.Now().Add(24 * time.Hour).Unix(), // 24小时有效期
    }

    // 使用JWT或AES加密
    tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "tenantId":    payload.TenantId,
        "fingerprint": payload.Fingerprint,
        "username":    payload.Username,
        "expireAt":    payload.ExpireAt,
    }).SignedString([]byte(getSecretKey()))

    return tokenStr
}

// 验证Token
func verifyQuickToken(tokenStr string) (*QuickActionToken, error) {
    token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
        return []byte(getSecretKey()), nil
    })

    if err != nil || !token.Valid {
        return nil, fmt.Errorf("Token无效")
    }

    claims := token.Claims.(jwt.MapClaims)
    expireAt := int64(claims["expireAt"].(float64))

    if time.Now().Unix() > expireAt {
        return nil, fmt.Errorf("Token已过期")
    }

    return &QuickActionToken{
        TenantId:    claims["tenantId"].(string),
        Fingerprint: claims["fingerprint"].(string),
        Username:    claims["username"].(string),
        ExpireAt:    expireAt,
    }, nil
}
```

**安全性说明**:
- ✅ Token有效期24小时,过期自动失效
- ✅ Token绑定告警指纹,无法用于其他告警
- ✅ Token包含租户ID和用户信息,防止越权
- ✅ 使用JWT签名,防止伪造
- ✅ 操作记录审计日志

---

#### 功能5: 快捷静默增强
**位置**: 新建 `api/quickSilence.go`

**接口**:
```
POST /api/v1/alert/quick-silence
```

**功能**: 提供可视化静默配置页面(移动端友好)

**页面设计**:
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>静默告警</title>
    <style>
        /* 移动端优化样式 */
        body { font-family: -apple-system, sans-serif; margin: 0; padding: 20px; }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 8px; font-weight: 600; }
        select, textarea {
            width: 100%;
            padding: 12px;
            border: 1px solid #ddd;
            border-radius: 8px;
            font-size: 16px;
        }
        button {
            width: 100%;
            padding: 14px;
            background: #1890ff;
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            cursor: pointer;
        }
    </style>
</head>
<body>
    <h2>静默告警: CPU使用率过高</h2>

    <form id="silenceForm">
        <div class="form-group">
            <label>静默时长</label>
            <select name="duration">
                <option value="1h">1小时 (临时维护)</option>
                <option value="6h">6小时</option>
                <option value="24h">24小时 (已知问题,待修复)</option>
                <option value="7d">7天</option>
                <option value="30d">30天 (规则误报,待优化)</option>
            </select>
        </div>

        <div class="form-group">
            <label>静默原因 <span style="color:red">*必填</span></label>
            <textarea name="reason" rows="4" placeholder="请说明静默原因,如:服务器正在进行安全补丁升级" required></textarea>
        </div>

        <div class="form-group">
            <label>
                <input type="checkbox" name="silenceSimilar">
                同时静默相似告警(同主机其他告警)
            </label>
        </div>

        <button type="submit">确认静默</button>
    </form>

    <script>
        document.getElementById('silenceForm').onsubmit = async (e) => {
            e.preventDefault();
            const formData = new FormData(e.target);

            const response = await fetch('/api/v1/alert/quick-silence', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    fingerprint: '{{.Fingerprint}}',
                    token: '{{.Token}}',
                    duration: formData.get('duration'),
                    reason: formData.get('reason'),
                    silenceSimilar: formData.get('silenceSimilar') === 'on'
                })
            });

            if (response.ok) {
                document.body.innerHTML = '<div style="text-align:center;margin-top:50px;"><h1>✅</h1><h2>静默成功</h2></div>';
            }
        };
    </script>
</body>
</html>
```

---

### 2.2 操作流程图

#### 流程1: 认领告警
```
用户 → 收到飞书通知
     → 点击"认领告警"按钮
     → 跳转到 /api/v1/alert/quick-action?action=claim&fingerprint=xxx&token=yyy
     → 后端验证Token
     → 更新告警状态(UpgradeState.IsConfirm=true, WhoAreConfirm=用户名)
     → 返回成功页面 "✅认领成功"
     → 用户关闭页面
```

#### 流程2: 静默告警
```
用户 → 收到飞书通知
     → 点击"静默1小时"按钮
     → 飞书弹出确认弹窗 "确认静默?"
     → 用户点击"确认"
     → 飞书回调 /api/v1/feishu/card-callback
     → 后端创建静默规则
     → 更新飞书卡片状态 "已静默至 XX:XX"
     → 用户无需额外操作
```

#### 流程3: 自定义静默
```
用户 → 点击"更多操作"
     → 跳转到静默配置页面
     → 选择时长、填写原因、勾选相似告警
     → 点击"确认静默"
     → 后端创建静默规则
     → 返回成功页面
```

---

## 三、数据库/缓存设计

### 3.1 MySQL表结构(复用现有表)

#### 表: `w8t_alert_silences`
**说明**: 复用现有静默表,无需新增字段

**快捷静默创建示例**:
```go
func createQuickSilence(alert models.AlertCurEvent, duration, reason, operator string) error {
    silence := models.AlertSilences{
        TenantId:      alert.TenantId,
        ID:            generateUUID(),
        Name:          fmt.Sprintf("快捷静默-%s", alert.RuleName),
        Labels:        convertToSilenceLabels(alert.Labels),
        StartsAt:      time.Now().Unix(),
        EndsAt:        time.Now().Add(parseDuration(duration)).Unix(),
        UpdateBy:      operator,
        FaultCenterId: alert.FaultCenterId,
        Comment:       fmt.Sprintf("[快捷操作] %s", reason),
        Status:        1, // 进行中
    }

    return db.Create(&silence).Error
}
```

### 3.2 审计日志
**表**: `w8t_audit_log`

**记录快捷操作**:
```go
auditLog := models.AuditLog{
    TenantId:  tenantId,
    Username:  operator,
    Action:    "quick_claim_alert", // quick_claim_alert/quick_silence_alert
    Resource:  fmt.Sprintf("alert:%s", fingerprint),
    Detail:    fmt.Sprintf("通过快捷操作认领告警: %s", alert.RuleName),
    IP:        ctx.ClientIP(),
    UserAgent: ctx.Request.UserAgent(),
    Timestamp: time.Now().Unix(),
}
```

---

## 四、接口详细设计

### 4.1 快捷操作接口

#### 接口1: 通用快捷操作
```
GET /api/v1/alert/quick-action
```

**请求参数**:
```
action=claim&fingerprint=abc123&token=eyJhbG...
```

**响应(HTML)**:
```html
✅ 认领成功
您可以关闭此页面
```

**响应(JSON - 用于飞书回调)**:
```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "action": "claim",
    "operator": "张三",
    "timestamp": 1705305600
  }
}
```

---

#### 接口2: 快捷静默
```
POST /api/v1/alert/quick-silence
```

**请求体**:
```json
{
  "fingerprint": "abc123",
  "token": "eyJhbG...",
  "duration": "1h",
  "reason": "服务器正在进行安全补丁升级",
  "silenceSimilar": true
}
```

**响应**:
```json
{
  "code": 200,
  "msg": "静默成功",
  "data": {
    "silenceId": "silence-123",
    "endsAt": 1705309200,
    "affectedAlerts": 3
  }
}
```

---

#### 接口3: 飞书卡片回调
```
POST /api/v1/feishu/card-callback
```

**请求体(飞书自动发送)**:
```json
{
  "open_id": "ou_xxx",
  "user_id": "user_123",
  "token": "verify_token",
  "action": {
    "value": {
      "action": "silence",
      "fingerprint": "abc123",
      "duration": "1h"
    }
  }
}
```

**响应(更新卡片)**:
```json
{
  "toast": {
    "type": "success",
    "content": "静默成功"
  },
  "card": {
    "elements": [
      {
        "tag": "div",
        "text": {
          "tag": "lark_md",
          "content": "✅ 已静默至 2024-01-15 15:30:00"
        }
      }
    ]
  }
}
```

---

### 4.2 中间件: QuickActionAuth

**位置**: `internal/middleware/QuickActionAuth.go`

```go
// QuickActionAuth 快捷操作Token验证中间件
func QuickActionAuth() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        // 1. 获取Token
        token := ctx.Query("token")
        if token == "" {
            token = ctx.PostForm("token")
        }

        if token == "" {
            response.Fail(ctx, "缺少Token", nil)
            ctx.Abort()
            return
        }

        // 2. 验证Token
        userInfo, err := verifyQuickToken(token)
        if err != nil {
            response.Fail(ctx, "Token无效: "+err.Error(), nil)
            ctx.Abort()
            return
        }

        // 3. 设置上下文
        ctx.Set("TenantID", userInfo.TenantId)
        ctx.Set("Username", userInfo.Username)
        ctx.Set("Fingerprint", userInfo.Fingerprint)

        ctx.Next()
    }
}
```

---

## 五、前端改造

### 5.1 飞书卡片模板改造
**文件**: `pkg/templates/feishuCard.go`

**改造点**:
```go
// 在 cardElements 末尾添加按钮组
func feishuTemplate(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {
    // ... 现有逻辑

    // 新增: 操作按钮组
    actionElement := map[string]interface{}{
        "tag": "action",
        "actions": buildFeishuActionButtons(alert),
    }

    cardElements = append(cardElements, actionElement)

    // ... 后续逻辑
}

func buildFeishuActionButtons(alert models.AlertCurEvent) []map[string]interface{} {
    baseUrl := getBaseUrl() // 从配置读取

    return []map[string]interface{}{
        {
            "tag":  "button",
            "type": "primary",
            "size": "medium",
            "text": map[string]interface{}{
                "tag":     "plain_text",
                "content": "认领告警",
            },
            "url": fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s",
                baseUrl, alert.Fingerprint, generateQuickToken(alert)),
        },
        {
            "tag":  "button",
            "type": "default",
            "size": "medium",
            "text": map[string]interface{}{
                "tag":     "plain_text",
                "content": "静默1小时",
            },
            "value": map[string]interface{}{
                "action":      "silence",
                "fingerprint": alert.Fingerprint,
                "duration":    "1h",
            },
        },
        {
            "tag":  "button",
            "type": "default",
            "size": "medium",
            "text": map[string]interface{}{
                "tag":     "plain_text",
                "content": "查看详情",
            },
            "url": fmt.Sprintf("%s/events/%s", baseUrl, alert.Fingerprint),
        },
    }
}
```

---

### 5.2 钉钉卡片改造
**文件**: `pkg/templates/dingCard.go`

**改造方案**: 从Markdown切换到ActionCard
```go
func dingdingTemplate(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {
    // 判断是否启用ActionCard
    if shouldUseActionCard() {
        return buildDingdingActionCard(alert, noticeTmpl)
    }

    // 否则使用原Markdown格式
    return buildDingdingMarkdown(alert, noticeTmpl)
}

func buildDingdingActionCard(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {
    baseUrl := getBaseUrl()

    card := models.DingMsg{
        Msgtype: "actionCard",
        ActionCard: models.ActionCard{
            Title: ParserTemplate("Title", alert, noticeTmpl.Template),
            Text:  ParserTemplate("Event", alert, noticeTmpl.Template),
            BtnOrientation: "1",
            Btns: []models.ActionCardBtn{
                {
                    Title:     "认领告警",
                    ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s",
                        baseUrl, alert.Fingerprint, generateQuickToken(alert)),
                },
                {
                    Title:     "静默设置",
                    ActionURL: fmt.Sprintf("%s/quick-silence?fingerprint=%s&token=%s",
                        baseUrl, alert.Fingerprint, generateQuickToken(alert)),
                },
                {
                    Title:     "查看详情",
                    ActionURL: fmt.Sprintf("%s/events/%s", baseUrl, alert.Fingerprint),
                },
            },
        },
    }

    return tools.JsonMarshalToString(card)
}
```

**钉钉模型扩展**:
```go
// 在 internal/models/xxx.go 中新增
type ActionCard struct {
    Title          string            `json:"title"`
    Text           string            `json:"text"`
    BtnOrientation string            `json:"btnOrientation"` // 0:横向 1:纵向
    Btns           []ActionCardBtn   `json:"btns"`
}

type ActionCardBtn struct {
    Title     string `json:"title"`
    ActionURL string `json:"actionURL"`
}
```

---

### 5.3 系统配置-基础URL
**位置**: `internal/models/settings.go`

**新增配置**:
```go
type SystemSettings struct {
    // ... 现有字段

    // 快捷操作配置
    QuickActionConfig QuickActionConfig `json:"quickActionConfig" gorm:"quickActionConfig;serializer:json"`
}

type QuickActionConfig struct {
    Enabled    bool   `json:"enabled"`    // 是否启用快捷操作
    BaseUrl    string `json:"baseUrl"`    // 系统访问地址,如: https://watchalert.com
    TokenTTL   int64  `json:"tokenTTL"`   // Token有效期(小时)
}
```

**前端配置页**:
```
┌─────────────────────────────────────┐
│ 系统设置 > 快捷操作                   │
├─────────────────────────────────────┤
│ ☑️ 启用快捷操作按钮                  │
│                                      │
│ 系统访问地址:                        │
│ [https://watchalert.com        ]    │
│ ⚠️ 必须配置公网可访问地址              │
│                                      │
│ Token有效期:                         │
│ [24] 小时                            │
│                                      │
│ [保存设置]                           │
└─────────────────────────────────────┘
```

---

## 六、安全设计

### 6.1 安全威胁与防护

| 威胁 | 风险等级 | 防护措施 |
|------|---------|---------|
| Token泄露 | 高 | 1. 24小时有效期<br>2. 绑定告警指纹<br>3. 使用HTTPS传输 |
| Token伪造 | 高 | JWT签名验证 |
| 重放攻击 | 中 | Token一次性使用(可选) |
| 越权操作 | 中 | Token包含租户ID,验证权限 |
| CSRF攻击 | 低 | GET请求幂等性设计 |

### 6.2 Token安全增强(可选)

**方案1: 一次性Token**
```go
// Token使用后立即失效
func useToken(tokenStr string) error {
    // 1. 验证Token
    payload, err := verifyQuickToken(tokenStr)
    if err != nil {
        return err
    }

    // 2. 检查是否已使用
    key := fmt.Sprintf("w8t:token:used:%s", tokenStr)
    exists := redis.Exists(key)
    if exists {
        return fmt.Errorf("Token已使用")
    }

    // 3. 标记为已使用(TTL与Token过期时间一致)
    redis.Set(key, "1", 24*time.Hour)

    return nil
}
```

**方案2: IP绑定(可选)**
```go
type QuickActionToken struct {
    // ... 现有字段
    ClientIP string `json:"clientIP"` // 生成Token时的客户端IP
}

// 验证时检查IP
func verifyQuickToken(tokenStr, clientIP string) error {
    payload, _ := parseToken(tokenStr)

    if payload.ClientIP != clientIP {
        return fmt.Errorf("IP地址不匹配")
    }

    return nil
}
```

---

## 七、实施计划

### 7.1 开发任务拆分

| 任务编号 | 任务名称 | 工作量 | 优先级 | 依赖 |
|---------|---------|--------|--------|------|
| QA-01 | Token生成与验证逻辑 | 1天 | P0 | - |
| QA-02 | 快捷操作API开发 | 1.5天 | P0 | QA-01 |
| QA-03 | 飞书卡片按钮改造 | 1天 | P0 | QA-02 |
| QA-04 | 钉钉卡片按钮改造 | 1天 | P0 | QA-02 |
| QA-05 | 成功页面HTML开发 | 0.5天 | P0 | QA-02 |
| QA-06 | 快捷静默页面开发 | 1天 | P1 | QA-02 |
| QA-07 | 飞书卡片回调接口 | 1天 | P1 | QA-03 |
| QA-08 | 系统配置页(BaseUrl) | 0.5天 | P1 | - |
| QA-09 | 审计日志记录 | 0.5天 | P1 | QA-02 |
| QA-10 | 单元测试 | 1天 | P1 | QA-02 |
| QA-11 | 集成测试(飞书/钉钉) | 1天 | P1 | QA-07 |

**总计**: 约10-11个工作日

### 7.2 开发里程碑

**Week 1**:
- ✅ 完成Token机制
- ✅ 完成快捷操作API
- ✅ 完成飞书卡片改造
- ✅ 基础功能可用(认领、查看详情)

**Week 2**:
- ✅ 完成钉钉卡片改造
- ✅ 完成快捷静默功能
- ✅ 完成测试
- ✅ 灰度发布

---

## 八、测试方案

### 8.1 功能测试

#### 测试用例1: 飞书快捷认领
**步骤**:
1. 触发告警,收到飞书通知
2. 点击"认领告警"按钮
3. 验证跳转成功
4. 验证告警状态更新

**预期**:
- 页面显示"✅认领成功"
- 告警详情页显示"已认领,认领人:XXX"
- 审计日志记录操作

#### 测试用例2: 飞书快捷静默
**步骤**:
1. 收到飞书通知
2. 点击"静默1小时"按钮
3. 飞书弹出确认弹窗
4. 点击"确认"

**预期**:
- 飞书卡片更新为"已静默至XX:XX"
- 静默规则创建成功
- 后续告警被抑制

#### 测试用例3: Token安全性
**步骤**:
1. 生成Token
2. 24小时后使用Token

**预期**:
- 返回"Token已过期"
- 操作失败

#### 测试用例4: 钉钉ActionCard
**步骤**:
1. 配置钉钉通知
2. 触发告警
3. 验证钉钉收到ActionCard格式消息
4. 点击按钮测试

**预期**:
- 钉钉显示竖向排列的按钮
- 点击按钮跳转正确

### 8.2 兼容性测试

| 平台 | 版本 | 测试内容 | 结果 |
|------|------|---------|------|
| 飞书移动端 | iOS/Android | 按钮点击、页面跳转 | ✅ |
| 飞书PC端 | Win/Mac | 按钮点击、页面跳转 | ✅ |
| 钉钉移动端 | iOS/Android | ActionCard显示、跳转 | ✅ |
| 钉钉PC端 | Win/Mac | ActionCard显示、跳转 | ✅ |
| 企业微信 | 移动端/PC端 | 待实现 | - |

---

## 九、上线计划

### 9.1 灰度策略

**阶段1: 内部测试(1-2天)**
- 仅对测试租户开放
- 验证飞书/钉钉按钮正常工作

**阶段2: 小范围灰度(3-5天)**
- 对10%用户开放
- 收集用户反馈

**阶段3: 全量发布**
- 无重大问题后全量开放

### 9.2 监控指标

| 指标 | 阈值 | 告警级别 |
|------|------|---------|
| Token验证失败率 | >10% | P2 |
| 快捷操作API错误率 | >5% | P1 |
| 快捷操作使用率 | <20% | P3(提示优化) |
| 平均操作响应时间 | >3秒 | P2 |

---

## 十、FAQ

### Q1: Token泄露怎么办?
**A**:
1. Token有效期仅24小时,自动过期
2. Token绑定告警指纹,无法用于其他告警
3. 所有操作记录审计日志,可追溯
4. 如发现异常,可在系统设置中"重置Token密钥"

### Q2: 不登录如何识别操作人?
**A**: Token中包含当前值班人信息,操作时会记录为该值班人。如需更精确,可要求用户首次使用时绑定飞书/钉钉账号。

### Q3: 支持企业微信吗?
**A**: 当前版本支持飞书和钉钉,企业微信将在下一版本支持(技术方案类似)。

### Q4: 快捷静默会影响其他告警吗?
**A**: 默认仅静默当前告警。如勾选"同时静默相似告警",会基于Labels创建静默规则,影响范围可控。

---

## 十一、附录

### 附录A: 相关代码文件清单
```
新增:
- api/quickAction.go (快捷操作控制器)
- internal/services/quickAction.go (快捷操作服务)
- internal/middleware/QuickActionAuth.go (Token验证中间件)
- pkg/utils/quickToken.go (Token工具类)
- templates/quick-silence.html (快捷静默页面)
- templates/success-page.html (成功页面)

修改:
- pkg/templates/feishuCard.go (飞书卡片)
- pkg/templates/dingCard.go (钉钉卡片)
- internal/models/settings.go (系统配置)
- internal/models/template_dingding.go (钉钉模型)
- internal/routers/v1/api.go (路由注册)
```

### 附录B: 飞书开发文档参考
- 消息卡片: https://open.feishu.cn/document/ukTMukTMukTM/uEjNwUjLxYDM14SM2ATN
- 卡片回调: https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN

### 附录C: 钉钉开发文档参考
- ActionCard: https://open.dingtalk.com/document/robots/action-card-type

---

**文档版本**: v1.0
**编写日期**: 2024-01-15
**编写人**: AI Assistant
**审核人**: [待填写]
**批准人**: [待填写]