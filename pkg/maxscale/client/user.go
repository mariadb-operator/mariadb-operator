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

type User struct {
	client *mdbhttp.Client
}

func (u *User) CreateAdmin(ctx context.Context, username, password string) error {
	payload := &Payload[UserAttributes]{
		Data: PayloadData[UserAttributes]{
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
		return fmt.Errorf("error creating admin user: %v", err)
	}
	return handleResponse(res, nil)
}

func (u *User) Get(ctx context.Context, username string) error {
	res, err := u.client.Delete(ctx, userPath(username), nil, nil)
	if err != nil {
		return fmt.Errorf("error getting user: %v", err)
	}
	return handleResponse(res, nil)
}

func (u *User) Delete(ctx context.Context, username string) error {
	res, err := u.client.Delete(ctx, userPath(username), nil, nil)
	if err != nil {
		return fmt.Errorf("error deleting user: %v", err)
	}
	return handleResponse(res, nil)
}

func userPath(username string) string {
	return fmt.Sprintf("users/inet/%s", username)
}
