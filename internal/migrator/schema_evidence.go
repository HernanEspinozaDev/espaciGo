package migrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

// schemaEvidenceStatement removes only the reviewed psql framing. The query
// bytes remain unchanged on disk and their original hash is in the report.
func schemaEvidenceStatement(source []byte) (string, error) {
	text := string(source)
	const begin = "BEGIN READ ONLY;\n"
	if strings.Count(text, begin) != 1 || !strings.HasSuffix(text, "\nCOMMIT;\n") {
		return "", errors.New("schema evidence query framing differs")
	}
	parts := strings.SplitN(text, begin, 2)
	for _, line := range strings.Split(parts[0], "\n") {
		if line != "" && !strings.HasPrefix(line, "--") && line != `\pset format unaligned` && line != `\pset tuples_only on` {
			return "", errors.New("schema evidence query has unexpected preamble")
		}
	}
	statement := strings.TrimSuffix(parts[1], "\nCOMMIT;\n")
	if !strings.HasPrefix(statement, "SELECT jsonb_build_object(") || !strings.HasSuffix(statement, ";") {
		return "", errors.New("schema evidence must be the reviewed SELECT")
	}
	return statement, nil
}

func collectSchemaEvidence(ctx context.Context, conn *pgx.Conn, source []byte) (json.RawMessage, error) {
	statement, err := schemaEvidenceStatement(source)
	if err != nil {
		return nil, err
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, errors.New("begin read-only schema evidence")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var encoded string
	// Extended protocol accepts a single statement; READ ONLY additionally
	// forbids a write even if a future reviewed SELECT calls a writing function.
	if err := tx.QueryRow(ctx, statement).Scan(&encoded); err != nil {
		return nil, errors.New("collect schema evidence failed")
	}
	var report map[string]json.RawMessage
	if json.Unmarshal([]byte(encoded), &report) != nil || string(report["format"]) != `"espacigo.img01.schema-evidence.v1"` {
		return nil, errors.New("schema evidence format differs")
	}
	digest := sha256.Sum256(source)
	report["query_sha256"], err = json.Marshal(hex.EncodeToString(digest[:]))
	if err != nil {
		return nil, err
	}
	return json.Marshal(report)
}

// SchemaEvidenceCloudSQL reads catalogs/ledger through the private connector.
// It neither initializes a ledger nor acquires a DDL/advisory lock.
func SchemaEvidenceCloudSQL(ctx context.Context, databaseURL, password, instance string, privateIP bool, setRole string, source []byte) (json.RawMessage, error) {
	if _, err := schemaEvidenceStatement(source); err != nil {
		return nil, err
	}
	config, err := cloudsql.NewPGXConfig(databaseURL, password)
	if err != nil {
		return nil, err
	}
	closeConnector, err := cloudsql.ConfigurePGX(ctx, config, instance, privateIP)
	if err != nil {
		return nil, err
	}
	defer closeConnector()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, errors.New("connect schema evidence database")
	}
	defer conn.Close(context.Background())
	if !validDatabaseRoleName(setRole) {
		return nil, errors.New("invalid schema evidence role")
	}
	if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{setRole}.Sanitize()); err != nil {
		return nil, errors.New("could not assume schema evidence role")
	}
	return collectSchemaEvidence(ctx, conn, source)
}
