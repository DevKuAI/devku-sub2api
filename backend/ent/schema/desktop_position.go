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

// DesktopPosition belongs permanently to one enterprise department.
type DesktopPosition struct{ ent.Schema }

func (DesktopPosition) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_positions"}}
}
func (DesktopPosition) Mixin() []ent.Mixin { return []ent.Mixin{mixins.TimeMixin{}} }
func (DesktopPosition) Fields() []ent.Field {
	return []ent.Field{
		field.String("public_id").MaxLen(40).NotEmpty().Unique().Immutable(),
		field.Int64("organization_id").Immutable(),
		field.Int64("department_id").Immutable(),
		field.String("name").MaxRuneLen(100).NotEmpty(),
		field.String("name_normalized").MaxLen(400).NotEmpty(),
		field.String("status").MaxLen(20).Default("active"),
		field.Int64("revision").Positive().Default(1),
	}
}
func (DesktopPosition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("organization", DesktopOrganization.Type).Field("organization_id").Unique().Required().Immutable(),
		edge.From("department", DesktopDepartment.Type).Ref("positions").Field("department_id").Unique().Required().Immutable(),
		edge.To("members", DesktopMember.Type),
	}
}
func (DesktopPosition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("organization_id", "department_id", "name_normalized").Unique().StorageKey("idx_desktop_positions_name"),
		index.Fields("organization_id", "id").Unique().StorageKey("idx_desktop_positions_org_id"),
	}
}
