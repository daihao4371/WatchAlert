package api

import (
	"fmt"
	"watchAlert/internal/middleware"
	"watchAlert/internal/services"
	"watchAlert/internal/types"

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
// 所有快捷操作都需要登录验证,确保安全性和审计准确性
// 使用专用的QuickActionLoginAuth中间件,未登录时重定向到登录页面(而非返回JSON 401)
func (q quickActionController) API(gin *gin.RouterGroup) {
	alert := gin.Group("alert")

	// 登录相关路由（无需中间件）
	alert.GET("quick-login", q.QuickLogin)     // 显示登录页面
	alert.POST("quick-login", q.DoQuickLogin)  // 处理登录请求

	// 快捷操作路由（需要登录验证）
	authGroup := alert.Group("")
	authGroup.Use(
		middleware.QuickActionAuth(),      // Token验证(验证操作合法性)
		middleware.QuickActionLoginAuth(), // 登录验证(获取真实操作人,未登录则重定向)
		middleware.ParseTenant(),
	)
	{
		authGroup.GET("quick-action", q.QuickAction)         // 快捷操作
		authGroup.GET("quick-silence", q.QuickSilenceForm)   // 自定义静默表单
		authGroup.POST("quick-silence", q.QuickSilence)      // 提交自定义静默
	}
}

// QuickAction 快捷操作接口
// 支持的操作类型：claim（认领）、silence（静默）、resolve（标记已处理）
// 必须登录后才能操作,从JWT Token中获取真实操作人
func (q quickActionController) QuickAction(ctx *gin.Context) {
	// 解析参数
	action := ctx.Query("action")
	fingerprint := ctx.Query("fingerprint")
	duration := ctx.DefaultQuery("duration", "1h")

	// 从上下文获取租户ID(由QuickActionAuth中间件设置)
	tenantIdVal, exists := ctx.Get("TenantID")
	if !exists {
		renderErrorPage(ctx, "缺少租户信息")
		return
	}
	tenantId := tenantIdVal.(string)

	// 从JWT Token获取真实操作人(由Auth中间件设置)
	usernameVal, exists := ctx.Get("username")
	if !exists {
		renderErrorPage(ctx, "用户未登录")
		return
	}
	username := usernameVal.(string)
	clientIP := ctx.ClientIP()

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
		err = services.QuickActionService.ClaimAlert(tenantId, fingerprint, username, clientIP)
		actionName = "认领"

	case "silence":
		err = services.QuickActionService.SilenceAlert(tenantId, fingerprint, duration, username, clientIP)
		actionName = "静默"

	case "resolve":
		err = services.QuickActionService.ResolveAlert(tenantId, fingerprint, username, clientIP)
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
	// 判断是否是告警不存在或Token过期的错误
	var icon, title, tip string
	if contains(errorMsg, "未找到指纹") || contains(errorMsg, "告警不存在") {
		icon = "⏰"
		title = "告警已失效"
		tip = "此告警可能已被处理或链接已过期（有效期24小时）"
	} else if contains(errorMsg, "Token已过期") || contains(errorMsg, "Token验证失败") {
		icon = "🔒"
		title = "链接已过期"
		tip = "快捷操作链接有效期为24小时，请从最新的告警通知中重新访问"
	} else {
		icon = "❌"
		title = "操作失败"
		tip = "请稍后重试或联系管理员"
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>%s</title>
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
        h1 { color: #ff9800; margin: 0 0 15px 0; font-size: 24px; font-weight: 600; }
        .error-msg {
            color: #666;
            font-size: 14px;
            line-height: 1.6;
            background: #fff3e0;
            padding: 12px;
            border-radius: 8px;
            border-left: 3px solid #ff9800;
            text-align: left;
            word-break: break-word;
        }
        .divider { height: 1px; background: #f0f0f0; margin: 20px 0; }
        .tip { color: #999; font-size: 12px; line-height: 1.5; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">%s</div>
        <h1>%s</h1>
        <div class="error-msg">%s</div>
        <div class="divider"></div>
        <p class="tip">%s</p>
    </div>
</body>
</html>
    `, title, icon, title, errorMsg, tip)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(400, html)
}

// contains 检查字符串是否包含子串（辅助函数）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
            background: linear-gradient(135deg, #f5f7fa 0%%, #e4e9f2 100%%);
            padding: 20px;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            max-width: 500px;
            width: 100%%;
            background: white;
            border-radius: 20px;
            padding: 32px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.08);
            animation: slideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1);
        }
        @keyframes slideUp {
            from {
                opacity: 0;
                transform: translateY(30px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        .header {
            text-align: center;
            margin-bottom: 28px;
        }
        .icon {
            font-size: 48px;
            margin-bottom: 12px;
        }
        h2 {
            color: #1a1a1a;
            margin-bottom: 8px;
            font-size: 24px;
            font-weight: 700;
        }
        .subtitle {
            color: #666;
            font-size: 14px;
        }
        .alert-name {
            color: #555;
            font-size: 14px;
            margin-bottom: 28px;
            padding: 14px 16px;
            background: linear-gradient(135deg, #fff9f0 0%%, #fff5e6 100%%);
            border-radius: 10px;
            border-left: 4px solid #ff6b35;
            font-weight: 500;
            word-break: break-word;
        }
        .form-group {
            margin-bottom: 22px;
        }
        label {
            display: block;
            margin-bottom: 10px;
            font-weight: 600;
            color: #2c3e50;
            font-size: 14px;
        }
        select, textarea {
            width: 100%%;
            padding: 13px 16px;
            border: 2px solid #e8ecef;
            border-radius: 10px;
            font-size: 14px;
            font-family: inherit;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            background: #f8f9fa;
            color: #2c3e50;
        }
        select:hover, textarea:hover {
            border-color: #cbd5e0;
            background: #fff;
        }
        select:focus, textarea:focus {
            outline: none;
            border-color: #5b8def;
            background: white;
            box-shadow: 0 0 0 4px rgba(91, 141, 239, 0.1);
        }
        textarea {
            resize: vertical;
            min-height: 90px;
            line-height: 1.6;
        }
        .required {
            color: #ef4444;
            margin-left: 3px;
            font-weight: 700;
        }
        .submit-btn {
            width: 100%%;
            padding: 15px;
            background: linear-gradient(135deg, #5b8def 0%%, #4c7ce5 100%%);
            color: white;
            border: none;
            border-radius: 10px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            box-shadow: 0 4px 14px rgba(91, 141, 239, 0.35);
            letter-spacing: 0.3px;
        }
        .submit-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(91, 141, 239, 0.45);
            background: linear-gradient(135deg, #4c7ce5 0%%, #3d6dd6 100%%);
        }
        .submit-btn:active {
            transform: translateY(0);
            box-shadow: 0 4px 14px rgba(91, 141, 239, 0.35);
        }
        .submit-btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
        .tip {
            text-align: center;
            color: #94a3b8;
            font-size: 13px;
            margin-top: 20px;
            line-height: 1.5;
        }
        option {
            padding: 10px;
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
// 必须登录后才能操作
func (q quickActionController) QuickSilence(ctx *gin.Context) {
	// 解析参数
	fingerprint := ctx.PostForm("fingerprint")
	duration := ctx.PostForm("duration")
	reason := ctx.PostForm("reason")

	// 从上下文获取租户信息
	tenantIdVal, exists := ctx.Get("TenantID")
	if !exists {
		renderErrorPage(ctx, "缺少租户信息")
		return
	}
	tenantId := tenantIdVal.(string)

	// 从JWT Token获取真实操作人(必须登录)
	usernameVal, exists := ctx.Get("username")
	if !exists {
		renderErrorPage(ctx, "用户未登录")
		return
	}
	username := usernameVal.(string)
	clientIP := ctx.ClientIP()

	// 校验必填参数
	if fingerprint == "" || duration == "" || reason == "" {
		renderErrorPage(ctx, "参数不完整")
		return
	}

	// 执行静默操作,传入reason和clientIP
	err := services.QuickActionService.SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason, clientIP)
	if err != nil {
		renderErrorPage(ctx, err.Error())
		return
	}

	// 渲染成功页面
	renderSuccessPage(ctx, "静默")
}

// QuickLogin 渲染快捷操作登录页面
// 用于快捷操作场景的专用登录页面，登录成功后自动跳转回原始操作URL
func (q quickActionController) QuickLogin(ctx *gin.Context) {
	// 获取redirect参数（原始快捷操作URL）
	redirectURL := ctx.Query("redirect")
	if redirectURL == "" {
		renderErrorPage(ctx, "缺少redirect参数")
		return
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>登录 - 快捷操作</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: linear-gradient(135deg, #e3f2fd 0%%, #bbdefb 100%%);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            padding: 20px;
        }
        .login-container {
            background: white;
            border-radius: 16px;
            padding: 40px 30px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.1);
            max-width: 400px;
            width: 100%%;
        }
        .logo {
            text-align: center;
            font-size: 48px;
            margin-bottom: 10px;
        }
        h1 {
            text-align: center;
            color: #333;
            margin-bottom: 10px;
            font-size: 24px;
        }
        .subtitle {
            text-align: center;
            color: #999;
            font-size: 14px;
            margin-bottom: 30px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            margin-bottom: 8px;
            color: #333;
            font-weight: 600;
            font-size: 14px;
        }
        input {
            width: 100%%;
            padding: 12px;
            border: 1px solid #ddd;
            border-radius: 8px;
            font-size: 14px;
            transition: border-color 0.3s;
        }
        input:focus {
            outline: none;
            border-color: #1976d2;
        }
        .login-btn {
            width: 100%%;
            padding: 14px;
            background: linear-gradient(135deg, #1976d2 0%%, #1565c0 100%%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .login-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(25, 118, 210, 0.3);
        }
        .login-btn:active {
            transform: translateY(0);
        }
        .login-btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
        .error-msg {
            color: #ff4d4f;
            font-size: 14px;
            margin-top: 10px;
            padding: 10px;
            background: #fff2f0;
            border-radius: 8px;
            display: none;
        }
        .tip {
            text-align: center;
            color: #999;
            font-size: 12px;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="logo">🔐</div>
        <h1>登录验证</h1>
        <p class="subtitle">完成登录后自动执行快捷操作</p>

        <form id="loginForm">
            <div class="form-group">
                <label for="username">用户名</label>
                <input type="text" id="username" name="username" required autocomplete="username">
            </div>

            <div class="form-group">
                <label for="password">密码</label>
                <input type="password" id="password" name="password" required autocomplete="current-password">
            </div>

            <button type="submit" class="login-btn" id="loginBtn">登录</button>
            <div class="error-msg" id="errorMsg"></div>
        </form>

        <p class="tip">🔒 安全连接 · 操作将记录审计日志</p>
    </div>

    <script>
        const loginForm = document.getElementById('loginForm');
        const loginBtn = document.getElementById('loginBtn');
        const errorMsg = document.getElementById('errorMsg');
        const redirectURL = %s; // 原始快捷操作URL

        loginForm.onsubmit = async (e) => {
            e.preventDefault();

            const username = document.getElementById('username').value;
            const password = document.getElementById('password').value;

            // 禁用按钮
            loginBtn.disabled = true;
            loginBtn.textContent = '登录中...';
            errorMsg.style.display = 'none';

            try {
                const response = await fetch('/api/v1/alert/quick-login', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        username: username,
                        password: password,
                        redirect: redirectURL
                    })
                });

                const result = await response.json();

                if (response.ok && result.code === 200) {
                    // 登录成功，保存token到Cookie
                    document.cookie = 'Authorization=' + result.data.token + '; path=/; max-age=86400';

                    // 跳转回原始URL
                    window.location.href = redirectURL;
                } else {
                    // 登录失败
                    errorMsg.textContent = result.msg || '登录失败，请检查用户名和密码';
                    errorMsg.style.display = 'block';
                    loginBtn.disabled = false;
                    loginBtn.textContent = '登录';
                }
            } catch (error) {
                errorMsg.textContent = '网络错误: ' + error.message;
                errorMsg.style.display = 'block';
                loginBtn.disabled = false;
                loginBtn.textContent = '登录';
            }
        };
    </script>
</body>
</html>
    `, fmt.Sprintf(`"%s"`, redirectURL))

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, html)
}

// DoQuickLogin 处理快捷操作登录请求
// 调用用户登录服务，返回JWT token
func (q quickActionController) DoQuickLogin(ctx *gin.Context) {
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Redirect string `json:"redirect"`
	}

	if err := ctx.ShouldBindJSON(&loginReq); err != nil {
		ctx.JSON(400, gin.H{
			"code": 400,
			"msg":  "参数错误",
			"data": nil,
		})
		return
	}

	// 调用用户登录服务
	result, errMsg := services.UserService.Login(&types.RequestUserLogin{
		UserName: loginReq.Username,
		Password: loginReq.Password,
	})

	if errMsg != nil {
		// 登录失败，errMsg 是 interface{} 类型，需要转换为字符串
		ctx.JSON(401, gin.H{
			"code": 401,
			"msg":  fmt.Sprintf("%v", errMsg),
			"data": nil,
		})
		return
	}

	// 登录成功，返回token
	ctx.JSON(200, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": result,
	})
}
