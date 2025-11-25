package services

import (
	"encoding/json"
	"fmt"
	"time"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
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
	q.ctx.Redis.Alert().PushAlertEvent(targetAlert)

	// 记录审计日志
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-认领告警", map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"operator":    username,
		"timestamp":   time.Now().Unix(),
	})

	return nil
}

// SilenceAlert 静默告警
// 创建静默规则，在指定时间内抑制该告警
func (q *quickActionService) SilenceAlert(tenantId, fingerprint, duration, username, clientIP string) error {
	return q.silenceAlert(tenantId, fingerprint, duration, username, "", clientIP)
}

// ResolveAlert 标记告警已处理
// 手动标记告警为已恢复状态
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
	q.ctx.Redis.Alert().PushAlertEvent(targetAlert)

	// 记录审计日志
	q.createAuditLog(tenantId, username, clientIP, "快捷操作-标记已处理", map[string]interface{}{
		"fingerprint": fingerprint,
		"ruleName":    targetAlert.RuleName,
		"operator":    username,
		"timestamp":   time.Now().Unix(),
	})

	return nil
}

// SilenceAlertWithReason 静默告警(带原因)
// 与SilenceAlert相比，此方法允许用户提供自定义的静默原因
func (q *quickActionService) SilenceAlertWithReason(tenantId, fingerprint, duration, username, reason, clientIP string) error {
	return q.silenceAlert(tenantId, fingerprint, duration, username, reason, clientIP)
}

// GetAlertByFingerprint 根据指纹获取告警
// 从Redis缓存中查找指定租户下匹配指纹的告警事件
func (q *quickActionService) GetAlertByFingerprint(tenantId, fingerprint string) (*models.AlertCurEvent, error) {
	// 获取租户下所有故障中心
	faultCenters, err := q.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil {
		return nil, fmt.Errorf("获取故障中心列表失败: %w", err)
	}

	// 遍历所有故障中心，查找匹配的告警
	for _, fc := range faultCenters {
		// 从缓存中获取当前故障中心的告警事件
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

	return nil, fmt.Errorf("未找到指纹为 %s 的告警", fingerprint)
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