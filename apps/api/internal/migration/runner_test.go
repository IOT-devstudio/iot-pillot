package migration

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// These tests require a disposable PostgreSQL server with CREATEDB permission.
// Each case creates its own database; never point this at production.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("IOT_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("IOT_MIGRATION_TEST_DSN is unset")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE "` + name + `"`); err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	return db
}

func execSQL(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func manual(t *testing.T, db *sql.DB, versions []uint) {
	t.Helper()
	for _, version := range versions {
		query, err := readUp(version)
		if err != nil {
			t.Fatal(err)
		}
		execSQL(t, db, string(query))
	}
}

func TestEmptyDatabaseAndRepeat(t *testing.T) {
	db := testDB(t)
	for range 2 {
		if err := Run(context.Background(), db, "up", nil); err != nil {
			t.Fatal(err)
		}
	}
	var version int
	var dirty bool
	if err := db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version != 9 || dirty {
		t.Fatalf("version=%d dirty=%v", version, dirty)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM recruitment_directions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("seed count=%d", count)
	}
}

func TestAdoptNonContiguousPreservesDataAndDDL(t *testing.T) {
	db := testDB(t)
	applied := []uint{1, 2, 3, 4, 5, 7, 9}
	manual(t, db, applied)
	execSQL(t, db, "INSERT INTO users (name, detail_email) VALUES ('existing','existing@example.test'); UPDATE recruitment_rounds SET name='custom' WHERE round_num=1")
	var before uint32
	if err := db.QueryRow("SELECT 'idx_mail_models_type'::regclass::oid").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db, "up", nil); err == nil {
		t.Fatal("unmanaged database accepted")
	}
	if err := Run(context.Background(), db, "check", applied); err != nil {
		t.Fatal(err)
	}
	var ledger bool
	if err := db.QueryRow("SELECT to_regclass('public.migration_adoptions') IS NOT NULL").Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger {
		t.Fatal("check wrote adoption ledger")
	}
	if err := Run(context.Background(), db, "adopt", applied); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Run(context.Background(), db, "up", nil); err != nil {
			t.Fatal(err)
		}
	}
	var after uint32
	if err := db.QueryRow("SELECT 'idx_mail_models_type'::regclass::oid").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("adopted DDL was replayed")
	}
	var name, qq string
	if err := db.QueryRow("SELECT name, coalesce(detail_qq,'') FROM users WHERE detail_email='existing@example.test'").Scan(&name, &qq); err != nil {
		t.Fatal(err)
	}
	if name != "existing" || qq != "" {
		t.Fatal("existing data changed")
	}
	if err := db.QueryRow("SELECT name FROM recruitment_rounds WHERE round_num=1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "custom" {
		t.Fatal("seed data was replayed")
	}
}

func TestAdoptRejectsWrongManifestAndDrift(t *testing.T) {
	for _, scenario := range []string{"missing6", "wrongtype", "wrongindex", "partial3"} {
		t.Run(scenario, func(t *testing.T) {
			db := testDB(t)
			actual := []uint{1, 2, 3, 4, 5, 7, 8, 9}
			manual(t, db, actual)
			claimed := append([]uint{}, actual...)
			switch scenario {
			case "missing6":
				claimed = []uint{1, 2, 3, 4, 5, 6, 7, 8, 9}
			case "wrongtype":
				execSQL(t, db, "ALTER TABLE users ALTER COLUMN detail_student_id TYPE text")
			case "wrongindex":
				execSQL(t, db, "DROP INDEX idx_forms_share_token; CREATE INDEX idx_forms_share_token ON forms(share_token)")
			case "partial3":
				execSQL(t, db, `ALTER TABLE mails RENAME COLUMN to_user_id TO "to"`)
			}
			if err := Run(context.Background(), db, "adopt", claimed); err == nil {
				t.Fatal("unsafe baseline accepted")
			}
			var exists bool
			if err := db.QueryRow("SELECT to_regclass('public.migration_adoptions') IS NOT NULL").Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists {
				t.Fatal("failed adoption left metadata")
			}
		})
	}
}

func TestMigrationFailureAndDirtyRetry(t *testing.T) {
	db := testDB(t)
	applied := []uint{1, 2, 3, 4}
	manual(t, db, applied)
	execSQL(t, db, "INSERT INTO users(name,detail_email) VALUES ('a','duplicate@example.test'),('b','duplicate@example.test')")
	if err := Run(context.Background(), db, "adopt", applied); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db, "up", nil); err == nil {
		t.Fatal("duplicate email migration succeeded")
	}
	var dirty bool
	if err := db.QueryRow("SELECT dirty FROM schema_migrations").Scan(&dirty); err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Fatal("failure not marked dirty")
	}
	if err := Run(context.Background(), db, "up", nil); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("retry=%v", err)
	}
}

func TestManifestValidation(t *testing.T) {
	for _, input := range []string{"", "1,1", "0", "10", "1,x", "1,3", "6"} {
		if _, err := ParseApplied(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	versions, err := ParseApplied("1,2,3,4,5,7,9")
	if err != nil || len(versions) != 7 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
}

func TestAdoptOnlyRecruitmentAndForms(t *testing.T) {
	db := testDB(t)
	applied := []uint{7, 9}
	manual(t, db, applied)
	if err := Run(context.Background(), db, "adopt", applied); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db, "up", nil); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentOperationRejected(t *testing.T) {
	db := testDB(t)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_lock(107, 20261014)"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock(107, 20261014)")
	if err := Run(context.Background(), db, "up", nil); err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("concurrent run: %v", err)
	}
}

func TestAdoptionChecksumChangeRejected(t *testing.T) {
	db := testDB(t)
	manual(t, db, []uint{1})
	if err := Run(context.Background(), db, "adopt", []uint{1}); err != nil {
		t.Fatal(err)
	}
	execSQL(t, db, "UPDATE migration_adoptions SET checksum='changed'")
	if err := Run(context.Background(), db, "up", nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum guard: %v", err)
	}
}

func TestAdoptRejectsIncompleteRecruitmentSeeds(t *testing.T) {
	for _, scenario := range []string{"no_seeds", "missing_direction"} {
		t.Run(scenario, func(t *testing.T) {
			db := testDB(t)
			if scenario == "no_seeds" {
				query, err := readUp(7)
				if err != nil {
					t.Fatal(err)
				}
				// Reproduce manually running the DDL but omitting the seed INSERTs.
				prefix := strings.SplitN(string(query), "INSERT INTO recruitment_rounds", 2)[0]
				execSQL(t, db, prefix+"COMMIT;")
			} else {
				manual(t, db, []uint{7})
				execSQL(t, db, "DELETE FROM recruitment_directions WHERE code='agent'")
			}
			if err := Run(context.Background(), db, "adopt", []uint{7}); err == nil {
				t.Fatal("incomplete seed state accepted")
			}
		})
	}
}
