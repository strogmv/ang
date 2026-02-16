package service

import (
	"context"
	"net/http"

	"github.com/strogmv/ang/internal/pkg/errors"
	"github.com/strogmv/ang/internal/pkg/helpers"
	"github.com/strogmv/ang/internal/port"
)

type BlogImpl struct {
	CommentRepo port.CommentRepository
	PostRepo    port.PostRepository
	PostTagRepo port.PostTagRepository
	TagRepo     port.TagRepository
	txManager   port.TxManager
}

func NewBlogImpl(commentRepo port.CommentRepository, postRepo port.PostRepository, postTagRepo port.PostTagRepository, tagRepo port.TagRepository, txManager port.TxManager) *BlogImpl {
	return &BlogImpl{CommentRepo: commentRepo, PostRepo: postRepo, PostTagRepo: postTagRepo, TagRepo: tagRepo, txManager: txManager}
}

func (s *BlogImpl) ArchivePost(ctx context.Context, req port.ArchivePostRequest) (resp port.ArchivePostResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *BlogImpl) CreateComment(ctx context.Context, req port.CreateCommentRequest) (resp port.CreateCommentResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *BlogImpl) CreatePost(ctx context.Context, req port.CreatePostRequest) (resp port.CreatePostResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *BlogImpl) CreateTag(ctx context.Context, req port.CreateTagRequest) (resp port.CreateTagResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *BlogImpl) DeleteComment(ctx context.Context, req port.DeleteCommentRequest) (resp port.DeleteCommentResponse, err error) {
	comment, err := s.CommentRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, err
	}
	if !(comment.AuthorID == req.UserID) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Not authorized")
	}
	if _, err := s.CommentRepo.DeleteByParent(ctx, comment.ID); err != nil {
		return resp, err
	}
	if err := s.CommentRepo.Delete(ctx, comment.ID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Ok, true); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) DeletePost(ctx context.Context, req port.DeletePostRequest) (resp port.DeletePostResponse, err error) {
	post, err := s.PostRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, err
	}
	if _, err := s.PostTagRepo.DeleteByPost(ctx, post.ID); err != nil {
		return resp, err
	}
	if err := s.PostRepo.Delete(ctx, post.ID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Ok, true); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) DeleteTag(ctx context.Context, req port.DeleteTagRequest) (resp port.DeleteTagResponse, err error) {
	tag, err := s.TagRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, err
	}
	if _, err := s.PostTagRepo.DeleteByTag(ctx, tag.ID); err != nil {
		return resp, err
	}
	if err := s.TagRepo.Delete(ctx, tag.ID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Ok, true); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) GetPost(ctx context.Context, req port.GetPostRequest) (resp port.GetPostResponse, err error) {
	post, err := s.PostRepo.FindBySlug(ctx, req.Slug)
	if err != nil {
		return resp, err
	}
	tags, err := s.TagRepo.ListByPost(ctx, post.ID)
	if err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.ID, post.ID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Title, post.Title); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Content, post.Content); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.AuthorID, post.AuthorID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.CreatedAt, post.CreatedAt); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Tags, tags); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) ListComments(ctx context.Context, req port.ListCommentsRequest) (resp port.ListCommentsResponse, err error) {
	comments, err := s.CommentRepo.ListByPost(ctx, req.PostID)
	if err != nil {
		return resp, err
	}
	totalCount, err := s.CommentRepo.CountByPost(ctx, req.PostID)
	if err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Data, comments); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Total, totalCount); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) ListMyPosts(ctx context.Context, req port.ListMyPostsRequest) (resp port.ListMyPostsResponse, err error) {
	if req.Status != "" {
		posts, err := s.PostRepo.ListByAuthorAndStatus(ctx, req.UserID, req.Status)
		if err != nil {
			return resp, err
		}
		if err := helpers.Assign(&resp.Data, posts); err != nil {
			return resp, err
		}
	} else {
		posts, err := s.PostRepo.ListByAuthor(ctx, req.UserID)
		if err != nil {
			return resp, err
		}
		if err := helpers.Assign(&resp.Data, posts); err != nil {
			return resp, err
		}
	}

	return resp, nil
}

func (s *BlogImpl) ListPosts(ctx context.Context, req port.ListPostsRequest) (resp port.ListPostsResponse, err error) {
	if req.Tag != "" {
		posts, err := s.PostRepo.ListPublishedByTag(ctx, "published", req.Tag)
		if err != nil {
			return resp, err
		}
		totalCount, err := s.PostRepo.CountPublishedByTag(ctx, "published", req.Tag)
		if err != nil {
			return resp, err
		}
		if err := helpers.Assign(&resp.Data, posts); err != nil {
			return resp, err
		}
		if err := helpers.Assign(&resp.Total, totalCount); err != nil {
			return resp, err
		}
	} else {
		posts, err := s.PostRepo.ListPublished(ctx, "published")
		if err != nil {
			return resp, err
		}
		totalCount, err := s.PostRepo.CountPublished(ctx, "published")
		if err != nil {
			return resp, err
		}
		if err := helpers.Assign(&resp.Data, posts); err != nil {
			return resp, err
		}
		if err := helpers.Assign(&resp.Total, totalCount); err != nil {
			return resp, err
		}
	}

	return resp, nil
}

func (s *BlogImpl) ListTags(ctx context.Context, req port.ListTagsRequest) (resp port.ListTagsResponse, err error) {
	tags, err := s.TagRepo.ListAll(ctx)
	if err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Data, tags); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) PublishPost(ctx context.Context, req port.PublishPostRequest) (resp port.PublishPostResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *BlogImpl) SubmitPost(ctx context.Context, req port.SubmitPostRequest) (resp port.SubmitPostResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *BlogImpl) UpdateComment(ctx context.Context, req port.UpdateCommentRequest) (resp port.UpdateCommentResponse, err error) {
	comment, err := s.CommentRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, err
	}
	if !(comment.AuthorID == req.UserID) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Not authorized to update this comment")
	}
	if err := helpers.Assign(&comment.Content, req.Content); err != nil {
		return resp, err
	}
	if err := s.CommentRepo.Save(ctx, comment); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Ok, true); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) UpdatePost(ctx context.Context, req port.UpdatePostRequest) (resp port.UpdatePostResponse, err error) {
	post, err := s.PostRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, err
	}
	if req.Title != "" {
		if err := helpers.Assign(&post.Title, req.Title); err != nil {
			return resp, err
		}
	}
	if req.Content != "" {
		if err := helpers.Assign(&post.Content, req.Content); err != nil {
			return resp, err
		}
	}
	if err := s.PostRepo.Save(ctx, post); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Ok, true); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *BlogImpl) UpdateTag(ctx context.Context, req port.UpdateTagRequest) (resp port.UpdateTagResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}
