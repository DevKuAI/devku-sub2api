package schema

import (
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DesktopOrganization holds the Desktop enterprise boundary.
type DesktopOrganization struct {
	ent.Schema
}

func (DesktopOrganization) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_organizations"}}
}

func (DesktopOrganization) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (DesktopOrganization) Fields() []ent.Field {
	return []ent.Field{
		field.String("public_id").MaxLen(40).NotEmpty().Unique().Immutable(),
		field.String("code").MaxLen(16).NotEmpty(),
		field.String("name").MaxLen(200).NotEmpty(),
		field.String("status").MaxLen(20).Default("active"),
		field.Int64("auth_version").Positive().Default(1),
		field.Int64("gateway_user_id"),
		field.Int64("group_id"),
		field.JSON("target_config", json.RawMessage{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (DesktopOrganization) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("gateway_user", User.Type).
			Ref("desktop_organizations").
			Field("gateway_user_id").
			Unique().
			Required(),
		edge.From("group", Group.Type).
			Ref("desktop_organizations").
			Field("group_id").
			Unique().
			Required(),
		edge.To("members", DesktopMember.Type),
	}
}

func (DesktopOrganization) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").
			Unique().
			StorageKey("idx_desktop_organizations_code_active").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("gateway_user_id").
			Unique().
			StorageKey("idx_desktop_organizations_gateway_user_active").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("group_id").
			StorageKey("idx_desktop_organizations_group_active").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("status", "updated_at").
			StorageKey("idx_desktop_organizations_status_updated_active").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
