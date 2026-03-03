package port

import (
	"context"
)

type Blog interface {
	ArchivePost(ctx context.Context, req ArchivePostRequest) (ArchivePostResponse, error)
	CreateComment(ctx context.Context, req CreateCommentRequest) (CreateCommentResponse, error)
	CreatePost(ctx context.Context, req CreatePostRequest) (CreatePostResponse, error)
	CreateTag(ctx context.Context, req CreateTagRequest) (CreateTagResponse, error)
	DeleteComment(ctx context.Context, req DeleteCommentRequest) (DeleteCommentResponse, error)
	DeletePost(ctx context.Context, req DeletePostRequest) (DeletePostResponse, error)
	DeleteTag(ctx context.Context, req DeleteTagRequest) (DeleteTagResponse, error)
	GetPost(ctx context.Context, req GetPostRequest) (GetPostResponse, error)
	ListComments(ctx context.Context, req ListCommentsRequest) (ListCommentsResponse, error)
	ListMyPosts(ctx context.Context, req ListMyPostsRequest) (ListMyPostsResponse, error)
	ListPosts(ctx context.Context, req ListPostsRequest) (ListPostsResponse, error)
	ListTags(ctx context.Context, req ListTagsRequest) (ListTagsResponse, error)
	PublishPost(ctx context.Context, req PublishPostRequest) (PublishPostResponse, error)
	SubmitPost(ctx context.Context, req SubmitPostRequest) (SubmitPostResponse, error)
	UpdateComment(ctx context.Context, req UpdateCommentRequest) (UpdateCommentResponse, error)
	UpdatePost(ctx context.Context, req UpdatePostRequest) (UpdatePostResponse, error)
	UpdateTag(ctx context.Context, req UpdateTagRequest) (UpdateTagResponse, error)
}

// Request/Response DTOs
type ArchivePostRequest struct {
	ID string `json:"ID"`
}

func (d *ArchivePostRequest) Validate() error {
	return nil
}

type ArchivePostResponse struct {
	Ok bool `json:"ok"`
}

func (d *ArchivePostResponse) Validate() error {
	return nil
}

type CreateCommentRequest struct {
	PostID  string `json:"postId"`
	Content string `json:"content"`
	UserID  string `json:"userId"`
}

func (d *CreateCommentRequest) Validate() error {
	return nil
}

type CreateCommentResponse struct {
	ID string `json:"ID"`
}

func (d *CreateCommentResponse) Validate() error {
	return nil
}

type CreatePostRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	UserID  string   `json:"userId"`
	Tags    []string `json:"tags"`
}

func (d *CreatePostRequest) Validate() error {
	return nil
}

type CreatePostResponse struct {
	ID   string `json:"ID"`
	Slug string `json:"slug"`
}

func (d *CreatePostResponse) Validate() error {
	return nil
}

type CreateTagRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (d *CreateTagRequest) Validate() error {
	return nil
}

type CreateTagResponse struct {
	ID   string `json:"ID"`
	Slug string `json:"slug"`
}

func (d *CreateTagResponse) Validate() error {
	return nil
}

type DeleteCommentRequest struct {
	ID     string `json:"ID"`
	UserID string `json:"userId"`
}

func (d *DeleteCommentRequest) Validate() error {
	return nil
}

type DeleteCommentResponse struct {
	Ok bool `json:"ok"`
}

func (d *DeleteCommentResponse) Validate() error {
	return nil
}

type DeletePostRequest struct {
	ID string `json:"ID"`
}

func (d *DeletePostRequest) Validate() error {
	return nil
}

type DeletePostResponse struct {
	Ok bool `json:"ok"`
}

func (d *DeletePostResponse) Validate() error {
	return nil
}

type DeleteTagRequest struct {
	ID string `json:"ID"`
}

func (d *DeleteTagRequest) Validate() error {
	return nil
}

type DeleteTagResponse struct {
	Ok bool `json:"ok"`
}

func (d *DeleteTagResponse) Validate() error {
	return nil
}

type GetPostRequest struct {
	Slug   string `json:"slug"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (d *GetPostRequest) Validate() error {
	return nil
}

type GetPostResponse struct {
	ID        string                    `json:"ID"`
	Title     string                    `json:"title"`
	Content   string                    `json:"content"`
	AuthorID  string                    `json:"authorId"`
	CreatedAt string                    `json:"createdAt"`
	Tags      []GetPostResponseTagsItem `json:"tags"`
}

func (d *GetPostResponse) Validate() error {
	return nil
}

type ListCommentsRequest struct {
	PostID string `json:"postId"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (d *ListCommentsRequest) Validate() error {
	return nil
}

type ListCommentsResponse struct {
	Data  []ListCommentsResponseData `json:"data"`
	Total int                        `json:"total"`
}

func (d *ListCommentsResponse) Validate() error {
	return nil
}

type ListMyPostsRequest struct {
	UserID string `json:"userId"`
	Status string `json:"status"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (d *ListMyPostsRequest) Validate() error {
	return nil
}

type ListMyPostsResponse struct {
	Data []ListMyPostsResponseData `json:"data"`
}

func (d *ListMyPostsResponse) Validate() error {
	return nil
}

type ListPostsRequest struct {
	Tag    string `json:"tag"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (d *ListPostsRequest) Validate() error {
	return nil
}

type ListPostsResponse struct {
	Data  []ListPostsResponseData `json:"data"`
	Total int                     `json:"total"`
}

func (d *ListPostsResponse) Validate() error {
	return nil
}

type ListTagsRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func (d *ListTagsRequest) Validate() error {
	return nil
}

type ListTagsResponse struct {
	Data []ListTagsResponseData `json:"data"`
}

func (d *ListTagsResponse) Validate() error {
	return nil
}

type PublishPostRequest struct {
	ID string `json:"ID"`
}

func (d *PublishPostRequest) Validate() error {
	return nil
}

type PublishPostResponse struct {
	Ok bool `json:"ok"`
}

func (d *PublishPostResponse) Validate() error {
	return nil
}

type SubmitPostRequest struct {
	ID string `json:"ID"`
}

func (d *SubmitPostRequest) Validate() error {
	return nil
}

type SubmitPostResponse struct {
	Ok bool `json:"ok"`
}

func (d *SubmitPostResponse) Validate() error {
	return nil
}

type UpdateCommentRequest struct {
	ID      string `json:"ID"`
	Content string `json:"content"`
	UserID  string `json:"userId"`
}

func (d *UpdateCommentRequest) Validate() error {
	return nil
}

type UpdateCommentResponse struct {
	Ok bool `json:"ok"`
}

func (d *UpdateCommentResponse) Validate() error {
	return nil
}

type UpdatePostRequest struct {
	ID      string `json:"ID"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (d *UpdatePostRequest) Validate() error {
	return nil
}

type UpdatePostResponse struct {
	Ok bool `json:"ok"`
}

func (d *UpdatePostResponse) Validate() error {
	return nil
}

type UpdateTagRequest struct {
	ID          string `json:"ID"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (d *UpdateTagRequest) Validate() error {
	return nil
}

type UpdateTagResponse struct {
	Ok bool `json:"ok"`
}

func (d *UpdateTagResponse) Validate() error {
	return nil
}

type GetPostResponseTagsItem struct {
	ID   string `json:"ID"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (d *GetPostResponseTagsItem) Validate() error {
	return nil
}

type ListCommentsResponseData struct {
	ID        string `json:"ID"`
	Content   string `json:"content"`
	UserID    string `json:"userId"`
	CreatedAt string `json:"createdAt"`
}

func (d *ListCommentsResponseData) Validate() error {
	return nil
}

type ListMyPostsResponseData struct {
	ID        string `json:"ID"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

func (d *ListMyPostsResponseData) Validate() error {
	return nil
}

type ListPostsResponseData struct {
	ID        string `json:"ID"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	AuthorID  string `json:"authorId"`
	CreatedAt string `json:"createdAt"`
}

func (d *ListPostsResponseData) Validate() error {
	return nil
}

type ListTagsResponseData struct {
	ID          string `json:"ID"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func (d *ListTagsResponseData) Validate() error {
	return nil
}
