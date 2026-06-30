package stardew_junimo

import (
	"testing"
)

func TestParseInviteCode_ValidPatterns(t *testing.T) {
	cases := []struct {
		output string
		want   string
	}{
		{"Invite Code: ABCD-1234-EFGH", "ABCD-1234-EFGH"},
		{"invitecode: XY12-3456-ABCD", "XY12-3456-ABCD"},
		{"InviteCode: AA11-BB22-CC33", "AA11-BB22-CC33"},
		{"some output\nABCD-1234\nmore", "ABCD-1234"},
		// Galaxy P2P codes have no hyphens
		{"Invite Code: SGCWS0Z572F2", "SGCWS0Z572F2"},
		{"(Invite code: SGCWS0Z572F2)", "SGCWS0Z572F2"},
		{"some output\nSGCWS0Z572F2\nmore", "SGCWS0Z572F2"},
		{"no code here", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := parseInviteCode(tc.output)
		if got != tc.want {
			t.Errorf("parseInviteCode(%q) = %q, want %q", tc.output, got, tc.want)
		}
	}
}

func TestMergeInviteCodeInPayload(t *testing.T) {
	result := mergeInviteCodeInPayload(`{"save_strategy":"new_game"}`, "ABCD-1234-WXYZ")
	if !containsStr(result, `"invite_code"`) {
		t.Errorf("invite_code not in payload: %s", result)
	}
	if !containsStr(result, "ABCD-1234-WXYZ") {
		t.Errorf("invite code value not in payload: %s", result)
	}
	if !containsStr(result, "save_strategy") {
		t.Errorf("existing key lost in merge: %s", result)
	}
}

func TestMergeInviteCodeInPayload_EmptyExisting(t *testing.T) {
	result := mergeInviteCodeInPayload("", "XXXX-1111")
	if !containsStr(result, `"invite_code"`) {
		t.Errorf("invite_code not in payload: %s", result)
	}
}

func TestLooksLikePortBindFailure(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "windows reserved port",
			text: "ports are not available: exposing port TCP 0.0.0.0:5800 -> 127.0.0.1:0: listen tcp 0.0.0.0:5800: bind: An attempt was made to access a socket in a way forbidden by its access permissions.",
			want: true,
		},
		{
			name: "already allocated",
			text: "Bind for 0.0.0.0:5800 failed: port is already allocated",
			want: true,
		},
		{
			name: "non port docker error",
			text: "docker compose up: docker command failed",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikePortBindFailure(tc.text)
			if got != tc.want {
				t.Fatalf("looksLikePortBindFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}
