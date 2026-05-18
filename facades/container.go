package facades

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/bwmarrin/snowflake"
	"github.com/casbin/casbin/v3"
	"github.com/go-playground/validator/v10"
	"github.com/go-redsync/redsync/v4"
	"github.com/spf13/cobra"

	"github.com/herhe-com/framework/contracts/ai"
	"github.com/herhe-com/framework/contracts/config"
	"github.com/herhe-com/framework/contracts/database"
	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/contracts/mongodb"
	"github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/contracts/search"
)

// RootPath is the registered application root path type.
type RootPath string

// Services is a type-indexed service registry.
type Services struct {
	mu       sync.RWMutex
	registry map[reflect.Type]any
}

var services = &Services{}

// Container returns the default framework services container.
func Container() *Services {
	return services
}

// SetContainer replaces the default services container.
func SetContainer(container *Services) {
	if container == nil {
		container = &Services{}
	}

	services = container
}

// Register stores a service by its explicit type.
func Register[T any](service T) {
	services.mu.Lock()
	defer services.mu.Unlock()

	services.ensureRegistry()
	services.registry[typeOf[T]()] = service
}

// Get returns a service registered by its explicit type.
func Get[T any]() (T, bool) {
	services.mu.RLock()
	defer services.mu.RUnlock()

	var zero T

	if services.registry == nil {
		return zero, false
	}

	service, ok := services.registry[typeOf[T]()]
	if !ok || isNil(service) {
		return zero, false
	}

	typed, ok := service.(T)
	if !ok {
		return zero, false
	}

	return typed, true
}

// MustGet returns a registered service or panics when it is missing.
func MustGet[T any]() T {
	service, ok := Get[T]()
	if !ok {
		panic(fmt.Sprintf("facades: service %s is not registered", typeOf[T]()))
	}

	return service
}

// Config returns the registered configuration service.
func Config() config.Application {
	return MustGet[config.Application]()
}

// Database returns the registered ORM service.
func Database() database.DB {
	return MustGet[database.DB]()
}

// Redis returns the registered Redis service.
func Redis() database.Redis {
	return MustGet[database.Redis]()
}

// OptionalRedis returns the registered Redis service when available.
func OptionalRedis() (database.Redis, bool) {
	return Get[database.Redis]()
}

// Mongo returns the registered MongoDB service.
func Mongo() mongodb.Mongo {
	return MustGet[mongodb.Mongo]()
}

// Storage returns the registered filesystem service.
func Storage() filesystem.Storage {
	return MustGet[filesystem.Storage]()
}

// Queue returns the registered queue service.
func Queue() queue.Queue {
	return MustGet[queue.Queue]()
}

// Search returns the registered search service.
func Search() search.Search {
	return MustGet[search.Search]()
}

// AI returns the registered AI service.
func AI() ai.AI {
	return MustGet[ai.AI]()
}

// Validator returns the registered validator.
func Validator() *validator.Validate {
	return MustGet[*validator.Validate]()
}

// Console returns the registered console command.
func Console() *cobra.Command {
	return MustGet[*cobra.Command]()
}

// Casbin returns the registered authorization enforcer.
func Casbin() *casbin.Enforcer {
	return MustGet[*casbin.Enforcer]()
}

// Locker returns the registered distributed lock service.
func Locker() *redsync.Redsync {
	return MustGet[*redsync.Redsync]()
}

// Snowflake returns the registered Snowflake node.
func Snowflake() *snowflake.Node {
	return MustGet[*snowflake.Node]()
}

// Root returns the registered application root path.
func Root() string {
	return string(MustGet[RootPath]())
}

func (s *Services) ensureRegistry() {
	if s.registry == nil {
		s.registry = make(map[reflect.Type]any)
	}
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func isNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
