package mute

import (
	"github.com/zeromicro/go-zero/core/logc"
	"regexp"
	"time"
	"watchAlert/internal/ctx"
	models "watchAlert/internal/models"
	"watchAlert/pkg/tools"
)

type MuteParams struct {
	EffectiveTime models.EffectiveTime
	RecoverNotify *bool
	IsRecovered   bool
	TenantId      string
	Labels        map[string]interface{}
	FaultCenterId string
}

func IsMuted(mute MuteParams) bool {
	if IsSilence(mute) {
		return true
	}

	if NotInTheEffectiveTime(mute) {
		return true
	}

	if RecoverNotify(mute) {
		return true
	}

	return false
}

// NotInTheEffectiveTime 判断生效时间
func NotInTheEffectiveTime(mp MuteParams) bool {
	if len(mp.EffectiveTime.Week) <= 0 {
		return false
	}

	// 获取当前日期
	currentTime := time.Now()
	currentWeekday := tools.TimeTransformToWeek(currentTime)
	for _, weekday := range mp.EffectiveTime.Week {
		if currentWeekday == weekday {
			currentTimeSeconds := tools.TimeTransformToSeconds(currentTime)
			return currentTimeSeconds < mp.EffectiveTime.StartTime || currentTimeSeconds > mp.EffectiveTime.EndTime
		}
	}

	return true
}

// RecoverNotify 判断是否推送恢复通知
func RecoverNotify(mp MuteParams) bool {
	return mp.IsRecovered && !*mp.RecoverNotify
}

// IsSilence 判断是否静默
func IsSilence(mute MuteParams) bool {
	return GetMatchedSilenceRule(mute) != nil
}

// GetMatchedSilenceRule 获取匹配的静默规则（如果有）
// 返回匹配的静默规则详情，如果没有匹配则返回nil
func GetMatchedSilenceRule(mute MuteParams) *models.AlertSilences {
	silenceCtx := ctx.Redis.Silence()
	// 获取静默列表中所有的id
	ids, err := silenceCtx.GetAlertMutes(mute.TenantId, mute.FaultCenterId)
	if err != nil {
		logc.Errorf(ctx.Ctx, err.Error())
		return nil
	}

	// 根据ID获取到详细的静默规则
	for _, id := range ids {
		muteRule, err := silenceCtx.WithIdGetMuteFromCache(mute.TenantId, mute.FaultCenterId, id)
		if err != nil {
			logc.Errorf(ctx.Ctx, err.Error())
			continue
		}

		if muteRule.Status != 1 {
			continue
		}

		if evalCondition(mute.Labels, muteRule.Labels) {
			return muteRule
		}
	}

	return nil
}

func evalCondition(metrics map[string]interface{}, muteLabels []models.SilenceLabel) bool {
	for _, muteLabel := range muteLabels {
		value, exists := metrics[muteLabel.Key]
		if !exists {
			return false
		}

		val, ok := value.(string)
		if !ok {
			continue
		}

		var matched bool
		switch muteLabel.Operator {
		case "==", "=":
			matched = regexp.MustCompile(muteLabel.Value).MatchString(val)
		case "!=":
			matched = !regexp.MustCompile(muteLabel.Value).MatchString(val)
		default:
			matched = false
		}

		if !matched {
			return false // 只要有一个不匹配，就不静默
		}
	}

	return true
}
