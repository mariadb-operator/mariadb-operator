package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
)

type Opts struct {
	Username string
	Password string
	Host     string
	Port     int32
	Database string
}

type MariaDB struct {
	db *sql.DB
}

func New(opts Opts) (*MariaDB, error) {
	dsn, err := buildDSN(opts)
	if err != nil {
		return nil, fmt.Errorf("error building DNS: %v", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &MariaDB{
		db: db,
	}, nil
}

func (m *MariaDB) Close() error {
	return m.db.Close()
}

type CreateUserOpts struct {
	Password           string
	MaxUserConnections int32
}

func (m *MariaDB) CreateUser(ctx context.Context, username string, opts CreateUserOpts) error {
	query := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' ", username, "%")
	if opts.Password != "" {
		query += fmt.Sprintf("IDENTIFIED BY '%s' ", opts.Password)
	}
	if opts.MaxUserConnections != 0 {
		query += fmt.Sprintf("WITH MAX_USER_CONNECTIONS %d ", opts.MaxUserConnections)
	}
	query += ";"

	_, err := m.db.ExecContext(ctx, query)

	return err
}

func buildDSN(opts Opts) (string, error) {
	if opts.Host == "" || opts.Port == 0 {
		return "", errors.New("invalid opts: host and port are mandatory")
	}
	config := mysql.NewConfig()
	config.Net = "tcp"
	config.Addr = fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	if opts.Username != "" && opts.Password != "" {
		config.User = opts.Username
		config.Passwd = opts.Password
	}
	if opts.Database != "" {
		config.DBName = opts.Database
	}

	return config.FormatDSN(), nil
}
