package regirsty

import (
	"context"
	"fmt"
	"sync"
)

var (
	pluginManager = &RegisterPlugin{
		plugins: make(map[string]Registry),
	}
)

// register plugin manger
type RegisterPlugin struct {
	plugins map[string]Registry
	lock    sync.Mutex
}

func (p *RegisterPlugin) registerPlugin(plugins Registry) (err error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	_, ok := p.plugins[plugins.Name()]

	if ok {
		return fmt.Errorf("register plugin already exist")
	}

	p.plugins[plugins.Name()] = plugins
	return
}

func (p *RegisterPlugin) initRegistry(ctx context.Context, name string, opts ...Option) (register Registry, err error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	register, ok := p.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %s not exist", name)
	}
	register.Init(ctx, opts...)
	return
}

func RegisteryPlugin(register Registry) (err error) {
	return pluginManager.registerPlugin(register)
}

func InitRegistery(ctx context.Context, name string, opts ...Option) (register Registry, err error) {
	return pluginManager.initRegistry(ctx, name, opts...)
}
