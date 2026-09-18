package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// DesktopConversationRecord stores one immutable, complete conversation turn.
type DesktopConversationRecord struct{ ent.Schema }

func (DesktopConversationRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "desktop_conversation_records"}}
}

func (DesktopConversationRecord) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("record_id", uuid.UUID{}).Immutable(),
		field.Int64("organization_id").Immutable(),
		field.Int64("member_id").Immutable(),
		field.UUID("installation_id", uuid.UUID{}).Immutable(),
		field.String("client").MaxLen(32).Immutable(),
		field.String("source_session_id").MaxRuneLen(512).SchemaType(map[string]string{dialect.Postgres: "varchar(512)"}).NotEmpty().Immutable(),
		field.String("source_turn_id").MaxRuneLen(512).SchemaType(map[string]string{dialect.Postgres: "varchar(512)"}).Optional().Nillable().Immutable(),
		field.Time("started_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
		field.Time("stopped_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
		field.Time("received_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
		field.String("cwd").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional().Nillable().Immutable(),
		field.JSON("prompts", json.RawMessage{}).Immutable(),
		field.JSON("response", json.RawMessage{}).Optional().Immutable(),
		field.String("capture_status").MaxLen(32).Immutable(),
		field.Int("prompt_count").Min(1).Max(64).Immutable(),
	}
}

func (DesktopConversationRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("organization", DesktopOrganization.Type).Field("organization_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("member", DesktopMember.Type).Field("member_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (DesktopConversationRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("organization_id", "record_id").Unique().StorageKey("idx_desktop_conversation_record_unique"),
		index.Fields("organization_id", "received_at", "id").StorageKey("idx_desktop_conversation_received").Annotations(entsql.DescColumns("received_at", "id")),
		index.Fields("organization_id", "member_id", "client", "installation_id", "source_session_id", "received_at", "id").StorageKey("idx_desktop_conversation_thread"),
	}
}
