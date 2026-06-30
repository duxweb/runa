package runtime

import (
	"fmt"

	runaprovider "github.com/duxweb/runa/provider"
)

// Module is a business module entry.
type Module = runaprovider.Module

// ModuleBase provides no-op lifecycle methods for modules.
type ModuleBase = runaprovider.ModuleBase

// ModuleDepends lets a module declare hard dependencies by module name.
type ModuleDepends = runaprovider.ModuleDepends

// ModuleInfo stores module registration status.
type ModuleInfo struct {
	Name    string
	Depends []string
	Status  string
}

func moduleDepends(module Module) []string {
	if depends, ok := module.(ModuleDepends); ok {
		return append([]string(nil), depends.Depends()...)
	}
	return nil
}

func sortModules(modules []Module) ([]Module, error) {
	seen := make(map[string]Module, len(modules))
	for _, module := range modules {
		if module == nil {
			continue
		}
		name := module.Name()
		if name == "" {
			return nil, fmt.Errorf("module name is required")
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("module %s already registered", name)
		}
		seen[name] = module
	}
	sorted := make([]Module, 0, len(seen))
	visiting := make(map[string]bool, len(seen))
	visited := make(map[string]bool, len(seen))
	var visit func(Module) error
	visit = func(module Module) error {
		name := module.Name()
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("module %s dependency cycle", name)
		}
		visiting[name] = true
		for _, depName := range moduleDepends(module) {
			dep, ok := seen[depName]
			if !ok {
				return fmt.Errorf("module %s depends on missing module %s", name, depName)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		sorted = append(sorted, module)
		return nil
	}
	for _, module := range modules {
		if module == nil {
			continue
		}
		if err := visit(module); err != nil {
			return nil, err
		}
	}
	return sorted, nil
}

func moduleInfos(modules []Module, status string) []ModuleInfo {
	items := make([]ModuleInfo, 0, len(modules))
	for _, module := range modules {
		if module == nil {
			continue
		}
		items = append(items, ModuleInfo{
			Name:    module.Name(),
			Depends: moduleDepends(module),
			Status:  status,
		})
	}
	return items
}
