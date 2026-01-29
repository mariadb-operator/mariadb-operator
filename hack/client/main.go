package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
)

var (
	host              string
	port              int
	username          string
	password          string
	database          string
	table             string
	truncateTable     bool
	timeout           time.Duration
	mode              string
	insertInterval    time.Duration
	validateThreshold time.Duration
	rawTargetTime     string
	targetTime        *time.Time
)

func init() {
	flag.StringVar(&host, "host", "mariadb-repl-primary.default.svc.cluster.local", "Database host")
	flag.IntVar(&port, "port", 3306, "Database port")
	flag.StringVar(&username, "username", "root", "Database username")
	flag.StringVar(&password, "password", "MariaDB11!", "Database password")
	flag.StringVar(&database, "database", "test", "Database name")
	flag.StringVar(&table, "table", "test", "Table name")
	flag.BoolVar(&truncateTable, "truncate", false, "Truncate table before inserting in insert mode")
	flag.DurationVar(&timeout, "timeout", 3*time.Second, "Connection timeout (e.g. 3s)")
	flag.StringVar(&mode, "mode", "insert", "Mode: insert or validate")
	flag.DurationVar(&insertInterval, "insert-interval", 1*time.Second, "Insert interval (e.g. 1s)")
	flag.DurationVar(&validateThreshold, "validate-threshold", 1*time.Second, "Validation threshold for gap checking (e.g. 1s)")
	flag.StringVar(&rawTargetTime, "target-time", "", "Target time for PITR validation (format: RFC3339 (1970-01-01T00:00:00Z))")
	flag.Parse()
}

func main() {
	if rawTargetTime != "" {
		parsed, err := time.Parse(time.RFC3339, rawTargetTime)
		if err != nil {
			log.Fatalf("error parsing time %s: %v", rawTargetTime, err)
		}
		targetTime = &parsed
	}
	client, err := sql.NewClient(
		sql.WithHost(host),
		sql.WithUsername(username),
		sql.WithPassword(password),
		sql.WithPort(int32(port)),
		sql.WithTimeout(timeout),
		sql.WithParams(map[string]string{
			"parseTime": "true",
		}),
	)
	if err != nil {
		log.Fatalf("error getting client: %v", err)
	}
	defer client.Close()

	ctx, cancel := newContext()
	defer cancel()

	if err := client.CreateDatabase(ctx, database, sql.DatabaseOpts{}); err != nil {
		log.Fatalf("error creating database: %v", err)
	}
	if err := createTable(ctx, client); err != nil {
		log.Fatalf("error creating table: %v", err)
	}

	switch mode {
	case "insert":
		insertMode(ctx, client)
	case "validate":
		validateMode(ctx, client)
	default:
		log.Fatalf("invalid mode: %s", mode)
	}
}

func newContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT}...,
	)
}

func insertMode(ctx context.Context, client *sql.Client) {
	if truncateTable {
		if err := client.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s.%s", escape(database), escape(table))); err != nil {
			log.Fatalf("error truncating table: %v", err)
		}
		log.Print("table truncated")
	}

	ticker := time.NewTicker(insertInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("context canceled, exiting")
			return
		case t := <-ticker.C:
			value := t.Unix()
			if err := insert(ctx, client, value); err != nil {
				log.Printf("error inserting: %v", err)
				continue
			}
			log.Printf("inserted at %d\n", value)
		}
	}
}

func validateMode(ctx context.Context, client *sql.Client) {
	query, args := buildValidateQuery(targetTime)
	rows, err := client.Query(ctx, query, args...)
	if err != nil {
		log.Fatalf("error querying table: %v", err)
	}
	defer rows.Close()

	var lastValue int64
	var count int64
	for rows.Next() {
		var id int64
		var value int64
		var createdAt time.Time
		if err := rows.Scan(&id, &value, &createdAt); err != nil {
			log.Fatalf("error scanning row: %v", err)
		}
		count++
		if lastValue != 0 {
			gap := time.Duration(value-lastValue) * time.Second
			if gap > validateThreshold {
				prevRow, err := getRowById(ctx, client, id-1)
				if err != nil {
					log.Fatalf("error getting previous row: %v", err)
				}
				log.Printf(`gap %v exceeds threshold %v.
Last row: id=%d, value=%d, created_at=%v
Current row: id=%d, value=%d, created_at=%v`,
					gap, validateThreshold,
					prevRow.id, prevRow.value, prevRow.createdAt,
					id, value, createdAt)
			}
		}
		lastValue = value
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("error iterating rows: %v", err)
	}
	log.Printf("processed %d rows", count)
}

func buildValidateQuery(targetTime *time.Time) (string, []interface{}) {
	query := fmt.Sprintf("SELECT id, value, created_at FROM %s.%s", escape(database), escape(table))
	var args []interface{}
	if targetTime != nil {
		query += " WHERE created_at <= ?"
		args = append(args, *targetTime)
	}
	query += " ORDER BY created_at ASC"
	return query, args
}

func getRowById(ctx context.Context, client *sql.Client, id int64) (row struct {
	id        int64
	value     int64
	createdAt time.Time
}, err error) {
	query := fmt.Sprintf("SELECT id, value, created_at FROM %s.%s WHERE id = ?", escape(database), escape(table))
	rowPtr := &row
	err = client.QueryRow(ctx, query, id).Scan(&rowPtr.id, &rowPtr.value, &rowPtr.createdAt)
	return
}

func createTable(ctx context.Context, client *sql.Client) error {
	ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.%s (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  value BIGINT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`, escape(database), escape(table))
	if err := client.Exec(ctx, ddl); err != nil {
		return err
	}
	return nil
}

func insert(ctx context.Context, client *sql.Client, value int64) error {
	query := fmt.Sprintf("INSERT INTO %s.%s (value) VALUES (?)", escape(database), escape(table))
	if err := client.Exec(ctx, query, value); err != nil {
		return err
	}
	return nil
}

func escape(name string) string {
	return "`" + name + "`"
}
