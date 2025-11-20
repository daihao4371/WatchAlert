package templates

import (
	"fmt"
	"strings"
	models2 "watchAlert/internal/models"
	"watchAlert/pkg/tools"
	"watchAlert/pkg/utils"
)

// quickActionConfig 快捷操作配置缓存（避免频繁查询数据库）
var quickActionConfig *models2.QuickActionConfig

// SetQuickActionConfig 设置快捷操作配置（由初始化程序调用）
func SetQuickActionConfig(config models2.QuickActionConfig) {
	quickActionConfig = &config
}

// getQuickActionConfig 获取快捷操作配置
func getQuickActionConfig() models2.QuickActionConfig {
	if quickActionConfig == nil {
		// 返回默认配置（禁用状态）
		disabled := false
		return models2.QuickActionConfig{
			Enabled: &disabled,
		}
	}
	return *quickActionConfig
}

// dingdingTemplate 钉钉消息模板
// 支持两种模式：
// 1. Markdown 模式（默认）- 传统文本消息
// 2. ActionCard 模式 - 带快捷操作按钮的卡片消息
func dingdingTemplate(alert models2.AlertCurEvent, noticeTmpl models2.NoticeTemplateExample) string {
	// 获取快捷操作配置
	quickConfig := getQuickActionConfig()

	// 如果启用快捷操作且配置了 BaseUrl 和 SecretKey，使用 ActionCard 模式
	if quickConfig.GetEnable() && quickConfig.BaseUrl != "" && quickConfig.SecretKey != "" {
		return buildDingdingActionCard(alert, noticeTmpl, quickConfig)
	}

	// 否则使用传统 Markdown 模式
	return buildDingdingMarkdown(alert, noticeTmpl)
}

// buildDingdingMarkdown 构建钉钉 Markdown 消息（传统模式）
func buildDingdingMarkdown(alert models2.AlertCurEvent, noticeTmpl models2.NoticeTemplateExample) string {
	Title := ParserTemplate("Title", alert, noticeTmpl.Template)
	Footer := ParserTemplate("Footer", alert, noticeTmpl.Template)

	// 解析值班用户，支持 @提及
	dutyUser := alert.DutyUser
	var dutyUsers []string
	for _, user := range strings.Split(dutyUser, " ") {
		u := strings.Trim(user, "@")
		dutyUsers = append(dutyUsers, u)
	}

	t := models2.DingMsg{
		Msgtype: "markdown",
		Markdown: &models2.Markdown{
			Title: Title,
			Text: "**" + Title + "**" +
				"\n" + "\n" +
				ParserTemplate("Event", alert, noticeTmpl.Template) +
				"\n" +
				Footer,
		},
		At: &models2.At{
			AtUserIds: dutyUsers,
			AtMobiles: dutyUsers,
			IsAtAll:   false,
		},
	}

	// 如果是 @all，则@所有人
	if strings.Trim(alert.DutyUser, " ") == "all" {
		t.At = &models2.At{
			AtUserIds: []string{},
			AtMobiles: []string{},
			IsAtAll:   true,
		}
	}

	return tools.JsonMarshalToString(t)
}

// buildDingdingActionCard 构建钉钉 ActionCard 消息（带快捷操作按钮）
func buildDingdingActionCard(alert models2.AlertCurEvent, noticeTmpl models2.NoticeTemplateExample, config models2.QuickActionConfig) string {
	Title := ParserTemplate("Title", alert, noticeTmpl.Template)
	EventText := ParserTemplate("Event", alert, noticeTmpl.Template)

	// 生成快捷操作 Token（24小时有效期）
	token, err := utils.GenerateQuickToken(
		alert.TenantId,
		alert.Fingerprint,
		alert.DutyUser,
		config.SecretKey,
	)
	if err != nil {
		// Token 生成失败，降级为 Markdown 模式
		return buildDingdingMarkdown(alert, noticeTmpl)
	}

	// 确定 API 调用地址（优先使用 ApiUrl，否则使用 BaseUrl）
	apiUrl := config.ApiUrl
	if apiUrl == "" {
		apiUrl = config.BaseUrl // 向后兼容：如果没有配置 ApiUrl，使用 BaseUrl
	}

	// 构建 ActionCard 消息（使用钉钉官方字段名）
	// 注意：ActionCard 模式下不应包含 markdown 和 at 字段
	card := models2.DingMsg{
		Msgtype: "actionCard",
		ActionCard: &models2.ActionCard{
			Title:          Title,
			Text:           "#### " + Title + "\n\n" + EventText,
			BtnOrientation: "1", // 按钮纵向排列，移动端体验更好
			Btns: []models2.ActionCardBtn{
				{
					Title:     "🔔 认领告警",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s", apiUrl, alert.Fingerprint, token),
				},
				{
					Title:     "🔕 静默告警",
					ActionURL: fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=1h", apiUrl, alert.Fingerprint, token),
				},
				{
					Title:     "📊 查看详情",
					ActionURL: fmt.Sprintf("%s/faultCenter/detail/%s", config.BaseUrl, alert.FaultCenterId),
				},
			},
		},
	}

	return tools.JsonMarshalToString(card)
}
