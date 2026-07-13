package updater

import (
	"fmt"
	"regexp"
	"strings"
)

var exactVersionPattern = regexp.MustCompile(`^(?:v|V)?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

var trustedRepositories = []string{
	"crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/stardew-server-anxi-panel",
	"ghcr.io/anxiyizhi/stardew-server-anxi-panel",
	"anxiyizhi/stardew-server-anxi-panel",
	"docker.io/anxiyizhi/stardew-server-anxi-panel",
	"docker.1ms.run/anxiyizhi/stardew-server-anxi-panel",
	"docker.m.daocloud.io/anxiyizhi/stardew-server-anxi-panel",
}

func NormalizeTargetVersion(value string) (string, error) {
	matches := exactVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return "", fmt.Errorf("目标版本必须是稳定的精确语义版本")
	}
	return strings.Join(matches[1:], "."), nil
}

// CompareStableVersions compares two exact stable semantic versions.
func CompareStableVersions(a, b string) (int, error) {
	left, err := NormalizeTargetVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := NormalizeTargetVersion(b)
	if err != nil {
		return 0, err
	}
	var av, bv [3]uint64
	if _, err := fmt.Sscanf(left, "%d.%d.%d", &av[0], &av[1], &av[2]); err != nil {
		return 0, err
	}
	if _, err := fmt.Sscanf(right, "%d.%d.%d", &bv[0], &bv[1], &bv[2]); err != nil {
		return 0, err
	}
	for i := range av {
		if av[i] < bv[i] {
			return -1, nil
		}
		if av[i] > bv[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func TrustedImageCandidates(version, currentImage string) ([]string, error) {
	normalized, err := NormalizeTargetVersion(version)
	if err != nil {
		return nil, err
	}
	repos := append([]string(nil), trustedRepositories...)
	if repo, ok := trustedRepositoryOf(currentImage); ok {
		repos = append([]string{repo}, repos...)
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(repos))
	for _, repo := range repos {
		ref := repo + ":" + normalized
		if !seen[ref] {
			seen[ref] = true
			result = append(result, ref)
		}
	}
	return result, nil
}

func ValidateTrustedImage(ref string) error {
	ref = strings.TrimSpace(ref)
	if strings.Contains(ref, "@") {
		return fmt.Errorf("不接受 digest 或混合镜像引用")
	}
	colon := strings.LastIndex(ref, ":")
	if colon <= strings.LastIndex(ref, "/") || colon == len(ref)-1 {
		return fmt.Errorf("镜像必须使用精确版本 tag")
	}
	repo, tag := ref[:colon], ref[colon+1:]
	if _, err := NormalizeTargetVersion(tag); err != nil {
		return err
	}
	for _, allowed := range trustedRepositories {
		if repo == allowed {
			return nil
		}
	}
	return fmt.Errorf("镜像仓库不在项目白名单中")
}

func trustedRepositoryOf(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	colon := strings.LastIndex(ref, ":")
	if colon > strings.LastIndex(ref, "/") {
		ref = ref[:colon]
	}
	for _, allowed := range trustedRepositories {
		if ref == allowed {
			return ref, true
		}
	}
	return "", false
}
