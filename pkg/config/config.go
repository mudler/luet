// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package config

import (
	"errors"
	"fmt"
	"os/user"
	"runtime"
	"time"

	v "github.com/spf13/viper"
)

var LuetCfg = NewLuetConfig(v.GetViper())

type LuetLoggingConfig struct {
	Path       string `mapstructure:"path"`
	JsonFormat bool   `mapstructure:"json_format"`
	Level      string `mapstructure:"level"`
}

type LuetGeneralConfig struct {
	SameOwner       bool `mapstructure:"same_owner"`
	Concurrency     int  `mapstructure:"concurrency"`
	Debug           bool `mapstructure:"debug"`
	ShowBuildOutput bool `mapstructure:"show_build_output"`
	SpinnerMs       int  `mapstructure:"spinner_ms"`
	SpinnerCharset  int  `mapstructure:"spinner_charset"`
	FatalWarns      bool `mapstructure:"fatal_warnings"`
}

type LuetSystemConfig struct {
	DatabaseEngine string `yaml:"database_engine" mapstructure:"database_engine"`
	DatabasePath   string `yaml:"database_path" mapstructure:"database_path"`
	Rootfs         string `yaml:"rootfs" mapstructure:"rootfs"`
	PkgsCachePath  string `yaml:"pkgs_cache_path" mapstructure:"pkgs_cache_path"`
}

type LuetRepository struct {
	Name           string            `json:"name" yaml:"name" mapstructure:"name"`
	Description    string            `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description"`
	Urls           []string          `json:"urls" yaml:"urls" mapstructure:"urls"`
	Type           string            `json:"type" yaml:"type" mapstructure:"type"`
	Mode           string            `json:"mode,omitempty" yaml:"mode,omitempty" mapstructure:"mode,omitempty"`
	Priority       int               `json:"priority,omitempty" yaml:"priority,omitempty" mapstructure:"priority"`
	Enable         bool              `json:"enable" yaml:"enable" mapstructure:"enable"`
	Cached         bool              `json:"cached,omitempty" yaml:"cached,omitempty" mapstructure:"cached,omitempty"`
	Authentication map[string]string `json:"auth,omitempty" yaml:"auth,omitempty" mapstructure:"auth,omitempty"`
	TreePath       string            `json:"tree_path,omitempty" yaml:"tree_path,omitempty" mapstructure:"tree_path"`

	// Serialized options not used in repository configuration

	// Incremented value that identify revision of the repository in a user-friendly way.
	Revision int `json:"revision,omitempty" yaml:"-,omitempty" mapstructure:"-,omitempty"`
	// Epoch time in seconds
	LastUpdate string `json:"last_update,omitempty" yaml:"-,omitempty" mapstructure:"-,omitempty"`
}

func NewLuetRepository(name, t, descr string, urls []string, priority int, enable, cached bool) *LuetRepository {
	return &LuetRepository{
		Name:        name,
		Description: descr,
		Urls:        urls,
		Type:        t,
		// Used in cached repositories
		Mode:           "",
		Priority:       priority,
		Enable:         enable,
		Cached:         cached,
		Authentication: make(map[string]string, 0),
		TreePath:       "",
	}
}

func NewEmptyLuetRepository() *LuetRepository {
	return &LuetRepository{
		Name:           "",
		Description:    "",
		Urls:           []string{},
		Type:           "",
		Priority:       9999,
		TreePath:       "",
		Enable:         false,
		Cached:         false,
		Authentication: make(map[string]string, 0),
	}
}

func (r *LuetRepository) String() string {
	return fmt.Sprintf("[%s] prio: %d, type: %s, enable: %t, cached: %t",
		r.Name, r.Priority, r.Type, r.Enable, r.Cached)
}

type LuetConfig struct {
	Viper *v.Viper

	Logging LuetLoggingConfig `mapstructure:"logging"`
	General LuetGeneralConfig `mapstructure:"general"`
	System  LuetSystemConfig  `mapstructure:"system"`

	RepositoriesConfDir []string         `mapstructure:"repos_confdir"`
	CacheRepositories   []LuetRepository `mapstructure:"repetitors"`
	SystemRepositories  []LuetRepository `mapstructure:"repositories"`
}

func NewLuetConfig(viper *v.Viper) *LuetConfig {
	if viper == nil {
		viper = v.New()
	}

	GenDefault(viper)
	return &LuetConfig{Viper: viper}
}

func GenDefault(viper *v.Viper) {
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.path", "")
	viper.SetDefault("logging.json_format", false)

	viper.SetDefault("general.concurrency", runtime.NumCPU())
	viper.SetDefault("general.debug", false)
	viper.SetDefault("general.show_build_output", false)
	viper.SetDefault("general.spinner_ms", 100)
	viper.SetDefault("general.spinner_charset", 22)
	viper.SetDefault("general.fatal_warnings", false)

	u, _ := user.Current()
	if u.Uid == "0" {
		viper.SetDefault("general.same_owner", true)
	} else {
		viper.SetDefault("general.same_owner", false)
	}

	viper.SetDefault("system.database_engine", "boltdb")
	viper.SetDefault("system.database_path", "/var/cache/luet")
	viper.SetDefault("system.rootfs", "/")
	viper.SetDefault("system.pkgs_cache_path", "packages")

	viper.SetDefault("repos_confdir", []string{"/etc/luet/repos.conf.d"})
	viper.SetDefault("cache_repositories", []string{})
	viper.SetDefault("system_repositories", []string{})
}

func (c *LuetConfig) AddSystemRepository(r LuetRepository) {
	c.SystemRepositories = append(c.SystemRepositories, r)
}

func (c *LuetConfig) GetLogging() *LuetLoggingConfig {
	return &c.Logging
}

func (c *LuetConfig) GetGeneral() *LuetGeneralConfig {
	return &c.General
}

func (c *LuetConfig) GetSystem() *LuetSystemConfig {
	return &c.System
}

func (c *LuetConfig) GetSystemRepository(name string) (*LuetRepository, error) {
	var ans *LuetRepository = nil

	for idx, repo := range c.SystemRepositories {
		if repo.Name == name {
			ans = &c.SystemRepositories[idx]
			break
		}
	}
	if ans == nil {
		return nil, errors.New("Repository " + name + " not found")
	}

	return ans, nil
}

func (c *LuetGeneralConfig) String() string {
	ans := fmt.Sprintf(`
general:
  concurrency: %d
  same_owner: %t
  debug: %t
  fatal_warnings: %t
  show_build_output: %t
  spinner_ms: %d
  spinner_charset: %d`, c.Concurrency, c.SameOwner, c.Debug,
		c.FatalWarns, c.ShowBuildOutput,
		c.SpinnerMs, c.SpinnerCharset)

	return ans
}

func (c *LuetGeneralConfig) GetSpinnerMs() time.Duration {
	duration, err := time.ParseDuration(fmt.Sprintf("%dms", c.SpinnerMs))
	if err != nil {
		return 100 * time.Millisecond
	}
	return duration
}

func (c *LuetLoggingConfig) String() string {
	ans := fmt.Sprintf(`
logging:
  path: %s
  json_format: %t
  level: %s`, c.Path, c.JsonFormat, c.Level)

	return ans
}

func (c *LuetSystemConfig) String() string {
	ans := fmt.Sprintf(`
system:
  database_engine: %s
  database_path: %s
  pkgs_cache_path: %s
  rootfs: %s`,
		c.DatabaseEngine, c.DatabasePath, c.PkgsCachePath, c.Rootfs)

	return ans
}
