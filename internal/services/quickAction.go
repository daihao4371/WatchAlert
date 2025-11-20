package services

import (
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
	ClaimAlert(tenantId, fingerprint, username string) error
	// SilenceAlert 静默告警
	SilenceAlert(tenantId, fingerprint, duration, username string) error
	// ResolveAlert 标记告警已处理
	ResolveAlert(tenantId, fingerprint, username string) error
}

func newInterQuickActionService(ctx *ctx.Context) InterQuickActionService {
	return &quickActionService{
		ctx: ctx,
	}
}

// ClaimAlert 认领告警
// 更新告警的认领状态，标记为已认领
func (q *quickActionService) ClaimAlert(tenantId, fingerprint, username string) error {
	// 获取租户下所有故障中心
	faultCenters, err := q.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil {
		return fmt.Errorf("获取故障中心列表失败: %w", err)
	}

	// 遍历所有故障中心，查找匹配的告警
	var targetAlert *models.AlertCurEvent
	for _, fc := range faultCenters {
		// 从缓存中获取当前故障中心的告警事件
		events, err := q.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, fc.ID))
		if err != nil {
			continue // 忽略错误，继续搜索下一个故障中心
		}

		// 查找匹配的告警
		for _, alert := range events {
			if alert.Fingerprint == fingerprint {
				targetAlert = alert
				break
			}
		}

		if targetAlert != nil {
			break // 找到了，退出外层循环
		}
	}

	if targetAlert == nil {
		return fmt.Errorf("未找到指纹为 %s 的告警", fingerprint)
	}

	// 检查是否已经被认领
	if targetAlert.ConfirmState.IsOk {
		return fmt.Errorf("告警已被 %s 认领", targetAlert.ConfirmState.ConfirmUsername)
	}

	// 更新认领状态
	targetAlert.ConfirmState.IsOk = true
	targetAlert.ConfirmState.ConfirmUsername = username
	targetAlert.ConfirmState.ConfirmActionTime = time.Now().Unix()

	// 推送更新后的告警到缓存（PushAlertEvent 没有返回值）
	q.ctx.Redis.Alert().PushAlertEvent(targetAlert)

	return nil
}

// SilenceAlert 静默告警
// 创建静默规则，在指定时间内抑制该告警
func (q *quickActionService) SilenceAlert(tenantId, fingerprint, duration, username string) error {
	// 获取租户下所有故障中心
	faultCenters, err := q.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil {
		return fmt.Errorf("获取故障中心列表失败: %w", err)
	}

	// 遍历所有故障中心，查找匹配的告警
	var targetAlert *models.AlertCurEvent
	for _, fc := range faultCenters {
		// 从缓存中获取当前故障中心的告警事件
		events, err := q.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, fc.ID))
		if err != nil {
			continue // 忽略错误，继续搜索下一个故障中心
		}

		// 查找匹配的告警
		for _, alert := range events {
			if alert.Fingerprint == fingerprint {
				targetAlert = alert
				break
			}
		}

		if targetAlert != nil {
			break // 找到了，退出外层循环
		}
	}

	if targetAlert == nil {
		return fmt.Errorf("未找到指纹为 %s 的告警", fingerprint)
	}

	// 解析静默时长
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return fmt.Errorf("无效的静默时长: %s", duration)
	}

	// 创建静默规则（使用 Labels 字段匹配指纹）
	silence := models.AlertSilences{
		TenantId: tenantId,
		ID:       tools.RandId(),
		Name:     fmt.Sprintf("快捷静默-%s", targetAlert.RuleName),
		Labels: []models.SilenceLabel{
			{
				Key:      "fingerprint",
				Value:    fingerprint,
				Operator: "=",
			},
		},
		Comment:       fmt.Sprintf("[快捷操作] 由 %s 静默 %s", username, duration),
		StartsAt:      time.Now().Unix(),
		EndsAt:        time.Now().Add(dur).Unix(),
		UpdateBy:      username,
		FaultCenterId: targetAlert.FaultCenterId,
	}

	// 保存静默规则到数据库
	err = q.ctx.DB.Silence().Create(silence)
	if err != nil {
		return fmt.Errorf("创建静默规则失败: %w", err)
	}

	return nil
}

// ResolveAlert 标记告警已处理
// 手动标记告警为已恢复状态
func (q *quickActionService) ResolveAlert(tenantId, fingerprint, username string) error {
	// 获取租户下所有故障中心
	faultCenters, err := q.ctx.DB.FaultCenter().List(tenantId, "")
	if err != nil {
		return fmt.Errorf("获取故障中心列表失败: %w", err)
	}

	// 遍历所有故障中心，查找匹配的告警
	var targetAlert *models.AlertCurEvent
	for _, fc := range faultCenters {
		// 从缓存中获取当前故障中心的告警事件
		events, err := q.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(tenantId, fc.ID))
		if err != nil {
			continue // 忽略错误，继续搜索下一个故障中心
		}

		// 查找匹配的告警
		for _, alert := range events {
			if alert.Fingerprint == fingerprint {
				targetAlert = alert
				break
			}
		}

		if targetAlert != nil {
			break // 找到了，退出外层循环
		}
	}

	if targetAlert == nil {
		return fmt.Errorf("未找到指纹为 %s 的告警", fingerprint)
	}

	// 检查告警是否已经恢复
	if targetAlert.IsRecovered {
		return fmt.Errorf("告警已经恢复")
	}

	// 标记为已恢复
	targetAlert.IsRecovered = true
	targetAlert.RecoverTime = time.Now().Unix()

	// 推送更新后的告警到缓存（PushAlertEvent 没有返回值）
	q.ctx.Redis.Alert().PushAlertEvent(targetAlert)

	return nil
}