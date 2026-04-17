package sql

import (
	"fmt"
	"strings"
)

var (
	// ERROR 1617 (HY000): There is no master connection  '<conn_name'
	// Ref: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1600-to-1699/e1617
	SQLConnectionNotExists = 1617
	// Error 1948 (HY000): Specified value for @@gtid_slave_pos contains no value for
	// replication domain 0. This conflicts with the binary log which contains GTID
	// 0-11-1176. If MASTER_GTID_POS=CURRENT_POS is used, the binlog position will
	// override the new value of @@gtid_slave_pos'
	// Ref: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1900-to-1999/e1948
	SQLGtidSlavePosNoValueForDomain = 1948
	// Ref: https://mariadb.com/docs/server/reference/error-codes/mariadb-error-codes-1000-to-1099/e1095
	SQLYouAreNotOwnerOfThread = 1095
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
func IsConnectionNotExists(err error) bool {
	return IsSQLErrorNumber(err, SQLConnectionNotExists)
}

// Cannot set `gtid_slave_pos`
func IsGtidSlavePosNoValueForDomain(err error) bool {
	return IsSQLErrorNumber(err, SQLGtidSlavePosNoValueForDomain)
}

// You are not owner of thread
func IgnoreYouAreNotOwnerOfThread(err error) error {
	return returnNilIfErrorIsNumber(err, SQLYouAreNotOwnerOfThread)
}
