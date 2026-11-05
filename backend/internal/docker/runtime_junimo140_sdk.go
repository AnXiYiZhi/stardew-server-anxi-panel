package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"time"
)

const maxSDKMetadataBytes uint64 = 64 * 1024

// Only bounded metadata from the two fixed libraries is returned. Each read
// repeats the ELF header and size; the library is never loaded or executed.
const inspectJunimo140SDKScript = `set -eu
root=$1
shift
test -d "$root"
test ! -L "$root"
cd "$root"
for dir in .steam-sdk .steam-sdk/linux64; do
    test -d "$dir"
    test ! -L "$dir"
done
for file in .steam-sdk/linux64/steamclient.so .steam-sdk/linux64/libsteamwebrtc.so; do
    offset=$1
    length=$2
    shift 2
    case "$offset" in ''|*[!0-9]*) exit 1 ;; esac
    case "$length" in ''|*[!0-9]*) exit 1 ;; esac
    test "$length" -gt 0
    test "$length" -le 65536
    test ! -L "$file"
    test -f "$file"
    size=$(stat -c %s -- "$file")
    case "$size" in ''|*[!0-9]*) exit 1 ;; esac
    test "$size" -ge 64
    header=$(dd if="$file" bs=1 count=64 status=none | base64 -w 0)
    block=$(dd if="$file" bs=1 skip="$offset" count="$length" status=none | base64 -w 0)
    printf '%s %s %s\n' "$size" "$header" "$block"
done`

type sdkELFRange struct{ offset, length uint64 }
type sdkELFMetadata struct {
	size   uint64
	header []byte
	data   []byte
}
type sdkELFSegment struct {
	kind, flags                 uint32
	offset, address, files, mem uint64
	align                       uint64
}

type sdkELFMetadataReader func([2]sdkELFRange, *[2]sdkELFMetadata) ([2]sdkELFMetadata, error)

// RuntimeInspectJunimo140SDK checks bounded ELF structure on an unused owned
// volume. It does not prove depot authenticity or SDK ABI compatibility.
func (c *Client) RuntimeInspectJunimo140SDK(ctx context.Context, dir, volume, imageRef, imageID string) (RuntimeGameFileStats, error) {
	return inspectSDKELFMetadata(func(requests [2]sdkELFRange, previous *[2]sdkELFMetadata) ([2]sdkELFMetadata, error) {
		return c.readJunimo140SDKMetadata(ctx, dir, volume, imageRef, imageID, requests, previous)
	})
}

func inspectSDKELFMetadata(read sdkELFMetadataReader) (RuntimeGameFileStats, error) {
	requests := [2]sdkELFRange{{0, 64}, {0, 64}}
	initial, err := read(requests, nil)
	if err != nil {
		return RuntimeGameFileStats{}, err
	}
	for i := range requests {
		requests[i], err = sdkELFProgramHeaderRange(initial[i])
		if err != nil {
			return RuntimeGameFileStats{}, err
		}
	}
	programs, err := read(requests, &initial)
	if err != nil {
		return RuntimeGameFileStats{}, err
	}
	for i := range requests {
		requests[i], err = sdkELFDynamicRange(programs[i])
		if err != nil {
			return RuntimeGameFileStats{}, err
		}
	}
	dynamic, err := read(requests, &initial)
	if err != nil {
		return RuntimeGameFileStats{}, err
	}
	for _, file := range dynamic {
		if err := validateSDKELFDynamic(file.data); err != nil {
			return RuntimeGameFileStats{}, err
		}
	}
	const maxTotal = uint64(1<<63 - 1)
	if initial[0].size > maxTotal-initial[1].size {
		return RuntimeGameFileStats{}, errors.New("installed SDK size exceeds the supported bound")
	}
	return RuntimeGameFileStats{SDKFiles: 2, SDKBytes: int64(initial[0].size + initial[1].size)}, nil
}

func (c *Client) readJunimo140SDKMetadata(ctx context.Context, dir, volume, imageRef, imageID string, requests [2]sdkELFRange, previous *[2]sdkELFMetadata) ([2]sdkELFMetadata, error) {
	return c.readJunimo140SDKMetadataWithScope(ctx, dir, volume, imageRef, imageID, requests, previous, nil)
}

func (c *Client) readJunimo140SDKMetadataWithScope(ctx context.Context, dir, volume, imageRef, imageID string, requests [2]sdkELFRange, previous *[2]sdkELFMetadata, projects []string) ([2]sdkELFMetadata, error) {
	var empty [2]sdkELFMetadata
	if err := c.junimo140ReadVolume(ctx, dir, volume, projects); err != nil {
		return empty, err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return empty, err
	}
	helperArgs, err := junimo140ReadHelperArgs(volume, "sdk-inspect", projects)
	if err != nil {
		return empty, err
	}
	args := append([]string{"run", "--rm"}, helperArgs...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only",
		"--mount", junimo140ReadMount(volume, projects),
		"--entrypoint", "timeout", imageID, "--kill-after=2s", "30s", "sh", "-c", inspectJunimo140SDKScript, "--", "/game")
	for _, request := range requests {
		if request.length == 0 || request.length > maxSDKMetadataBytes {
			return empty, errors.New("SDK metadata request exceeds the supported bound")
		}
		args = append(args, strconv.FormatUint(request.offset, 10), strconv.FormatUint(request.length, 10))
	}
	result, raw, err := c.runJunimo140Read(ctx, "inspect installed Linux Steam SDK metadata", dir, time.Minute, nil, volume, imageID, projects, args...)
	if err != nil {
		// CommandError normally retains stdout. Encoded binary metadata must not
		// escape through errors or job logs.
		var commandErr CommandError
		if errors.As(err, &commandErr) {
			commandErr.Result.Stdout = ""
			return empty, commandErr
		}
		return empty, errors.New("installed SDK metadata inspection failed")
	}
	return parseJunimo140SDKMetadata(raw, result.StdoutTruncated, requests, previous)
}

func parseJunimo140SDKMetadata(output string, truncated bool, requests [2]sdkELFRange, previous *[2]sdkELFMetadata) ([2]sdkELFMetadata, error) {
	var result [2]sdkELFMetadata
	if truncated {
		return result, errors.New("installed SDK metadata response was truncated")
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != len(result) {
		return result, errors.New("invalid installed SDK metadata response")
	}
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 || requests[i].length == 0 || requests[i].length > maxSDKMetadataBytes {
			return result, errors.New("invalid installed SDK metadata frame")
		}
		size, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || size < 64 {
			return result, errors.New("invalid installed SDK file size")
		}
		header, err := base64.StdEncoding.DecodeString(fields[1])
		if err != nil || len(header) != 64 {
			return result, errors.New("invalid installed SDK ELF header response")
		}
		data, err := base64.StdEncoding.DecodeString(fields[2])
		if err != nil || uint64(len(data)) != requests[i].length || !sdkELFRangeWithin(uint64(size), requests[i].offset, requests[i].length) {
			return result, errors.New("incomplete installed SDK metadata range")
		}
		if previous != nil && (previous[i].size != uint64(size) || !bytes.Equal(previous[i].header, header)) {
			return result, errors.New("installed SDK changed during metadata inspection")
		}
		result[i] = sdkELFMetadata{size: uint64(size), header: header, data: data}
	}
	return result, nil
}

func sdkELFRangeWithin(size, offset, length uint64) bool {
	return offset <= size && length <= size-offset
}

func sdkELFProgramHeaderRange(file sdkELFMetadata) (sdkELFRange, error) {
	h := file.header
	if len(h) != 64 || !bytes.Equal(h[:7], []byte{127, 'E', 'L', 'F', 2, 1, 1}) || h[7] != 0 && h[7] != 3 ||
		binary.LittleEndian.Uint16(h[16:]) != 3 || binary.LittleEndian.Uint16(h[18:]) != 62 || binary.LittleEndian.Uint32(h[20:]) != 1 || binary.LittleEndian.Uint16(h[52:]) != 64 {
		return sdkELFRange{}, errors.New("installed SDK is not a supported Linux x86-64 ELF shared object")
	}
	offset := binary.LittleEndian.Uint64(h[32:])
	count := binary.LittleEndian.Uint16(h[56:])
	if binary.LittleEndian.Uint16(h[54:]) != 56 || count == 0 || count > 128 || offset < 64 || !sdkELFRangeWithin(file.size, offset, uint64(count)*56) {
		return sdkELFRange{}, errors.New("installed SDK program header table is missing or outside the file")
	}
	sectionOffset := binary.LittleEndian.Uint64(h[40:])
	sectionCount := binary.LittleEndian.Uint16(h[60:])
	sectionNames := binary.LittleEndian.Uint16(h[62:])
	if sectionOffset == 0 {
		if sectionCount != 0 || sectionNames != 0 {
			return sdkELFRange{}, errors.New("installed SDK section table declaration is inconsistent")
		}
	} else if binary.LittleEndian.Uint16(h[58:]) != 64 || sectionCount == 0 || sectionNames >= sectionCount || !sdkELFRangeWithin(file.size, sectionOffset, uint64(sectionCount)*64) {
		return sdkELFRange{}, errors.New("installed SDK section header table is unsupported or outside the file")
	}
	return sdkELFRange{offset, uint64(count) * 56}, nil
}

func sdkELFDynamicRange(file sdkELFMetadata) (sdkELFRange, error) {
	programs, err := sdkELFProgramHeaderRange(file)
	if err != nil {
		return sdkELFRange{}, err
	}
	if uint64(len(file.data)) != programs.length {
		return sdkELFRange{}, errors.New("installed SDK program header table is incomplete")
	}
	var loads []sdkELFSegment
	var dynamic *sdkELFSegment
	executable := false
	for offset := 0; offset < len(file.data); offset += 56 {
		p := file.data[offset : offset+56]
		segment := sdkELFSegment{kind: binary.LittleEndian.Uint32(p), flags: binary.LittleEndian.Uint32(p[4:]), offset: binary.LittleEndian.Uint64(p[8:]), address: binary.LittleEndian.Uint64(p[16:]), files: binary.LittleEndian.Uint64(p[32:]), mem: binary.LittleEndian.Uint64(p[40:]), align: binary.LittleEndian.Uint64(p[48:])}
		if !sdkELFRangeWithin(file.size, segment.offset, segment.files) || segment.mem > ^uint64(0)-segment.address {
			return sdkELFRange{}, errors.New("installed SDK segment exceeds its file or virtual address bounds")
		}
		switch segment.kind {
		case 1: // PT_LOAD
			if segment.files > segment.mem || segment.align > 1 && (segment.align&(segment.align-1) != 0 || segment.offset%segment.align != segment.address%segment.align) {
				return sdkELFRange{}, errors.New("installed SDK load segment has invalid size or alignment")
			}
			loads = append(loads, segment)
			executable = executable || segment.flags&1 != 0 && segment.files > 0
		case 2: // PT_DYNAMIC
			if dynamic != nil || segment.files == 0 || segment.files > maxSDKMetadataBytes || segment.files%16 != 0 || segment.files > segment.mem {
				return sdkELFRange{}, errors.New("installed SDK dynamic table is missing, ambiguous, or outside supported bounds")
			}
			copy := segment
			dynamic = &copy
		}
	}
	if !executable || dynamic == nil {
		return sdkELFRange{}, errors.New("installed SDK lacks executable load or dynamic segments")
	}
	for _, load := range loads {
		if dynamic.offset >= load.offset && dynamic.address >= load.address &&
			sdkELFRangeWithin(load.files, dynamic.offset-load.offset, dynamic.files) &&
			sdkELFRangeWithin(load.files, dynamic.address-load.address, dynamic.files) &&
			dynamic.offset-load.offset == dynamic.address-load.address {
			return sdkELFRange{dynamic.offset, dynamic.files}, nil
		}
	}
	return sdkELFRange{}, errors.New("installed SDK dynamic table is not covered by a file-backed load segment")
}

func validateSDKELFDynamic(data []byte) error {
	if len(data) == 0 || len(data)%16 != 0 || uint64(len(data)) > maxSDKMetadataBytes {
		return errors.New("installed SDK dynamic table is incomplete")
	}
	for offset := 0; offset < len(data); offset += 16 {
		if binary.LittleEndian.Uint64(data[offset:]) == 0 {
			return nil
		}
	}
	return errors.New("installed SDK dynamic table has no DT_NULL terminator")
}
