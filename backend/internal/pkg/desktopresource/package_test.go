package desktopresource

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixtureManifest(kind string) Manifest {
	sum := sha256.Sum256([]byte("resource payload"))
	m := Manifest{SchemaVersion: 1, Key: "test_resource", Kind: kind, Name: "Resource", Version: "1.2.3", Platform: "darwin-arm64", Entry: "bin/server", Targets: []string{"chatgpt_codex"}}
	if kind == "skill" {
		m.Platform = "any"
		m.Entry = "skill/SKILL.md"
	}
	m.Files = map[string]string{m.Entry: hex.EncodeToString(sum[:])}
	return m
}

func makeArchive(t *testing.T, raw []byte, entries []zip.FileHeader, data map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	manifest, err := w.Create("manifest.json")
	require.NoError(t, err)
	_, err = manifest.Write(raw)
	require.NoError(t, err)
	for _, header := range entries {
		entry, err := w.CreateHeader(&header)
		require.NoError(t, err)
		_, err = entry.Write([]byte(data[header.Name]))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	filename := filepath.Join(t.TempDir(), "resource.zip")
	require.NoError(t, os.WriteFile(filename, buf.Bytes(), 0600))
	return filename
}

func TestInspectValidPackages(t *testing.T) {
	for _, kind := range []string{"mcp", "skill"} {
		t.Run(kind, func(t *testing.T) {
			m := fixtureManifest(kind)
			raw, err := json.Marshal(m)
			require.NoError(t, err)
			file := makeArchive(t, raw, []zip.FileHeader{{Name: m.Entry, Method: zip.Deflate}}, map[string]string{m.Entry: "resource payload"})
			pkg, err := Inspect(file)
			require.NoError(t, err)
			data, err := os.ReadFile(file)
			require.NoError(t, err)
			sum := sha256.Sum256(data)
			require.Equal(t, hex.EncodeToString(sum[:]), pkg.SHA256)
			require.Equal(t, int64(len(data)), pkg.SizeBytes)
			require.Equal(t, m, pkg.Manifest)
		})
	}
}

func TestInspectRejectsUnsafeArchives(t *testing.T) {
	m := fixtureManifest("mcp")
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	symlink := zip.FileHeader{Name: "link"}
	symlink.SetMode(os.ModeSymlink | 0777)
	tests := []struct {
		name    string
		raw     []byte
		extra   []zip.FileHeader
		payload string
	}{
		{"hash mismatch", raw, nil, "changed"},
		{"unknown field", bytes.Replace(raw, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":1,"unknown":true`), 1), nil, "resource payload"},
		{"duplicate JSON key", bytes.Replace(raw, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":1,"schemaVersion":1`), 1), nil, "resource payload"},
		{"missing description", bytes.Replace(raw, []byte(`"description":"",`), nil, 1), nil, "resource payload"},
		{"null args", bytes.Replace(raw, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":1,"args":null`), 1), nil, "resource payload"},
		{"trailing JSON", append(append([]byte{}, raw...), []byte(`{}`)...), nil, "resource payload"},
		{"traversal", raw, []zip.FileHeader{{Name: "../escape"}}, "resource payload"},
		{"absolute", raw, []zip.FileHeader{{Name: "/escape"}}, "resource payload"},
		{"Windows drive", raw, []zip.FileHeader{{Name: "C:/escape"}}, "resource payload"},
		{"Windows reserved", raw, []zip.FileHeader{{Name: "NUL.txt"}}, "resource payload"},
		{"backslash", raw, []zip.FileHeader{{Name: `bin\escape`}}, "resource payload"},
		{"symlink", raw, []zip.FileHeader{symlink}, "resource payload"},
		{"undeclared", raw, []zip.FileHeader{{Name: "extra"}}, "resource payload"},
		{"case duplicate", raw, []zip.FileHeader{{Name: "BIN/server"}}, "resource payload"},
		{"duplicate manifest", raw, []zip.FileHeader{{Name: "manifest.json"}}, "resource payload"},
		{"parent file collision", raw, []zip.FileHeader{{Name: "bin"}}, "resource payload"},
		{"oversize manifest", []byte(strings.Repeat(" ", int(MaxManifestBytes)+1)), nil, "resource payload"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries := append([]zip.FileHeader{{Name: m.Entry}}, test.extra...)
			_, err := Inspect(makeArchive(t, test.raw, entries, map[string]string{m.Entry: test.payload}))
			require.Error(t, err)
		})
	}
}

func TestManifestRules(t *testing.T) {
	tests := map[string]func(*Manifest){
		"unsupported schema": func(m *Manifest) { m.SchemaVersion = 2 },
		"key":                func(m *Manifest) { m.Key = "../escape" },
		"leading zero":       func(m *Manifest) { m.Version = "01.2.3" },
		"version digits":     func(m *Manifest) { m.Version = "1000000.2.3" },
		"prerelease":         func(m *Manifest) { m.Version = "1.2.3-beta" },
		"blank name":         func(m *Manifest) { m.Name = "  " },
		"byte limit":         func(m *Manifest) { m.Name = strings.Repeat("中", 67) },
		"source credentials": func(m *Manifest) { m.SourceURL = "https://user:pass@example.com" },
		"source scheme":      func(m *Manifest) { m.SourceURL = "http://example.com" },
		"MCP any":            func(m *Manifest) { m.Platform = "any" },
		"missing entry":      func(m *Manifest) { m.Entry = "bin/missing" },
		"args":               func(m *Manifest) { m.Args = []string{"\x00"} },
		"targets":            func(m *Manifest) { m.Targets = []string{"chatgpt_codex", "chatgpt_codex"} },
		"credential mode":    func(m *Manifest) { m.CredentialMode = "token" },
		"Skill entry":        func(m *Manifest) { m.Kind = "skill"; m.Platform = "any" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) { m := fixtureManifest("mcp"); change(&m); require.Error(t, m.Validate()) })
	}
	for _, change := range []func(*Manifest){func(m *Manifest) { m.Args = []string{"run"} }, func(m *Manifest) { m.CredentialMode = "enterprise_model" }, func(m *Manifest) { m.Files["outside.txt"] = strings.Repeat("a", 64) }} {
		m := fixtureManifest("skill")
		change(&m)
		require.Error(t, m.Validate())
	}
}

func TestArchiveMetadataLimits(t *testing.T) {
	for _, files := range [][]*zip.File{
		make([]*zip.File, MaxEntries+1),
		{{FileHeader: zip.FileHeader{Name: "big", UncompressedSize64: uint64(MaxZIPBytes) + 1}}},
		{{FileHeader: zip.FileHeader{Name: "a", UncompressedSize64: uint64(MaxZIPBytes)}}, {FileHeader: zip.FileHeader{Name: "b", UncompressedSize64: uint64(MaxZIPBytes)}}, {FileHeader: zip.FileHeader{Name: "c", UncompressedSize64: 1}}},
	} {
		_, err := archiveEntries(&zip.Reader{File: files})
		require.Error(t, err)
	}
}
