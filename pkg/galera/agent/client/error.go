package client

import (
	"net/http"

	"github.com/mariadb-operator/mariadb-operator/pkg/galera/errors"
)

func IsNotFound(err error) bool {
	if clientErr, ok := err.(*errors.Error); ok {
		return clientErr.HTTPCode == http.StatusNotFound
	}
	return false
}
