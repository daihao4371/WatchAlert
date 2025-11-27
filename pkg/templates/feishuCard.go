package templates

import (
	"fmt"
	"strings"
	"watchAlert/internal/models"
	"watchAlert/pkg/tools"
	"watchAlert/pkg/utils"

	"github.com/bytedance/sonic"
)

// Template 飞书消息卡片模版
func feishuTemplate(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {

	var cardContentString string
	if *noticeTmpl.EnableFeiShuJsonCard {
		defaultTemplate := models.FeiShuJsonCardMsg{
			MsgType: "interactive",
		}
		var tmplC models.JsonCards
		switch alert.IsRecovered {
		case false:
			cardContentString = noticeTmpl.TemplateFiring
		case true:
			cardContentString = noticeTmpl.TemplateRecover
		}
		cardContentString = ParserTemplate("Card", alert, cardContentString)
		_ = sonic.Unmarshal([]byte(cardContentString), &tmplC)
		defaultTemplate.Card = tmplC
		cardContentString = tools.JsonMarshalToString(defaultTemplate)

	} else {
		defaultTemplate := models.FeiShuJsonCardMsg{
			MsgType: "interactive",
			Card: models.JsonCards{
				Config: tools.ConvertStructToMap(models.Configs{
					EnableForward: true,
					WidthMode:     models.WidthModeDefault,
				}),
			},
		}
		cardHeader := models.Headers{
			Template: ParserTemplate("TitleColor", alert, noticeTmpl.Template),
			Title: models.Titles{
				Content: ParserTemplate("Title", alert, noticeTmpl.Template),
				Tag:     "plain_text",
			},
		}
		cardElements := []models.Elements{
			{
				Tag:            "column_set",
				FlexMode:       "none",
				BackgroupStyle: "default",
				Columns: []models.Columns{
					{
						Tag:           "column",
						Width:         "weighted",
						Weight:        1,
						VerticalAlign: "top",
						Elements: []models.ColumnsElements{
							{
								Tag: "div",
								Text: models.Texts{
									Content: ParserTemplate("Event", alert, noticeTmpl.Template),
									Tag:     "lark_md",
								},
							},
						},
					},
				},
			},
			{
				Tag: "hr",
			},
			{
				Tag: "note",
				Elements: []models.ElementsElements{
					{
						Tag:     "plain_text",
						Content: ParserTemplate("Footer", alert, noticeTmpl.Template),
					},
				},
			},
		}

		// 转换cardElements为map列表
		defaultTemplate.Card.Elements = tools.ConvertSliceToMapList(cardElements)

		// 添加快捷操作按钮（如果启用）
		actionButtonsMap := buildFeishuActionButtonsMap(alert)
		if actionButtonsMap != nil {
			defaultTemplate.Card.Elements = append(defaultTemplate.Card.Elements, actionButtonsMap)
		}

		defaultTemplate.Card.Header = tools.ConvertStructToMap(cardHeader)
		cardContentString = tools.JsonMarshalToString(defaultTemplate)

	}

	// 需要将所有换行符进行转义
	cardContentString = strings.Replace(cardContentString, "\n", "\\n", -1)

	return cardContentString

}

// buildFeishuActionButtonsMap 构建飞书快捷操作按钮(返回map格式)
// 由于Elements模型不包含Actions字段,直接返回map结构
func buildFeishuActionButtonsMap(alert models.AlertCurEvent) map[string]interface{} {
	// 获取快捷操作配置
	quickConfig := getQuickActionConfig()

	// 检查配置是否启用且必需字段齐全
	if !quickConfig.GetEnable() || quickConfig.BaseUrl == "" || quickConfig.SecretKey == "" {
		return nil
	}

	// 生成快捷操作Token(24小时有效期)
	token, err := utils.GenerateQuickToken(
		alert.TenantId,
		alert.Fingerprint,
		alert.DutyUser,
		quickConfig.SecretKey,
	)
	if err != nil {
		// Token生成失败,降级处理,不显示按钮
		return nil
	}

	// 确定API调用地址(优先使用ApiUrl,否则使用BaseUrl)
	apiUrl := quickConfig.ApiUrl
	if apiUrl == "" {
		apiUrl = quickConfig.BaseUrl
	}

	// 构建按钮数组
	buttons := []map[string]interface{}{
		// 认领告警按钮
		{
			"tag":  "button",
			"type": "primary",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "🔔 认领告警",
			},
			"url": fmt.Sprintf("%s/api/v1/alert/quick-action?action=claim&fingerprint=%s&token=%s",
				apiUrl, alert.Fingerprint, token),
		},
		// 静默告警按钮(默认1小时,保持兼容)
		{
			"tag":  "button",
			"type": "default",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "🔕 静默告警",
			},
			"url": fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=1h",
				apiUrl, alert.Fingerprint, token),
		},
		// 静默1小时
		{
			"tag":  "button",
			"type": "default",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "🕐 静默1小时",
			},
			"url": fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=1h",
				apiUrl, alert.Fingerprint, token),
		},
		// 静默6小时
		{
			"tag":  "button",
			"type": "default",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "🕕 静默6小时",
			},
			"url": fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=6h",
				apiUrl, alert.Fingerprint, token),
		},
		// 静默24小时
		{
			"tag":  "button",
			"type": "default",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "🕙 静默24小时",
			},
			"url": fmt.Sprintf("%s/api/v1/alert/quick-action?action=silence&fingerprint=%s&token=%s&duration=24h",
				apiUrl, alert.Fingerprint, token),
		},
		// 自定义静默(跳转到自定义页面)
		{
			"tag":  "button",
			"type": "default",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "⚙️ 自定义静默",
			},
			"url": fmt.Sprintf("%s/api/v1/alert/quick-silence?fingerprint=%s&token=%s",
				apiUrl, alert.Fingerprint, token),
		},
		// 查看详情按钮
		{
			"tag":  "button",
			"type": "default",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": "📊 查看详情",
			},
			"url": buildDetailUrl(alert, quickConfig.BaseUrl),
		},
	}

	// 返回action元素的map结构
	return map[string]interface{}{
		"tag":     "action",
		"actions": buttons,
	}
}

// buildDetailUrl 构建详情页URL
// 如果有FaultCenterId,跳转到故障中心详情页
// 否则跳转到对应的监控规则列表页
func buildDetailUrl(alert models.AlertCurEvent, baseUrl string) string {
	if alert.FaultCenterId != "" {
		return fmt.Sprintf("%s/faultCenter/detail/%s", baseUrl, alert.FaultCenterId)
	}
	// Probing事件没有FaultCenterId,跳转到拨测规则列表
	return fmt.Sprintf("%s/probing", baseUrl)
}
