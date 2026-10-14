package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type desktopUpdateRepoStub struct {
	mu       sync.Mutex
	releases []DesktopUpdateRelease
	listErr  error
}

func (r *desktopUpdateRepoStub) Create(_ context.Context, release *DesktopUpdateRelease) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.releases = append(r.releases, *release)
	return nil
}

func (r *desktopUpdateRepoStub) List(_ context.Context, params pagination.PaginationParams, _ DesktopUpdateListFilters) ([]DesktopUpdateRelease, *pagination.PaginationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := append([]DesktopUpdateRelease(nil), r.releases...)
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize}, nil
}

func (r *desktopUpdateRepoStub) Get(_ context.Context, publicID string) (*DesktopUpdateRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.releases {
		if r.releases[i].PublicID == publicID {
			item := r.releases[i]
			return &item, nil
		}
	}
	return nil, ErrDesktopUpdateNotFound
}

func (r *desktopUpdateRepoStub) UpdateDraft(_ context.Context, publicID string, input DesktopUpdateDraftInput) (*DesktopUpdateRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.releases {
		if r.releases[i].PublicID == publicID {
			r.releases[i].Version = input.Version
			r.releases[i].Notes = input.Notes
			r.releases[i].Artifacts = input.Artifacts
			item := r.releases[i]
			return &item, nil
		}
	}
	return nil, ErrDesktopUpdateNotFound
}

func (r *desktopUpdateRepoStub) Publish(_ context.Context, publicID string, actorID int64, publishedAt time.Time, options ...DesktopUpdatePublishInput) (*DesktopUpdateRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.releases {
		if r.releases[i].PublicID == publicID {
			r.releases[i].Status = DesktopUpdateStatusPublished
			r.releases[i].PublishedAt = &publishedAt
			if actorID > 0 {
				r.releases[i].PublishedBy = &actorID
			}
			item := r.releases[i]
			return &item, nil
		}
	}
	return nil, ErrDesktopUpdateNotFound
}

func (r *desktopUpdateRepoStub) Withdraw(_ context.Context, publicID string, actorID int64, reason string, withdrawnAt time.Time) (*DesktopUpdateRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.releases {
		if r.releases[i].PublicID == publicID {
			r.releases[i].Status = DesktopUpdateStatusWithdrawn
			r.releases[i].WithdrawnAt = &withdrawnAt
			r.releases[i].WithdrawalReason = &reason
			if actorID > 0 {
				r.releases[i].WithdrawnBy = &actorID
			}
			item := r.releases[i]
			return &item, nil
		}
	}
	return nil, ErrDesktopUpdateNotFound
}

func (r *desktopUpdateRepoStub) ListPublished(_ context.Context) ([]DesktopUpdateRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	items := make([]DesktopUpdateRelease, 0, len(r.releases))
	for i := range r.releases {
		if r.releases[i].Status == DesktopUpdateStatusPublished {
			items = append(items, r.releases[i])
		}
	}
	return items, nil
}

func desktopUpdateTestArtifacts(prefix string) DesktopUpdateArtifacts {
	return DesktopUpdateArtifacts{
		DesktopUpdateDarwinARM64: desktopUpdateTestArtifact(prefix, "darwin-arm64.dmg", "arm-signature"),
		DesktopUpdateDarwinX64:   desktopUpdateTestArtifact(prefix, "darwin-x64.app", "x64-signature"),
		DesktopUpdateWindowsX64:  desktopUpdateTestArtifact(prefix, "windows-x64.exe", "windows-signature"),
	}
}

func TestNormalizeDesktopUpdateArtifactFileNameDoesNotRestrictSuffix(t *testing.T) {
	for _, fileName := range []string{
		"DevKu.dmg",
		"DevKu.app",
		"DevKu.msi",
		"DevKu.exe",
		"DevKu.custom",
	} {
		t.Run(fileName, func(t *testing.T) {
			normalized, err := normalizeDesktopUpdateArtifactFileName(fileName)
			require.NoError(t, err)
			require.Equal(t, fileName, normalized)
		})
	}
}

func desktopUpdateTestArtifact(prefix, fileName, signature string) DesktopUpdateArtifact {
	objectKey := "desktop-updates/" + prefix + "/" + fileName
	return DesktopUpdateArtifact{
		URL: "https://example.com/" + objectKey, Signature: signature, ObjectKey: objectKey,
		FileName: fileName, Size: 1234, SHA256: strings.Repeat("a", 64),
	}
}

func TestNormalizeDesktopUpdateVersionUsesStrictTriplets(t *testing.T) {
	for _, version := range []string{"0.0.0", "1.2.3", "10.20.30"} {
		normalized, err := NormalizeDesktopUpdateVersion(version)
		require.NoError(t, err)
		require.Equal(t, version, normalized)
	}

	for _, version := range []string{"v1.2.3", "1.2", "1.2.3-beta.1", "01.2.3", "1.2.3+build"} {
		_, err := NormalizeDesktopUpdateVersion(version)
		require.Error(t, err, version)
		require.Equal(t, "DESKTOP_UPDATE_VERSION_INVALID", infraerrors.Reason(err))
	}
}

func TestNormalizeDesktopUpdateArtifactsAllowsAvailablePlatformsAndRequiresHTTPS(t *testing.T) {
	artifacts := desktopUpdateTestArtifacts("1.2.3")
	normalized, err := NormalizeDesktopUpdateArtifacts(artifacts)
	require.NoError(t, err)
	require.Equal(t, artifacts, normalized)

	delete(artifacts, DesktopUpdateWindowsX64)
	_, err = NormalizeDesktopUpdateArtifacts(artifacts)
	require.NoError(t, err)
	_, err = NormalizeDesktopUpdateArtifacts(DesktopUpdateArtifacts{})
	require.Error(t, err)

	artifacts = desktopUpdateTestArtifacts("1.2.3")
	invalid := artifacts[DesktopUpdateDarwinARM64]
	invalid.URL = "http://example.com/app.tar.gz"
	artifacts[DesktopUpdateDarwinARM64] = invalid
	_, err = NormalizeDesktopUpdateArtifacts(artifacts)
	require.Equal(t, "DESKTOP_UPDATE_FIELDS_INVALID", infraerrors.Reason(err))
}

type desktopUpdateArtifactStorageStub struct {
	body []byte
	key  string
}

func (s *desktopUpdateArtifactStorageStub) Upload(_ context.Context, key, _ string, body io.Reader, _ int64) (string, error) {
	s.key = key
	data, err := io.ReadAll(body)
	s.body = data
	return "https://downloads.example.com/" + key, err
}

func TestDesktopUpdateUploadArtifactStreamsToConfiguredOSS(t *testing.T) {
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{{
		PublicID: "upd_one", Version: "1.2.3", Status: DesktopUpdateStatusDraft, Artifacts: desktopUpdateTestArtifacts("1.2.3"),
	}}}
	storage := &desktopUpdateArtifactStorageStub{}
	svc := NewDesktopUpdateService(repo)
	svc.storageConfig = config.DesktopUpdateStorageConfig{
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Region: "cn-hangzhou", Bucket: "desktop-releases",
		AccessKeyID: "ak", SecretAccessKey: "sk", Prefix: "desktop-updates/", PublicBaseURL: "https://downloads.example.com", MaxUploadBytes: 2048,
	}
	svc.storageFactory = func(context.Context, *config.DesktopUpdateStorageConfig) (DesktopUpdateArtifactStorage, error) {
		return storage, nil
	}

	payload := []byte("desktop artifact")
	artifact, err := svc.UploadArtifact(context.Background(), "upd_one", DesktopUpdateDarwinARM64, "DevKu.app", "application/octet-stream", int64(len(payload)), bytes.NewReader(payload))
	require.NoError(t, err)
	require.Equal(t, payload, storage.body)
	require.Contains(t, storage.key, "desktop-updates/1.2.3/darwin-aarch64/artifact_")
	require.Equal(t, "DevKu.app", artifact.FileName)
	require.Equal(t, int64(len(payload)), artifact.Size)
	require.Equal(t, HashDesktopOpaqueToken(string(payload)), artifact.SHA256)
}

func TestDesktopUpdateUploadArtifactRejectsPublishedRelease(t *testing.T) {
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{{PublicID: "upd_one", Version: "1.2.3", Status: DesktopUpdateStatusPublished}}}
	svc := NewDesktopUpdateService(repo)

	_, err := svc.UploadArtifact(context.Background(), "upd_one", DesktopUpdateDarwinARM64, "DevKu.dmg", "application/x-apple-diskimage", 10, bytes.NewReader(make([]byte, 10)))
	require.Equal(t, "DESKTOP_UPDATE_STATE_INVALID", infraerrors.Reason(err))
}

func TestDesktopUpdateUploadRejectsOlderArtifactBeforeReadingBody(t *testing.T) {
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{{
		PublicID: "upd_one", Version: "0.3.4", Status: DesktopUpdateStatusDraft,
	}}}
	svc := NewDesktopUpdateService(repo)
	payload := bytes.NewReader([]byte("old installer"))
	_, err := svc.UploadArtifact(context.Background(), "upd_one", DesktopUpdateWindowsX64,
		"Devku.Desktop_0.3.3_x64-setup.nsis.zip", "application/zip", int64(payload.Len()), payload)
	require.Equal(t, "DESKTOP_UPDATE_ARTIFACT_VERSION_MISMATCH", infraerrors.Reason(err))
	require.Equal(t, len("old installer"), payload.Len())
}

func TestDesktopUpdateDraftAndPublishRejectVersionMismatch(t *testing.T) {
	artifacts := desktopUpdateTestArtifacts("0.3.4")
	artifacts[DesktopUpdateWindowsX64] = desktopUpdateTestArtifact("0.3.4", "Devku.Desktop_0.3.3_x64-setup.nsis.zip", "signature")
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{{
		PublicID: "upd_one", Version: "0.3.4", Status: DesktopUpdateStatusDraft, Artifacts: artifacts,
	}}}
	svc := NewDesktopUpdateService(repo)
	_, err := svc.CreateDraft(context.Background(), DesktopUpdateDraftInput{Version: "0.3.4", Artifacts: artifacts})
	require.Equal(t, "DESKTOP_UPDATE_ARTIFACT_VERSION_MISMATCH", infraerrors.Reason(err))
	_, err = svc.UpdateDraft(context.Background(), "upd_one", DesktopUpdateDraftInput{Version: "0.3.4", Artifacts: artifacts})
	require.Equal(t, "DESKTOP_UPDATE_ARTIFACT_VERSION_MISMATCH", infraerrors.Reason(err))
	_, err = svc.Publish(context.Background(), "upd_one", 7)
	require.Equal(t, "DESKTOP_UPDATE_ARTIFACT_VERSION_MISMATCH", infraerrors.Reason(err))
	require.Equal(t, DesktopUpdateStatusDraft, repo.releases[0].Status)
}

func TestDesktopUpdateCheckDoesNotAdvertiseMismatchedArtifact(t *testing.T) {
	artifacts := desktopUpdateTestArtifacts("0.3.4")
	artifacts[DesktopUpdateWindowsX64] = desktopUpdateTestArtifact("0.3.4", "Devku.Desktop_0.3.3_x64-setup.nsis.zip", "signature")
	publishedAt := time.Now().UTC()
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{{
		Version: "0.3.4", Status: DesktopUpdateStatusPublished, PublishedAt: &publishedAt, Artifacts: artifacts,
	}}}
	svc := NewDesktopUpdateService(repo)
	result, available, err := svc.Check(context.Background(), "windows", "x86_64", "0.3.3")
	require.Equal(t, "DESKTOP_UPDATE_METADATA_INVALID", infraerrors.Reason(err))
	require.False(t, available)
	require.Nil(t, result)
	artifacts[DesktopUpdateWindowsX64] = desktopUpdateTestArtifact("0.3.4", "Devku.Desktop_0.3.4_x64-setup.nsis.zip", "signature")
	result, available, err = svc.Check(context.Background(), "windows", "x86_64", "0.3.3")
	require.NoError(t, err)
	require.True(t, available)
	require.Equal(t, "0.3.4", result.Version)
}

func TestDesktopUpdateDraftAllowsIncompleteArtifactsUntilPublish(t *testing.T) {
	artifacts := DesktopUpdateArtifacts{
		DesktopUpdateDarwinARM64: {},
		DesktopUpdateDarwinX64:   {},
		DesktopUpdateWindowsX64:  {},
	}
	normalized, err := normalizeDesktopUpdateDraftArtifacts(artifacts)
	require.NoError(t, err)
	require.Equal(t, artifacts, normalized)

	_, err = NormalizeDesktopUpdateArtifacts(artifacts)
	require.Equal(t, "DESKTOP_UPDATE_FIELDS_INVALID", infraerrors.Reason(err))
}

func TestDesktopUpdateDraftRejectsArtifactMetadataOutsideConfiguredStorage(t *testing.T) {
	svc := NewDesktopUpdateService(&desktopUpdateRepoStub{})
	svc.storageConfig = config.DesktopUpdateStorageConfig{
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Region: "cn-hangzhou", Bucket: "desktop-releases",
		AccessKeyID: "ak", SecretAccessKey: "sk", Prefix: "desktop-updates/", PublicBaseURL: "https://downloads.example.com", MaxUploadBytes: 2048,
	}
	artifacts := desktopUpdateTestArtifacts("1.2.3")

	_, err := svc.CreateDraft(context.Background(), DesktopUpdateDraftInput{Version: "1.2.3", Artifacts: artifacts})
	require.Equal(t, "DESKTOP_UPDATE_FIELDS_INVALID", infraerrors.Reason(err))
}

func TestDesktopUpdateCheckSelectsLatestAndMapsPlatforms(t *testing.T) {
	publishedAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{
		{PublicID: "upd_old", Version: "1.9.0", Status: DesktopUpdateStatusPublished, PublishedAt: &publishedAt, Artifacts: desktopUpdateTestArtifacts("1.9.0")},
		{PublicID: "upd_new", Version: "1.10.0", Notes: "Latest", Status: DesktopUpdateStatusPublished, PublishedAt: &publishedAt, Artifacts: desktopUpdateTestArtifacts("1.10.0")},
	}}
	svc := NewDesktopUpdateService(repo)

	for _, test := range []struct {
		target string
		arch   string
		path   string
	}{
		{target: "darwin", arch: "aarch64", path: "darwin-arm64.dmg"},
		{target: "darwin", arch: "x86_64", path: "darwin-x64.app"},
		{target: "windows", arch: "x86_64", path: "windows-x64.exe"},
	} {
		result, available, err := svc.Check(context.Background(), test.target, test.arch, "1.0.0")
		require.NoError(t, err)
		require.True(t, available)
		require.Equal(t, "1.10.0", result.Version)
		require.Contains(t, result.URL, test.path)
	}

	result, available, err := svc.Check(context.Background(), "darwin", "aarch64", "1.10.0")
	require.NoError(t, err)
	require.False(t, available)
	require.Nil(t, result)
}

func TestDesktopUpdateCheckRejectsUnsupportedPlatformAndInvalidMetadata(t *testing.T) {
	publishedAt := time.Now().UTC()
	svc := NewDesktopUpdateService(&desktopUpdateRepoStub{releases: []DesktopUpdateRelease{{
		Version: "1.2.0", Status: DesktopUpdateStatusPublished, PublishedAt: &publishedAt, Artifacts: desktopUpdateTestArtifacts("1.2.0"),
	}}})

	_, _, err := svc.Check(context.Background(), "linux", "x86_64", "1.0.0")
	require.Equal(t, "DESKTOP_UPDATE_PLATFORM_UNSUPPORTED", infraerrors.Reason(err))

	svc = NewDesktopUpdateService(&desktopUpdateRepoStub{releases: []DesktopUpdateRelease{{
		Version: "1.2.0", Status: DesktopUpdateStatusPublished, Artifacts: desktopUpdateTestArtifacts("1.2.0"),
	}}})
	_, _, err = svc.Check(context.Background(), "darwin", "aarch64", "1.0.0")
	require.Equal(t, "DESKTOP_UPDATE_METADATA_INVALID", infraerrors.Reason(err))
}

func TestDesktopUpdateCheckPropagatesRepositoryFailure(t *testing.T) {
	svc := NewDesktopUpdateService(&desktopUpdateRepoStub{listErr: errors.New("database unavailable")})
	_, _, err := svc.Check(context.Background(), "darwin", "aarch64", "1.0.0")
	require.EqualError(t, err, "database unavailable")
}

func TestDesktopUpdateWithdrawFallsBackToPreviousPublishedVersion(t *testing.T) {
	publishedAt := time.Now().UTC()
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{
		{PublicID: "upd_previous", Version: "1.1.0", Status: DesktopUpdateStatusPublished, PublishedAt: &publishedAt, Artifacts: desktopUpdateTestArtifacts("1.1.0")},
		{PublicID: "upd_latest", Version: "1.2.0", Status: DesktopUpdateStatusPublished, PublishedAt: &publishedAt, Artifacts: desktopUpdateTestArtifacts("1.2.0")},
	}}
	svc := NewDesktopUpdateService(repo)

	_, err := svc.Withdraw(context.Background(), "upd_latest", 7, "Rollback due to startup regression")
	require.NoError(t, err)
	result, available, err := svc.Check(context.Background(), "windows", "x86_64", "1.0.0")
	require.NoError(t, err)
	require.True(t, available)
	require.Equal(t, "1.1.0", result.Version)
}

func (r *desktopUpdateRepoStub) GetByVersion(ctx context.Context, version string) (*DesktopUpdateRelease, error) {
	items, _, err := r.List(ctx, pagination.PaginationParams{Page: 1, PageSize: 100}, DesktopUpdateListFilters{})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Version == version {
			return r.Get(ctx, item.PublicID)
		}
	}
	return nil, ErrDesktopUpdateNotFound
}

func TestDesktopUpdateGrayAudienceAndPlatformFallback(t *testing.T) {
	now := time.Now().UTC()
	repo := &desktopUpdateRepoStub{releases: []DesktopUpdateRelease{
		{Version: "1.0.1", Status: DesktopUpdateStatusPublished, RolloutMode: "full", PublishedAt: &now, Artifacts: desktopUpdateTestArtifacts("1.0.1")},
		{Version: "1.0.2", Status: DesktopUpdateStatusPublished, RolloutMode: "gray", GatewayUserIDs: []int64{7}, PublishedAt: &now, Artifacts: DesktopUpdateArtifacts{DesktopUpdateWindowsX64: desktopUpdateTestArtifacts("1.0.2")[DesktopUpdateWindowsX64]}},
		{Version: "2.0.0", Status: DesktopUpdateStatusPublished, RolloutMode: "gray", GatewayUserIDs: []int64{7}, PublishedAt: &now, Artifacts: DesktopUpdateArtifacts{DesktopUpdateDarwinARM64: desktopUpdateTestArtifacts("2.0.0")[DesktopUpdateDarwinARM64]}},
	}}
	svc := NewDesktopUpdateService(repo)
	for _, tc := range []struct {
		target, arch string
		user         int64
		want         string
	}{
		{"windows", "x86_64", 7, "1.0.2"}, {"windows", "x86_64", 8, "1.0.1"}, {"windows", "x86_64", 0, "1.0.1"},
		{"darwin", "aarch64", 7, "2.0.0"}, {"darwin", "x86_64", 7, "1.0.1"},
	} {
		result, available, err := svc.Check(context.Background(), tc.target, tc.arch, "1.0.0", tc.user)
		require.NoError(t, err)
		require.True(t, available)
		require.Equal(t, tc.want, result.Version)
		require.NotEmpty(t, result.URL)
	}
	_, available, err := svc.Check(context.Background(), "windows", "x86_64", "1.0.2", 7)
	require.NoError(t, err)
	require.False(t, available)
	_, err = svc.Publish(context.Background(), "unused", 1, DesktopUpdatePublishInput{RolloutMode: "gray"})
	require.ErrorIs(t, err, ErrDesktopUpdateInvalidFields)
}

func (r *desktopUpdateRepoStub) Delete(_ context.Context, publicID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.releases {
		if r.releases[i].PublicID == publicID {
			r.releases = append(r.releases[:i], r.releases[i+1:]...)
			return nil
		}
	}
	return ErrDesktopUpdateNotFound
}
