package service

import (
	"fmt"
	"regexp"
	"strings"

	pluginv1 "github.com/Wei-Shaw/sub2api/pkg/pluginapi/v1"
	"golang.org/x/mod/semver"
)

type PluginHostInfo struct {
	Version   string
	BuildType string
}

var forkPluginHostVersionPattern = regexp.MustCompile(`^v?((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))\.(0|[1-9][0-9]*)$`)

func EvaluatePluginCompatibility(manifest PluginManifest, host PluginHostInfo) PluginCompatibility {
	hostVersion, compatibilityVersion := normalizePluginHostVersion(host.Version)
	result := PluginCompatibility{
		CurrentSub2API:     host.Version,
		RequiredSub2API:    manifest.Requires.Sub2API,
		RecommendedSub2API: manifest.Requires.RecommendedSub2APIVersion,
		PluginProtocol:     manifest.Requires.PluginProtocol,
		TransportAPI:       manifest.Requires.TransportAPI,
		UIBridge:           manifest.Requires.UIBridge,
	}
	if hostVersion != compatibilityVersion {
		result.CompatibilitySub2API = strings.TrimPrefix(compatibilityVersion, "v")
	}
	if manifest.Requires.PluginProtocol != pluginv1.ProtocolVersion ||
		manifest.Requires.TransportAPI != pluginv1.TransportAPIVersion ||
		manifest.Requires.UIBridge != pluginv1.UIBridgeVersion {
		result.Status = "incompatible"
		result.Message = "插件协议版本与当前 Sub2API 不兼容"
		return result
	}
	if !matchesSemverRange(compatibilityVersion, manifest.Requires.Sub2API) {
		result.Status = "incompatible"
		result.Message = fmt.Sprintf("当前 Sub2API %s 不满足插件要求 %s", host.Version, manifest.Requires.Sub2API)
		if result.CompatibilitySub2API != "" {
			result.Message = fmt.Sprintf("当前 Sub2API %s（上游兼容基线 %s）不满足插件要求 %s", host.Version, result.CompatibilitySub2API, manifest.Requires.Sub2API)
		}
		return result
	}
	result.Compatible = true
	for _, tested := range manifest.Requires.TestedSub2APIVersions {
		testedVersion, _ := normalizePluginHostVersion(tested)
		if testedVersion != "" && testedVersion == hostVersion {
			result.Tested = true
			break
		}
	}
	if result.Tested {
		result.Status = "compatible"
		result.Message = "当前 Sub2API 版本已由插件声明测试"
	} else {
		result.Status = "untested"
		result.Message = "版本范围兼容，但插件未声明已测试当前 Sub2API 版本"
		if result.CompatibilitySub2API != "" {
			result.Message = fmt.Sprintf("上游基线 %s 满足版本范围，但插件未声明已测试当前 fork 版本 %s", result.CompatibilitySub2API, host.Version)
		}
	}
	return result
}

// normalizePluginHostVersion keeps exact test declarations separate from the
// upstream SemVer baseline used by plugin compatibility ranges.
func normalizePluginHostVersion(version string) (exact, baseline string) {
	if normalized := normalizeSemver(version); normalized != "" {
		return normalized, normalized
	}
	match := forkPluginHostVersionPattern.FindStringSubmatch(strings.TrimSpace(version))
	if match == nil {
		return "", ""
	}
	return "v" + match[1] + "." + match[5], "v" + match[1]
}

func normalizeSemver(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

func matchesSemverRange(version, expression string) bool {
	v := normalizeSemver(version)
	if v == "" {
		return false
	}
	tokens := strings.Fields(strings.ReplaceAll(expression, ",", " "))
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		op := "="
		raw := token
		for _, candidate := range []string{">=", "<=", ">", "<", "="} {
			if strings.HasPrefix(token, candidate) {
				op = candidate
				raw = strings.TrimSpace(strings.TrimPrefix(token, candidate))
				break
			}
		}
		bound := normalizeSemver(raw)
		if bound == "" {
			return false
		}
		comparison := semver.Compare(v, bound)
		matched := map[string]bool{
			">=": comparison >= 0,
			"<=": comparison <= 0,
			">":  comparison > 0,
			"<":  comparison < 0,
			"=":  comparison == 0,
		}[op]
		if !matched {
			return false
		}
	}
	return true
}
