package service

import (
	"context"

	"github.com/strogmv/ang/internal/port"
)

type BlogCached struct {
	base port.Blog
}

func NewBlogCached(base port.Blog) *BlogCached {
	return &BlogCached{base: base}
}
func (c *BlogCached) ArchivePost(ctx context.Context, req port.ArchivePostRequest) (port.ArchivePostResponse, error) {
	return c.base.ArchivePost(ctx, req)
}
func (c *BlogCached) CreateComment(ctx context.Context, req port.CreateCommentRequest) (port.CreateCommentResponse, error) {
	return c.base.CreateComment(ctx, req)
}
func (c *BlogCached) CreatePost(ctx context.Context, req port.CreatePostRequest) (port.CreatePostResponse, error) {
	return c.base.CreatePost(ctx, req)
}
func (c *BlogCached) CreateTag(ctx context.Context, req port.CreateTagRequest) (port.CreateTagResponse, error) {
	return c.base.CreateTag(ctx, req)
}
func (c *BlogCached) DeleteComment(ctx context.Context, req port.DeleteCommentRequest) (port.DeleteCommentResponse, error) {
	return c.base.DeleteComment(ctx, req)
}
func (c *BlogCached) DeletePost(ctx context.Context, req port.DeletePostRequest) (port.DeletePostResponse, error) {
	return c.base.DeletePost(ctx, req)
}
func (c *BlogCached) DeleteTag(ctx context.Context, req port.DeleteTagRequest) (port.DeleteTagResponse, error) {
	return c.base.DeleteTag(ctx, req)
}
func (c *BlogCached) GetPost(ctx context.Context, req port.GetPostRequest) (port.GetPostResponse, error) {
	return c.base.GetPost(ctx, req)
}
func (c *BlogCached) ListComments(ctx context.Context, req port.ListCommentsRequest) (port.ListCommentsResponse, error) {
	return c.base.ListComments(ctx, req)
}
func (c *BlogCached) ListMyPosts(ctx context.Context, req port.ListMyPostsRequest) (port.ListMyPostsResponse, error) {
	return c.base.ListMyPosts(ctx, req)
}
func (c *BlogCached) ListPosts(ctx context.Context, req port.ListPostsRequest) (port.ListPostsResponse, error) {
	return c.base.ListPosts(ctx, req)
}
func (c *BlogCached) ListTags(ctx context.Context, req port.ListTagsRequest) (port.ListTagsResponse, error) {
	return c.base.ListTags(ctx, req)
}
func (c *BlogCached) PublishPost(ctx context.Context, req port.PublishPostRequest) (port.PublishPostResponse, error) {
	return c.base.PublishPost(ctx, req)
}
func (c *BlogCached) SubmitPost(ctx context.Context, req port.SubmitPostRequest) (port.SubmitPostResponse, error) {
	return c.base.SubmitPost(ctx, req)
}
func (c *BlogCached) UpdateComment(ctx context.Context, req port.UpdateCommentRequest) (port.UpdateCommentResponse, error) {
	return c.base.UpdateComment(ctx, req)
}
func (c *BlogCached) UpdatePost(ctx context.Context, req port.UpdatePostRequest) (port.UpdatePostResponse, error) {
	return c.base.UpdatePost(ctx, req)
}
func (c *BlogCached) UpdateTag(ctx context.Context, req port.UpdateTagRequest) (port.UpdateTagResponse, error) {
	return c.base.UpdateTag(ctx, req)
}
