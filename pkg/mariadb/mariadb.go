package mariadb

import (
	"database/sql"
	"fmt"

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
	dsn := fmt.Sprintf("%s:%s@%s:%d/%s",
		opts.Username,
		opts.Password,
		opts.Host,
		opts.Port,
		opts.Database,
	)
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
