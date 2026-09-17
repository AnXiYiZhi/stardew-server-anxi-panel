package web

import (
	"errors"
	"strings"
	"testing"
)

func TestInstallRequestFailureMessage(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"docker pull: lookup private-host.invalid: no such host token=secret-value", "无法解析"},
		{"prepare /private/data: no space left on device", "存储空间"},
		{"permission denied while trying to connect to the Docker daemon socket", "Docker Socket"},
		{"内置兼容矩阵不是可安装的 recommended 状态", "兼容清单"},
		{"opaque error /private/data token=secret-value", "安装任务启动失败"},
	} {
		message := installRequestFailureMessage(errors.New(tc.raw), "安装任务启动失败")
		if !strings.Contains(message, tc.want) {
			t.Fatal(message)
		}
		if strings.Contains(message, "secret-value") || strings.Contains(message, "/private") || strings.Contains(message, "private-host") {
			t.Fatal("raw details exposed")
		}
	}
}
