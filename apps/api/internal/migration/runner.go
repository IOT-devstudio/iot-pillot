// Package migration owns schema deployment, independently of API startup.
package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/lib/pq"
)

// legacyVersion is the highest version that existed before the tool took over
// schema management. It bounds only the one-time manual `check`/`adopt`
// manifest: versions above it were never applied by hand, so claiming them is
// rejected. Migrations added later (0010+) run through `up` and never appear in
// a manual manifest.
const legacyVersion = 9

// ParseApplied validates the one-time, explicitly reviewed manual history.
// Dependencies reflect only the tables each historical SQL actually requires.
func ParseApplied(input string) ([]uint, error) {
	seen := map[uint]bool{}
	var result []uint
	for _, item := range strings.Split(input, ",") {
		v, err := strconv.ParseUint(strings.TrimSpace(item), 10, 32)
		if err != nil || v == 0 || v > legacyVersion || seen[uint(v)] {
			return nil, fmt.Errorf("applied must contain distinct versions from 1 to %d", legacyVersion)
		}
		seen[uint(v)] = true
		result = append(result, uint(v))
	}
	for v, dependencies := range map[uint][]uint{3: {2}, 4: {2}, 5: {1, 2}, 6: {1}} {
		for _, dependency := range dependencies {
			if seen[v] && !seen[dependency] {
				return nil, fmt.Errorf("version %d requires version %d", v, dependency)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func readUp(version uint) ([]byte, error) {
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), fmt.Sprintf("%04d_", version)) && strings.HasSuffix(entry.Name(), ".up.sql") {
			return migrations.Files.ReadFile(entry.Name())
		}
	}
	return nil, fmt.Errorf("migration %d is unavailable", version)
}

func checksum(version uint) (string, error) {
	query, err := readUp(version)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(query)), nil
}

type adoptedSource struct {
	source.Driver
	applied map[uint]bool
}

func (s adoptedSource) ReadUp(version uint) (io.ReadCloser, string, error) {
	if s.applied[version] {
		return io.NopCloser(strings.NewReader("-- verified manual migration; no DDL replay\n")), "adopted", nil
	}
	return s.Driver.ReadUp(version)
}

// Run uses a dedicated session for the entire check/adopt/up operation. A second
// invocation fails immediately instead of racing baseline verification or up.
func Run(ctx context.Context, db *sql.DB, command string, applied []uint) error {
	if command != "up" && command != "check" && command != "adopt" {
		return errors.New("command must be up, check or adopt")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return errors.New("database connection failed")
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(107, 20261014)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return errors.New("another migration operation holds the lock")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// A failed historical BEGIN/COMMIT script leaves the session in an aborted
		// transaction. Roll it back before releasing both our lock and migrate's
		// session lock, then return a clean session to database/sql's pool.
		conn.ExecContext(cleanup, `ROLLBACK; SELECT pg_advisory_unlock_all(); RESET ALL`)
	}()
	// Explicit public-only path avoids accidentally operating on a user's schema.
	if _, err := conn.ExecContext(ctx, `SET search_path TO public; SET lock_timeout TO '15s'; SET statement_timeout TO '120s'`); err != nil {
		return err
	}
	if command != "up" {
		return adopt(ctx, conn, applied, command == "check")
	}

	var tracked, ledger, existing bool
	if err := conn.QueryRowContext(ctx, `SELECT
		to_regclass('public.schema_migrations') IS NOT NULL,
		to_regclass('public.migration_adoptions') IS NOT NULL,
		EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname='public' AND c.relkind IN ('r','p')
		AND c.relname NOT IN ('schema_migrations','migration_adoptions'))`).Scan(&tracked, &ledger, &existing); err != nil {
		return err
	}
	if tracked {
		var version int
		var dirty bool
		err := conn.QueryRowContext(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if dirty {
			return fmt.Errorf("migration version %d is dirty; inspect and repair manually", version)
		}
		if errors.Is(err, sql.ErrNoRows) && existing && !ledger {
			return errors.New("unmanaged existing database: check and adopt a verified applied list first")
		}
	} else if existing && !ledger {
		return errors.New("unmanaged existing database: check and adopt a verified applied list first")
	}
	adopted := map[uint]bool{}
	if ledger {
		rows, err := conn.QueryContext(ctx, `SELECT version,checksum FROM public.migration_adoptions ORDER BY version`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var v uint
			var saved string
			if err := rows.Scan(&v, &saved); err != nil {
				rows.Close()
				return err
			}
			current, err := checksum(v)
			if err != nil || current != saved {
				rows.Close()
				return fmt.Errorf("adopted migration %d checksum mismatch", v)
			}
			adopted[v] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	src, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return err
	}
	driver, err := postgres.WithConnection(ctx, conn, &postgres.Config{SchemaName: "public", StatementTimeout: 120 * time.Second})
	if err != nil {
		src.Close()
		return err
	}
	engine, err := migrate.NewWithInstance("embedded", adoptedSource{src, adopted}, "postgres", driver)
	if err != nil {
		src.Close()
		return err
	}
	// conn stays owned here: closing the engine would close it before unlocking.
	defer src.Close()
	if err := engine.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		// The driver's error includes SQL/detail; keep data and credentials out of logs.
		return errors.New("migration up failed; inspect schema_migrations and database logs; dirty state is preserved")
	}
	return nil
}

func adopt(ctx context.Context, conn *sql.Conn, applied []uint, dryRun bool) error {
	parts := make([]string, len(applied))
	for i, v := range applied {
		parts[i] = strconv.Itoa(int(v))
	}
	checked, err := ParseApplied(strings.Join(parts, ","))
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tracked bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL OR to_regclass('public.migration_adoptions') IS NOT NULL`).Scan(&tracked); err != nil {
		return err
	}
	if tracked {
		return errors.New("database already has migration metadata; adoption is a one-time operation")
	}
	if err := verifyLegacy(ctx, tx, checked); err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE public.migration_adoptions (
		version bigint PRIMARY KEY CHECK (version BETWEEN 1 AND `+strconv.Itoa(legacyVersion)+`),
		checksum text NOT NULL,
		adopted_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	for _, v := range checked {
		sum, err := checksum(v)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO public.migration_adoptions(version,checksum) VALUES($1,$2)`, v, sum); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Existing SQL has its own transaction envelope. Remove only the two standalone
// envelope lines when building the reference, so its DDL is always rolled back.
var envelope = regexp.MustCompile(`(?m)^(BEGIN|COMMIT);\s*$`)

func verifyLegacy(ctx context.Context, tx *sql.Tx, applied []uint) error {
	schema := fmt.Sprintf("iot_migration_reference_%d", time.Now().UnixNano())
	if _, err := tx.ExecContext(ctx, `CREATE SCHEMA `+pq.QuoteIdentifier(schema)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path TO `+pq.QuoteIdentifier(schema)); err != nil {
		return err
	}
	for _, v := range applied {
		query, err := readUp(v)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, envelope.ReplaceAllString(string(query), "")); err != nil {
			return fmt.Errorf("applied list cannot form a valid reference schema at version %d", v)
		}
	}
	expected, err := snapshot(ctx, tx, schema)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path TO public`); err != nil {
		return err
	}
	actual, err := snapshot(ctx, tx, "public")
	if err != nil {
		return err
	}
	for key, value := range expected {
		if actual[key] != value {
			return fmt.Errorf("schema does not match applied list: %s; no adoption performed", key)
		}
	}
	for key := range actual {
		if _, ok := expected[key]; !ok {
			return fmt.Errorf("unexpected schema object: %s; no adoption performed", key)
		}
	}
	for _, version := range applied {
		if version != 7 {
			continue
		}
		// These seed identities are required by 0007. Names, open flags and sort
		// orders remain editable business data and must never be reset on adopt.
		var complete bool
		if err := tx.QueryRowContext(ctx, `SELECT
			EXISTS(SELECT 1 FROM public.recruitment_rounds WHERE round_num=1)
			AND (SELECT count(*) FROM public.recruitment_directions
			WHERE code IN ('front-end','back-end','agent'))=3`).Scan(&complete); err != nil {
			return err
		}
		if !complete {
			return errors.New("version 7 seed identities are incomplete; inspect partial execution or intentional removal before adoption")
		}
	}
	// Reference schema must not survive a successful adoption commit either.
	_, err = tx.ExecContext(ctx, `DROP SCHEMA `+pq.QuoteIdentifier(schema)+` CASCADE`)
	return err
}

// Historical scope is explicit: unrelated tables are not part of the baseline.
// Extra columns/indexes on these tables require review rather than being ignored.
var legacyTables = []string{
	"users", "mail_models", "mails", "recruitment_rounds", "recruitment_directions",
	"recruitment_applications", "recruitment_application_timeline",
	"recruitment_invitations", "recruitment_candidates", "recruitment_candidate_timeline",
	"forms", "form_fields", "form_submissions",
}

func snapshot(ctx context.Context, tx *sql.Tx, schema string) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `
	WITH tables AS (
	 SELECT c.oid,c.relname,c.relkind,c.relpersistence FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
	 WHERE n.nspname=$1 AND c.relname=ANY($2) AND c.relkind IN ('r','p')
	)
	SELECT 'table:'||relname, relkind::text||':'||relpersistence::text FROM tables
	UNION ALL
	SELECT 'column:'||t.relname||'.'||a.attname,
 format_type(a.atttypid,a.atttypmod)||'|'||a.attnotnull::text||'|'||coalesce(pg_get_expr(d.adbin,d.adrelid),'')||'|'||a.attidentity::text||'|'||a.attgenerated::text
	FROM tables t JOIN pg_attribute a ON a.attrelid=t.oid
	LEFT JOIN pg_attrdef d ON d.adrelid=t.oid AND d.adnum=a.attnum
	WHERE a.attnum>0 AND NOT a.attisdropped
	UNION ALL
	SELECT 'index:'||t.relname||'.'||c.relname, pg_get_indexdef(i.indexrelid)||'|'||i.indisvalid::text
	FROM tables t JOIN pg_index i ON i.indrelid=t.oid JOIN pg_class c ON c.oid=i.indexrelid
	UNION ALL
	SELECT 'constraint:'||t.relname||'.'||c.conname, pg_get_constraintdef(c.oid)||'|'||c.convalidated::text
	FROM tables t JOIN pg_constraint c ON c.conrelid=t.oid
	`, schema, pq.Array(legacyTables))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		// pg_get_* can qualify identifiers depending on session search_path.
		value = strings.ReplaceAll(value, pq.QuoteIdentifier(schema)+".", "")
		value = strings.ReplaceAll(value, schema+".", "")
		result[key] = value
	}
	return result, rows.Err()
}
