package controller

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
)

func miniMaxPlanBalanceURLs(baseURL string) []string {
	upstream, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil
	}
	switch strings.ToLower(upstream.Hostname()) {
	case "minimax.io", "api.minimax.io", "www.minimax.io", "platform.minimax.io":
		return []string{
			"https://www.minimax.io/v1/token_plan/remains",
			"https://api.minimax.io/v1/api/openplatform/coding_plan/remains",
		}
	case "", "api.minimax.chat", "api.minimaxi.com", "www.minimaxi.com", "platform.minimaxi.com",
		"minimax.cn", "api.minimax.cn", "www.minimax.cn", "platform.minimax.cn":
		return []string{
			"https://www.minimax.cn/v1/token_plan/remains",
			"https://www.minimaxi.com/v1/token_plan/remains",
			"https://api.minimaxi.com/v1/api/openplatform/coding_plan/remains",
		}
	default:
		return nil
	}
}

func fetchMiniMaxPlanBalance(channel *model.Channel) (*model.ChannelBalanceDetail, error) {
	endpoints := miniMaxPlanBalanceURLs(channel.GetBaseURL())
	if len(endpoints) == 0 {
		return nil, common.NewMessage("MiniMax Token Plan quota requires an official MiniMax base URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	models := channel.GetModels()
	var mapping map[string]string
	_ = common.UnmarshalJsonStr(channel.GetModelMapping(), &mapping)
	for i, name := range models {
		if upstream := mapping[name]; upstream != "" {
			models[i] = upstream
		}
	}
	var lastErr error
	for _, endpoint := range endpoints {
		body, err := fetchChannelBalanceBody(ctx, channel, endpoint)
		if err == nil {
			var detail *model.ChannelBalanceDetail
			detail, err = parseMiniMaxPlanBalance(body, models, strings.HasSuffix(endpoint, "/token_plan/remains"))
			if err == nil {
				return detail, nil
			}
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, errors.Join(common.NewMessage("Could not query MiniMax Token Plan quota"), lastErr)
}

func parseMiniMaxPlanBalance(body []byte, models []string, allowCountFallback bool) (*model.ChannelBalanceDetail, error) {
	var response struct {
		Base struct {
			Code int `json:"status_code"`
		} `json:"base_resp"`
		Remains []struct {
			Model          string   `json:"model_name"`
			IntervalLeft   *float64 `json:"current_interval_remaining_percent"`
			IntervalTotal  int64    `json:"current_interval_total_count"`
			IntervalCount  *int64   `json:"current_interval_usage_count"`
			IntervalStatus int      `json:"current_interval_status"`
			IntervalEnd    int64    `json:"end_time"`
			WeeklyLeft     *float64 `json:"current_weekly_remaining_percent"`
			WeeklyTotal    int64    `json:"current_weekly_total_count"`
			WeeklyCount    *int64   `json:"current_weekly_usage_count"`
			WeeklyStatus   int      `json:"current_weekly_status"`
			WeeklyEnd      int64    `json:"weekly_end_time"`
		} `json:"model_remains"`
	}
	invalid := common.NewMessage("MiniMax did not return usable Token Plan quota")
	if err := common.Unmarshal(body, &response); err != nil || response.Base.Code != 0 {
		return nil, invalid
	}
	selected := make(map[int]bool)
	for _, name := range models {
		name = strings.ToLower(strings.TrimSpace(name))
		best, bestScore := -1, 0
		for i, item := range response.Remains {
			bucket := strings.ToLower(strings.TrimSpace(item.Model))
			score := 0
			switch {
			case bucket == name && name != "":
				score = 3
			case strings.HasSuffix(bucket, "*") && strings.HasPrefix(name, strings.TrimSuffix(bucket, "*")):
				score = 2
			case bucket == "general":
				score = 1
			}
			if score > bestScore {
				best, bestScore = i, score
			}
		}
		if best >= 0 {
			selected[best] = true
		}
	}
	var windows []jsplugin.BalanceWindow
	for i, item := range response.Remains {
		if len(selected) > 0 && !selected[i] {
			continue
		}
		model := strings.TrimSpace(item.Model)
		if model == "" {
			continue
		}
		for _, window := range []struct {
			labels jsplugin.LocalizedText
			left   *float64
			total  int64
			count  *int64
			status int
			reset  int64
		}{
			{jsplugin.LocalizedText{"en": "5-hour quota", "zh": "5 小时额度", "zh-TW": "5 小時額度", "fr": "Quota sur 5 h", "ja": "5 時間枠", "ru": "Лимит на 5 часов", "vi": "Hạn mức 5 giờ"},
				item.IntervalLeft, item.IntervalTotal, item.IntervalCount, item.IntervalStatus, item.IntervalEnd},
			{jsplugin.LocalizedText{"en": "Weekly quota", "zh": "每周额度", "zh-TW": "每週額度", "fr": "Quota hebdomadaire", "ja": "週間枠", "ru": "Недельный лимит", "vi": "Hạn mức tuần"},
				item.WeeklyLeft, item.WeeklyTotal, item.WeeklyCount, item.WeeklyStatus, item.WeeklyEnd},
		} {
			if window.status == 3 {
				continue
			}
			var left float64
			switch {
			case window.status == 2:
				left = 0
			case window.left != nil && *window.left >= 0 && *window.left <= 100:
				left = *window.left
			case window.left == nil && allowCountFallback && window.count != nil && window.total > 0 && *window.count >= 0 && *window.count <= window.total:
				left = float64(*window.count) / float64(window.total) * 100
			default:
				continue
			}
			if !strings.EqualFold(model, "general") {
				for locale, label := range window.labels {
					window.labels[locale] = model + " · " + label
				}
			}
			resetAt := window.reset
			if resetAt >= 1e12 {
				resetAt /= 1000
			}
			windows = append(windows, jsplugin.BalanceWindow{
				Name: window.labels, Used: 100 - left, Limit: 100, Unit: "%", ResetAt: max(0, resetAt),
			})
		}
	}
	if len(windows) == 0 {
		return nil, invalid
	}
	return &model.ChannelBalanceDetail{BalanceResult: jsplugin.BalanceResult{Windows: windows}}, nil
}
