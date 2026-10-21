package migrations

import (
	"errors"
	"io/fs"
	"testing"
)

func TestDesktopOrganizationStructureConstraintIsNotAutomaticallyEmbedded(t *testing.T) {
	if _, err := FS.ReadFile("267_desktop_organization_structure.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := FS.ReadFile("deferred/desktop_member_position_required.sql"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("final constraint must remain a separate deployment step: %v", err)
	}
}
