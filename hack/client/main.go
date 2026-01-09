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
	host           string
	port           int
	username       string
	password       string
	database       string
	table          string
	timeout        time.Duration
	insertInterval time.Duration
)

func main() {
	flag.StringVar(&host, "host", "mariadb-repl-primary.default.svc.cluster.local", "Database host")
	flag.IntVar(&port, "port", 3306, "Database port")
	flag.StringVar(&username, "username", "root", "Database username")
	flag.StringVar(&password, "password", "MariaDB11!", "Database password")
	flag.StringVar(&database, "database", "test", "Database name")
	flag.StringVar(&table, "table", "test", "Table name")
	flag.DurationVar(&timeout, "timeout", 3*time.Second, "Connection timeout (e.g. 3s)")
	flag.DurationVar(&insertInterval, "insert-interval", 1*time.Second, "Insert interval (e.g. 1s)")
	flag.Parse()

	client, err := sql.NewClient(
		sql.WithHost(host),
		sql.WithUsername(username),
		sql.WithPassword(password),
		sql.WithPort(int32(port)),
		sql.WithTimeout(timeout),
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

	ticker := time.NewTicker(insertInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("context cancelled, exiting")
			return
		case t := <-ticker.C:
			value := t.Unix()
			if err := insert(ctx, client, value); err != nil {
				log.Printf("error inserting: %v", err)
				continue
			}
			log.Printf("inserted at %d \n", value)
		}
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
