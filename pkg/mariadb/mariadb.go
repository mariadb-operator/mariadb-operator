package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

type Client struct {
	db *sql.DB
}

func NewClient(opts Opts) (*Client, error) {
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

	return &Client{
		db: db,
	}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

type CreateUserOpts struct {
	IdentifiedBy       string
	MaxUserConnections int32
}

func (c *Client) CreateUser(ctx context.Context, username string, opts CreateUserOpts) error {
	query := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' ", username, "%")
	if opts.IdentifiedBy != "" {
		query += fmt.Sprintf("IDENTIFIED BY '%s' ", opts.IdentifiedBy)
	}
	if opts.MaxUserConnections != 0 {
		query += fmt.Sprintf("WITH MAX_USER_CONNECTIONS %d ", opts.MaxUserConnections)
	}
	query += ";"

	_, err := c.db.ExecContext(ctx, query)
	return err
}

func (m *Client) DropUser(ctx context.Context, username string) error {
	query := fmt.Sprintf("DROP USER IF EXISTS '%s';", username)

	_, err := m.db.ExecContext(ctx, query)
	return err
}

type GrantOpts struct {
	Privileges  []string
	Database    string
	Table       string
	Username    string
	GrantOption bool
}

func (c *Client) Grant(ctx context.Context, opts GrantOpts) error {
	query := fmt.Sprintf("GRANT %s ON %s.%s TO '%s'@'%s' ",
		strings.Join(opts.Privileges, ","),
		opts.Database,
		opts.Table,
		opts.Username,
		"%",
	)
	if opts.GrantOption {
		query += "WITH GRANT OPTION "
	}
	query += ";"

	_, err := c.db.ExecContext(ctx, query)
	return err
}

func (c *Client) Revoke(ctx context.Context, opts GrantOpts) error {
	privileges := []string{}
	privileges = append(privileges, opts.Privileges...)
	if opts.GrantOption {
		privileges = append(privileges, "GRANT OPTION")
	}
	query := fmt.Sprintf("REVOKE %s ON %s.%s FROM '%s'@'%s';",
		strings.Join(privileges, ","),
		opts.Database,
		opts.Table,
		opts.Username,
		"%",
	)

	_, err := c.db.ExecContext(ctx, query)
	return err
}

type DatabaseOpts struct {
	CharacterSet string
	Collate      string
}

func (c *Client) CreateDatabase(ctx context.Context, database string, opts DatabaseOpts) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s ", database)
	if opts.CharacterSet != "" {
		query += fmt.Sprintf("CHARACTER SET = '%s' ", opts.CharacterSet)
	}
	if opts.Collate != "" {
		query += fmt.Sprintf("COLLATE = '%s' ", opts.Collate)
	}
	query += ";"

	_, err := c.db.ExecContext(ctx, query)
	return err
}

func (c *Client) DropDatabase(ctx context.Context, database string) error {
	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s;", database)

	_, err := c.db.ExecContext(ctx, query)
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
