package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesktopStructureNameAndInputValidation(t *testing.T) {
	for _, sample := range []string{" Café ", "CAFE\u0301"} {
		display, key, err := NormalizeDesktopUnitName(sample)
		require.NoError(t, err)
		require.Equal(t, "café", key)
		require.NotEmpty(t, display)
	}
	for _, sample := range []string{" ", strings.Repeat("字", 101), "bad\x00name"} {
		_, _, err := NormalizeDesktopUnitName(sample)
		require.ErrorIs(t, err, ErrDesktopValidation)
	}
	name := "Engineer"
	in := DesktopSaveUnitInput{Name: &name}
	require.Error(t, ValidateDesktopUnitInput(DesktopPositionKind, "", &in))
	in = DesktopSaveUnitInput{Name: &name, DepartmentID: "dep_other", ExpectedRevision: 1}
	require.Error(t, ValidateDesktopUnitInput(DesktopPositionKind, "pos_one", &in))
	revision := int64(0)
	require.NoError(t, ValidateDesktopAssignment(&DesktopAssignPositionInput{PositionID: "pos_one", ExpectedRevision: &revision}))
	require.Error(t, ValidateDesktopAssignment(&DesktopAssignPositionInput{PositionID: "pos_one"}))
}

func TestDesktopMemberCreateRequiresPositionBeforeSideEffects(t *testing.T) {
	s := &DesktopService{}
	_, err := s.CreateMember(context.Background(), "org", DesktopMemberCreateInput{Name: "Member", Phone: "13800138000"})
	require.ErrorIs(t, err, ErrDesktopValidation)
}
