package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DesktopMemberAPIKey preserves the current and historical Model Token ownership.
type DesktopMemberAPIKey struct {
	ent.Schema
}

func (DesktopMemberAPIKey) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_member_api_keys"}}
}

func (DesktopMemberAPIKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("member_id"),
		field.Int64("api_key_id").Unique(),
		field.Time("assigned_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("retired_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (DesktopMemberAPIKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", DesktopMember.Type).
			Ref("api_key_assignments").
			Field("member_id").
			Unique().
			Required(),
		edge.From("api_key", APIKey.Type).
			Ref("desktop_member_assignment").
			Field("api_key_id").
			Unique().
			Required(),
	}
}

func (DesktopMemberAPIKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("member_id").
			Unique().
			StorageKey("idx_desktop_member_api_keys_current").
			Annotations(entsql.IndexWhere("retired_at IS NULL")),
		index.Fields("member_id", "assigned_at").
			StorageKey("idx_desktop_member_api_keys_history"),
	}
}
