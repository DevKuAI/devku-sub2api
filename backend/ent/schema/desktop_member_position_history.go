package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DesktopMemberPositionHistory preserves the assignment known at each change.
type DesktopMemberPositionHistory struct{ ent.Schema }

func (DesktopMemberPositionHistory) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_member_position_history"}}
}
func (DesktopMemberPositionHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("organization_id").Immutable(), field.Int64("member_id").Immutable(), field.Int64("position_id").Immutable(),
		field.Int64("position_revision").Positive().Immutable(),
		field.String("department_name_snapshot").MaxRuneLen(100).Immutable(), field.String("position_name_snapshot").MaxRuneLen(100).Immutable(),
		field.Time("started_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
		field.Time("ended_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("source").MaxLen(32).Immutable(),
		field.Int64("changed_by_user_id").Optional().Nillable().Immutable(),
		field.String("request_id").MaxLen(128).Default("").Immutable(),
		field.String("reason").MaxRuneLen(500).Default("").Immutable(),
	}
}
func (DesktopMemberPositionHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("member_id", "position_revision").Unique().StorageKey("idx_desktop_position_history_revision"),
		index.Fields("member_id").Unique().StorageKey("idx_desktop_position_history_current").Annotations(entsql.IndexWhere("ended_at IS NULL")),
	}
}
