package service

import (
	"context"

	"github.com/strogmv/ang/internal/port"
)

type BlogImpl struct {
	CommentRepo  port.CommentRepository
	PostRepo     port.PostRepository
	PostTagRepo  port.PostTagRepository
	TagRepo      port.TagRepository
	txManager    port.TxManager
	auditService port.Audit
}

func NewBlogImpl(commentRepo port.CommentRepository, postRepo port.PostRepository, postTagRepo port.PostTagRepository, tagRepo port.TagRepository, txManager port.TxManager, auditService port.Audit) *BlogImpl {
	return &BlogImpl{CommentRepo: commentRepo, PostRepo: postRepo, PostTagRepo: postTagRepo, TagRepo: tagRepo, txManager: txManager, auditService: auditService}
}

func (s *BlogImpl) ArchivePost(ctx context.Context, req port.ArchivePostRequest) (resp port.ArchivePostResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) CreateComment(ctx context.Context, req port.CreateCommentRequest) (resp port.CreateCommentResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) CreatePost(ctx context.Context, req port.CreatePostRequest) (resp port.CreatePostResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) CreateTag(ctx context.Context, req port.CreateTagRequest) (resp port.CreateTagResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) DeleteComment(ctx context.Context, req port.DeleteCommentRequest) (resp port.DeleteCommentResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) DeletePost(ctx context.Context, req port.DeletePostRequest) (resp port.DeletePostResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) DeleteTag(ctx context.Context, req port.DeleteTagRequest) (resp port.DeleteTagResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) GetPost(ctx context.Context, req port.GetPostRequest) (resp port.GetPostResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) ListComments(ctx context.Context, req port.ListCommentsRequest) (resp port.ListCommentsResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) ListMyPosts(ctx context.Context, req port.ListMyPostsRequest) (resp port.ListMyPostsResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) ListPosts(ctx context.Context, req port.ListPostsRequest) (resp port.ListPostsResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) ListTags(ctx context.Context, req port.ListTagsRequest) (resp port.ListTagsResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) PublishPost(ctx context.Context, req port.PublishPostRequest) (resp port.PublishPostResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) SubmitPost(ctx context.Context, req port.SubmitPostRequest) (resp port.SubmitPostResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) UpdateComment(ctx context.Context, req port.UpdateCommentRequest) (resp port.UpdateCommentResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) UpdatePost(ctx context.Context, req port.UpdatePostRequest) (resp port.UpdatePostResponse, err error) {
	return resp, nil
}

func (s *BlogImpl) UpdateTag(ctx context.Context, req port.UpdateTagRequest) (resp port.UpdateTagResponse, err error) {
	return resp, nil
}
