package client

import (
	"context"
	"fmt"

	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
)

type UserAttributes struct {
	Account  string  `json:"account"`
	Password *string `json:"password,omitempty"`
}

type UserClient struct {
	ReadClient[UserAttributes]
	client *mdbhttp.Client
}

func NewUserClient(client *mdbhttp.Client) *UserClient {
	return &UserClient{
		ReadClient: NewListClient[UserAttributes](client, "users/inet"),
		client:     client,
	}
}

func (u *UserClient) CreateAdmin(ctx context.Context, username, password string) error {
	payload := &Object[UserAttributes]{
		Data: Data[UserAttributes]{
			ID:   username,
			Type: ObjectTypeUsers,
			Attributes: UserAttributes{
				Account:  "admin",
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
