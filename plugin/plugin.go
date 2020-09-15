package plugin

import (
	"github.com/hashicorp/go-multierror"
	"github.com/khorevaa/r2gitsync/cmd/flags"
	"github.com/khorevaa/r2gitsync/context"
	"github.com/khorevaa/r2gitsync/plugin/subscription"
	. "github.com/khorevaa/r2gitsync/plugin/types"
	"github.com/pkg/errors"
)

// Plugin is the interface for plugins to micro. It differs from go-micro in that it's for
// the micro API, Web, Sidecar, CLI. It's a method of building middleware for the HTTP side.
type Symbol interface {

	// Global Flags
	Flags() []flags.Flag
	// Sub-modules
	Modules() []string

	// Name of the plugin
	String() string
	Desc() string
	Version() string
	ShortVersion() string
	Name() string
	Init() Plugin
}

type Plugin interface {
	Subscribe(ctx context.Context) Subscriber
}

// Manager is the plugin manager which stores plugins and allows them to be retrieved.
// This is used by all the components of micro.
type Manager interface {
	Plugins(module string) []Symbol
	EnabledPlugins(module string) []Symbol
	Register(plugin Symbol) error
	IsRegistered(plugin Symbol, module string) bool
	RegisterFlags(command string, cmd command, ctx context.Context)
	Enable(name string)
	Disable(name string)
}

type RegisteredPluginList map[string]*RegisteredPlugin

func (pl RegisteredPluginList) Items() (arr []*RegisteredPlugin) {

	arr = make([]*RegisteredPlugin, len(pl))

	for _, registeredPlugin := range pl {
		arr = append(arr, registeredPlugin)
	}
	return
}

func (pl *RegisteredPluginList) Add(rp RegisteredPlugin) {

	if pl.Find(rp.ID) != nil {
		return
	}

	pl.items = append(pl.items, &rp)
}

func (pl RegisteredPluginList) Find(id string) *RegisteredPlugin {

	for _, item := range pl.items {

		if item.ID == id {
			return item
		}

	}
	return nil
}

type PluginsMetadata struct {
	ID           string
	Name         string
	Version      string
	ShortVersion string
	Desc         string
	Modules      []string
	Flags        []flags.Flag
}

type RegisteredPlugin struct {
	PluginsMetadata
	Enable bool
}

// Plugins lists the global plugins
func Plugins() (pl map[string][]string) {
	return defaultManager.Plugins()
}

func GetPluginInfo(name, module string) (Symbol, bool) {

	return defaultManager.Plugin(name, module)

}

// Register registers a global plugins
func Register(names ...Symbol) error {

	mErr := &multierror.Error{}

	for _, name := range names {
		err := defaultManager.Register(name)

		mErr = multierror.Append(mErr, errors.Wrapf(err, "plugin <%s>", name))
	}

	return mErr.ErrorOrNil()
}

// Enable a global plugins
func Enable(names ...string) {

	for _, name := range names {
		defaultManager.Enable(name)
	}
}

// Disable a global plugins
func Disable(names ...string) {

	for _, name := range names {
		defaultManager.Disable(name)
	}
}

func RegistryFlags(modName string, cmd command, ctx context.Context) {

	defaultManager.RegisterFlags(modName, cmd, ctx)

}

func SubscribeManager(modName string) *subscription.SubscribeManager {
	return defaultManager.SubscribeManager(modName)
}

// NewManager creates a new plugin manager
func NewManager() Manager {
	return newManager()
}

func Subscription(handlers ...interface{}) Subscriber {

	return subscriber{
		handlers: handlers,
	}
}

func Subscribe(module string, ctx context.Context) error {

	return defaultManager.Subscribe(module, ctx)
}

func LoadPlugins(dir string) error {

	loader := NewPluginsLoader(dir)

	err := loader.LoadPlugins(false)

	if err != nil {
		return err
	}

	return Register(loader.Plugins()...)

}
