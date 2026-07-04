package scripts_test

import (
	"strings"
	"testing"
)

// Mirrors xQuakShell internal/usecase/github_plugin_preview.go parseGitHubAssetName.
func parseGitHubAssetName(filename string) (osName, arch string) {
	name := strings.ToLower(filename)
	name = strings.TrimSuffix(name, ".exe")
	parts := strings.Split(name, "-")
	if len(parts) < 3 {
		return "", ""
	}
	arch = parts[len(parts)-1]
	osName = parts[len(parts)-2]
	switch osName {
	case "windows", "linux", "darwin":
	default:
		return "", ""
	}
	switch arch {
	case "amd64", "arm64", "386", "arm":
	default:
		return "", ""
	}
	return osName, arch
}

func TestGitHubReleaseAssetName(t *testing.T) {
	const (
		localBinary   = "xqs-plugin-telnet.exe"
		releaseAsset  = "xqs-plugin-telnet-windows-amd64.exe"
	)

	if osName, arch := parseGitHubAssetName(localBinary); osName != "" || arch != "" {
		t.Fatalf("local binary must not match GitHub asset pattern, got %s/%s", osName, arch)
	}

	osName, arch := parseGitHubAssetName(releaseAsset)
	if osName != "windows" || arch != "amd64" {
		t.Fatalf("release asset not parsed: got %s/%s", osName, arch)
	}
}
