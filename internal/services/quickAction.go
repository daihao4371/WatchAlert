package services

import (
	"encoding/json"
	"fmt"
	"time"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/pkg/sender"
	"watchAlert/pkg/tools"
)

type quickActionService struct {
	ctx *ctx.Context
}

type InterQuickActionService interface {
	// ClaimAlert 认领告警
	ClaimAlert(tenantId, fingerprint, username, clientIP string) error
	// SilenceAlert 静默告警
	SilenceAlert(tenantId, fingerprint, duration, username, clientIP string) error
	// SilenceAlertWithReason 静默告警(带原因)
	SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason, clientIP string) error
	// ResolveAlert 标记告警已处理
	ResolveAlert(tenantId, fingerprint, username, clientIP string) error
	// GetAlertByFingerprint 根据指纹获取告警
	GetAlertByFingerprint(tenantId, fingerprint string) (*models.AlertCurEvent, error)
}

func newInterQuickActionService(ctx *ctx.Context) InterQuickActionService {
	return &quickActionService{
		ctx: ctx,
	}
}

// ClaimAlert 认领告警
// 更新告警的认领状态，标记为已认领
// 支持普通告警和拨测告警
func (q *quickActionService) ClaimAlert(tenantId, fingerprint, username, clientIP string) error {
	// 获取目标告警
	targetAlert, err := q.GetAlertByFingerprint(tenantId, fingerprint)
	if err != nil {
		return err
	}

	// 检查是否已经被认领
	if targetAlert.ConfirmState.IsOk {
		return fmt.Errorf("告警已被 %s 认领", targetAlert.ConfirmState.ConfirmUsername)
	}

	// 更新认领状态
	targetAlert.ConfirmState.IsOk = true
	targetAlert.ConfirmState.ConfirmUsername = username
	targetAlert.ConfirmState.ConfirmActionTime = time.Now().Unix()

	// 推送更新后的告警到缓存
	// 注意: 拨测告警没有FaultCenterId,所以这里只更新普通告警
	if targetAlert.FaultCenterId != "" {
		q.ctx.Redis.Alert().PushAlertEvent(targetAlert)
	}
	// 拨测告警的认领状态暂不持久化到ProbingCache
	// 因为ProbingCache设计上不包含ConfirmState字段

	// 记录审计日志
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-认领告警", map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"operator":    username,
		"timestamp":   time.Now().Unix(),
	})

	// 发送确认消息到群聊(异步，失败不影响主流程)
	go func() {
		if err := q.sendConfirmationMessage(targetAlert, "claim", username); err != nil {
			fmt.Printf("发送确认消息失败: %v\n", err)
		}
	}()

	return nil
}

// SilenceAlert 静默告警
// 创建静默规则，在指定时间内抑制该告警
func (q *quickActionService) SilenceAlert(tenantId, fingerprint, duration, username, clientIP string) error {
	return q.silenceAlert(tenantId, fingerprint, duration, username, "", clientIP)
}

// ResolveAlert 标记告警已处理
// 手动标记告警为已恢复状态
// 支持普通告警和拨测告警
func (q *quickActionService) ResolveAlert(tenantId, fingerprint, username, clientIP string) error {
	// 获取目标告警
	targetAlert, err := q.GetAlertByFingerprint(tenantId, fingerprint)
	if err != nil {
		return err
	}

	// 检查告警是否已经恢复
	if targetAlert.IsRecovered {
		return fmt.Errorf("告警已经恢复")
	}

	// 标记为已恢复
	targetAlert.IsRecovered = true
	targetAlert.RecoverTime = time.Now().Unix()

	// 推送更新后的告警到缓存
	// 对于普通告警,更新AlertCache
	if targetAlert.FaultCenterId != "" {
		q.ctx.Redis.Alert().PushAlertEvent(targetAlert)
	} else {
		// 对于拨测告警,需要更新ProbingCache
		err := q.updateProbingEventRecovery(tenantId, targetAlert.RuleId, fingerprint)
		if err != nil {
			return fmt.Errorf("更新拨测告警恢复状态失败: %w", err)
		}
	}

	// 记录审计日志
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-标记已处理", map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"operator":    username,
		"timestamp":   time.Now().Unix(),
	})

	// 发送确认消息到群聊(异步，失败不影响主流程)
	go func() {
		if err := q.sendConfirmationMessage(targetAlert, "resolve", username); err != nil {
			fmt.Printf("发送确认消息失败: %v\n", err)
		}
	}()

	return nil
}

// updateProbingEventRecovery 更新拨测事件的恢复状态
// 从缓存中读取拨测事件,更新恢复状态后写回
func (q *quickActionService) updateProbingEventRecovery(tenantId, ruleId, fingerprint string) error {
	cacheKey := models.BuildProbingEventCacheKey(tenantId, ruleId)

	// 获取拨测事件
	probingEvent, err := q.ctx.Redis.Probing().GetProbingEventCache(cacheKey)
	if err != nil {
		return err
	}

	// 验证指纹匹配
	if probingEvent.Fingerprint != fingerprint {
		return fmt.Errorf("指纹不匹配")
	}

	// 更新恢复状态
	probingEvent.IsRecovered = true
	probingEvent.RecoverTime = time.Now().Unix()
	probingEvent.LastSendTime = 0 // 重置发送时间,触发恢复通知

	// 写回缓存
	q.ctx.Redis.Probing().SetProbingEventCache(*probingEvent, 0)

	return nil
}

// SilenceAlertWithReason 静默告警(带原因)
// 与SilenceAlert相比，此方法允许用户提供自定义的静默原因
func (q *quickActionService) SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason, clientIP string) error {
	return q.silenceAlert(tenantId, fingerprint, duration, username, reason, clientIP)
}

// GetAlertByFingerprint 根据指纹获取告警
// 从Redis缓存中查找指定租户下匹配指纹的告警事件
// 支持查找普通告警(AlertCache)和拨测告警(ProbingCache)
func (q *quickActionService) GetAlertByFingerprint(tenantId, fingerprint string) (*models.AlertCurEvent, error) {
	// 1. 先在普通告警缓存(AlertCache)中查找
	faultCenters, err := q.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil {
		return nil, fmt.Errorf("获取故障中心列表失败: %w", err)
	}

	// 遍历所有故障中心，查找匹配的告警
	for _, fc := range faultCenters {
		// 从AlertCache中获取当前故障中心的告警事件
		events, err := q.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, fc.ID))
		if err != nil {
			continue // 忽略错误，继续搜索下一个故障中心
		}

		// 查找匹配的告警
		for _, alert := range events {
			if alert.Fingerprint == fingerprint {
				return alert, nil
			}
		}
	}

	// 2. 如果在普通告警中没找到，尝试从拨测告警缓存(ProbingCache)中查找
	probingAlert, err := q.findProbingAlertByFingerprint(tenantId, fingerprint)
	if err == nil && probingAlert != nil {
		return probingAlert, nil
	}

	return nil, fmt.Errorf("未找到指纹为 %s 的告警 或者告警失效了", fingerprint)
}

// findProbingAlertByFingerprint 从拨测告警缓存中查找指定指纹的告警
// 遍历所有拨测规则的缓存，找到匹配的拨测事件并转换为标准告警格式
func (q *quickActionService) findProbingAlertByFingerprint(tenantId, fingerprint string) (*models.AlertCurEvent, error) {
	// 获取租户下所有启用的拨测规则
	var probingRules []models.ProbingRule
	err := q.ctx.DB.DB().Where("tenant_id = ? AND enabled = ?", tenantId, true).Find(&probingRules).Error
	if err != nil {
		return nil, err
	}

	// 遍历每个拨测规则，查找匹配的告警
	for _, rule := range probingRules {
		// 构建拨测事件缓存key
		cacheKey := models.BuildProbingEventCacheKey(rule.TenantId, rule.RuleId)

		// 从ProbingCache获取拨测事件
		probingEvent, err := q.ctx.Redis.Probing().GetProbingEventCache(cacheKey)
		if err != nil {
			continue // 忽略错误，继续下一个规则
		}

		// 检查指纹是否匹配
		if probingEvent.Fingerprint == fingerprint {
			// 将ProbingEvent转换为AlertCurEvent
			alertEvent := q.convertProbingEventToAlertEvent(probingEvent)
			return &alertEvent, nil
		}
	}

	return nil, fmt.Errorf("未在拨测告警中找到指纹: %s", fingerprint)
}

// convertProbingEventToAlertEvent 将拨测事件转换为标准告警事件
// 确保拨测告警也能被快捷操作正确处理
func (q *quickActionService) convertProbingEventToAlertEvent(probingEvent *models.ProbingEvent) models.AlertCurEvent {
	return models.AlertCurEvent{
		TenantId:               probingEvent.TenantId,
		RuleName:               probingEvent.RuleName,
		RuleId:                 probingEvent.RuleId,
		Fingerprint:            probingEvent.Fingerprint,
		Labels:                 probingEvent.Labels,
		Annotations:            probingEvent.Annotations,
		IsRecovered:            probingEvent.IsRecovered,
		FirstTriggerTime:       probingEvent.FirstTriggerTime,
		FirstTriggerTimeFormat: probingEvent.FirstTriggerTimeFormat,
		RepeatNoticeInterval:   probingEvent.RepeatNoticeInterval,
		LastEvalTime:           probingEvent.LastEvalTime,
		LastSendTime:           probingEvent.LastSendTime,
		RecoverTime:            probingEvent.RecoverTime,
		RecoverTimeFormat:      probingEvent.RecoverTimeFormat,
		DutyUser:               probingEvent.DutyUser,
		// 注意: Probing告警没有FaultCenterId,ConfirmState等字段
		// 这些字段保持默认值
	}
}

// ------------------------ 私有辅助方法 ------------------------

// silenceAlert 静默告警的内部实现（避免代码重复）
// 参数reason为空时，使用默认注释；否则追加自定义原因
func (q *quickActionService) silenceAlert(tenantId, fingerprint, duration, username, reason, clientIP string) error {
	// 获取告警信息
	targetAlert, err := q.GetAlertByFingerprint(tenantId, fingerprint)
	if err != nil {
		return err
	}

	// 检查是否已经存在该指纹的激活静默规则(防止重复静默)
	existingSilence, err := q.findActiveSilenceByFingerprint(tenantId, fingerprint)
	if err == nil && existingSilence != nil {
		// 计算剩余静默时间
		remainingTime := existingSilence.EndsAt - time.Now().Unix()
		if remainingTime > 0 {
			remainingDuration := time.Duration(remainingTime) * time.Second
			return fmt.Errorf("该告警已处于静默状态,剩余时长: %s", q.formatDurationChinese(remainingDuration.String()))
		}
	}

	// 解析静默时长
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return fmt.Errorf("无效的静默时长: %s", duration)
	}

	// 构建静默注释（根据是否有自定义原因）
	comment := fmt.Sprintf("[快捷操作] 由 %s 静默 %s", username, duration)
	if reason != "" {
		comment = fmt.Sprintf("%s\n原因: %s", comment, reason)
	}

	// 创建静默规则
	silence := models.AlertSilences{
		TenantId: tenantId,
		ID:       "s-" + tools.RandId(),
		Name:     fmt.Sprintf("快捷静默-%s", targetAlert.RuleName),
		Labels: []models.SilenceLabel{
			{
				Key:      "fingerprint",
				Value:    fingerprint,
				Operator: "=",
			},
		},
		Comment:       comment,
		StartsAt:      time.Now().Unix(),
		EndsAt:        time.Now().Add(dur).Unix(),
		UpdateAt:      time.Now().Unix(),
		UpdateBy:      username,
		FaultCenterId: targetAlert.FaultCenterId,
		Status:        1, // 状态设置为启用
	}

	// 先推送到Redis缓存，使静默规则立即生效
	q.ctx.Redis.Silence().PushAlertMute(silence)

	// 再保存到数据库进行持久化
	err = q.ctx.DB.Silence().Create(silence)
	if err != nil {
		return fmt.Errorf("创建静默规则失败: %w", err)
	}

	// 记录审计日志
	auditData := map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"duration":    duration,
		"operator":    username,
		"silenceId":   silence.ID,
		"timestamp":   time.Now().Unix(),
	}
	if reason != "" {
		auditData["reason"] = reason
	}
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-静默告警", auditData)

	// 发送确认消息到群聊(异步，失败不影响主流程)
	go func() {
		if err := q.sendConfirmationMessage(targetAlert, "silence", username, duration); err != nil {
			fmt.Printf("发送确认消息失败: %v\n", err)
		}
	}()

	return nil
}

// createAuditLog 创建审计日志（通用方法，避免代码重复）
// 将操作详情记录到审计日志表中，用于追踪和审计
func (q *quickActionService) createAuditLog(tenantId, username, clientIP, auditType string, data map[string]interface{}) {
	// 将数据序列化为JSON字符串
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		// 序列化失败时，记录原始错误信息而非终止操作
		bodyBytes = []byte(fmt.Sprintf("{\"error\": \"序列化失败: %s\"}", err.Error()))
	}

	// 构建审计日志记录
	auditLog := models.AuditLog{
		TenantId:   tenantId,
		ID:         "Trace" + tools.RandId(),
		Username:   username,
		IPAddress:  clientIP,
		Method:     "QUICK_ACTION", // 标识为快捷操作
		Path:       "/api/v1/alert/quick-action",
		CreatedAt:  time.Now().Unix(),
		StatusCode: 200,
		Body:       string(bodyBytes),
		AuditType:  auditType,
	}

	// 异步写入审计日志（失败不影响主流程）
	go func() {
		if err := q.ctx.DB.AuditLog().Create(auditLog); err != nil {
			// 审计日志写入失败，仅打印错误，不中断业务流程
			fmt.Printf("审计日志写入失败: %v\n", err)
		}
	}()
}

// getWebhookFromAlert 从告警事件中反查Webhook配置
// 通过告警的FaultCenterId获取故障中心，再从NoticeIds中获取通知对象的Webhook信息
// 返回值: hook(Webhook地址), sign(签名), noticeType(通知类型: feishu/dingtalk), error
func (q *quickActionService) getWebhookFromAlert(alert *models.AlertCurEvent) (string, string, string, error) {
	// 1. 获取故障中心信息（包含通知对象ID列表）
	var faultCenter models.FaultCenter
	var err error

	if alert.FaultCenterId != "" {
		// 普通告警：通过FaultCenterId获取故障中心（name参数为空字符串）
		faultCenter, err = q.ctx.DB.FaultCenter().Get(alert.TenantId, alert.FaultCenterId, "")
		if err != nil {
			return "", "", "", fmt.Errorf("获取故障中心失败: %w", err)
		}
	} else {
		// 拨测告警：通过RuleId查找拨测规则
		return q.getWebhookFromProbingRule(alert)
	}

	// 2. 遍历故障中心的通知对象ID，查找飞书通知
	for _, noticeId := range faultCenter.NoticeIds {
		noticeObj, err := q.ctx.DB.Notice().Get(alert.TenantId, noticeId)
		if err != nil {
			continue // 跳过获取失败的通知对象
		}

		// 检查是否为飞书通知
		if noticeObj.NoticeType == "FeiShu" {
			// 返回Webhook配置（DefaultHook优先，如果为空则查找Routes）
			hook, sign := q.extractWebhookFromNotice(&noticeObj, alert)
			if hook != "" {
				return hook, sign, "feishu", nil
			}
		}
	}

	return "", "", "", fmt.Errorf("未找到飞书通知配置")
}

// getWebhookFromProbingRule 从拨测规则中获取Webhook配置
// 拨测规则直接包含NoticeId字段
func (q *quickActionService) getWebhookFromProbingRule(alert *models.AlertCurEvent) (string, string, string, error) {
	// 查询拨测规则
	var probingRule models.ProbingRule
	err := q.ctx.DB.DB().
		Where("tenant_id = ? AND rule_id = ?", alert.TenantId, alert.RuleId).
		First(&probingRule).Error
	if err != nil {
		return "", "", "", fmt.Errorf("获取拨测规则失败: %w", err)
	}

	// 获取通知对象
	noticeObj, err := q.ctx.DB.Notice().Get(alert.TenantId, probingRule.NoticeId)
	if err != nil {
		return "", "", "", fmt.Errorf("获取通知对象失败: %w", err)
	}

	// 检查是否为飞书通知
	if noticeObj.NoticeType != "FeiShu" {
		return "", "", "", fmt.Errorf("不是飞书通知类型")
	}

	// 提取Webhook配置
	hook, sign := q.extractWebhookFromNotice(&noticeObj, alert)
	if hook == "" {
		return "", "", "", fmt.Errorf("未找到有效的Webhook配置")
	}

	return hook, sign, "feishu", nil
}

// extractWebhookFromNotice 从通知对象中提取Webhook配置
// 优先使用DefaultHook，如果为空则根据告警等级从Routes中查找
func (q *quickActionService) extractWebhookFromNotice(notice *models.AlertNotice, alert *models.AlertCurEvent) (string, string) {
	// 优先使用默认Webhook
	if notice.DefaultHook != "" {
		return notice.DefaultHook, notice.DefaultSign
	}

	// 如果没有默认Webhook，从Routes中根据告警等级查找
	for _, route := range notice.Routes {
		if route.Severity == alert.Severity {
			return route.Hook, route.Sign
		}
	}

	// 如果没有匹配的等级，尝试使用第一个Route
	if len(notice.Routes) > 0 {
		return notice.Routes[0].Hook, notice.Routes[0].Sign
	}

	return "", ""
}

// buildConfirmationMessage 构建确认消息内容（飞书卡片格式）
// 用于告知群成员快捷操作的执行结果
// 注意: 确认消息不包含操作按钮,避免用户重复操作
// duration是可选参数,用于静默操作时显示具体时长
func (q *quickActionService) buildConfirmationMessage(
	alert *models.AlertCurEvent,
	actionType, username string,
	duration ...string, // 可选参数，用于静默时传递时长
) string {
	// 根据操作类型生成操作描述和图标
	var actionDesc, actionIcon, headerColor, noteText string
	switch actionType {
	case "claim":
		actionDesc = "认领"
		actionIcon = "🔔"
		headerColor = "blue"
		noteText = "该告警已被认领,后续操作将由认领人负责"
	case "silence":
		// 如果提供了duration参数,显示具体静默时长
		if len(duration) > 0 && duration[0] != "" {
			actionDesc = fmt.Sprintf("静默 %s", q.formatDurationChinese(duration[0]))
		} else {
			actionDesc = "静默"
		}
		actionIcon = "🔕"
		headerColor = "orange"
		noteText = "告警已静默,在静默期间不会再次发送通知"
	case "resolve":
		actionDesc = "标记已处理"
		actionIcon = "✅"
		headerColor = "green"
		noteText = "该告警已标记为已处理状态"
	default:
		actionDesc = actionType
		actionIcon = "ℹ️"
		headerColor = "grey"
		noteText = "操作已完成"
	}

	// 构建飞书交互式卡片
	card := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"template": headerColor,
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": fmt.Sprintf("%s 告警快捷操作通知", actionIcon),
				},
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"fields": []map[string]interface{}{
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**告警名称**\n%s", alert.RuleName),
							},
						},
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**操作类型**\n%s", actionDesc),
							},
						},
					},
				},
				{
					"tag": "div",
					"fields": []map[string]interface{}{
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**操作人**\n%s", username),
							},
						},
						{
							"is_short": true,
							"text": map[string]interface{}{
								"tag":     "lark_md",
								"content": fmt.Sprintf("**操作时间**\n%s", time.Now().Format("2006-01-02 15:04:05")),
							},
						},
					},
				},
				{
					"tag": "hr",
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("💡 **提示**: %s", noteText),
					},
				},
				{
					"tag": "note",
					"elements": []map[string]interface{}{
						{
							"tag":     "plain_text",
							"content": "此消息由 WatchAlert 告警系统自动发送 | 原告警按钮已失效",
						},
					},
				},
			},
		},
	}

	return tools.JsonMarshalToString(card)
}

// sendConfirmationMessage 发送确认消息到群聊
// 操作成功后自动发送一条新消息，告知群成员操作结果
// duration参数是可选的，仅在静默操作时需要传递
func (q *quickActionService) sendConfirmationMessage(
	alert *models.AlertCurEvent,
	actionType, username string,
	duration ...string, // 可选参数，用于静默时传递时长
) error {
	// 1. 获取Webhook信息
	hook, sign, noticeType, err := q.getWebhookFromAlert(alert)
	if err != nil {
		return fmt.Errorf("无法发送确认消息: %w", err)
	}

	// 目前仅支持飞书
	if noticeType != "feishu" {
		return fmt.Errorf("不支持的通知类型: %s", noticeType)
	}

	// 2. 构建确认消息内容(传递duration参数)
	message := q.buildConfirmationMessage(alert, actionType, username, duration...)

	// 3. 解析消息为map结构
	msg := make(map[string]interface{})
	if err := json.Unmarshal([]byte(message), &msg); err != nil {
		return fmt.Errorf("消息解析失败: %w", err)
	}

	// 4. 调用飞书发送器发送消息
	feishuSender := sender.NewFeiShuSender()
	params := sender.SendParams{
		Hook:    hook,
		Sign:    sign,
		Content: message,
	}
	return feishuSender.Send(params)
}

// findActiveSilenceByFingerprint 查找指定指纹的激活静默规则
// 用于防止重复静默同一个告警
func (q *quickActionService) findActiveSilenceByFingerprint(tenantId, fingerprint string) (*models.AlertSilences, error) {
	// 查询数据库中的所有静默规则
	var silences []models.AlertSilences
	err := q.ctx.DB.DB().
		Where("tenant_id = ? AND status = ?", tenantId, 1). // status=1 表示启用状态
		Find(&silences).Error
	if err != nil {
		return nil, err
	}

	// 当前时间戳
	now := time.Now().Unix()

	// 遍历静默规则,查找匹配指纹且仍在有效期内的规则
	for _, silence := range silences {
		// 检查静默规则是否已过期
		if silence.EndsAt <= now {
			continue
		}

		// 检查静默规则的标签是否匹配该指纹
		for _, label := range silence.Labels {
			if label.Key == "fingerprint" && label.Value == fingerprint && label.Operator == "=" {
				return &silence, nil
			}
		}
	}

	return nil, fmt.Errorf("未找到激活的静默规则")
}

// formatDurationChinese 将Go的duration格式(如"1h"、"6h"、"24h")转换为中文友好格式
// 支持的输入格式: "1h" -> "1小时", "30m" -> "30分钟", "24h" -> "24小时"
func (q *quickActionService) formatDurationChinese(durationStr string) string {
	// 解析duration字符串
	dur, err := time.ParseDuration(durationStr)
	if err != nil {
		return durationStr // 解析失败,返回原始字符串
	}

	// 转换为秒数
	totalSeconds := int64(dur.Seconds())

	// 计算各个时间单位
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60

	// 构建中文格式
	var result string
	if days > 0 {
		result = fmt.Sprintf("%d天", days)
		if hours > 0 {
			result += fmt.Sprintf("%d小时", hours)
		}
	} else if hours > 0 {
		result = fmt.Sprintf("%d小时", hours)
		if minutes > 0 {
			result += fmt.Sprintf("%d分钟", minutes)
		}
	} else if minutes > 0 {
		result = fmt.Sprintf("%d分钟", minutes)
	} else {
		result = fmt.Sprintf("%d秒", totalSeconds)
	}

	return result
}