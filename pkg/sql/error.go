package sql

import (
	"fmt"
	"strings"
)

var (
	// Ref: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1095
	SQLYouAreNotOwnerOfThread = 1095
	// ERROR 1617 (HY000): There is no master connection  '<conn_name'
	// Ref: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1600-to-1699/e1617
	SQLConnectionNotExists = 1617
)

// IsSQLErrorNumber checks if the error's string message contains the pattern
// "Error NNNN" where NNNN is the specified error number.
func IsSQLErrorNumber(err error, number int) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), fmt.Sprintf("Error %d", number))
}

func returnNilIfErrorIsNumber(err error, number int) error {
	if IsSQLErrorNumber(err, number) {
		return nil
	}

	return err
}

// Connection Not Exists
func IgnoreConnectionNotExists(err error) error {
	return returnNilIfErrorIsNumber(err, SQLConnectionNotExists)
}

// You are not owner of thread
func IgnoreYouAreNotOwnerOfThread(err error) error {
	return returnNilIfErrorIsNumber(err, SQLYouAreNotOwnerOfThread)
}
