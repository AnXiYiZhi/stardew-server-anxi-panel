package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

func TestWriteNexusErrorMessagesAreReadableChinese(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		statusCode int
		code       string
		message    string
	}{
		{
			name:       "api key missing",
			err:        sj.ErrNexusAPIKeyMissing,
			statusCode: http.StatusServiceUnavailable,
			code:       "nexus_api_key_missing",
			message:    "未配置 Nexus Mods API Key",
		},
		{
			name:       "auth required",
			err:        sj.ErrNexusAuthRequired,
			statusCode: http.StatusBadGateway,
			code:       "nexus_auth_required",
			message:    "该查询需要 Nexus OAuth/认证能力",
		},
		{
			name:       "not found",
			err:        &sj.NexusAPIError{StatusCode: http.StatusNotFound},
			statusCode: http.StatusNotFound,
			code:       "nexus_mod_not_found",
			message:    "未找到该 Mod",
		},
		{
			name:       "unauthorized",
			err:        &sj.NexusAPIError{StatusCode: http.StatusForbidden},
			statusCode: http.StatusBadGateway,
			code:       "nexus_unauthorized",
			message:    "Nexus API Key 无效或权限不足",
		},
		{
			name:       "rate limited",
			err:        &sj.NexusAPIError{StatusCode: http.StatusTooManyRequests},
			statusCode: http.StatusTooManyRequests,
			code:       "nexus_rate_limited",
			message:    "Nexus 请求过于频繁，请稍后重试",
		},
		{
			name:       "generic request failure",
			err:        errors.New("nexus request failed"),
			statusCode: http.StatusBadGateway,
			code:       "nexus_request_failed",
			message:    "Nexus 请求失败，请稍后重试",
		},
		{
			name:       "network failure",
			err:        &sj.NexusRequestError{Err: errors.New("dial tcp: i/o timeout")},
			statusCode: http.StatusBadGateway,
			code:       "nexus_network_failed",
			message:    "Nexus 网络连接失败，请确认面板服务器能访问 api.nexusmods.com",
		},
	}

	s := &server{logger: slog.Default()}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.writeNexusError(rec, tc.err)

			if rec.Code != tc.statusCode {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.statusCode, rec.Body.String())
			}

			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
			}
			if body.Error.Code != tc.code {
				t.Fatalf("code = %q, want %q", body.Error.Code, tc.code)
			}
			if body.Error.Message != tc.message {
				t.Fatalf("message = %q, want %q", body.Error.Message, tc.message)
			}
			if strings.ContainsAny(body.Error.Message, "璇鎼鏈澶鈹") {
				t.Fatalf("message still looks mojibake: %q", body.Error.Message)
			}
		})
	}
}
