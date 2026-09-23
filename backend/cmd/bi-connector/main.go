// Command bi-connector manages server-side BI ingestion credentials.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/bi"
	"github.com/Wei-Shaw/sub2api/internal/config"
	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		var apiError *bi.Error
		if errors.As(err, &apiError) {
			fmt.Fprintln(os.Stderr, apiError.Error())
			os.Exit(1)
		}
		// Driver errors may contain connection parameters; do not print them.
		fmt.Fprintf(os.Stderr, "BI connector operation failed (%T). Check the operation and database configuration.\n", err)
		os.Exit(1)
	}
}

func run() error {
	operation := flag.String("operation", "", "issue, revoke or sync-sub2api")
	since := flag.String("since", "", "Earliest usage timestamp for sync-sub2api (RFC3339)")
	limit := flag.Int("limit", 500, "Maximum billing-log rows per import batch")
	organization := flag.String("organization", "", "Existing BI organization public ID")
	source := flag.String("source", "", "Stable ingestion source ID")
	namespace := flag.String("namespace", "", "Owned entity ID prefix, without the trailing colon")
	kinds := flag.String("kinds", "", "Comma-separated allowed record kinds")
	ttl := flag.Duration("ttl", 24*time.Hour, "Credential lifetime")
	id := flag.String("id", "", "Credential ID to revoke")
	flag.Parse()
	if *operation != "issue" && *operation != "revoke" && *operation != "sync-sub2api" {
		return fmt.Errorf("unsupported operation")
	}
	dsn := os.Getenv("BI_DATABASE_DSN")
	if dsn == "" {
		return fmt.Errorf("BI_DATABASE_DSN is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if *operation == "sync-sub2api" {
		start, err := time.Parse(time.RFC3339, *since)
		if err != nil {
			return fmt.Errorf("since must be an RFC3339 timestamp")
		}
		svc := bi.NewService(db, config.BIConfig{}, nil, nil)
		connector, err := svc.AuthorizeConnector(ctx, os.Getenv("BI_CONNECTOR_TOKEN"))
		if err != nil {
			return err
		}
		batch, err := svc.PrepareSub2apiImport(ctx, connector, start, *limit)
		if err != nil {
			return err
		}
		if len(batch.Records) == 0 {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "idle", "records": 0})
		}
		raw, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		job, err := svc.SubmitImport(ctx, connector, batch.Checkpoint, raw)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(job)
	}
	if *operation == "revoke" {
		if err := bi.RevokeConnector(ctx, db, *id); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"revoked": true, "credential_id": *id})
	}
	allowed := []string{}
	for _, kind := range strings.Split(*kinds, ",") {
		if kind = strings.TrimSpace(kind); kind != "" {
			allowed = append(allowed, kind)
		}
	}
	expires := time.Now().UTC().Add(*ttl)
	credentialID, token, err := bi.RegisterConnector(ctx, db, bi.ConnectorRegistration{
		OrganizationID: *organization, SourceID: *source, Namespace: *namespace, AllowedKinds: allowed, ExpiresAt: expires,
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"credential_id": credentialID, "token": token, "expires_at": expires})
}
