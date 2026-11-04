package docker

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func sdkInspectionELFFixture(size int) []byte {
	data := make([]byte, size)
	copy(data, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	binary.LittleEndian.PutUint16(data[16:], 3)
	binary.LittleEndian.PutUint16(data[18:], 62)
	binary.LittleEndian.PutUint32(data[20:], 1)
	binary.LittleEndian.PutUint64(data[32:], 64)
	binary.LittleEndian.PutUint16(data[52:], 64)
	binary.LittleEndian.PutUint16(data[54:], 56)
	binary.LittleEndian.PutUint16(data[56:], 2)
	load := data[64:120]
	binary.LittleEndian.PutUint32(load, 1)
	binary.LittleEndian.PutUint32(load[4:], 5)
	binary.LittleEndian.PutUint64(load[32:], uint64(size))
	binary.LittleEndian.PutUint64(load[40:], uint64(size)+64)
	binary.LittleEndian.PutUint64(load[48:], 4096)
	dynamic := data[120:176]
	binary.LittleEndian.PutUint32(dynamic, 2)
	binary.LittleEndian.PutUint32(dynamic[4:], 6)
	binary.LittleEndian.PutUint64(dynamic[8:], 256)
	binary.LittleEndian.PutUint64(dynamic[16:], 256)
	binary.LittleEndian.PutUint64(dynamic[32:], 48)
	binary.LittleEndian.PutUint64(dynamic[40:], 48)
	binary.LittleEndian.PutUint64(dynamic[48:], 8)
	binary.LittleEndian.PutUint64(data[256:], 5)
	binary.LittleEndian.PutUint64(data[272:], 6)
	// The final dynamic entry at offset 288 is DT_NULL.
	return data
}

func sdkFixtureMetadata(files [2][]byte, requests [2]sdkELFRange, previous *[2]sdkELFMetadata) ([2]sdkELFMetadata, error) {
	var lines []string
	for i, data := range files {
		header := data
		if len(header) > 64 {
			header = header[:64]
		}
		var block []byte
		if sdkELFRangeWithin(uint64(len(data)), requests[i].offset, requests[i].length) {
			block = data[requests[i].offset : requests[i].offset+requests[i].length]
		}
		lines = append(lines, fmt.Sprintf("%d %s %s", len(data), base64.StdEncoding.EncodeToString(header), base64.StdEncoding.EncodeToString(block)))
	}
	return parseJunimo140SDKMetadata(strings.Join(lines, "\n"), false, requests, previous)
}

func TestJunimo140SDKELFStructure(t *testing.T) {
	cases := []struct {
		name   string
		change func([]byte) []byte
		accept bool
	}{
		{"valid", func(b []byte) []byte { return b }, true},
		{"different_payload", func(b []byte) []byte { return append(b, []byte("another SDK release")...) }, true},
		{"section_size_without_table", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[58:], 64); return b }, true},
		{"complete_section_table", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[40:], 384)
			binary.LittleEndian.PutUint16(b[58:], 64)
			binary.LittleEndian.PutUint16(b[60:], 2)
			binary.LittleEndian.PutUint16(b[62:], 1)
			return b
		}, true},
		{"zero_alignment", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[112:], 0); return b }, true},
		{"unit_alignment", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[112:], 1); return b }, true},
		{"header_stub_80", func(b []byte) []byte { return b[:80] }, false},
		{"header_stub_96", func(b []byte) []byte { return b[:96] }, false},
		{"short_header", func(b []byte) []byte { return b[:63] }, false},
		{"wrong_magic", func(b []byte) []byte { b[0] = 0; return b }, false},
		{"elf32", func(b []byte) []byte { b[4] = 1; return b }, false},
		{"big_endian", func(b []byte) []byte { b[5] = 2; return b }, false},
		{"wrong_ident_version", func(b []byte) []byte { b[6] = 2; return b }, false},
		{"other_os_abi", func(b []byte) []byte { b[7] = 9; return b }, false},
		{"executable", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[16:], 2); return b }, false},
		{"wrong_machine", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[18:], 183); return b }, false},
		{"wrong_version", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[20:], 2); return b }, false},
		{"wrong_header_size", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[52:], 32); return b }, false},
		{"no_program_table", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[56:], 0); return b }, false},
		{"extended_program_count", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[56:], 65535); return b }, false},
		{"too_many_program_headers", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[56:], 129); return b }, false},
		{"wrong_program_header_size", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[54:], 55); return b }, false},
		{"partial_program_table", func(b []byte) []byte { return b[:175] }, false},
		{"program_table_offset_overflow", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[32:], ^uint64(0)-15); return b }, false},
		{"section_table_outside_file", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[40:], 500)
			binary.LittleEndian.PutUint16(b[58:], 64)
			binary.LittleEndian.PutUint16(b[60:], 1)
			return b
		}, false},
		{"section_offset_overflow", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[40:], ^uint64(0)-15)
			binary.LittleEndian.PutUint16(b[58:], 64)
			binary.LittleEndian.PutUint16(b[60:], 1)
			return b
		}, false},
		{"extended_section_count", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[40:], 384)
			binary.LittleEndian.PutUint16(b[58:], 64)
			return b
		}, false},
		{"bad_section_name_index", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[40:], 384)
			binary.LittleEndian.PutUint16(b[58:], 64)
			binary.LittleEndian.PutUint16(b[60:], 1)
			binary.LittleEndian.PutUint16(b[62:], 1)
			return b
		}, false},
		{"inconsistent_section_declaration", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[60:], 1); return b }, false},
		{"segment_file_outside", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[96:], uint64(len(b))+1); return b }, false},
		{"segment_file_overflow", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[72:], ^uint64(0)-1)
			binary.LittleEndian.PutUint64(b[96:], 32)
			return b
		}, false},
		{"segment_address_overflow", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[80:], ^uint64(0)-1)
			binary.LittleEndian.PutUint64(b[104:], 32)
			return b
		}, false},
		{"load_file_larger_than_memory", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[104:], uint64(len(b))-1); return b }, false},
		{"load_bad_alignment", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[112:], 3); return b }, false},
		{"load_incongruent_alignment", func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[72:], 1)
			binary.LittleEndian.PutUint64(b[96:], uint64(len(b))-1)
			return b
		}, false},
		{"no_executable_load", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[68:], 4); return b }, false},
		{"empty_executable_load", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[96:], 0); return b }, false},
		{"dynamic_not_load_covered", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[96:], 256); return b }, false},
		{"dynamic_mapping_mismatch", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[136:], 384); return b }, false},
		{"dynamic_outside_file", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[128:], uint64(len(b))); return b }, false},
		{"dynamic_unaligned_records", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[152:], 47); return b }, false},
		{"dynamic_empty", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[152:], 0); return b }, false},
		{"dynamic_larger_than_memory", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[160:], 47); return b }, false},
		{"dynamic_over_metadata_limit", func(b []byte) []byte {
			b = sdkInspectionELFFixture(70000)
			binary.LittleEndian.PutUint64(b[152:], maxSDKMetadataBytes+16)
			binary.LittleEndian.PutUint64(b[160:], maxSDKMetadataBytes+16)
			return b
		}, false},
		{"duplicate_dynamic", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[64:], 2); return b }, false},
		{"missing_dynamic", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[120:], 4); return b }, false},
		{"dynamic_without_null", func(b []byte) []byte { binary.LittleEndian.PutUint64(b[288:], 1); return b }, false},
		{"non_load_segment_outside_file", func(b []byte) []byte {
			binary.LittleEndian.PutUint16(b[56:], 3)
			binary.LittleEndian.PutUint32(b[176:], 4)
			binary.LittleEndian.PutUint64(b[184:], ^uint64(0)-1)
			binary.LittleEndian.PutUint64(b[208:], 16)
			return b
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := [2][]byte{tc.change(sdkInspectionELFFixture(512)), sdkInspectionELFFixture(640)}
			reads := 0
			stats, err := inspectSDKELFMetadata(func(requests [2]sdkELFRange, previous *[2]sdkELFMetadata) ([2]sdkELFMetadata, error) {
				reads++
				for _, request := range requests {
					if request.length == 0 || request.length > maxSDKMetadataBytes {
						t.Fatal("unbounded metadata requested")
					}
				}
				return sdkFixtureMetadata(files, requests, previous)
			})
			if !tc.accept {
				if err == nil {
					t.Fatal("invalid or incomplete ELF object accepted")
				}
				return
			}
			if err != nil || reads != 3 || stats.SDKFiles != 2 || stats.SDKBytes != int64(len(files[0])+len(files[1])) || stats.GameFiles != 0 || stats.GameBytes != 0 {
				t.Fatalf("valid SDK structure or statistics rejected: reads=%d stats=%+v err=%v", reads, stats, err)
			}
		})
	}
}

func TestJunimo140SDKMetadataResponseRejectsIncompleteOrChanged(t *testing.T) {
	header := sdkInspectionELFFixture(512)[:64]
	encoded := base64.StdEncoding.EncodeToString(header)
	frame := "512 " + encoded + " " + encoded
	output := frame + "\n" + frame
	requests := [2]sdkELFRange{{0, 64}, {0, 64}}
	previous, err := parseJunimo140SDKMetadata(output, false, requests, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, output string
		truncated    bool
	}{
		{"missing_frame", frame, false},
		{"extra_frame", output + "\n" + frame, false},
		{"size_overflow", strings.Replace(output, "512", "18446744073709551615", 1), false},
		{"invalid_header", strings.Replace(output, encoded, "invalid", 1), false},
		{"partial_data", "512 " + encoded + " " + base64.StdEncoding.EncodeToString(header[:63]) + "\n" + frame, false},
		{"changed_size", strings.Replace(output, "512", "513", 1), false},
		{"truncated_response", output, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseJunimo140SDKMetadata(tc.output, tc.truncated, requests, &previous); err == nil {
				t.Fatal("invalid metadata response accepted")
			}
		})
	}
	changed := append([]byte(nil), header...)
	changed[8] = 1
	changedFrame := "512 " + base64.StdEncoding.EncodeToString(changed) + " " + encoded
	if _, err := parseJunimo140SDKMetadata(changedFrame+"\n"+frame, false, requests, &previous); err == nil {
		t.Fatal("changed header accepted")
	}
	for _, request := range []sdkELFRange{{^uint64(0), 64}, {0, maxSDKMetadataBytes + 1}, {0, 0}} {
		bad := requests
		bad[0] = request
		if _, err := parseJunimo140SDKMetadata(output, false, bad, nil); err == nil {
			t.Fatal("unbounded or overflowing range accepted")
		}
	}
	if err := validateSDKELFDynamic(make([]byte, 15)); err == nil {
		t.Fatal("partial dynamic record accepted")
	}
}

func TestJunimo140SDKInspectionScript(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires a Linux filesystem and shell utilities")
	}
	for _, command := range []string{"sh", "stat", "dd", "base64"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Fatalf("required SDK inspection utility %s: %v", command, err)
		}
	}
	for _, name := range []string{"valid", "different_sdk_payload", "truncated_96", "missing_library", "empty_library", "library_directory", "library_symlink", "linux64_symlink", "sdk_symlink", "root_symlink"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "game")
			libraryDir := filepath.Join(root, ".steam-sdk", "linux64")
			if err := os.MkdirAll(libraryDir, 0o700); err != nil {
				t.Fatal(err)
			}
			client, webrtc := filepath.Join(libraryDir, "steamclient.so"), filepath.Join(libraryDir, "libsteamwebrtc.so")
			clientData, webrtcData := sdkInspectionELFFixture(512), sdkInspectionELFFixture(640)
			switch name {
			case "different_sdk_payload":
				clientData = append(clientData, []byte("current SDK payload")...)
			case "empty_library":
				clientData = nil
			case "truncated_96":
				clientData = clientData[:96]
			}
			if err := os.WriteFile(client, clientData, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(webrtc, webrtcData, 0o600); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "missing_library":
				if err := os.Remove(webrtc); err != nil {
					t.Fatal(err)
				}
			case "library_directory":
				if err := os.Remove(client); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(client, 0o700); err != nil {
					t.Fatal(err)
				}
			case "library_symlink":
				if err := os.Rename(client, client+".real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(client+".real", client); err != nil {
					t.Fatal(err)
				}
			case "linux64_symlink", "sdk_symlink", "root_symlink":
				target := libraryDir
				if name == "sdk_symlink" {
					target = filepath.Dir(libraryDir)
				} else if name == "root_symlink" {
					target = root
				}
				if err := os.Rename(target, target+".real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target+".real", target); err != nil {
					t.Fatal(err)
				}
			}
			reads := 0
			stats, err := inspectSDKELFMetadata(func(requests [2]sdkELFRange, previous *[2]sdkELFMetadata) ([2]sdkELFMetadata, error) {
				reads++
				args := []string{"-c", inspectJunimo140SDKScript, "--", root}
				for _, request := range requests {
					args = append(args, strconv.FormatUint(request.offset, 10), strconv.FormatUint(request.length, 10))
				}
				output, err := exec.Command("sh", args...).CombinedOutput()
				if err != nil {
					return [2]sdkELFMetadata{}, errors.New("SDK shell metadata read failed")
				}
				return parseJunimo140SDKMetadata(string(output), false, requests, previous)
			})
			accept := name == "valid" || name == "different_sdk_payload"
			if !accept {
				if err == nil {
					t.Fatal("unsafe or truncated SDK accepted by shell and production parser")
				}
				return
			}
			if err != nil || reads != 3 || stats.SDKFiles != 2 || stats.SDKBytes != int64(len(clientData)+len(webrtcData)) {
				t.Fatalf("valid SDK failed shared production inspection: reads=%d stats=%+v err=%v", reads, stats, err)
			}
		})
	}
}

func TestJunimo140SDKInspectionRejectsUnownedVolume(t *testing.T) {
	c := NewClient(Options{DockerPath: "must-not-run"})
	if _, err := c.RuntimeInspectJunimo140SDK(context.Background(), t.TempDir(), "production", "image", "sha256:"+strings.Repeat("a", 64)); err == nil {
		t.Fatal("SDK inspection accepted a volume outside the transaction")
	}
}

func TestJunimo140SDKInspectionHelperCanBeRecovered(t *testing.T) {
	const project = "sdk-inspection-test"
	const applyID = "apply_0123456789abcdef01234567"
	image := "sha256:" + strings.Repeat("b", 64)
	id := strings.Repeat("a", 64)
	record := map[string]any{
		"ID": id, "Name": "/anxi-j140-0123456789abcdef01234567-smapi-sdk-inspect", "Image": image,
		"Config": map[string]any{"Image": image, "Labels": map[string]string{
			"com.anxi-panel.junimo140-helper":        "true",
			"com.anxi-panel.runtime-update-apply-id": applyID,
			"com.anxi-panel.compose-project":         project,
			"com.anxi-panel.junimo140-operation":     "smapi-sdk-inspect",
		}},
		"HostConfig": map[string]bool{"AutoRemove": true},
	}
	marshal := func() string {
		t.Helper()
		data, err := json.Marshal([]any{record})
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	scope := RuntimeJunimo140HelperScope{ApplyID: applyID, Projects: []string{project}, ImageRef: "sdvd/server:1.5.0-preview.140", ImageID: image}
	if err := validateJunimo140Helpers(marshal(), []string{id}, scope); err != nil {
		t.Fatalf("owned SDK helper could not be recovered: %v", err)
	}
	record["Config"].(map[string]any)["Labels"].(map[string]string)["com.anxi-panel.junimo140-operation"] = "smapi-unrecognized"
	if err := validateJunimo140Helpers(marshal(), []string{id}, scope); err == nil {
		t.Fatal("unknown helper operation was admitted during recovery")
	}
}
