package config

import (
	"path/filepath"
	"sort"
	"strings"
)

// LoadFiles loads base config files, current-env files, then environment variables.
func LoadFiles(store *Store, configPath string, env string, prefixes ...string) error {
	if store == nil {
		return nil
	}
	if err := loadConfigFiles(store, configPath, env, false); err != nil {
		return err
	}
	if err := loadConfigFiles(store, configPath, env, true); err != nil {
		return err
	}
	sources := make([]Source, 0, len(prefixes))
	for _, prefix := range prefixes {
		sources = append(sources, Env(prefix))
	}
	if err := store.Load(sources...); err != nil {
		return err
	}
	return store.Reload()
}

func loadConfigFiles(store *Store, configPath string, env string, envOnly bool) error {
	files, err := filepath.Glob(filepath.Join(configPath, "*.toml"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	envSuffix := "." + env + ".toml"
	for _, file := range files {
		isCurrentEnv := strings.HasSuffix(file, envSuffix)
		isEnvFile := configEnvName(file) != ""
		if envOnly {
			if !isCurrentEnv {
				continue
			}
		} else if isEnvFile {
			continue
		}
		if err := store.Load(FileDomain(file, configDomain(file))); err != nil {
			return err
		}
	}
	return nil
}

func configDomain(file string) string {
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if index := strings.Index(name, "."); index >= 0 {
		name = name[:index]
	}
	return name
}

func configEnvName(file string) string {
	nameWithExt := filepath.Base(file)
	ext := filepath.Ext(nameWithExt)
	if ext != ".toml" {
		return ""
	}
	name := strings.TrimSuffix(nameWithExt, ext)
	index := strings.LastIndex(name, ".")
	if index < 0 {
		return ""
	}
	return name[index+1:]
}
