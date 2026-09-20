package service

import (
	"context"
	"testing"
	"time"

	pluginv1 "github.com/Wei-Shaw/sub2api/pkg/pluginapi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluatePluginCompatibility(t *testing.T) {
	manifest := testPluginManifest(nil)
	host := PluginHostInfo{Version: "0.1.179", BuildType: "release"}

	result := EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.True(t, result.Tested)
	assert.Equal(t, "compatible", result.Status)

	manifest.Requires.TestedSub2APIVersions = []string{"0.1.178"}
	result = EvaluatePluginCompatibility(manifest, host)
	require.True(t, result.Compatible)
	assert.False(t, result.Tested)
	assert.Equal(t, "untested", result.Status)

	manifest.Requires.Sub2API = ">=0.2.0 <0.3.0"
	result = EvaluatePluginCompatibility(manifest, host)
	assert.False(t, result.Compatible)
	assert.Equal(t, "incompatible", result.Status)
}

func TestEvaluatePluginCompatibilityRejectsProtocolMismatch(t *testing.T) {
	manifest := testPluginManifest(nil)
	manifest.Requires.PluginProtocol = pluginv1.ProtocolVersion + 1

	result := EvaluatePluginCompatibility(manifest, PluginHostInfo{Version: "0.1.179"})

	assert.False(t, result.Compatible)
	assert.Equal(t, "incompatible", result.Status)
}

func TestMatchesSemverRange(t *testing.T) {
	assert.True(t, matchesSemverRange("0.1.179", ">=0.1.170, <0.2.0"))
	assert.True(t, matchesSemverRange("v1.2.3", "=1.2.3"))
	assert.False(t, matchesSemverRange("0.1.169", ">=0.1.170 <0.2.0"))
	assert.False(t, matchesSemverRange("dev", ">=0.1.0"))
	assert.False(t, matchesSemverRange("0.1.179", "^0.1.0"))
}

func TestEvaluatePluginCompatibilityForkHost(t *testing.T) {
	for _, tc := range []struct {
		name       string
		version    string
		tested     []string
		compatible bool
		isTested   bool
	}{
		{"upstream", "0.2.7", []string{"0.2.7"}, true, true},
		{"fork baseline", "0.2.7.0", []string{"0.2.7"}, true, false},
		{"fork revision", "v0.2.7.12", []string{"0.2.7"}, true, false},
		{"tested fork", "0.2.7.12", []string{" v0.2.7.12 "}, true, true},
		{"different fork revision", "0.2.7.12", []string{"0.2.7.11"}, true, false},
		{"invalid tested version", "0.2.7.0", []string{"invalid", "0.2.7.00"}, true, false},
		{"older baseline", "0.2.6.99", []string{"0.2.6.99"}, false, false},
		{"upper bound", "0.3.0.0", []string{"0.3.0.0"}, false, false},
		{"upstream prerelease", "0.2.7-rc.1", nil, false, false},
		{"invalid revision", "0.2.7.01", nil, false, false},
		{"extra component", "0.2.7.0.1", nil, false, false},
		{"unsupported fork prerelease", "0.2.7.0-rc.1", nil, false, false},
		{"development build", "dev", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := testPluginManifest(nil)
			manifest.Requires.Sub2API = ">=0.2.7 <0.3.0"
			manifest.Requires.TestedSub2APIVersions = tc.tested
			result := EvaluatePluginCompatibility(manifest, PluginHostInfo{Version: tc.version})
			assert.Equal(t, tc.compatible, result.Compatible)
			assert.Equal(t, tc.isTested, result.Tested)
			assert.Equal(t, tc.version, result.CurrentSub2API)
		})
	}
}

func TestEvaluatePluginCompatibilityForkProtocolRequirements(t *testing.T) {
	for _, protocol := range []string{"plugin", "transport", "ui"} {
		t.Run(protocol, func(t *testing.T) {
			manifest := testPluginManifest(nil)
			manifest.Requires.Sub2API = ">=0.2.7 <0.3.0"
			switch protocol {
			case "plugin":
				manifest.Requires.PluginProtocol++
			case "transport":
				manifest.Requires.TransportAPI++
			case "ui":
				manifest.Requires.UIBridge++
			}
			result := EvaluatePluginCompatibility(manifest, PluginHostInfo{Version: "0.2.7.0"})
			assert.False(t, result.Compatible)
			assert.Equal(t, "插件协议版本与当前 Sub2API 不兼容", result.Message)
		})
	}
}

func TestPluginManagerRefreshesForkCompatibilityForDisplay(t *testing.T) {
	for _, tc := range []struct {
		name, version, state, wantState string
		enabled                         bool
	}{
		{"compatible after upgrade", "0.2.7.0", PluginStateIncompatible, PluginStateDisabled, false},
		{"outside range", "0.3.0.0", PluginStateIncompatible, PluginStateIncompatible, false},
		{"runtime error", "0.2.7.0", PluginStateError, PluginStateError, false},
		{"active binding", "0.2.7.0", PluginStateIncompatible, PluginStateIncompatible, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := testPluginManifest(nil)
			manifest.Requires.Sub2API = ">=0.2.7 <0.3.0"
			repo := &pluginTokenRepository{installation: &PluginInstallation{
				ID: 7, State: tc.state, Manifest: manifest, LastError: "previous error",
				Bindings: []PluginBinding{{Capability: PluginCapabilityOpenAIOAuthOutbound, Platform: PlatformOpenAI, AccountType: AccountTypeOAuth, Enabled: tc.enabled}},
			}}
			manager := &PluginManager{repo: repo, hostInfo: PluginHostInfo{Version: tc.version}}
			listed, err := manager.List(context.Background())
			require.NoError(t, err)
			detail, err := manager.Get(context.Background(), 7)
			require.NoError(t, err)
			for _, result := range []*PluginInstallation{listed[0], detail} {
				assert.Equal(t, tc.wantState, result.State)
				assert.Equal(t, tc.version, result.Compatibility.CurrentSub2API)
				assert.NotEmpty(t, result.Compatibility.CompatibilitySub2API)
				assert.False(t, result.RuntimeHealthy)
				if tc.wantState == PluginStateDisabled {
					assert.Empty(t, result.LastError)
				} else {
					assert.Equal(t, "previous error", result.LastError)
				}
			}
			assert.Equal(t, tc.state, repo.installation.State)
			assert.Equal(t, "previous error", repo.installation.LastError)
		})
	}
}

type pluginCompatibilityGateRepository struct {
	pluginTokenRepository
	stateUpdates int
}

func (r *pluginCompatibilityGateRepository) UpdateState(context.Context, int64, string, string, *time.Time, string, string) error {
	r.stateUpdates++
	return nil
}

func TestPluginManagerForkHostKeepsUntestedEnableGate(t *testing.T) {
	manifest := testPluginManifest(nil)
	manifest.Requires.Sub2API = ">=0.2.7 <0.3.0"
	manifest.Requires.TestedSub2APIVersions = []string{"0.2.7"}
	repo := &pluginCompatibilityGateRepository{pluginTokenRepository: pluginTokenRepository{
		installation: &PluginInstallation{ID: 7, State: PluginStateIncompatible, Manifest: manifest},
	}}
	manager := &PluginManager{repo: repo, hostInfo: PluginHostInfo{Version: "0.2.7.0"}}

	_, err := manager.Enable(context.Background(), 7, false, 100)

	require.ErrorContains(t, err, "插件未声明已测试当前 Sub2API 版本")
	assert.Zero(t, repo.stateUpdates)
}
