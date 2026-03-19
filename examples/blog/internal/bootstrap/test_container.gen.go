package bootstrap

import (
	"github.com/example/blog/internal/adapter/mock"
	"github.com/example/blog/internal/config"
	"github.com/example/blog/internal/port"
	"github.com/example/blog/internal/service"
)

type TestOption func(*TestContainer)

type TestContainer struct {
	Config                *config.Config
	Effects               *EffectRegistry
	CommentRepository     *mock.MockCommentRepository
	commentRepositoryImpl port.CommentRepository
	PostRepository        *mock.MockPostRepository
	postRepositoryImpl    port.PostRepository
	PostTagRepository     *mock.MockPostTagRepository
	postTagRepositoryImpl port.PostTagRepository
	TagRepository         *mock.MockTagRepository
	tagRepositoryImpl     port.TagRepository
	UserRepository        *mock.MockUserRepository
	userRepositoryImpl    port.UserRepository
	TxManager             *mock.MockTxManager
	txManagerImpl         port.TxManager
	Publisher             *mock.MockPublisher
	publisherImpl         port.Publisher
	SvcAuth               port.Auth
	svcAuthOverride       port.Auth
	SvcBlog               port.Blog
	svcBlogOverride       port.Blog
}

// NewTestContainer creates a mock-first bootstrap container for unit tests.
func NewTestContainer(opts ...TestOption) *TestContainer {
	c := &TestContainer{Config: &config.Config{}}
	c.CommentRepository = mock.NewCommentRepository()
	c.commentRepositoryImpl = c.CommentRepository
	c.PostRepository = mock.NewPostRepository()
	c.postRepositoryImpl = c.PostRepository
	c.PostTagRepository = mock.NewPostTagRepository()
	c.postTagRepositoryImpl = c.PostTagRepository
	c.TagRepository = mock.NewTagRepository()
	c.tagRepositoryImpl = c.TagRepository
	c.UserRepository = mock.NewUserRepository()
	c.userRepositoryImpl = c.UserRepository
	c.TxManager = mock.NewTxManager()
	c.txManagerImpl = c.TxManager
	c.Publisher = mock.NewPublisher()
	c.publisherImpl = c.Publisher
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	c.Effects = NewTestEffectRegistry(c.publisherImpl, nil, nil)
	if c.svcAuthOverride != nil {
		c.SvcAuth = c.svcAuthOverride
	} else {
		c.SvcAuth = service.NewAuthImpl(
			c.userRepositoryImpl,
			c.Effects.Publisher,
		)
	}
	if c.svcBlogOverride != nil {
		c.SvcBlog = c.svcBlogOverride
	} else {
		c.SvcBlog = service.NewBlogImpl(
			c.commentRepositoryImpl,
			c.postRepositoryImpl,
			c.postTagRepositoryImpl,
			c.tagRepositoryImpl,
			c.txManagerImpl,
			c.Effects.Publisher,
		)
	}
	return c
}

// NewTestContainerWith applies partial overrides on top of NewTestContainer.
func NewTestContainerWith(opts ...TestOption) *TestContainer {
	return NewTestContainer(opts...)
}

func WithConfig(cfg *config.Config) TestOption {
	return func(c *TestContainer) {
		if cfg != nil {
			c.Config = cfg
		}
	}
}

func WithCommentRepository(v port.CommentRepository) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.commentRepositoryImpl = v
		}
	}
}

func WithPostRepository(v port.PostRepository) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.postRepositoryImpl = v
		}
	}
}

func WithPostTagRepository(v port.PostTagRepository) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.postTagRepositoryImpl = v
		}
	}
}

func WithTagRepository(v port.TagRepository) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.tagRepositoryImpl = v
		}
	}
}

func WithUserRepository(v port.UserRepository) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.userRepositoryImpl = v
		}
	}
}

func WithTxManager(v port.TxManager) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.txManagerImpl = v
		}
	}
}

func WithPublisher(v port.Publisher) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.publisherImpl = v
			if c.Effects != nil {
				c.Effects.Publisher = v
			}
		}
	}
}

func WithAuthService(v port.Auth) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.svcAuthOverride = v
			c.SvcAuth = v
		}
	}
}

func WithBlogService(v port.Blog) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.svcBlogOverride = v
			c.SvcBlog = v
		}
	}
}
