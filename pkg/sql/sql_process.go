package sql

import (
	"context"
	"fmt"
	"slices"
	"strconv"
)

// Process is a partial representation of a single process returned by SHOW PROCESSLIST
type Process struct {
	ID      int
	Command string
	// Time is the time the process has been running in seconds
	Time int
}

var SafeCommands = []string{
	"Query",          // Executing a statement.
	"Sleep",          // Waiting for the client to send a new statement
	"Delayed insert", // Executing a prepared statement.
	"Fetch",          // Fetching the results of an executed prepared statement
	"Field List",     // Retrieving table column information.
	"Long Data",      // Retrieving long data from the result of executing a prepared statement
	"Ping",           // Handling a server Ping request
	"Prepare",        // Preparing a prepared statement
	"Reset stmt",     // Resetting a prepared statement
}

// IsSafeToTerminate kills processes that are safe to terminate
// Ref: https://mariadb.com/docs/server/ha-and-performance/optimization-and-tuning/buffers-caches-and-threads/thread-command-values
func (p *Process) IsSafeToTerminate() bool {
	if p == nil {
		return false
	}

	return slices.Contains(SafeCommands, p.Command)
}

// GetProcessList will return all the processes of `SHOW PROCESSLIST`
// @WARN: How big can this get? We should monitor cases where there are many processes open, we may want to paginate.
func (c *Client) GetProcessList(ctx context.Context) ([]Process, error) {
	rows, err := c.QueryColumnMaps(ctx, "SHOW PROCESSLIST")
	if err != nil {
		return nil, err
	}
	processes := make([]Process, len(rows))

	for i, row := range rows {
		process := Process{}

		if column, ok := row["Id"]; ok {
			id, err := strconv.Atoi(column)
			if err != nil {
				return nil, err
			}
			process.ID = id
		}

		if column, ok := row["Command"]; ok {
			process.Command = column
		}

		if column, ok := row["Time"]; ok {
			time, err := strconv.Atoi(column)
			if err != nil {
				return nil, err
			}
			process.Time = time
		}

		processes[i] = process
	}

	return processes, nil
}

// SoftKillProcess will kill a process without affecting critical operations.
// @NOTE: This still returns an error, so See: IgnoreYouAreNotOwnerOfThread()
func (c *Client) SoftKillProcess(ctx context.Context, process Process) error {
	return c.Exec(ctx, fmt.Sprintf("KILL SOFT CONNECTION %d", process.ID))
}
