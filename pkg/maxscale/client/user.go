package client

import (
	"context"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type UserAccount string

const (
	UserAccountBasic UserAccount = "basic"
	UserAccountAdmin UserAccount = "admin"
)

type UserAttributes struct {
	Account  UserAccount `json:"account"`
	Password *string     `json:"password,omitempty"`
}

type UserClient struct {
	GenericClient[UserAttributes]
}

func NewUserClient(client *mdbhttp.Client) *UserClient {
	return &UserClient{
		GenericClient: NewGenericClient[UserAttributes](
			client,
			"users/inet",
			ObjectTypeUsers,
		),
	}
}

func (u *UserClient) DeleteDefaultAdmin(ctx context.Context) error {
	return u.Delete(ctx, defaultAdminUser)
}
