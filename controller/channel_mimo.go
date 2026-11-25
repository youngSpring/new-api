package controller

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/shopspring/decimal"
)

func validateMiMoKey(channelType int, key string) error {
	switch channelType {
	case constant.ChannelTypeXiaomiMiMoPlan:
		if suffix, ok := strings.CutPrefix(key, "tp-"); ok && suffix != "" {
			return nil
		}
		if suffix, ok := strings.CutPrefix(key, "ttp-"); ok && suffix != "" {
			return nil
		}
		return common.NewMessage("MiMo Token Plan requires a tp- or ttp- API key")
	case constant.ChannelTypeXiaomiMiMoAPI:
		if suffix, ok := strings.CutPrefix(key, "sk-"); ok && suffix != "" {
			return nil
		}
		return common.NewMessage("MiMo Pay-as-you-go API requires an sk- API key")
	}
	return nil
}

func fetchXiaomiMiMoBalance(channel *model.Channel) (*model.ChannelBalanceDetail, error) {
	key := strings.TrimSpace(channel.Key)
	if err := validateMiMoKey(channel.Type, key); err != nil {
		return nil, err
	}
	endpoint := "balance"
	if channel.Type == constant.ChannelTypeXiaomiMiMoPlan {
		if !strings.HasPrefix(key, "tp-") {
			return nil, common.NewMessage("MiMo Token Plan balance requires a personal tp- API key")
		}
		endpoint = "token_plan"
	}
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(channel.Type)
	}
	upstream, err := url.Parse(baseURL)
	if err != nil || upstream.Scheme != "https" || upstream.Hostname() == "" || upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" {
		return nil, common.NewMessage("MiMo balance queries require an HTTPS base URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	body, err := fetchChannelBalanceBody(ctx, channel, upstream.JoinPath("v1", "platform", endpoint).String())
	if err != nil {
		return nil, err
	}
	if channel.Type == constant.ChannelTypeXiaomiMiMoPlan {
		return parseXiaomiMiMoPlanBalance(body)
	}
	return parseXiaomiMiMoAPIBalance(body)
}

func parseXiaomiMiMoPlanBalance(body []byte) (*model.ChannelBalanceDetail, error) {
	invalid := common.NewMessage("Invalid MiMo balance response")
	var response struct {
		Period struct {
			EndAt string `json:"end_at"`
		} `json:"period"`
		Quota struct {
			Total     dto.StringValue  `json:"total_credits"`
			Used      dto.StringValue  `json:"used_credits"`
			Remaining *dto.StringValue `json:"remaining_credits"`
		} `json:"quota"`
	}
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, invalid
	}
	total, err := strconv.ParseInt(string(response.Quota.Total), 10, 64)
	if err != nil || total < 0 || total > 1<<53-1 {
		return nil, invalid
	}
	used, err := strconv.ParseInt(string(response.Quota.Used), 10, 64)
	if err != nil || used < 0 || used > 1<<53-1 {
		return nil, invalid
	}
	remaining := max(0, total-used)
	if response.Quota.Remaining != nil {
		reported, err := strconv.ParseInt(string(*response.Quota.Remaining), 10, 64)
		if err != nil || reported != remaining {
			return nil, invalid
		}
	}
	window := jsplugin.BalanceWindow{
		Name: jsplugin.LocalizedText{"en": "Token Plan"},
		Used: float64(used), Limit: float64(total), Unit: "Credits",
	}
	if response.Period.EndAt != "" {
		end, err := time.Parse(time.DateTime, response.Period.EndAt)
		if err != nil {
			return nil, invalid
		}
		window.ResetAt = end.Unix()
	}
	amount := float64(remaining)
	return &model.ChannelBalanceDetail{BalanceResult: jsplugin.BalanceResult{
		Amount: &amount, Currency: "Credits", Windows: []jsplugin.BalanceWindow{window},
	}}, nil
}

func parseXiaomiMiMoAPIBalance(body []byte) (*model.ChannelBalanceDetail, error) {
	invalid := common.NewMessage("Invalid MiMo balance response")
	var response struct {
		BalanceInfos []struct {
			Currency       string `json:"currency"`
			TotalAvailable string `json:"total_available"`
			FrozenBalance  string `json:"frozen_balance"`
		} `json:"balance_infos"`
	}
	if err := common.Unmarshal(body, &response); err != nil || len(response.BalanceInfos) != 1 {
		return nil, invalid
	}
	balance := response.BalanceInfos[0]
	if len(balance.Currency) != 3 || strings.IndexFunc(balance.Currency, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0 {
		return nil, invalid
	}
	if len(balance.TotalAvailable) > 20 || len(balance.FrozenBalance) > 20 ||
		strings.ContainsAny(balance.TotalAvailable+balance.FrozenBalance, "eE") {
		return nil, invalid
	}
	total, err := decimal.NewFromString(balance.TotalAvailable)
	if err != nil {
		return nil, invalid
	}
	frozen, err := decimal.NewFromString(balance.FrozenBalance)
	if err != nil || frozen.IsNegative() {
		return nil, invalid
	}
	amount := total.Sub(frozen).InexactFloat64()
	return &model.ChannelBalanceDetail{BalanceResult: jsplugin.BalanceResult{
		Amount: &amount, Currency: balance.Currency,
	}}, nil
}
