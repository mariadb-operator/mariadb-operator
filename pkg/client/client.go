package client

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-multierror"
)

var (
	ErrWaitReplicaTimeout = errors.New("timeout waiting for replica to be synced")
)

type Opts struct {
	Username string
	Password string
	Host     string
	Port     int32
	Database string
	Params   map[string]string
}

type Client struct {
	db *sql.DB
}

func NewClient(opts Opts) (*Client, error) {
	dsn, err := BuildDSN(opts)
	if err != nil {
		return nil, fmt.Errorf("error building DNS: %v", err)
	}
	db, err := Connect(dsn)
	if err != nil {
		return nil, err
	}
	return &Client{
		db: db,
	}, nil
}

func BuildDSN(opts Opts) (string, error) {
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
	if opts.Params != nil {
		config.Params = opts.Params
	}

	return config.FormatDSN(), nil
}

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

func (c *Client) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := c.db.ExecContext(ctx, sql, args...)
	return err
}

func (c *Client) ExecFlushingPrivileges(ctx context.Context, sql string, args ...any) error {
	var errBundle *multierror.Error
	if err := c.Exec(ctx, sql, args...); err != nil {
		errBundle = multierror.Append(errBundle, err)
	}
	if err := c.FlushPrivileges(ctx); err != nil {
		errBundle = multierror.Append(errBundle, err)
	}
	return errBundle.ErrorOrNil()
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

	return c.ExecFlushingPrivileges(ctx, query)
}

func (c *Client) DropUser(ctx context.Context, username string) error {
	query := fmt.Sprintf("DROP USER IF EXISTS '%s';", username)

	return c.ExecFlushingPrivileges(ctx, query)
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
		escapeWildcard(opts.Database),
		escapeWildcard(opts.Table),
		opts.Username,
		"%",
	)
	if opts.GrantOption {
		query += "WITH GRANT OPTION "
	}
	query += ";"

	return c.ExecFlushingPrivileges(ctx, query)
}

func (c *Client) Revoke(ctx context.Context, opts GrantOpts) error {
	privileges := []string{}
	privileges = append(privileges, opts.Privileges...)
	if opts.GrantOption {
		privileges = append(privileges, "GRANT OPTION")
	}
	query := fmt.Sprintf("REVOKE %s ON %s.%s FROM '%s'@'%s';",
		strings.Join(privileges, ","),
		escapeWildcard(opts.Database),
		escapeWildcard(opts.Table),
		opts.Username,
		"%",
	)

	return c.ExecFlushingPrivileges(ctx, query)
}

func (c *Client) FlushPrivileges(ctx context.Context) error {
	return c.Exec(ctx, "FLUSH PRIVILEGES;")
}

func escapeWildcard(s string) string {
	if s == "*" {
		return s
	}
	return fmt.Sprintf("`%s`", s)
}

type DatabaseOpts struct {
	CharacterSet string
	Collate      string
}

func (c *Client) CreateDatabase(ctx context.Context, database string, opts DatabaseOpts) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` ", database)
	if opts.CharacterSet != "" {
		query += fmt.Sprintf("CHARACTER SET = '%s' ", opts.CharacterSet)
	}
	if opts.Collate != "" {
		query += fmt.Sprintf("COLLATE = '%s' ", opts.Collate)
	}
	query += ";"

	return c.Exec(ctx, query)
}

func (c *Client) DropDatabase(ctx context.Context, database string) error {
	return c.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", database))
}

func (c *Client) GlobalVar(ctx context.Context, variable string) (string, error) {
	sql := fmt.Sprintf("SELECT @@global.%s;", variable)
	row := c.db.QueryRowContext(ctx, sql)

	var val string
	if err := row.Scan(&val); err != nil {
		return "", nil
	}
	return val, nil
}

func (c *Client) SetGlobalVar(ctx context.Context, variable string, value string) error {
	sql := fmt.Sprintf("SET @@global.%s=%s;", variable, value)
	return c.Exec(ctx, sql)
}

func (c *Client) SetGlobalVars(ctx context.Context, keyVal map[string]string) error {
	for k, v := range keyVal {
		if err := c.SetGlobalVar(ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) SetReadOnly(ctx context.Context) error {
	return c.SetGlobalVar(ctx, "read_only", "1")
}

func (c *Client) DisableReadOnly(ctx context.Context) error {
	return c.SetGlobalVar(ctx, "read_only", "0")
}

func (c *Client) ResetMaster(ctx context.Context) error {
	return c.Exec(ctx, "RESET MASTER;")
}

func (c *Client) StartSlave(ctx context.Context, connName string) error {
	sql := fmt.Sprintf("START SLAVE '%s';", connName)
	return c.Exec(ctx, sql)
}

func (c *Client) StopAllSlaves(ctx context.Context) error {
	return c.Exec(ctx, "STOP ALL SLAVES;")
}

func (c *Client) ResetAllSlaves(ctx context.Context) error {
	return c.Exec(ctx, "RESET SLAVE ALL;")
}

func (c *Client) WaitForReplicaGtid(ctx context.Context, gtid string, timeout time.Duration) error {
	sql := fmt.Sprintf("SELECT MASTER_GTID_WAIT('%s', %d);", gtid, int(timeout.Seconds()))
	row := c.db.QueryRowContext(ctx, sql)

	var result int
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("error scanning result: %v", err)
	}

	switch result {
	case 0:
		return nil
	case -1:
		return ErrWaitReplicaTimeout
	default:
		return fmt.Errorf("unexpected result: %d", result)
	}
}

type ChangeMasterOpts struct {
	Connection string
	Host       string
	User       string
	Password   string
	Gtid       string
	Retries    int
}

func (c *Client) ChangeMaster(ctx context.Context, opts *ChangeMasterOpts) error {
	tpl := createTpl("change-master.sql", `CHANGE MASTER '{{ .Connection }}' TO 
MASTER_HOST='{{ .Host }}',
MASTER_USER='{{ .User }}',
MASTER_PASSWORD='{{ .Password }}',
MASTER_USE_GTID={{ .Gtid }},
MASTER_CONNECT_RETRY={{ .Retries }};
`)
	buf := new(bytes.Buffer)
	err := tpl.Execute(buf, opts)
	if err != nil {
		return fmt.Errorf("error generating change master query: %v", err)
	}
	return c.Exec(ctx, buf.String())
}

func (c *Client) ResetSlavePos(ctx context.Context) error {
	sql := fmt.Sprintf("SET @@global.%s='';", "gtid_slave_pos")
	return c.Exec(ctx, sql)
}

func createTpl(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}
