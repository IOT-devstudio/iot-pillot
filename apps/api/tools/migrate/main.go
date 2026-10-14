// iot-migrate is intentionally separate from API startup and its dependencies.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/migration"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: iot-migrate up | check/adopt --applied 1,2,3,4,5,7,9")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	appliedFlag := flags.String("applied", "", "verified manually applied versions (check/adopt only)")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	var applied []uint
	if command == "check" || command == "adopt" {
		var err error
		applied, err = migration.ParseApplied(*appliedFlag)
		if err != nil {
			return err
		}
	} else if *appliedFlag != "" {
		return fmt.Errorf("applied is only accepted by check/adopt")
	}
	if command != "up" && command != "check" && command != "adopt" {
		return fmt.Errorf("command must be up, check or adopt")
	}
	// Credentials are read only from process env, never from command-line flags.
	u := url.URL{Scheme: "postgres", Path: "/" + os.Getenv("IOT_PILOT_DB_NAME")}
	for _, key := range []string{"HOST", "PORT", "USER", "PASSWORD", "NAME"} {
		if os.Getenv("IOT_PILOT_DB_"+key) == "" {
			return fmt.Errorf("missing IOT_PILOT_DB_%s", key)
		}
	}
	u.Host = net.JoinHostPort(os.Getenv("IOT_PILOT_DB_HOST"), os.Getenv("IOT_PILOT_DB_PORT"))
	u.User = url.UserPassword(os.Getenv("IOT_PILOT_DB_USER"), os.Getenv("IOT_PILOT_DB_PASSWORD"))
	query := url.Values{"sslmode": {envDefault("IOT_PILOT_DB_SSLMODE", "disable")}, "connect_timeout": {"10"}}
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		return fmt.Errorf("database configuration is invalid")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection failed (credentials omitted)")
	}
	if err := migration.Run(ctx, db, command, applied); err != nil {
		return err
	}
	fmt.Printf("migration %s succeeded\n", command)
	return nil
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
