package middleware

import (
	"github.com/gin-gonic/gin"
	"watchAlert/internal/ctx"
	"watchAlert/pkg/response"
	"watchAlert/pkg/utils"
)

// QuickActionAuth 快捷操作Token验证中间件
// 用于验证快捷操作链接中的Token，无需传统的登录Token
func QuickActionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Token（支持query参数和form参数）
		token := c.Query("token")
		if token == "" {
			token = c.PostForm("token")
		}

		// Token为空，返回错误
		if token == "" {
			response.Fail(c, "缺少Token参数", "")
			c.Abort()
			return
		}

		// 获取系统配置中的SecretKey
		settings, err := ctx.DO().DB.Setting().Get()
		if err != nil {
			response.Fail(c, "获取系统配置失败", "")
			c.Abort()
			return
		}

		secretKey := settings.QuickActionConfig.SecretKey
		if secretKey == "" {
			response.Fail(c, "快捷操作未配置密钥", "")
			c.Abort()
			return
		}

		// 验证Token
		payload, err := utils.VerifyQuickToken(token, secretKey)
		if err != nil {
			response.Fail(c, "Token验证失败: "+err.Error(), "")
			c.Abort()
			return
		}

		// 将Token信息设置到上下文中，供后续处理使用
		c.Set("TenantID", payload.TenantId)
		c.Set("Username", payload.Username)
		c.Set("Fingerprint", payload.Fingerprint)

		c.Next()
	}
}