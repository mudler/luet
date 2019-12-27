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
	"fmt"
	"runtime"
	"time"

	v "github.com/spf13/viper"
)

var LuetCfg = NewLuetConfig(v.GetViper())

type LuetLoggingConfig struct {
	Path  string `mapstructure:"path"`
	Level string `mapstructure:"level"`
}

type LuetGeneralConfig struct {
	Concurrency     int  `mapstructure:"concurrency"`
	Debug           bool `mapstructure:"debug"`
	ShowBuildOutput bool `mapstructure:"show_build_output"`
	SpinnerMs       int  `mapstructure:"spinner_ms"`
	SpinnerCharset  int  `mapstructure:"spinner_charset"`
}

type LuetRepository struct {
	Name           string            `yaml:"name" mapstructure:"name"`
	Description    string            `yaml:"description,omitempty" mapstructure:"description"`
	Urls           []string          `yaml:"urls" mapstructure:"urls"`
	Type           string            `yaml:"type" mapstructure:"type"`
	Mode           string            `yaml:"mode,omitempty" mapstructure:"mode"`
	Priority       int               `yaml:"priority,omitempty" mapstructure:"priority"`
	Authentication map[string]string `yaml:"auth,omitempty" mapstructure:"auth"`
}

type LuetConfig struct {
	Viper *v.Viper

	Logging LuetLoggingConfig `mapstructure:"logging"`
	General LuetGeneralConfig `mapstructure:"general"`

	RepositoriesConfDir []string         `mapstructure:"repos_confdir"`
	CacheRepositories   []LuetRepository `mapstructure:"cache_repositories"`
	SystemRepositories  []LuetRepository `mapstructure:"system_repositories"`
}

func (r *LuetRepository) String() string {
	return fmt.Sprintf("[%s] prio: %d, type: %s", r.Name, r.Priority, r.Type)
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

	viper.SetDefault("general.concurrency", runtime.NumCPU())
	viper.SetDefault("general.debug", false)
	viper.SetDefault("general.show_build_output", false)
	viper.SetDefault("general.spinner_ms", 100)
	viper.SetDefault("general.spinner_charset", 22)

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

func (c *LuetGeneralConfig) String() string {
	ans := fmt.Sprintf(`
general:
  concurrency: %d
  debug: %t
  show_build_output: %t
  spinner_ms: %d
  spinner_charset: %d
`, c.Concurrency, c.Debug, c.ShowBuildOutput,
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
  level: %s
`, c.Path, c.Level)

	return ans
}
