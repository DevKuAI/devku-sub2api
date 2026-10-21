package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"golang.org/x/text/unicode/norm"
)

const (
	DesktopDepartmentKind = "departments"
	DesktopPositionKind   = "positions"
)

var (
	ErrDesktopStructureNotFound   = infraerrors.NotFound("ORGANIZATION_UNIT_NOT_FOUND", "department or position not found")
	ErrDesktopStructureConflict   = infraerrors.Conflict("ORGANIZATION_STRUCTURE_CONFLICT", "organization structure has changed; reload and try again")
	ErrDesktopStructureInUse      = infraerrors.Conflict("ORGANIZATION_UNIT_IN_USE", "department or position is still in use")
	ErrDesktopStructureDisabled   = infraerrors.Conflict("ORGANIZATION_UNIT_DISABLED", "department or position is disabled")
	ErrDesktopStructureNameExists = infraerrors.Conflict("ORGANIZATION_UNIT_NAME_EXISTS", "a department or position with this name already exists")
)

type DesktopStructureRef struct {
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
}

type DesktopMemberAssignment struct {
	Department       *DesktopStructureRef `json:"department"`
	Position         *DesktopStructureRef `json:"position"`
	PositionRevision int64                `json:"position_revision"`
	AssignmentStatus string               `json:"assignment_status"`
}

func DesktopAssignmentView(m *DesktopMember) DesktopMemberAssignment {
	status := "unassigned"
	if m.Position != nil {
		status = "assigned"
	}
	return DesktopMemberAssignment{Department: m.Department, Position: m.Position, PositionRevision: m.PositionRevision, AssignmentStatus: status}
}

type DesktopOrganizationUnit struct {
	DesktopStructureRef
	Status      string               `json:"status"`
	Revision    int64                `json:"revision"`
	Department  *DesktopStructureRef `json:"department,omitempty"`
	MemberCount int                  `json:"member_count"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

type DesktopStructureFilter struct{ Search, Status, DepartmentID string }

type DesktopSaveUnitInput struct {
	Name             *string `json:"name"`
	Status           *string `json:"status"`
	DepartmentID     string  `json:"department_id,omitempty"`
	ExpectedRevision int64   `json:"expected_revision"`
	NameNormalized   string  `json:"-"`
	PublicID         string  `json:"-"`
}

type DesktopAssignPositionInput struct {
	PositionID       string `json:"position_id"`
	ExpectedRevision *int64 `json:"expected_revision"`
	Reason           string `json:"reason"`
	ActorID          int64  `json:"-"`
	RequestID        string `json:"-"`
}

type DesktopPositionHistory struct {
	Position   DesktopStructureRef `json:"position"`
	Department DesktopStructureRef `json:"department"`
	Revision   int64               `json:"position_revision"`
	StartedAt  time.Time           `json:"started_at"`
	EndedAt    *time.Time          `json:"ended_at"`
	Source     string              `json:"source"`
	Reason     string              `json:"reason"`
}

type DesktopStructureStatus struct {
	Status              string `json:"status"`
	UnassignedMembers   int    `json:"unassigned_members"`
	InconsistentMembers int    `json:"inconsistent_members"`
	ConstraintEnabled   bool   `json:"constraint_enabled"`
}

// DesktopStructureRepository keeps hierarchy and assignment writes atomic.
type DesktopStructureRepository interface {
	ListOrganizationUnits(context.Context, string, string, pagination.PaginationParams, DesktopStructureFilter) ([]DesktopOrganizationUnit, *pagination.PaginationResult, error)
	GetOrganizationUnit(context.Context, string, string, string) (*DesktopOrganizationUnit, error)
	SaveOrganizationUnit(context.Context, string, string, string, DesktopSaveUnitInput) (*DesktopOrganizationUnit, error)
	AssignMemberPosition(context.Context, string, string, DesktopAssignPositionInput) (*DesktopMember, error)
	MemberPositionHistory(context.Context, string, string, pagination.PaginationParams) ([]DesktopPositionHistory, *pagination.PaginationResult, error)
	OrganizationStructureStatus(context.Context, string) (*DesktopStructureStatus, error)
}

func NormalizeDesktopUnitName(value string) (string, string, error) {
	name := norm.NFC.String(strings.TrimSpace(value))
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 100 {
		return "", "", ErrDesktopValidation.WithMetadata(map[string]string{"field": "name"})
	}
	return name, strings.ToLower(name), nil
}

// StructureScope derives enterprise scope from the web identity for managed reads and writes.
func (s *DesktopService) StructureScope(ctx context.Context, org string, managerID int64) (DesktopStructureRepository, string, error) {
	repo := s.repo
	if managerID > 0 {
		managed, organization, err := s.managedService(ctx, managerID)
		if err != nil {
			return nil, "", err
		}
		repo, org = managed.repo, organization.PublicID
	} else if _, err := repo.GetOrganization(ctx, org); err != nil {
		return nil, "", err
	}
	result, ok := repo.(DesktopStructureRepository)
	if !ok {
		return nil, "", ErrDesktopUsageUnavailable
	}
	return result, org, nil
}

func ValidateDesktopUnitInput(kind, id string, in *DesktopSaveUnitInput) error {
	if kind != DesktopDepartmentKind && kind != DesktopPositionKind {
		return ErrDesktopValidation
	}
	if id == "" {
		if in.Name == nil || in.ExpectedRevision != 0 || in.Status != nil {
			return ErrDesktopValidation
		}
		if kind == DesktopPositionKind && strings.TrimSpace(in.DepartmentID) == "" {
			return ErrDesktopValidation.WithMetadata(map[string]string{"field": "department_id"})
		}
		prefix := "dep"
		if kind == DesktopPositionKind {
			prefix = "pos"
		}
		value, err := GenerateDesktopPublicID(prefix)
		if err != nil {
			return err
		}
		in.PublicID = value
	} else if in.ExpectedRevision < 1 || in.DepartmentID != "" || (in.Name == nil && in.Status == nil) {
		return ErrDesktopValidation
	}
	if kind == DesktopDepartmentKind && in.DepartmentID != "" {
		return ErrDesktopValidation
	}
	if in.Name != nil {
		name, normalized, err := NormalizeDesktopUnitName(*in.Name)
		if err != nil {
			return err
		}
		in.Name = &name
		in.NameNormalized = normalized
	}
	if in.Status != nil {
		return validateDesktopStatus(*in.Status)
	}
	return nil
}

func ValidateDesktopAssignment(in *DesktopAssignPositionInput) error {
	in.PositionID = strings.TrimSpace(in.PositionID)
	in.Reason = norm.NFC.String(strings.TrimSpace(in.Reason))
	if in.PositionID == "" || in.ExpectedRevision == nil || *in.ExpectedRevision < 0 || !utf8.ValidString(in.Reason) || strings.ContainsRune(in.Reason, 0) || utf8.RuneCountInString(in.Reason) > 500 {
		return ErrDesktopValidation
	}
	return nil
}

// GetWebMember shares scoped lookup without exposing credentials to its DTO.
func (s *DesktopService) GetWebMember(ctx context.Context, org, member string, managerID int64) (*DesktopMember, error) {
	if managerID > 0 {
		managed, organization, err := s.managedService(ctx, managerID)
		if err != nil {
			return nil, err
		}
		return managed.repo.GetMember(ctx, organization.PublicID, member)
	}
	return s.repo.GetMember(ctx, org, member)
}
