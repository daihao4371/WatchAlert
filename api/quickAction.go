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
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
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
