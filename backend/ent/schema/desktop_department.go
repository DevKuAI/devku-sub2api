package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// DesktopDepartment represents a single-level department within an enterprise.
type DesktopDepartment struct{ ent.Schema }

func (DesktopDepartment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_departments"}}
}
func (DesktopDepartment) Mixin() []ent.Mixin { return []ent.Mixin{mixins.TimeMixin{}} }
func (DesktopDepartment) Fields() []ent.Field {
	return []ent.Field{
		field.String("public_id").MaxLen(40).NotEmpty().Unique().Immutable(),
		field.Int64("organization_id").Immutable(),
		field.String("name").MaxRuneLen(100).NotEmpty(),
		field.String("name_normalized").MaxLen(400).NotEmpty(),
		field.String("status").MaxLen(20).Default("active"),
		field.Int64("revision").Positive().Default(1),
	}
}
func (DesktopDepartment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("organization", DesktopOrganization.Type).Field("organization_id").Unique().Required().Immutable(),
		edge.To("positions", DesktopPosition.Type),
	}
}
func (DesktopDepartment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("organization_id", "name_normalized").Unique().StorageKey("idx_desktop_departments_name"),
		index.Fields("organization_id", "id").Unique().StorageKey("idx_desktop_departments_org_id"),
	}
}
