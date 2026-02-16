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
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

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
// PURPOSE: AuthImpl implements port.Auth interface
// PATTERN: Service Layer - orchestrates business logic using repositories
// DEPENDENCIES:
//   - UserRepo: CRUD operations for User entity
//   - publisher: Event publisher (NATS) for domain events
//
// ============================================================================
type AuthImpl struct {
	UserRepo     port.UserRepository
	publisher    port.Publisher
	auditService port.Audit
}

// ============================================================================
// SECTION: Constructor
// PURPOSE: Create new AuthImpl with dependency injection
// USAGE: Called from main.go during application bootstrap
// ============================================================================
func NewAuthImpl(
	userRepo port.UserRepository,
	publisher port.Publisher,
	auditService port.Audit,
) *AuthImpl {
	return &AuthImpl{
		UserRepo:     userRepo,
		publisher:    publisher,
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
// METHOD: GetProfile
// Source: cue/api/auth.cue:101
// INPUT: port.GetProfileRequest
// OUTPUT: port.GetProfileResponse
func (s *AuthImpl) GetProfile(ctx context.Context, req port.GetProfileRequest) (resp port.GetProfileResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Auth"), slog.String("method", "GetProfile"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	if s.UserRepo == nil {
		return resp, errors.New(http.StatusInternalServerError, "Internal Error", "User repository not configured")
	}
	user, err := s.UserRepo.FindByID(ctx, req.UserId)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if user == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "User not found")
	}
	resp.ID = user.ID
	resp.Email = user.Email
	resp.Name = user.Name
	resp.Role = user.Role
	resp.AvatarURL = user.AvatarURL
	resp.CreatedAt = user.CreatedAt

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}

// METHOD: Login
// Source: cue/api/auth.cue:56
// INPUT: port.LoginRequest
// OUTPUT: port.LoginResponse
func (s *AuthImpl) Login(ctx context.Context, req port.LoginRequest) (resp port.LoginResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Auth"), slog.String("method", "Login"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	if s.UserRepo == nil {
		return resp, errors.New(http.StatusInternalServerError, "Internal Error", "User repository not configured")
	}
	user, err := s.UserRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if user == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Invalid credentials")
	}
	valid, err := checkPassword(req.Password, user.PasswordHash)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if !(valid) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Invalid credentials")
	}
	tokens, err := generateTokens(user)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}

	deferredHooks = append(deferredHooks, func(hookCtx context.Context) error {
		if s.publisher != nil {
			_ = s.publisher.PublishUserLoggedIn(hookCtx, domain.UserLoggedIn{UserID: user.ID})
		}
		return nil
	})
	resp.AccessToken = tokens.AccessToken
	resp.RefreshToken = tokens.RefreshToken
	resp.User.ID = user.ID
	resp.User.Email = user.Email
	resp.User.Name = user.Name
	resp.User.Role = user.Role

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}

// METHOD: Register
// Source: cue/api/auth.cue:12
// INPUT: port.RegisterRequest
// OUTPUT: port.RegisterResponse
func (s *AuthImpl) Register(ctx context.Context, req port.RegisterRequest) (resp port.RegisterResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Auth"), slog.String("method", "Register"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	if s.UserRepo == nil {
		return resp, errors.New(http.StatusInternalServerError, "Internal Error", "User repository not configured")
	}
	existing, err := s.UserRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if existing == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "existing not found")
	}
	if !(existing == nil) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Email already registered")
	}
	var newUser domain.User
	newUser.Email = req.Email
	newUser.Name = req.Name
	hash, err := hashPassword(req.Password)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	newUser.PasswordHash = hash
	newUser.Role = "reader"
	newUser.ID = uuid.NewString()
	newUser.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if err = s.UserRepo.Save(ctx, &newUser); err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}

	deferredHooks = append(deferredHooks, func(hookCtx context.Context) error {
		if s.publisher != nil {
			_ = s.publisher.PublishUserRegistered(hookCtx, domain.UserRegistered{UserID: newUser.ID, Email: newUser.Email})
		}
		return nil
	})
	resp.ID = newUser.ID
	resp.Email = newUser.Email
	resp.Name = newUser.Name

	// Execute post-commit hooks
	for _, hook := range deferredHooks {
		if hookErr := hook(ctx); hookErr != nil {
			l.Error("Post-commit hook failed", slog.Any("error", hookErr))
		}
	}

	l.Debug("Exiting method", slog.String("status", "success"))
	return resp, nil
}

// METHOD: UpdateProfile
// Source: cue/api/auth.cue:130
// INPUT: port.UpdateProfileRequest
// OUTPUT: port.UpdateProfileResponse
func (s *AuthImpl) UpdateProfile(ctx context.Context, req port.UpdateProfileRequest) (resp port.UpdateProfileResponse, err error) {
	// Scope tracking to prevent redeclaration errors

	// Deep Logging for AI Traceability & Debugging
	l := logger.From(ctx).With(slog.String("service", "Auth"), slog.String("method", "UpdateProfile"))
	l.Debug("Entering method", slog.Any("req", req))
	// Auto-validation of input DTO
	if err = helpers.Validate(req); err != nil {
		l.Warn("Validation failed", slog.Any("error", err))
		return resp, errors.New(http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	_ = errors.New
	_ = http.StatusOK
	var deferredHooks []func(context.Context) error
	if s.UserRepo == nil {
		return resp, errors.New(http.StatusInternalServerError, "Internal Error", "User repository not configured")
	}
	user, err := s.UserRepo.FindByID(ctx, req.UserId)
	if err != nil {
		return resp, errors.WithIntent(err, ":0 ()")
	}
	if user == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "User not found")
	}
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}
	if err = s.UserRepo.Save(ctx, user); err != nil {
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
