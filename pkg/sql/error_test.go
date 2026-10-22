package sql

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsUnknownSystemVariable(t *testing.T) {
	// Returned by servers built without Galera support, such as Amazon RDS, when reading wsrep_on.
	unknownVariable := &mysql.MySQLError{
		Number:   1193,
		SQLState: [5]byte{'H', 'Y', '0', '0', '0'},
		Message:  "Unknown system variable 'wsrep_on'",
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "unknown system variable",
			err:  unknownVariable,
			want: true,
		},
		{
			name: "wrapped unknown system variable",
			err:  fmt.Errorf("error getting system variable: %w", unknownVariable),
			want: true,
		},
		{
			name: "other server error",
			err: &mysql.MySQLError{
				Number:   1141,
				SQLState: [5]byte{'4', '2', '0', '0', '0'},
				Message:  "There is no such grant defined for user 'test' on host '%'",
			},
			want: false,
		},
		{
			name: "connection error",
			err:  errors.New("dial tcp 10.96.0.10:3306: connect: connection refused"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnknownSystemVariable(tt.err); got != tt.want {
				t.Errorf("IsUnknownSystemVariable() = %v, want %v", got, tt.want)
			}
		})
	}
}
