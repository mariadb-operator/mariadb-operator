package client

import (
	"context"
	"fmt"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type UserAccount string

const (
	UserAccountAdmin UserAccount = "admin"
	UserAccountBasic UserAccount = "basic"
)

type UserAttributes struct {
	Account  UserAccount `json:"account"`
	Password *string     `json:"password,omitempty"`
}

type UserClient struct {
	client *mdbhttp.Client
}

func (u *UserClient) CreateAdmin(ctx context.Context, username, password string) error {
	payload := &Object[UserAttributes]{
		Data: Data[UserAttributes]{
			ID:   username,
			Type: ObjectTypeUsers,
			Attributes: UserAttributes{
				Account:  UserAccountAdmin,
				Password: &password,
			},
		},
	}
	res, err := u.client.Post(ctx, "users/inet", payload, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (u *UserClient) Get(ctx context.Context, username string) error {
	res, err := u.client.Get(ctx, userPath(username), nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (u *UserClient) Delete(ctx context.Context, username string) error {
	res, err := u.client.Delete(ctx, userPath(username), nil, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}

func (u *UserClient) DeleteDefaultAdmin(ctx context.Context) error {
	return u.Delete(ctx, defaultAdminUser)
}

func userPath(username string) string {
	return fmt.Sprintf("users/inet/%s", username)
}
