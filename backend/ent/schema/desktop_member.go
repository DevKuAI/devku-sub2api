package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DesktopMember holds an employee identity inside one Desktop organization.
type DesktopMember struct {
	ent.Schema
}

func (DesktopMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_members"}}
}

func (DesktopMember) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (DesktopMember) Fields() []ent.Field {
	return []ent.Field{
		field.String("public_id").MaxLen(40).NotEmpty().Unique().Immutable(),
		field.Int64("organization_id"),
		field.String("name").MaxLen(100).NotEmpty(),
		field.String("name_normalized").MaxLen(100).NotEmpty(),
		field.String("phone").MaxLen(16).NotEmpty(),
		field.String("status").MaxLen(20).Default("active"),
		field.Int64("auth_version").Positive().Default(1),
		field.Bool("api_key_suspended_by_organization").Default(false),
	}
}

func (DesktopMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("organization", DesktopOrganization.Type).
			Ref("members").
			Field("organization_id").
			Unique().
			Required(),
		edge.To("api_key_assignments", DesktopMemberAPIKey.Type),
	}
}

func (DesktopMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("organization_id", "phone").
			Unique().
			StorageKey("idx_desktop_members_org_phone_active").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("organization_id", "status").
			StorageKey("idx_desktop_members_org_status_active").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
