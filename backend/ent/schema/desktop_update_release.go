package schema

import (
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DesktopUpdateRelease stores immutable Desktop updater metadata after publication.
type DesktopUpdateRelease struct {
	ent.Schema
}

func (DesktopUpdateRelease) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_update_releases"}}
}

func (DesktopUpdateRelease) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (DesktopUpdateRelease) Fields() []ent.Field {
	return []ent.Field{
		field.String("public_id").MaxLen(40).NotEmpty().Unique().Immutable(),
		field.String("version").MaxLen(64).NotEmpty().Unique(),
		field.String("notes").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("artifacts", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("status").MaxLen(20).Default("draft"),
		field.Int64("created_by").Optional().Nillable(),
		field.Int64("updated_by").Optional().Nillable(),
		field.Int64("published_by").Optional().Nillable(),
		field.Int64("withdrawn_by").Optional().Nillable(),
		field.Time("published_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("withdrawn_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("withdrawal_reason").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (DesktopUpdateRelease) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "published_at"),
		index.Fields("created_at"),
	}
}
