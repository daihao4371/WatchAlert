package api

import (
	"fmt"
	"watchAlert/internal/middleware"
	"watchAlert/internal/services"

	"github.com/gin-gonic/gin"
)

type quickActionController struct{}

type InterQuickActionController interface {
	API(gin *gin.RouterGroup)
	QuickAction(ctx *gin.Context)
}

// QuickActionController 全局控制器实例（与其他控制器保持一致的命名规范）
var QuickActionController = &quickActionController{}

// API 注册快捷操作路由
// 快捷操作使用自定义Token验证，无需传统登录Auth
func (q quickActionController) API(gin *gin.RouterGroup) {
	a := gin.Group("alert")
	// 使用快捷操作Token验证中间件，不使用Auth中间件
	a.Use(
		middleware.QuickActionAuth(),
		middleware.ParseTenant(),
	)
	{
		a.GET("quick-action", q.QuickAction)
		a.GET("quick-silence", q.QuickSilenceForm)  // 自定义静默表单页面
		a.POST("quick-silence", q.QuickSilence)     // 提交自定义静默
	}
}

// QuickAction 快捷操作接口
// 支持的操作类型：claim（认领）、silence（静默）、resolve（标记已处理）
func (q quickActionController) QuickAction(ctx *gin.Context) {
	// 解析参数
	action := ctx.Query("action")
	fingerprint := ctx.Query("fingerprint")
	duration := ctx.DefaultQuery("duration", "1h") // 静默时长，默认1小时

	// 从上下文获取Token中的信息（已由中间件验证并设置）
	tenantIdVal, _ := ctx.Get("TenantID")
	usernameVal, _ := ctx.Get("Username")

	tenantId := tenantIdVal.(string)
	username := usernameVal.(string)

	// 校验操作类型
	if action == "" {
		renderErrorPage(ctx, "操作类型不能为空")
		return
	}

	// 执行对应的操作
	var err error
	var actionName string

	switch action {
	case "claim":
		// 认领告警
		err = services.QuickActionService.ClaimAlert(tenantId, fingerprint, username)
		actionName = "认领"

	case "silence":
		// 静默告警
		err = services.QuickActionService.SilenceAlert(tenantId, fingerprint, duration, username)
		actionName = "静默"

	case "resolve":
		// 标记已处理
		err = services.QuickActionService.ResolveAlert(tenantId, fingerprint, username)
		actionName = "标记已处理"

	default:
		renderErrorPage(ctx, "不支持的操作类型: "+action)
		return
	}

	// 处理操作结果
	if err != nil {
		renderErrorPage(ctx, err.Error())
		return
	}

	// 渲染成功页面
	renderSuccessPage(ctx, actionName)
}

// renderSuccessPage 渲染操作成功页面（移动端友好）
func renderSuccessPage(ctx *gin.Context, actionName string) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>操作成功</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #a8edea 0%%, #fed6e3 100%%);
            padding: 20px;
        }
        .container {
            text-align: center;
            background: white;
            padding: 40px 30px;
            border-radius: 16px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.15);
            max-width: 400px;
            width: 100%%;
            animation: slideUp 0.4s ease-out;
        }
        @keyframes slideUp {
            from {
                opacity: 0;
                transform: translateY(20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        .icon {
            font-size: 64px;
            margin-bottom: 20px;
            animation: scaleIn 0.5s ease-out 0.2s both;
        }
        @keyframes scaleIn {
            from {
                transform: scale(0);
            }
            to {
                transform: scale(1);
            }
        }
        h1 {
            color: #52c41a;
            margin: 0 0 15px 0;
            font-size: 24px;
            font-weight: 600;
        }
        p {
            color: #666;
            font-size: 14px;
            line-height: 1.6;
        }
        .divider {
            height: 1px;
            background: #f0f0f0;
            margin: 20px 0;
        }
        .tip {
            color: #999;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">✅</div>
        <h1>%s成功</h1>
        <p>操作已成功完成</p>
        <div class="divider"></div>
        <p class="tip">您可以关闭此页面</p>
    </div>
    <script>
        // 3秒后自动尝试关闭页面（部分浏览器支持）
        setTimeout(function() {
            window.close();
        }, 3000);
    </script>
</body>
</html>
    `, actionName)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, html)
}

// renderErrorPage 渲染操作失败页面
func renderErrorPage(ctx *gin.Context, errorMsg string) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>操作失败</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #f5f7fa 0%%, #c3cfe2 100%%);
            padding: 20px;
        }
        .container {
            text-align: center;
            background: white;
            padding: 40px 30px;
            border-radius: 16px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.15);
            max-width: 400px;
            width: 100%%;
        }
        .icon { font-size: 64px; margin-bottom: 20px; }
        h1 { color: #ff4d4f; margin: 0 0 15px 0; font-size: 24px; font-weight: 600; }
        .error-msg {
            color: #666;
            font-size: 14px;
            line-height: 1.6;
            background: #fff2f0;
            padding: 12px;
            border-radius: 8px;
            border-left: 3px solid #ff4d4f;
            text-align: left;
            word-break: break-word;
        }
        .divider { height: 1px; background: #f0f0f0; margin: 20px 0; }
        .tip { color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">❌</div>
        <h1>操作失败</h1>
        <div class="error-msg">%s</div>
        <div class="divider"></div>
        <p class="tip">请稍后重试或联系管理员</p>
    </div>
</body>
</html>
    `, errorMsg)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(400, html)
}

// QuickSilenceForm 渲染自定义静默表单页面
func (q quickActionController) QuickSilenceForm(ctx *gin.Context) {
	fingerprint := ctx.Query("fingerprint")
	token := ctx.Query("token")

	// 获取告警信息用于显示
	tenantIdVal, _ := ctx.Get("TenantID")
	tenantId := tenantIdVal.(string)

	// 获取告警详情(用于显示告警名称)
	alert, err := services.QuickActionService.GetAlertByFingerprint(tenantId, fingerprint)
	alertTitle := "告警"
	if err == nil && alert != nil {
		alertTitle = alert.RuleName
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>自定义静默</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #ffecd2 0%%, #fcb69f 100%%);
            padding: 20px;
            min-height: 100vh;
        }
        .container {
            max-width: 500px;
            margin: 0 auto;
            background: white;
            border-radius: 16px;
            padding: 30px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.15);
        }
        h2 {
            color: #333;
            margin-bottom: 10px;
            font-size: 22px;
        }
        .alert-name {
            color: #666;
            font-size: 14px;
            margin-bottom: 25px;
            padding: 10px;
            background: #f5f5f5;
            border-radius: 8px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            margin-bottom: 8px;
            font-weight: 600;
            color: #333;
            font-size: 14px;
        }
        select, textarea {
            width: 100%%;
            padding: 12px;
            border: 1px solid #ddd;
            border-radius: 8px;
            font-size: 14px;
            font-family: inherit;
            transition: border-color 0.3s;
        }
        select:focus, textarea:focus {
            outline: none;
            border-color: #667eea;
        }
        textarea {
            resize: vertical;
            min-height: 80px;
        }
        .required {
            color: #ff4d4f;
            margin-left: 2px;
        }
        .submit-btn {
            width: 100%%;
            padding: 14px;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .submit-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        .submit-btn:active {
            transform: translateY(0);
        }
        .submit-btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
        }
        .option-desc {
            color: #999;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h2>🔕 自定义静默</h2>
        <div class="alert-name">告警: %s</div>

        <form id="silenceForm">
            <div class="form-group">
                <label>静默时长 <span class="required">*</span></label>
                <select name="duration" required>
                    <option value="1h">1小时 (临时问题)</option>
                    <option value="6h">6小时 (短期维护)</option>
                    <option value="24h">24小时 (已知问题,待修复)</option>
                    <option value="72h">3天 (计划维护)</option>
                    <option value="168h">7天 (长期维护)</option>
                    <option value="720h">30天 (规则误报,待优化)</option>
                </select>
            </div>

            <div class="form-group">
                <label>静默原因 <span class="required">*</span></label>
                <textarea
                    name="reason"
                    placeholder="请说明静默原因，如：服务器正在进行安全补丁升级"
                    required
                ></textarea>
            </div>

            <button type="submit" class="submit-btn" id="submitBtn">确认静默</button>
        </form>
    </div>

    <script>
        const form = document.getElementById('silenceForm');
        const submitBtn = document.getElementById('submitBtn');

        form.onsubmit = async (e) => {
            e.preventDefault();

            const formData = new FormData(e.target);
            const duration = formData.get('duration');
            const reason = formData.get('reason');

            if (!reason.trim()) {
                alert('请填写静默原因');
                return;
            }

            // 禁用提交按钮
            submitBtn.disabled = true;
            submitBtn.textContent = '提交中...';

            try {
                const response = await fetch('/api/v1/alert/quick-silence', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    },
                    body: 'fingerprint=%s&token=%s&duration=' + duration + '&reason=' + encodeURIComponent(reason)
                });

                if (response.ok) {
                    document.body.innerHTML = '<div style="display:flex;justify-content:center;align-items:center;min-height:100vh;"><div style="text-align:center;background:white;padding:40px;border-radius:16px;box-shadow:0 10px 40px rgba(0,0,0,0.15);"><div style="font-size:64px;margin-bottom:20px;">✅</div><h1 style="color:#52c41a;margin:0 0 15px 0;font-size:24px;">静默成功</h1><p style="color:#666;font-size:14px;">您可以关闭此页面</p></div></div>';
                    setTimeout(() => window.close(), 2000);
                } else {
                    const text = await response.text();
                    alert('静默失败: ' + text);
                    submitBtn.disabled = false;
                    submitBtn.textContent = '确认静默';
                }
            } catch (error) {
                alert('请求失败: ' + error.message);
                submitBtn.disabled = false;
                submitBtn.textContent = '确认静默';
            }
        };
    </script>
</body>
</html>
    `, alertTitle, fingerprint, token)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, html)
}

// QuickSilence 处理自定义静默提交
func (q quickActionController) QuickSilence(ctx *gin.Context) {
	// 解析参数
	fingerprint := ctx.PostForm("fingerprint")
	duration := ctx.PostForm("duration")
	reason := ctx.PostForm("reason")

	// 从上下文获取Token中的信息
	tenantIdVal, _ := ctx.Get("TenantID")
	usernameVal, _ := ctx.Get("Username")

	tenantId := tenantIdVal.(string)
	username := usernameVal.(string)

	// 校验必填参数
	if fingerprint == "" || duration == "" || reason == "" {
		renderErrorPage(ctx, "参数不完整")
		return
	}

	// 执行静默操作,传入reason
	err := services.QuickActionService.SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason)
	if err != nil {
		renderErrorPage(ctx, err.Error())
		return
	}

	// 渲染成功页面
	renderSuccessPage(ctx, "静默")
}
