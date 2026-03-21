package bootstrap

import (
	"github.com/example/minimal/internal/adapter/mock"
	"github.com/example/minimal/internal/config"
	"github.com/example/minimal/internal/port"
	"github.com/example/minimal/internal/service"
)

type TestOption func(*TestContainer)

type TestContainer struct {
	Config                     *config.Config
	Effects                    *EffectRegistry
	UserRepository             *mock.MockUserRepository
	userRepositoryImpl         port.UserRepository
	NotificationDispatcher     *mock.MockNotificationDispatcher
	notificationDispatcherImpl port.NotificationDispatcher
	SvcNotifications           port.Notifications
	svcNotificationsOverride   port.Notifications
	SvcUser                    port.User
	svcUserOverride            port.User
}

// NewTestContainer creates a mock-first bootstrap container for unit tests.
func NewTestContainer(opts ...TestOption) *TestContainer {
	c := &TestContainer{Config: &config.Config{}}
	c.UserRepository = mock.NewUserRepository()
	c.userRepositoryImpl = c.UserRepository
	c.NotificationDispatcher = mock.NewNotificationDispatcher()
	c.notificationDispatcherImpl = c.NotificationDispatcher
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	c.Effects = NewTestEffectRegistry(nil, nil, nil)
	if c.svcNotificationsOverride != nil {
		c.SvcNotifications = c.svcNotificationsOverride
	} else {
		c.SvcNotifications = service.NewNotificationsImpl(
			c.notificationDispatcherImpl,
		)
	}
	if c.svcUserOverride != nil {
		c.SvcUser = c.svcUserOverride
	} else {
		c.SvcUser = service.NewUserImpl(
			c.userRepositoryImpl,
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

func WithUserRepository(v port.UserRepository) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.userRepositoryImpl = v
		}
	}
}

func WithNotificationDispatcher(v port.NotificationDispatcher) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.notificationDispatcherImpl = v
		}
	}
}

func WithNotificationsService(v port.Notifications) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.svcNotificationsOverride = v
			c.SvcNotifications = v
		}
	}
}

func WithUserService(v port.User) TestOption {
	return func(c *TestContainer) {
		if v != nil {
			c.svcUserOverride = v
			c.SvcUser = v
		}
	}
}
