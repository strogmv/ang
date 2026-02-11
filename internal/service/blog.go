package service

// ============================================================================ 
// SECTION: Imports
// PURPOSE: Import required packages for service implementation
// IMPORTS:
//   - context: Request context propagation
//   - port: Interface definitions (repositories, services)
//   - domain: Domain entities and value objects
//   - errors: Custom error types with HTTP status codes
// ============================================================================ 
import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/strogmv/ang/internal/config"
	"github.com/strogmv/ang/internal/domain"
	"github.com/strogmv/ang/internal/pkg/auth"
	"github.com/strogmv/ang/internal/pkg/errors"
	"github.com/strogmv/ang/internal/pkg/helpers"
	"github.com/strogmv/ang/internal/pkg/logger"
	"github.com/strogmv/ang/internal/pkg/presence"
	"github.com/strogmv/ang/internal/port"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Blank identifiers to suppress unused import warnings
var (
	_ = port.TxManager(nil)
	_ = errors.New
	_ = http.StatusOK
	_ = time.Now
	_ = uuid.UUID{}
	_ domain.User
	_ = helpers.CopyNonEmptyFields
	_ = bcrypt.GenerateFromPassword
	_ = auth.IssueAccessToken
	_ = sort.Strings
	_ = config.Config{}
	_ = json.Marshal
	_ = fmt.Printf
	_ = strings.Split
	_ = presence.Get
)

// ============================================================================ 
// SECTION: Service Struct Definition
// PURPOSE: BlogImpl implements port.Blog interface
// PATTERN: Service Layer - orchestrates business logic using repositories
// DEPENDENCIES:
//   - CommentRepo: CRUD operations for Comment entity
//   - PostRepo: CRUD operations for Post entity
//   - PostTagRepo: CRUD operations for PostTag entity
//   - TagRepo: CRUD operations for Tag entity
//   - txManager: Database transaction manager for atomic operations
// ============================================================================ 
type BlogImpl struct {
	CommentRepo port.CommentRepository
	PostRepo port.PostRepository
	PostTagRepo port.PostTagRepository
	TagRepo port.TagRepository
	txManager port.TxManager
	auditService port.Audit
}

// ============================================================================ 
// SECTION: Constructor
// PURPOSE: Create new BlogImpl with dependency injection
// USAGE: Called from main.go during application bootstrap
// ============================================================================ 
func NewBlogImpl(
	commentRepo port.CommentRepository,
	postRepo port.PostRepository,
	postTagRepo port.PostTagRepository,
	tagRepo port.TagRepository,
	txManager port.TxManager,
	auditService port.Audit,
) *BlogImpl {
	return &BlogImpl{
		CommentRepo: commentRepo,
		PostRepo: postRepo,
		PostTagRepo: postTagRepo,
		TagRepo: tagRepo,
		txManager: txManager,
		auditService: auditService,
	}
}

// ============================================================================ 
// SECTION: Flow Step Templates
// PURPOSE: Generate code for each flow step action defined in CUE
// ============================================================================

// ============================================================================ 
// SECTION: Service Methods
// ============================================================================
// METHOD: ArchivePost
// Source: cue/api/posts.cue:239
// INPUT: port.ArchivePostRequest
// OUTPUT: port.ArchivePostResponse
func (s *BlogImpl) ArchivePost(ctx context.Context, req port.ArchivePostRequest) (resp port.ArchivePostResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "ArchivePost"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	post, err := s.PostRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if post == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Post not found")
	}
	if err = post.TransitionTo("archived"); err != nil {
		return resp, err
	}
	if err = s.PostRepo.Save(ctx, post); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: CreateComment
// Source: cue/api/comments.cue:12
// INPUT: port.CreateCommentRequest
// OUTPUT: port.CreateCommentResponse
func (s *BlogImpl) CreateComment(ctx context.Context, req port.CreateCommentRequest) (resp port.CreateCommentResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "CreateComment"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	post, err := s.PostRepo.FindByID(ctx, req.PostID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if post == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Post not found")
	}
		    var newComment domain.Comment
	newComment.PostID = req.PostID
	newComment.UserID = req.UserId
	newComment.Content = req.Content
	newComment.ID = uuid.NewString()
	newComment.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if err = s.CommentRepo.Save(ctx, &newComment); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.ID = newComment.ID

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: CreatePost
// Source: cue/api/posts.cue:12
// INPUT: port.CreatePostRequest
// OUTPUT: port.CreatePostResponse
func (s *BlogImpl) CreatePost(ctx context.Context, req port.CreatePostRequest) (resp port.CreatePostResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "CreatePost"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	slug, err := slugify(req.Title)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	existing, err := s.PostRepo.FindByID(ctx, slug)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if existing == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "existing not found")
	}
	if existing != nil {
	slug, err = appendRandom(slug)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	} 
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		    var newPost domain.Post
	newPost.Title = req.Title
	newPost.Content = req.Content
	newPost.Slug = slug
	newPost.AuthorID = req.UserId
	newPost.Status = "draft"
	newPost.ID = uuid.NewString()
	newPost.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if err = s.PostRepo.Save(txCtx, &newPost); err != nil {
		return errors.WithIntent(err, ":0 ()")
	}
	for _, tagName := range req.Tags {
	tag, err := s.TagRepo.FindByID(txCtx, tagName)
	if err != nil {
		return errors.WithIntent(err, ":0 ()")
	}
	if tag == nil {
		return errors.New(http.StatusNotFound, "Not Found", "tag not found")
	}
	if tag != nil {
	assoc.PostID = newPost.ID
	assoc.TagID = tag.ID
	if err = s.PostTagRepo.Save(txCtx, assoc); err != nil {
		return errors.WithIntent(err, ":0 ()")
	}
	} 
	}
		return nil
	})
	if err != nil {
		return resp, err
	}
	// Execute hooks deferred during this transaction block
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}
	deferredHooks = nil // Clear after execution
	resp.ID = newPost.ID
	resp.Slug = newPost.Slug

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: CreateTag
// Source: cue/api/tags.cue:34
// INPUT: port.CreateTagRequest
// OUTPUT: port.CreateTagResponse
func (s *BlogImpl) CreateTag(ctx context.Context, req port.CreateTagRequest) (resp port.CreateTagResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "CreateTag"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	slug, err := slugify(req.Name)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	existing, err := s.TagRepo.FindByID(ctx, slug)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if existing == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "existing not found")
	}
	if !(existing == nil) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Tag already exists")
	}
		    var newTag domain.Tag
	newTag.Name = req.Name
	newTag.Slug = slug
	newTag.Description = req.Description
	newTag.ID = uuid.NewString()
	newTag.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if err = s.TagRepo.Save(ctx, &newTag); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.ID = newTag.ID
	resp.Slug = newTag.Slug

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: DeleteComment
// Source: cue/api/comments.cue:98
// INPUT: port.DeleteCommentRequest
// OUTPUT: port.DeleteCommentResponse
func (s *BlogImpl) DeleteComment(ctx context.Context, req port.DeleteCommentRequest) (resp port.DeleteCommentResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "DeleteComment"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	comment, err := s.CommentRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if comment == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Comment not found")
	}
	if !(comment.UserID == req.UserId) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Not authorized")
	}
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
	if err = s.CommentRepo.Delete(txCtx, comment.ID); err != nil {
		return err
	}
	if err = s.CommentRepo.Delete(txCtx, comment.ID); err != nil {
		return err
	}
		return nil
	})
	if err != nil {
		return resp, err
	}
	// Execute hooks deferred during this transaction block
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}
	deferredHooks = nil // Clear after execution
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: DeletePost
// Source: cue/api/posts.cue:260
// INPUT: port.DeletePostRequest
// OUTPUT: port.DeletePostResponse
func (s *BlogImpl) DeletePost(ctx context.Context, req port.DeletePostRequest) (resp port.DeletePostResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "DeletePost"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	post, err := s.PostRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if post == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Post not found")
	}
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
	if err = s.PostTagRepo.Delete(txCtx, post.ID); err != nil {
		return err
	}
	if err = s.PostRepo.Delete(txCtx, post.ID); err != nil {
		return err
	}
		return nil
	})
	if err != nil {
		return resp, err
	}
	// Execute hooks deferred during this transaction block
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}
	deferredHooks = nil // Clear after execution
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: DeleteTag
// Source: cue/api/tags.cue:102
// INPUT: port.DeleteTagRequest
// OUTPUT: port.DeleteTagResponse
func (s *BlogImpl) DeleteTag(ctx context.Context, req port.DeleteTagRequest) (resp port.DeleteTagResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "DeleteTag"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	tag, err := s.TagRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if tag == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Tag not found")
	}
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
	if err = s.PostTagRepo.Delete(txCtx, tag.ID); err != nil {
		return err
	}
	if err = s.TagRepo.Delete(txCtx, tag.ID); err != nil {
		return err
	}
		return nil
	})
	if err != nil {
		return resp, err
	}
	// Execute hooks deferred during this transaction block
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}
	deferredHooks = nil // Clear after execution
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: GetPost
// Source: cue/api/posts.cue:65
// INPUT: port.GetPostRequest
// OUTPUT: port.GetPostResponse
func (s *BlogImpl) GetPost(ctx context.Context, req port.GetPostRequest) (resp port.GetPostResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "GetPost"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	post, err := s.PostRepo.FindByID(ctx, req.Slug)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if post == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Post not found")
	}
	tags, err := s.TagRepo.ListByPost(ctx, post.ID)
	if err != nil {
		return resp, err
	}
	resp.ID = post.ID
	resp.Title = post.Title
	resp.Content = post.Content
	resp.AuthorID = post.AuthorID
	resp.CreatedAt = post.CreatedAt
	resp.Tags = tags

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: ListComments
// Source: cue/api/comments.cue:41
// INPUT: port.ListCommentsRequest
// OUTPUT: port.ListCommentsResponse
func (s *BlogImpl) ListComments(ctx context.Context, req port.ListCommentsRequest) (resp port.ListCommentsResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "ListComments"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	comments, err := s.CommentRepo.ListByPost(ctx, req.PostID)
	if err != nil {
		return resp, err
	}
	totalCount, err := s.CommentRepo.FindByID(ctx, req.PostID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if totalCount == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "totalCount not found")
	}
	resp.Data = comments
	resp.Total = totalCount

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: ListMyPosts
// Source: cue/api/posts.cue:136
// INPUT: port.ListMyPostsRequest
// OUTPUT: port.ListMyPostsResponse
func (s *BlogImpl) ListMyPosts(ctx context.Context, req port.ListMyPostsRequest) (resp port.ListMyPostsResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "ListMyPosts"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	if req.Status != "" {
	posts, err := s.PostRepo.ListByAuthorAndStatus(ctx, req.UserId)
	if err != nil {
		return resp, err
	}
	}  else {
	posts, err := s.PostRepo.ListByAuthor(ctx, req.UserId)
	if err != nil {
		return resp, err
	}
	} 
	resp.Data = posts

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: ListPosts
// Source: cue/api/posts.cue:100
// INPUT: port.ListPostsRequest
// OUTPUT: port.ListPostsResponse
func (s *BlogImpl) ListPosts(ctx context.Context, req port.ListPostsRequest) (resp port.ListPostsResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "ListPosts"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	if req.Tag != "" {
	posts, err := s.PostRepo.ListPublishedByTag(ctx, req.Tag)
	if err != nil {
		return resp, err
	}
	totalCount, err := s.PostRepo.FindByID(ctx, req.Tag)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if totalCount == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "totalCount not found")
	}
	}  else {
	posts, err := s.PostRepo.ListPublished(ctx)
	if err != nil {
		return resp, err
	}
	totalCount, err := s.PostRepo.FindByID(ctx, <no value>)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if totalCount == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "totalCount not found")
	}
	} 
	resp.Data = posts
	resp.Total = totalCount

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: ListTags
// Source: cue/api/tags.cue:12
// INPUT: port.ListTagsRequest
// OUTPUT: port.ListTagsResponse
func (s *BlogImpl) ListTags(ctx context.Context, req port.ListTagsRequest) (resp port.ListTagsResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "ListTags"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	tags, err := s.TagRepo.ListAll(ctx)
	if err != nil {
		return resp, err
	}
	resp.Data = tags

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: PublishPost
// Source: cue/api/posts.cue:218
// INPUT: port.PublishPostRequest
// OUTPUT: port.PublishPostResponse
func (s *BlogImpl) PublishPost(ctx context.Context, req port.PublishPostRequest) (resp port.PublishPostResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "PublishPost"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	post, err := s.PostRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if post == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Post not found")
	}
	if err = post.TransitionTo("published"); err != nil {
		return resp, err
	}
	if err = s.PostRepo.Save(ctx, post); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: SubmitPost
// Source: cue/api/posts.cue:197
// INPUT: port.SubmitPostRequest
// OUTPUT: port.SubmitPostResponse
func (s *BlogImpl) SubmitPost(ctx context.Context, req port.SubmitPostRequest) (resp port.SubmitPostResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "SubmitPost"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	post, err := s.PostRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if post == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Post not found")
	}
	if err = post.TransitionTo("pending"); err != nil {
		return resp, err
	}
	if err = s.PostRepo.Save(ctx, post); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: UpdateComment
// Source: cue/api/comments.cue:71
// INPUT: port.UpdateCommentRequest
// OUTPUT: port.UpdateCommentResponse
func (s *BlogImpl) UpdateComment(ctx context.Context, req port.UpdateCommentRequest) (resp port.UpdateCommentResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "UpdateComment"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	comment, err := s.CommentRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if comment == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Comment not found")
	}
	if !(comment.UserID == req.UserId) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Not authorized to update this comment")
	}
	comment.Content = req.Content
	if err = s.CommentRepo.Save(ctx, comment); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: UpdatePost
// Source: cue/api/posts.cue:167
// INPUT: port.UpdatePostRequest
// OUTPUT: port.UpdatePostResponse
func (s *BlogImpl) UpdatePost(ctx context.Context, req port.UpdatePostRequest) (resp port.UpdatePostResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "UpdatePost"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	post, err := s.PostRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if post == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Post not found")
	}
	if req.Title != "" {
	post.Title = req.Title
	} 
	if req.Content != "" {
	post.Content = req.Content
	} 
	if err = s.PostRepo.Save(ctx, post); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
// METHOD: UpdateTag
// Source: cue/api/tags.cue:70
// INPUT: port.UpdateTagRequest
// OUTPUT: port.UpdateTagResponse
func (s *BlogImpl) UpdateTag(ctx context.Context, req port.UpdateTagRequest) (resp port.UpdateTagResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Blog"), slog.String("method", "UpdateTag"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	tag, err := s.TagRepo.FindByID(ctx, req.ID)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if tag == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Tag not found")
	}
	if req.Name != "" {
	tag.Name = req.Name
	slug, err := slugify(req.Name)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	tag.Slug = slug
	} 
	if req.Description != "" {
	tag.Description = req.Description
	} 
	if err = s.TagRepo.Save(ctx, tag); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	resp.Ok = true

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}
