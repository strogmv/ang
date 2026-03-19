package port

import (
	"context"
)

type User interface {
	ListUsers(ctx context.Context, req ListUsersRequest) (ListUsersResponse, error)
}

// Request/Response DTOs
type ListUsersRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func (d *ListUsersRequest) Validate() error {
	return nil
}

type ListUsersResponse struct {
	Data []ListUsersResponseData `json:"data"`
}

func (d *ListUsersResponse) Validate() error {
	return nil
}

type ListUsersResponseData struct {
	ID    string `json:"ID"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (d *ListUsersResponseData) Validate() error {
	return nil
}
