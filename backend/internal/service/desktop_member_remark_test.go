package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type desktopRemarkRepository struct {
	desktopRepositoryStub
	created *DesktopMember
	updated *DesktopUpdateMemberInput
}

func (r *desktopRemarkRepository) ScopedToGatewayUser(userID int64) DesktopRepository {
	r.scopedUserIDs = append(r.scopedUserIDs, userID)
	return r
}

func (r *desktopRemarkRepository) CreateMember(_ context.Context, _ string, input DesktopCreateMemberInput) (*DesktopMember, error) {
	r.created = input.Member
	return input.Member, nil
}

func (r *desktopRemarkRepository) UpdateMember(_ context.Context, _, _ string, input DesktopUpdateMemberInput) (*DesktopMember, []string, error) {
	r.updated = &input
	return &DesktopMember{Remark: *input.Remark}, nil, nil
}

func TestDesktopMemberRemarkNormalization(t *testing.T) {
	for _, remark := range []string{"", "   ", " 财务部\n主管 ", strings.Repeat("字", 500)} {
		value, err := NormalizeDesktopRemark(remark)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(remark), value)
	}
	_, err := NormalizeDesktopRemark(strings.Repeat("字", 501))
	require.ErrorIs(t, err, ErrDesktopValidation)
	value, err := NormalizeDesktopRemark("e\u0301")
	require.NoError(t, err)
	require.Equal(t, "é", value)
}

func TestDesktopMemberCreateRemarkForAdminAndManager(t *testing.T) {
	for _, managed := range []bool{false, true} {
		repo := &desktopRemarkRepository{desktopRepositoryStub: desktopRepositoryStub{organization: &DesktopOrganization{PublicID: "org_owned", Status: DesktopStatusActive}}}
		svc := &DesktopService{repo: repo, apiKeys: &APIKeyService{cfg: &config.Config{}}}
		var member *DesktopMember
		var err error
		if managed {
			member, err = svc.CreateManagedMember(context.Background(), 42, "Member", "13800138000", " 财务部 ")
			require.Equal(t, []int64{42}, repo.scopedUserIDs)
		} else {
			member, err = svc.CreateMember(context.Background(), "org_owned", "Member", "13800138000", " 财务部 ")
		}
		require.NoError(t, err)
		require.Equal(t, "财务部", member.Remark)
		require.Equal(t, "财务部", repo.created.Remark)
	}
}

func TestDesktopMemberRemarkUpdateAllowsClearWithoutCredentialRevocation(t *testing.T) {
	repo := &desktopRemarkRepository{}
	svc := &DesktopService{repo: repo}
	for _, remark := range []string{" 财务部 ", ""} {
		updated, err := svc.UpdateMember(context.Background(), "org_one", "mem_one", DesktopUpdateMemberInput{Remark: &remark}, nil)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(remark), updated.Remark)
		require.Nil(t, repo.updated.Name)
		require.Nil(t, repo.updated.Phone)
		require.False(t, repo.updated.RevokeCredential)
	}
	tooLong := strings.Repeat("字", 501)
	_, err := svc.UpdateMember(context.Background(), "org_one", "mem_one", DesktopUpdateMemberInput{Remark: &tooLong}, nil)
	require.ErrorIs(t, err, ErrDesktopValidation)
}
