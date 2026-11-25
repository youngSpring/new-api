package controller

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDeepSeekBalancePreservesUpstreamCurrency(t *testing.T) {
	tests := []struct {
		name            string
		responseJSON    string
		usdExchangeRate float64
		want            float64
		wantCurrency    string
		wantAmount      float64
		wantErrContains string
	}{
		{
			name:            "prefers USD when USD precedes CNY",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"12.50"},{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 7.3,
			want:            12.5,
			wantCurrency:    "USD",
			wantAmount:      12.5,
		},
		{
			name:            "prefers USD when CNY precedes USD",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"},{"currency":"USD","total_balance":"12.50"}]}`,
			usdExchangeRate: 7.3,
			want:            12.5,
		},
		{
			name:            "converts CNY when USD is absent",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 7.3,
			want:            10,
			wantCurrency:    "CNY",
			wantAmount:      73,
		},
		{
			name:            "converts CNY when USD balance is zero",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"0.00"},{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 7.3,
			want:            10,
		},
		{
			name:            "converts CNY when zero USD follows CNY",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"},{"currency":"USD","total_balance":"0.00"}]}`,
			usdExchangeRate: 7.3,
			want:            10,
		},
		{
			name:            "returns zero when USD balance is zero and CNY is absent",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"0.00"}]}`,
			usdExchangeRate: 7.3,
			want:            0,
		},
		{
			name:            "returns error when USD and CNY are absent",
			responseJSON:    `{"balance_infos":[{"currency":"EUR","total_balance":"10.00"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "currency USD or CNY not found",
		},
		{
			name:            "returns USD parse error instead of falling back to CNY",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"invalid"},{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "invalid syntax",
		},
		{
			name:            "rejects NaN USD balance",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"NaN"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "USD balance must be finite",
		},
		{
			name:            "rejects negative USD balance",
			responseJSON:    `{"balance_infos":[{"currency":"USD","total_balance":"-1.00"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "USD balance must be non-negative",
		},
		{
			name:            "rejects positive infinity CNY balance",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"+Inf"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "CNY balance must be finite",
		},
		{
			name:            "rejects negative CNY balance",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"-7.30"}]}`,
			usdExchangeRate: 7.3,
			wantErrContains: "CNY balance must be non-negative",
		},
		{
			name:            "returns error for non-positive CNY exchange rate",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: 0,
			wantErrContains: "USD exchange rate must be greater than zero",
		},
		{
			name:            "rejects NaN CNY exchange rate",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: math.NaN(),
			wantErrContains: "USD exchange rate must be finite",
		},
		{
			name:            "rejects positive infinity CNY exchange rate",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`,
			usdExchangeRate: math.Inf(1),
			wantErrContains: "USD exchange rate must be finite",
		},
		{
			name:            "rejects CNY conversion overflow",
			responseJSON:    `{"balance_infos":[{"currency":"CNY","total_balance":"1.7976931348623157e+308"}]}`,
			usdExchangeRate: math.SmallestNonzeroFloat64,
			wantErrContains: "converted USD balance must be finite",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var response DeepSeekUsageResponse
			require.NoError(t, common.Unmarshal([]byte(test.responseJSON), &response))

			result, err := getDeepSeekBalance(response, test.usdExchangeRate)
			if test.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.InDelta(t, test.want, result.Balance, 1e-12)
			if test.wantCurrency != "" {
				require.NotNil(t, result.Detail)
				require.NotNil(t, result.Detail.Amount)
				assert.Equal(t, test.wantCurrency, result.Detail.Currency)
				assert.Equal(t, test.wantAmount, *result.Detail.Amount)
			}
		})
	}
}

func TestMiniMaxAndDeepSeekBalanceDetails(t *testing.T) {
	setupChannelCreateTest(t)
	t.Run("MiniMax uses percentages, matches model, and keeps its windows after reload", func(t *testing.T) {
		detail, err := parseMiniMaxPlanBalance([]byte(`{
			"base_resp":{"status_code":0},
			"model_remains":[
				{"model_name":"video","current_interval_remaining_percent":0},
				{"model_name":"general","current_interval_remaining_percent":74.5,"current_interval_total_count":0,"end_time":1795500000000,"current_weekly_status":2,"weekly_end_time":1795600000000}
			]}`), []string{"MiniMax-M2.7"}, true)
		require.NoError(t, err)
		require.Len(t, detail.Windows, 2)
		assert.Equal(t, 25.5, detail.Windows[0].Used)
		assert.Equal(t, int64(1795500000), detail.Windows[0].ResetAt)
		assert.Equal(t, 100.0, detail.Windows[1].Used)
		assert.Nil(t, detail.Amount)

		channel := &model.Channel{Type: constant.ChannelTypeMiniMax, Name: "minimax-plan", Balance: 42}
		require.NoError(t, model.DB.Create(channel).Error)
		t.Cleanup(func() { require.NoError(t, model.DB.Delete(channel).Error) })
		require.NotNil(t, recordBuiltinBalance(channel, channelBalanceResult{Detail: detail}, nil))
		stored, err := model.GetChannelById(channel.Id, true)
		require.NoError(t, err)
		assert.Equal(t, 42.0, stored.Balance)
		assert.Equal(t, detail, stored.GetBalanceDetail())
	})
	t.Run("DeepSeek retains its native wallet and the legacy USD balance", func(t *testing.T) {
		var response DeepSeekUsageResponse
		require.NoError(t, common.Unmarshal([]byte(`{"balance_infos":[{"currency":"CNY","total_balance":"73.00"}]}`), &response))
		result, err := getDeepSeekBalance(response, 7.3)
		require.NoError(t, err)
		channel := &model.Channel{Type: constant.ChannelTypeDeepSeek, Name: "deepseek-wallet", Balance: result.Balance}
		require.NoError(t, model.DB.Create(channel).Error)
		t.Cleanup(func() { require.NoError(t, model.DB.Delete(channel).Error) })
		require.NotNil(t, recordBuiltinBalance(channel, result, nil))
		stored, err := model.GetChannelById(channel.Id, true)
		require.NoError(t, err)
		assert.Equal(t, 10.0, stored.Balance)
		assert.Equal(t, result.Detail, stored.GetBalanceDetail())
	})
}

func TestMiniMaxQuotaUsesRegionAndDocumentedCounts(t *testing.T) {
	assert.Contains(t, miniMaxPlanBalanceURLs("https://api.minimax.cn/v1"), "https://www.minimaxi.com/v1/token_plan/remains")
	assert.Contains(t, miniMaxPlanBalanceURLs("https://api.minimax.io/anthropic"), "https://www.minimax.io/v1/token_plan/remains")
	assert.Empty(t, miniMaxPlanBalanceURLs("https://example.com"))
	body := []byte(`{"model_remains":[{"model_name":"general","current_interval_total_count":100,"current_interval_usage_count":20,"current_weekly_status":3,"current_weekly_remaining_percent":100}]}`)
	detail, err := parseMiniMaxPlanBalance(body, nil, true)
	require.NoError(t, err)
	require.Len(t, detail.Windows, 1)
	assert.Equal(t, 80.0, detail.Windows[0].Used)
	_, err = parseMiniMaxPlanBalance(body, nil, false)
	require.Error(t, err)
}

func TestUpdateChannelNewAPIBalanceUsesTheSmallerRemainingQuota(t *testing.T) {
	for _, tc := range []struct {
		name    string
		usage   string
		want    float64
		wantErr string
	}{
		{name: "user quota below the key quota", usage: `{"code":true,"data":{"total_available":2000000,"unlimited_quota":false,"user_quota":{"total_available":500000}}}`, want: 0.5},
		{name: "key quota below the user quota", usage: `{"code":true,"data":{"total_available":500000,"unlimited_quota":false,"user_quota":{"total_available":2000000}}}`, want: 0.5},
		{name: "unlimited key uses the user quota", usage: `{"code":true,"data":{"total_available":0,"unlimited_quota":true,"user_quota":{"total_available":3000000}}}`, want: 3},
		{name: "key quota alone without user quota", usage: `{"code":true,"data":{"total_available":1500000,"unlimited_quota":false}}`, want: 1.5},
		{name: "unlimited key without user quota", usage: `{"code":true,"data":{"total_available":0,"unlimited_quota":true}}`, wantErr: "Allow querying user quota"},
		{name: "upstream rejects the key", usage: `{"success":false,"message":"invalid token"}`, wantErr: "invalid token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupSub2APIBalanceTest(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/usage/token/":
					assert.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
					_, _ = w.Write([]byte(tc.usage))
				case "/api/status":
					assert.Empty(t, r.Header.Get("Authorization"), "the key is not sent to the public status endpoint")
					_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":1000000}}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			baseURL := server.URL + "/"
			channel := &model.Channel{Type: constant.ChannelTypeNewAPI, BaseURL: &baseURL, Key: "sk-upstream", Name: "new-api-upstream"}
			require.NoError(t, model.DB.Create(channel).Error)

			result, err := updateChannelBalance(channel)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.want, result.Balance, 1e-9)
			assert.InDelta(t, tc.want, channel.Balance, 1e-9)
		})
	}
}

func TestUpdateXiaomiMiMoChannelBalance(t *testing.T) {
	setupChannelCreateTest(t)
	require.NoError(t, i18n.Init())
	periodEnd := time.Date(2026, 12, 1, 1, 2, 3, 0, time.UTC).Unix()
	for _, tc := range []struct {
		name       string
		channel    int
		key        string
		path       string
		body       string
		currency   string
		remaining  float64
		used       float64
		limit      float64
		resetAt    int64
		wantStatus int
	}{
		{"plan", constant.ChannelTypeXiaomiMiMoPlan, "tp-test", "/v1/platform/token_plan",
			`{"quota":{"total_credits":100,"used_credits":25,"remaining_credits":75},"period":{"end_at":"2026-12-01 01:02:03"}}`,
			"Credits", 75, 25, 100, periodEnd, common.ChannelStatusEnabled},
		{"empty plan", constant.ChannelTypeXiaomiMiMoPlan, "tp-test", "/v1/platform/token_plan",
			`{"quota":{"total_credits":"100","used_credits":"100","remaining_credits":0}}`,
			"Credits", 0, 100, 100, 0, common.ChannelStatusAutoDisabled},
		{"metered", constant.ChannelTypeXiaomiMiMoAPI, "sk-test", "/v1/platform/balance",
			`{"balance_infos":[{"currency":"CNY","total_available":"100.35","frozen_balance":"0.10"}]}`,
			"CNY", 100.25, 0, 0, 0, common.ChannelStatusEnabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replies := make(chan string, 1)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "Bearer "+tc.key, r.Header.Get("Authorization"))
				if r.URL.Path == "/v1/models" {
					_, _ = w.Write([]byte(`{"data":[{"id":"mimo-v2.6-flash"}]}`))
					return
				}
				assert.Equal(t, tc.path, r.URL.Path)
				select {
				case body := <-replies:
					_, _ = w.Write([]byte(body))
				default:
					_, _ = w.Write([]byte(tc.body))
				}
			}))
			defer server.Close()

			baseURL := server.URL + "/v1/"
			autoBan := 1
			upstream := &model.Channel{Type: tc.channel, BaseURL: &baseURL, Key: tc.key, Name: tc.name,
				Balance: 42, Status: common.ChannelStatusEnabled, AutoBan: &autoBan}
			upstream.SetSetting(dto.ChannelSettings{TLSInsecureSkipVerify: true})
			require.NoError(t, model.DB.Create(upstream).Error)
			t.Cleanup(func() { require.NoError(t, model.DB.Delete(upstream).Error) })

			models, err := fetchChannelUpstreamModelIDs(upstream)
			require.NoError(t, err)
			assert.Equal(t, []string{"mimo-v2.6-flash"}, models)

			result, err := updateChannelBalance(upstream)
			require.NoError(t, err)
			require.NotNil(t, result.Detail)
			require.NotNil(t, result.Detail.Amount)
			assert.Equal(t, tc.remaining, *result.Detail.Amount)
			assert.Equal(t, tc.currency, result.Detail.Currency)
			if tc.limit > 0 {
				require.Len(t, result.Detail.Windows, 1)
				assert.Equal(t, tc.used, result.Detail.Windows[0].Used)
				assert.Equal(t, tc.limit, result.Detail.Windows[0].Limit)
				assert.Equal(t, tc.resetAt, result.Detail.Windows[0].ResetAt)
			}
			replies <- `{}`
			_, err = updateChannelBalance(upstream)
			require.Error(t, err)
			failed, err := model.GetChannelById(upstream.Id, true)
			require.NoError(t, err)
			failedDetail := failed.GetBalanceDetail()
			require.NotNil(t, failedDetail)
			assert.NotEmpty(t, failedDetail.Error)
			assert.Equal(t, result.Detail.Amount, failedDetail.Amount)

			updateBalanceInRun(upstream)
			stored, err := model.GetChannelById(upstream.Id, true)
			require.NoError(t, err)
			require.NotNil(t, stored.GetBalanceDetail())
			assert.Empty(t, stored.GetBalanceDetail().Error)
			assert.Equal(t, float64(42), stored.Balance)
			assert.Equal(t, tc.wantStatus, stored.Status)
		})
	}
}

func TestXiaomiMiMoChannelUsesMatchingCredentials(t *testing.T) {
	assert.NoError(t, validateChannel(&model.Channel{Type: constant.ChannelTypeXiaomiMiMoPlan, Key: "tp-personal\nttp-team"}, true))
	assert.NoError(t, validateChannel(&model.Channel{Type: constant.ChannelTypeXiaomiMiMoAPI, Key: "sk-metered"}, true))
	assert.Error(t, validateChannel(&model.Channel{Type: constant.ChannelTypeXiaomiMiMoPlan, Key: "sk-metered"}, true))
	assert.Error(t, validateChannel(&model.Channel{Type: constant.ChannelTypeXiaomiMiMoAPI, Key: "tp-personal"}, true))
	_, err := fetchXiaomiMiMoBalance(&model.Channel{Type: constant.ChannelTypeXiaomiMiMoPlan, Key: "ttp-team"})
	require.ErrorContains(t, err, "personal tp-")

	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "/v1/platform/token_plan", r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/redirect-target", http.StatusFound)
	}))
	defer upstream.Close()
	channel := &model.Channel{Type: constant.ChannelTypeXiaomiMiMoPlan, Key: "tp-test", BaseURL: &upstream.URL}
	channel.SetSetting(dto.ChannelSettings{TLSInsecureSkipVerify: true})
	_, err = fetchXiaomiMiMoBalance(channel)
	require.ErrorContains(t, err, "HTTP 302")
}
