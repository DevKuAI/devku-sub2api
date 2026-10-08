//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDesktopMemberRemarkPersistenceAndIsolation(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "remark", 10)
	other := newDesktopRepositoryFixture(t, "otherremark", 10)
	publicID, err := service.GenerateDesktopPublicID("mem")
	require.NoError(t, err)
	member, err := fixture.repo.CreateMember(ctx, fixture.organization.PublicID, service.DesktopCreateMemberInput{
		Member: &service.DesktopMember{PublicID: publicID, Name: "Member", NameNormalized: "Member", Phone: desktopTestPhone(fixture.suffix), Remark: strings.Repeat("字", 500)},
		APIKey: &service.APIKey{Key: "sk-remark-" + fixture.organization.PublicID, Name: "Remark", Status: service.StatusAPIKeyActive},
	})
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("字", 500), member.Remark)
	version := member.AuthVersion
	keyID := *member.CurrentAPIKeyID
	rows, _, err := fixture.repo.ListMembers(ctx, fixture.organization.PublicID, pagination.DefaultPagination(), service.DesktopMemberListFilters{})
	require.NoError(t, err)
	require.Equal(t, member.Remark, rows[0].Remark)
	scoped := fixture.repo.ScopedToGatewayUser(other.user.ID)
	remark := "财务部\n主管"
	_, _, err = scoped.UpdateMember(ctx, fixture.organization.PublicID, member.PublicID, service.DesktopUpdateMemberInput{Remark: &remark})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	updated, keys, err := fixture.repo.UpdateMember(ctx, fixture.organization.PublicID, member.PublicID, service.DesktopUpdateMemberInput{Remark: &remark})
	require.NoError(t, err)
	require.Equal(t, remark, updated.Remark)
	require.Equal(t, version, updated.AuthVersion)
	require.Equal(t, keyID, *updated.CurrentAPIKeyID)
	require.Empty(t, keys)
	remark = ""
	updated, _, err = fixture.repo.UpdateMember(ctx, fixture.organization.PublicID, member.PublicID, service.DesktopUpdateMemberInput{Remark: &remark})
	require.NoError(t, err)
	require.Empty(t, updated.Remark)
	plain := fixture.createMember(t, "plainremark", 1)
	require.Empty(t, plain.Remark)
}
