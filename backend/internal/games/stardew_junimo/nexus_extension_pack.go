package stardew_junimo

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NexusInstallerExtensionFileName is the stable browser download filename.
const NexusInstallerExtensionFileName = "anxi-nexus-installer.zip"

// ErrNexusInstallerExtensionNotFound means the panel runtime cannot find the
// checked-in browser extension source directory to package.
var ErrNexusInstallerExtensionNotFound = errors.New("Nexus browser extension source not found")

// EnsureNexusInstallerExtensionZip returns an existing valid instance-local
// extension ZIP when present, otherwise creates one from the bundled extension
// source. This lets production deployments pre-place the package while keeping
// local development self-healing.
func EnsureNexusInstallerExtensionZip(dataDir string) (string, error) {
	outPath := nexusInstallerExtensionZipPath(dataDir)
	wantVersion, haveWant := expectedNexusInstallerExtensionVersion()

	// Reuse the instance-local package only when it is structurally valid AND
	// matches the bundled extension version. Without the version gate a stale
	// cached ZIP would shadow every future extension update (the validator only
	// checks that manifest.json/background.js exist, not their contents).
	if err := validateNexusInstallerExtensionZip(outPath); err == nil {
		if !haveWant || nexusInstallerExtensionZipMatchesVersion(outPath, wantVersion) {
			return outPath, nil
		}
	}

	// Prefer the prebuilt image/repo package, but only when its version matches
	// the bundled source; otherwise fall through to repackaging from source.
	if prebuiltPath, err := findPrebuiltNexusInstallerExtensionZip(); err == nil {
		if !haveWant || nexusInstallerExtensionZipMatchesVersion(prebuiltPath, wantVersion) {
			if err := copyNexusInstallerExtensionZip(prebuiltPath, outPath); err == nil {
				if err := validateNexusInstallerExtensionZip(outPath); err == nil {
					return outPath, nil
				}
			}
		}
	}
	return ExportNexusInstallerExtensionZip(dataDir)
}

// expectedNexusInstallerExtensionVersion reads the version of the bundled
// extension source. The bool is false when the source (or its version) cannot
// be determined, in which case callers skip the version gate and fall back to
// structural validation only.
func expectedNexusInstallerExtensionVersion() (string, bool) {
	sourceDir, err := findNexusInstallerExtensionSource()
	if err != nil {
		return "", false
	}
	version, err := readManifestVersionFromDir(sourceDir)
	if err != nil || version == "" {
		return "", false
	}
	return version, true
}

func readManifestVersionFromDir(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return "", err
	}
	return parseManifestVersion(data)
}

func readManifestVersionFromZip(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return "", err
		}
		return parseManifestVersion(data)
	}
	return "", errors.New("manifest.json not found in extension package")
}

func parseManifestVersion(data []byte) (string, error) {
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	return strings.TrimSpace(manifest.Version), nil
}

// nexusInstallerExtensionZipMatchesVersion reports whether the package at path
// carries exactly the wanted manifest version. Exact match (not >=) is
// deliberate: the bundled source is the source of truth, so any mismatch —
// upgrade or downgrade — should trigger a refresh.
func nexusInstallerExtensionZipMatchesVersion(path, want string) bool {
	got, err := readManifestVersionFromZip(path)
	return err == nil && got != "" && got == want
}

const nexusInstallerExtensionInstructions = `安装步骤：
1. 解压本 ZIP 文件。
2. 打开 Chrome 或 Edge 的扩展管理页。
3. 开启“开发人员模式”。
4. 点击“加载已解压的扩展”，选择刚解压出来、包含 manifest.json 的文件夹。
5. 回到面板 Mods 下载页，点击“检测扩展”。

注意：
- 请在同一个浏览器里登录面板管理员账号和 Nexus Mods。
- 如果面板地址或端口变化，请回到面板重新点击“检测扩展”同步地址。
`

// ExportNexusInstallerExtensionZip packages the checked-in browser extension
// into the instance local-container, next to the SMAPI bundle area.
func ExportNexusInstallerExtensionZip(dataDir string) (string, error) {
	sourceDir, err := findNexusInstallerExtensionSource()
	if err != nil {
		return "", err
	}

	outPath := nexusInstallerExtensionZipPath(dataDir)
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create extension package directory: %w", err)
	}

	tmp, err := os.CreateTemp(outDir, ".anxi-nexus-installer-*.zip")
	if err != nil {
		return "", fmt.Errorf("create extension package: %w", err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	zw := zip.NewWriter(tmp)
	if err := addNexusInstallerExtensionFiles(zw, sourceDir); err != nil {
		_ = zw.Close()
		return "", err
	}
	if err := addZipTextFile(zw, "安装说明.txt", nexusInstallerExtensionInstructions); err != nil {
		_ = zw.Close()
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("finalize extension package: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close extension package: %w", err)
	}

	_ = os.Remove(outPath)
	if err := os.Rename(tmpPath, outPath); err != nil {
		return "", fmt.Errorf("publish extension package: %w", err)
	}
	success = true
	return outPath, nil
}

func nexusInstallerExtensionZipPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "browser-extensions", NexusInstallerExtensionFileName)
}

func addNexusInstallerExtensionFiles(zw *zip.Writer, sourceDir string) error {
	return filepath.WalkDir(sourceDir, func(pathOnDisk string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(sourceDir, pathOnDisk)
		if err != nil {
			return err
		}
		entryName := filepath.ToSlash(relPath)
		if entryName == "." || strings.HasPrefix(entryName, "../") || filepath.IsAbs(entryName) {
			return fmt.Errorf("invalid extension package entry %q", entryName)
		}
		if strings.HasSuffix(strings.ToLower(entryName), ".zip") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = entryName
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(pathOnDisk)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
		return nil
	})
}

func addZipTextFile(zw *zip.Writer, name, content string) error {
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, content)
	return err
}

func validateNexusInstallerExtensionZip(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	hasManifest := false
	hasBackground := false
	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			hasManifest = true
		}
		if f.Name == "background.js" {
			hasBackground = true
		}
		if filepath.IsAbs(f.Name) || strings.HasPrefix(f.Name, "../") || strings.Contains(f.Name, "/../") {
			return fmt.Errorf("extension package contains unsafe entry %q", f.Name)
		}
	}
	if !hasManifest {
		return errors.New("extension package missing root manifest.json")
	}
	if !hasBackground {
		return errors.New("extension package missing root background.js")
	}
	return nil
}

func copyNexusInstallerExtensionZip(sourcePath, outPath string) error {
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create extension package directory: %w", err)
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	tmp, err := os.CreateTemp(outDir, ".anxi-nexus-installer-copy-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, in); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Remove(outPath)
	if err := os.Rename(tmpPath, outPath); err != nil {
		return err
	}
	success = true
	return nil
}

func findNexusInstallerExtensionSource() (string, error) {
	var candidates []string
	addRoot := func(root string) {
		if root == "" {
			return
		}
		candidates = append(candidates,
			filepath.Join(root, "browser-extensions", "nexus-slow-installer"),
			filepath.Join(root, "nexus-slow-installer"),
		)
	}
	addRootAndParents := func(start string) {
		current := filepath.Clean(start)
		for {
			addRoot(current)
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		addRootAndParents(cwd)
	}
	if exe, err := os.Executable(); err == nil {
		addRootAndParents(filepath.Dir(exe))
	}
	addRoot("/app")

	seen := make(map[string]bool)
	var checked []string
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		checked = append(checked, candidate)
		if info, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	sort.Strings(checked)
	return "", fmt.Errorf("%w: checked %s", ErrNexusInstallerExtensionNotFound, strings.Join(checked, ", "))
}

func findPrebuiltNexusInstallerExtensionZip() (string, error) {
	var candidates []string
	addRoot := func(root string) {
		if root == "" {
			return
		}
		candidates = append(candidates,
			filepath.Join(root, "browser-extensions", NexusInstallerExtensionFileName),
			filepath.Join(root, "browser-extensions", "nexus-slow-installer", NexusInstallerExtensionFileName),
			filepath.Join(root, NexusInstallerExtensionFileName),
		)
	}
	addRootAndParents := func(start string) {
		current := filepath.Clean(start)
		for {
			addRoot(current)
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		addRootAndParents(cwd)
	}
	if exe, err := os.Executable(); err == nil {
		addRootAndParents(filepath.Dir(exe))
	}
	addRoot("/app")

	seen := make(map[string]bool)
	var checked []string
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		checked = append(checked, candidate)
		if err := validateNexusInstallerExtensionZip(candidate); err == nil {
			return candidate, nil
		}
	}
	sort.Strings(checked)
	return "", fmt.Errorf("prebuilt Nexus browser extension package not found: checked %s", strings.Join(checked, ", "))
}
