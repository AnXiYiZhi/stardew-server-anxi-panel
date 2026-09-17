package installerrors

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCatalogSamples(t *testing.T) {
	data, err := os.ReadFile("testdata/samples.json")
	if err != nil {
		t.Fatal(err)
	}
	var samples []struct{ ID, Line string }
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, sample := range samples {
		t.Run(sample.ID, func(t *testing.T) {
			rule := Match(sample.Line)
			if rule == nil || rule.ID != sample.ID {
				t.Fatalf("unexpected rule: %v", rule)
			}
			covered[rule.ID] = true
		})
	}
	for _, rule := range catalog.Rules {
		if !covered[rule.ID] {
			t.Errorf("missing sample for %s", rule.ID)
		}
	}
}

func TestEvidenceRecoveryAndExitCodes(t *testing.T) {
	var evidence Evidence
	exit5 := errors.New("SteamCMD install exited with code 5")
	evidence.Observe("ERROR (Invalid Password)")
	for i := 0; i < 3000; i++ {
		evidence.Observe("Waiting for client shutdown...")
	}
	if got := evidence.Message(exit5, "steamcmd_failed"); got != catalog.Rules[0].Message {
		t.Fatal(got)
	}
	evidence.Reset()
	if got := evidence.Message(exit5, "steamcmd_failed"); !strings.Contains(got, "无法确定原因") {
		t.Fatal(got)
	}
	evidence.Observe("That Steam Guard code was invalid.")
	evidence.Authenticated()
	evidence.Observe("no space left on device")
	evidence.Observe("App '413150' state is 0x402 after update job")
	if got := evidence.Message(exit5, "steamcmd_failed"); !strings.Contains(got, "存储空间") {
		t.Fatal(got)
	}
	evidence.Reset()
	evidence.Observe("No subscription")
	if got := evidence.Message(errors.New("SteamCMD install finished without success marker"), "steamcmd_failed"); !strings.Contains(got, "下载许可") {
		t.Fatal(got)
	}
	evidence.Reset()
	for _, code := range []string{"5", "126", "127", "137", "139", "143", "254"} {
		got := evidence.Message(errors.New("container exited with code "+code), "")
		if !strings.Contains(got, code) || !strings.Contains(got, "退出码") {
			t.Fatal(got)
		}
	}
	if got := evidence.Message(errors.New("unknown secret=do-not-display"), ""); got != catalog.Unknown {
		t.Fatal(got)
	}
	cause := errors.New("cause")
	if !errors.Is(&ExplainedError{Message: "安装失败", Cause: cause}, cause) {
		t.Fatal("lost wrapped cause")
	}
}
