package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresource"
	"github.com/stretchr/testify/require"
)

type resourceRepositoryStub struct {
	DesktopResourceRepository
	writes   int
	err      error
	artifact *DesktopResourceArtifact
}

func (r *resourceRepositoryStub) Publish(_ context.Context, p desktopresource.Package, a DesktopResourceArtifact, actor int64) (*DesktopResourceRecord, error) {
	r.writes++
	r.artifact = &a
	return &DesktopResourceRecord{DesktopResource: DesktopResource{Key: p.Manifest.Key, Artifacts: []DesktopResourceArtifact{a}}, CreatedBy: actor}, r.err
}
func (r *resourceRepositoryStub) List(context.Context, DesktopResourceFilter) ([]DesktopResourceRecord, int64, error) {
	return []DesktopResourceRecord{}, 0, nil
}
func (r *resourceRepositoryStub) Artifact(context.Context, string, string, string) (*DesktopResourceArtifact, error) {
	return r.artifact, r.err
}

type resourceStorageStub struct {
	expiry time.Duration
	signs  int
	writes int
	data   []byte
	key    string
	err    error
}

func (s *resourceStorageStub) Upload(_ context.Context, key string, r io.Reader, _ int64) error {
	s.writes++
	s.key = key
	data, err := io.ReadAll(r)
	s.data = data
	if err != nil {
		return err
	}
	return s.err
}
func (s *resourceStorageStub) Open(context.Context, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(bytes.NewReader(s.data)), int64(len(s.data)), s.err
}

func resourceZIP(t *testing.T) []byte {
	t.Helper()
	payload := []byte("tool")
	sum := sha256.Sum256(payload)
	m := desktopresource.Manifest{SchemaVersion: 1, Key: "example", Kind: "mcp", Name: "Example", Version: "1.0.0", Platform: "darwin-arm64", Entry: "bin/tool", Targets: []string{"chatgpt_codex"}, Files: map[string]string{"bin/tool": hex.EncodeToString(sum[:])}}
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range map[string][]byte{"manifest.json": raw, "bin/tool": payload} {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return b.Bytes()
}

func TestDesktopResourcePublicationBoundaries(t *testing.T) {
	for _, scenario := range []string{"success", "invalid", "storage", "database", "read-limit"} {
		t.Run(scenario, func(t *testing.T) {
			repo := &resourceRepositoryStub{}
			storage := &resourceStorageStub{}
			svc := NewDesktopResourceService(repo, storage, &config.Config{DesktopUpdateStorage: config.DesktopUpdateStorageConfig{Prefix: "existing/"}})
			data := resourceZIP(t)
			var body io.Reader = bytes.NewReader(data)
			switch scenario {
			case "invalid":
				body = strings.NewReader("not a zip")
			case "storage":
				storage.err = errors.New("storage down")
			case "database":
				repo.err = errors.New("ambiguous commit")
			case "read-limit":
				body = iotest.ErrReader(&http.MaxBytesError{Limit: desktopresource.MaxZIPBytes})
			}
			_, err := svc.Publish(context.Background(), body, 42)
			if scenario == "success" {
				require.NoError(t, err)
				require.Equal(t, data, storage.data)
				require.Contains(t, storage.key, "existing/resources/example/1.0.0/darwin-arm64/")
			} else {
				require.Error(t, err)
			}
			if scenario == "invalid" || scenario == "read-limit" {
				require.Zero(t, storage.writes)
			}
			if scenario != "success" && scenario != "database" {
				require.Zero(t, repo.writes)
			}
			if scenario == "read-limit" {
				require.ErrorIs(t, err, ErrResourceTooLarge)
			}
		})
	}
}

func TestDesktopResourceValidationDoesNotPublish(t *testing.T) {
	repo := &resourceRepositoryStub{}
	storage := &resourceStorageStub{}
	svc := NewDesktopResourceService(repo, storage, nil)
	result, err := svc.Validate(context.Background(), bytes.NewReader(resourceZIP(t)))
	require.NoError(t, err)
	require.Nil(t, result.Current)
	require.Zero(t, repo.writes)
	require.Zero(t, storage.writes)
}

func TestDesktopResourceDownloadChecksStorageLength(t *testing.T) {
	repo := &resourceRepositoryStub{artifact: &DesktopResourceArtifact{SizeBytes: 4, ObjectKey: "key"}}
	storage := &resourceStorageStub{data: []byte("bad")}
	svc := NewDesktopResourceService(repo, storage, nil)
	_, _, err := svc.Download(context.Background(), "id", "1.0.0", "darwin-arm64")
	require.ErrorIs(t, err, ErrResourceStorage)
	repo.err = ErrResourceNotFound
	_, _, err = svc.Download(context.Background(), "id", "1.0.0", "darwin-arm64")
	require.ErrorIs(t, err, ErrResourceNotFound)
}

func TestDesktopResourcePublicDTOAndStatusValidation(t *testing.T) {
	r := DesktopResourceRecord{DesktopResource: DesktopResource{Artifacts: []DesktopResourceArtifact{{ObjectKey: "private-key", CreatedBy: 1}}}}
	public := r.Public()
	require.Empty(t, public.Artifacts[0].ObjectKey)
	require.Zero(t, public.Artifacts[0].CreatedBy)
	require.NotEmpty(t, r.Artifacts[0].ObjectKey)
	svc := NewDesktopResourceService(nil, nil, nil)
	for _, status := range []string{"disabled", "unknown"} {
		_, err := svc.SetStatus(context.Background(), "id", status, "", 1)
		require.ErrorIs(t, err, ErrResourceValidation)
	}
}

func (s *resourceStorageStub) Presign(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.key = key
	s.expiry = ttl
	s.signs++
	return "https://private.example.com/file.zip?signature=example", s.err
}

func TestDesktopResourceDownloadURLRequiresVisibleArtifact(t *testing.T) {
	for _, scenario := range []string{"success", "disabled", "invalid", "storage"} {
		t.Run(scenario, func(t *testing.T) {
			repo := &resourceRepositoryStub{artifact: &DesktopResourceArtifact{ObjectKey: "immutable-key", SizeBytes: 12, SHA256: strings.Repeat("b", 64)}}
			storage := &resourceStorageStub{}
			version := "1.0.0"
			switch scenario {
			case "disabled":
				repo.err = ErrResourceNotFound
			case "invalid":
				version = "../bad"
			case "storage":
				storage.err = errors.New("signing failed")
			}
			svc := NewDesktopResourceService(repo, storage, nil)
			before := time.Now().UTC().Truncate(time.Second)
			link, err := svc.DownloadURL(context.Background(), "res_one", version, "darwin-arm64")
			if scenario == "success" {
				require.NoError(t, err)
				require.Equal(t, 300, link.ExpiresIn)
				require.WithinDuration(t, before.Add(5*time.Minute), link.ExpiresAt, time.Second)
				require.Equal(t, repo.artifact.SHA256, link.SHA256)
				require.EqualValues(t, 12, link.SizeBytes)
				require.Equal(t, 5*time.Minute, storage.expiry)
				require.Equal(t, "immutable-key", storage.key)
			} else {
				require.Error(t, err)
				require.Nil(t, link)
			}
			if scenario == "disabled" || scenario == "invalid" {
				require.Zero(t, storage.signs)
			}
		})
	}
}
