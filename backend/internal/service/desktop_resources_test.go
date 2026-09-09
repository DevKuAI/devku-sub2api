package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func resourceZIP(t *testing.T, mutate func(*DesktopResourceManifest, map[string][]byte), duplicate bool) []byte {
	t.Helper()
	files := map[string][]byte{"bin/tool": []byte("fixture")}
	m := DesktopResourceManifest{SchemaVersion: 1, Key: "devku_image", Kind: "mcp", Name: "Image", Version: "0.1.0", Platform: "darwin-arm64", Entry: "bin/tool", Targets: []string{"workbuddy", "chatgpt_codex"}}
	if mutate != nil {
		mutate(&m, files)
	}
	if m.Files == nil {
		m.Files = map[string]string{}
		for n, b := range files {
			m.Files[n] = fmt.Sprintf("%x", sha256.Sum256(b))
		}
	}
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	files["manifest.json"] = raw
	for n, b := range files {
		f, err := w.Create(n)
		require.NoError(t, err)
		_, err = f.Write(b)
		require.NoError(t, err)
	}
	if duplicate {
		_, err = w.Create("bin/tool")
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return out.Bytes()
}
func TestDesktopResourcePackageValidation(t *testing.T) {
	_, hash, err := ValidateDesktopResourcePackage(resourceZIP(t, nil, false))
	require.NoError(t, err)
	require.Len(t, hash, 64)
	for _, name := range []string{"../escape", "C:/escape", "BIN/TOOL", "bin/CON.exe", "bin/tool ", "bin", "/absolute", "bin\\tool"} {
		t.Run(name, func(t *testing.T) {
			_, _, err := ValidateDesktopResourcePackage(resourceZIP(t, func(m *DesktopResourceManifest, f map[string][]byte) { f[name] = []byte("x") }, false))
			require.ErrorIs(t, err, ErrDesktopResourceInvalid)
		})
	}
	cases := []func(*DesktopResourceManifest, map[string][]byte){
		func(m *DesktopResourceManifest, f map[string][]byte) { m.Files = map[string]string{"bin/tool": "bad"} },
		func(m *DesktopResourceManifest, f map[string][]byte) { m.Files = map[string]string{} },
		func(m *DesktopResourceManifest, f map[string][]byte) { m.Entry = "missing" },
		func(m *DesktopResourceManifest, f map[string][]byte) { m.Platform = "darwin-any" },
		func(m *DesktopResourceManifest, f map[string][]byte) { m.CredentialMode = "api_key" },
		func(m *DesktopResourceManifest, f map[string][]byte) { m.Targets = []string{"workbuddy", "workbuddy"} },
		func(m *DesktopResourceManifest, f map[string][]byte) {
			m.Kind = "skill"
			m.Platform = "any"
			m.Entry = "skill/SKILL.md"
			f[m.Entry] = []byte("# Skill")
		},
	}
	for _, mutate := range cases {
		_, _, err := ValidateDesktopResourcePackage(resourceZIP(t, mutate, false))
		require.ErrorIs(t, err, ErrDesktopResourceInvalid)
	}
	_, _, err = ValidateDesktopResourcePackage(resourceZIP(t, nil, true))
	require.ErrorIs(t, err, ErrDesktopResourceInvalid)
	_, _, err = ValidateDesktopResourcePackage([]byte("not zip"))
	require.ErrorIs(t, err, ErrDesktopResourceInvalid)
	_, _, err = ValidateDesktopResourcePackage(resourceZIP(t, func(m *DesktopResourceManifest, f map[string][]byte) {
		delete(f, "bin/tool")
		m.Kind = "skill"
		m.Platform = "any"
		m.Entry = "skill/SKILL.md"
		f[m.Entry] = []byte("---\nname: sample\ndescription: sample\n---\n# Skill")
	}, false))
	require.NoError(t, err)
}

type resourceRepoSpy struct{ org int64 }

func (r *resourceRepoSpy) List(_ context.Context, org int64, q DesktopResourceQuery) ([]DesktopResource, error) {
	r.org = org
	return []DesktopResource{}, nil
}
func (r *resourceRepoSpy) GetArtifact(_ context.Context, org int64, _, _, _ string) (*DesktopResourceArtifact, []byte, error) {
	r.org = org
	return nil, nil, ErrDesktopResourceNotFound
}
func (r *resourceRepoSpy) Publish(context.Context, *int64, *int64, DesktopResourceManifest, string, []byte) (*DesktopResource, error) {
	panic("not called")
}
func TestDesktopResourceUsesAuthorizedOrganization(t *testing.T) {
	repo := &resourceRepoSpy{}
	s := NewDesktopResourceService(repo)
	member := &DesktopAuthorizedMember{Organization: &DesktopOrganization{ID: 42}, Member: &DesktopMember{ID: 7}}
	_, err := s.List(context.Background(), member, DesktopResourceQuery{Scope: "enterprise"})
	require.NoError(t, err)
	require.EqualValues(t, 42, repo.org)
	_, err = s.List(context.Background(), member, DesktopResourceQuery{Scope: "visible"})
	require.Error(t, err)
	_, err = s.List(context.Background(), nil, DesktopResourceQuery{Scope: "public"})
	require.ErrorIs(t, err, ErrDesktopUnauthenticated)
	_, _, err = s.Download(context.Background(), member, "res_other", "0.1.0", "darwin-arm64")
	require.ErrorIs(t, err, ErrDesktopResourceNotFound)
	require.EqualValues(t, 42, repo.org)
}
