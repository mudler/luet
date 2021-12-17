// Copyright © 2019-2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package types

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mudler/luet/pkg/api/core/config"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	pkg "github.com/mudler/luet/pkg/package"
	solver "github.com/mudler/luet/pkg/solver"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var AvailableResolvers = strings.Join([]string{solver.QLearningResolverType}, " ")

type LuetLoggingConfig struct {
	// Path of the logfile
	Path string `yaml:"path" mapstructure:"path"`
	// Enable/Disable logging to file
	EnableLogFile bool `yaml:"enable_logfile" mapstructure:"enable_logfile"`
	// Enable JSON format logging in file
	JsonFormat bool `yaml:"json_format" mapstructure:"json_format"`

	// Log level
	Level string `yaml:"level" mapstructure:"level"`

	// Enable emoji
	EnableEmoji bool `yaml:"enable_emoji" mapstructure:"enable_emoji"`
	// Enable/Disable color in logging
	Color bool `yaml:"color" mapstructure:"color"`

	// NoSpinner disable spinner
	NoSpinner bool `yaml:"no_spinner" mapstructure:"no_spinner"`
}

type LuetGeneralConfig struct {
	SameOwner       bool `yaml:"same_owner,omitempty" mapstructure:"same_owner"`
	Concurrency     int  `yaml:"concurrency,omitempty" mapstructure:"concurrency"`
	Debug           bool `yaml:"debug,omitempty" mapstructure:"debug"`
	ShowBuildOutput bool `yaml:"show_build_output,omitempty" mapstructure:"show_build_output"`
	FatalWarns      bool `yaml:"fatal_warnings,omitempty" mapstructure:"fatal_warnings"`
	HTTPTimeout     int  `yaml:"http_timeout,omitempty" mapstructure:"http_timeout"`
	Quiet           bool `yaml:"quiet" mapstructure:"quiet"`
}

type LuetSolverOptions struct {
	solver.Options `yaml:"options,omitempty"`
	Type           string            `yaml:"type,omitempty" mapstructure:"type"`
	LearnRate      float32           `yaml:"rate,omitempty" mapstructure:"rate"`
	Discount       float32           `yaml:"discount,omitempty" mapstructure:"discount"`
	MaxAttempts    int               `yaml:"max_attempts,omitempty" mapstructure:"max_attempts"`
	Implementation solver.SolverType `yaml:"implementation,omitempty" mapstructure:"implementation"`
}

func (opts LuetSolverOptions) ResolverIsSet() bool {
	switch opts.Type {
	case solver.QLearningResolverType:
		return true
	default:
		return false
	}
}

func (opts LuetSolverOptions) Resolver() solver.PackageResolver {
	switch opts.Type {
	case solver.QLearningResolverType:
		if opts.LearnRate != 0.0 {
			return solver.NewQLearningResolver(opts.LearnRate, opts.Discount, opts.MaxAttempts, 999999)

		}
		return solver.SimpleQLearningSolver()
	}

	return &solver.Explainer{}
}

func (opts *LuetSolverOptions) CompactString() string {
	return fmt.Sprintf("type: %s rate: %f, discount: %f, attempts: %d, initialobserved: %d",
		opts.Type, opts.LearnRate, opts.Discount, opts.MaxAttempts, 999999)
}

type LuetSystemConfig struct {
	DatabaseEngine string `yaml:"database_engine" mapstructure:"database_engine"`
	DatabasePath   string `yaml:"database_path" mapstructure:"database_path"`
	Rootfs         string `yaml:"rootfs" mapstructure:"rootfs"`
	PkgsCachePath  string `yaml:"pkgs_cache_path" mapstructure:"pkgs_cache_path"`
	TmpDirBase     string `yaml:"tmpdir_base" mapstructure:"tmpdir_base"`
}

// Init reads the config and replace user-defined paths with
// absolute paths where necessary, and construct the paths for the cache
// and database on the real system
func (c *LuetConfig) Init() error {
	if err := c.System.init(); err != nil {
		return err
	}

	if err := c.loadConfigProtect(); err != nil {
		return err
	}

	// Load repositories
	if err := c.loadRepositories(); err != nil {
		return err
	}

	return nil
}

func (s *LuetSystemConfig) init() error {
	if err := s.setRootfs(); err != nil {
		return err
	}

	if err := s.setDBPath(); err != nil {
		return err
	}

	s.setCachePath()

	return nil
}

func (s *LuetSystemConfig) setRootfs() error {
	p, err := fileHelper.Rel2Abs(s.Rootfs)
	if err != nil {
		return err
	}

	s.Rootfs = p
	return nil
}

func (sc LuetSystemConfig) GetRepoDatabaseDirPath(name string) string {
	dbpath := filepath.Join(sc.DatabasePath, "repos/"+name)
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		panic(err)
	}
	return dbpath
}

func (sc *LuetSystemConfig) setDBPath() error {
	dbpath := filepath.Join(sc.Rootfs,
		sc.DatabasePath)
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		return err
	}
	sc.DatabasePath = dbpath
	return nil
}

func (sc *LuetSystemConfig) setCachePath() {
	var cachepath string
	if sc.PkgsCachePath != "" {
		if !filepath.IsAbs(cachepath) {
			cachepath = filepath.Join(sc.DatabasePath, sc.PkgsCachePath)
			os.MkdirAll(cachepath, os.ModePerm)
		} else {
			cachepath = sc.PkgsCachePath
		}
	} else {
		// Create dynamic cache for test suites
		cachepath, _ = ioutil.TempDir(os.TempDir(), "cachepkgs")
	}

	sc.PkgsCachePath = cachepath // Be consistent with the path we set
}

type FinalizerEnv struct {
	Key   string `json:"key" yaml:"key" mapstructure:"key"`
	Value string `json:"value" yaml:"value" mapstructure:"value"`
}

type Finalizers []FinalizerEnv

func (f Finalizers) Slice() (sl []string) {
	for _, kv := range f {
		sl = append(sl, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
	}
	return
}

type LuetConfig struct {
	Logging LuetLoggingConfig `yaml:"logging,omitempty" mapstructure:"logging"`
	General LuetGeneralConfig `yaml:"general,omitempty" mapstructure:"general"`
	System  LuetSystemConfig  `yaml:"system" mapstructure:"system"`
	Solver  LuetSolverOptions `yaml:"solver,omitempty" mapstructure:"solver"`

	RepositoriesConfDir  []string         `yaml:"repos_confdir,omitempty" mapstructure:"repos_confdir"`
	ConfigProtectConfDir []string         `yaml:"config_protect_confdir,omitempty" mapstructure:"config_protect_confdir"`
	ConfigProtectSkip    bool             `yaml:"config_protect_skip,omitempty" mapstructure:"config_protect_skip"`
	ConfigFromHost       bool             `yaml:"config_from_host,omitempty" mapstructure:"config_from_host"`
	SystemRepositories   LuetRepositories `yaml:"repositories,omitempty" mapstructure:"repositories"`

	FinalizerEnvs Finalizers `json:"finalizer_envs,omitempty" yaml:"finalizer_envs,omitempty" mapstructure:"finalizer_envs,omitempty"`

	ConfigProtectConfFiles []config.ConfigProtectConfFile `yaml:"-" mapstructure:"-"`
}

func (c *LuetConfig) GetSystemDB() pkg.PackageDatabase {
	switch c.System.DatabaseEngine {
	case "boltdb":
		return pkg.NewBoltDatabase(
			filepath.Join(c.System.DatabasePath, "luet.db"))
	default:
		return pkg.NewInMemoryDatabase(true)
	}
}

func (c *LuetConfig) AddSystemRepository(r LuetRepository) {
	c.SystemRepositories = append(c.SystemRepositories, r)
}

func (c *LuetConfig) SetFinalizerEnv(k, v string) {
	keyPresent := false
	envs := []FinalizerEnv{}

	for _, kv := range c.FinalizerEnvs {
		if kv.Key == k {
			keyPresent = true
			envs = append(envs, FinalizerEnv{Key: kv.Key, Value: v})
		} else {
			envs = append(envs, kv)
		}
	}
	if !keyPresent {
		envs = append(envs, FinalizerEnv{Key: k, Value: v})
	}

	c.FinalizerEnvs = envs
}

func (c *LuetConfig) YAML() ([]byte, error) {
	return yaml.Marshal(c)
}

func (c *LuetConfig) addProtectFile(file *config.ConfigProtectConfFile) {
	if c.ConfigProtectConfFiles == nil {
		c.ConfigProtectConfFiles = []config.ConfigProtectConfFile{*file}
	} else {
		c.ConfigProtectConfFiles = append(c.ConfigProtectConfFiles, *file)
	}
}

func (c *LuetConfig) loadRepositories() error {
	var regexRepo = regexp.MustCompile(`.yml$|.yaml$`)
	rootfs := ""

	// Respect the rootfs param on read repositories
	if !c.ConfigFromHost {
		rootfs = c.System.Rootfs
	}

	for _, rdir := range c.RepositoriesConfDir {

		rdir = filepath.Join(rootfs, rdir)

		files, err := ioutil.ReadDir(rdir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			if !regexRepo.MatchString(file.Name()) {
				continue
			}

			content, err := ioutil.ReadFile(path.Join(rdir, file.Name()))
			if err != nil {
				continue
			}

			r, err := LoadRepository(content)
			if err != nil {
				continue
			}

			if r.Name == "" || len(r.Urls) == 0 || r.Type == "" {
				continue
			}

			c.AddSystemRepository(*r)
		}
	}
	return nil
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

func (c *LuetConfig) loadConfigProtect() error {
	var regexConfs = regexp.MustCompile(`.yml$`)
	rootfs := ""

	// Respect the rootfs param on read repositories
	if !c.ConfigFromHost {
		rootfs = c.System.Rootfs
	}

	for _, cdir := range c.ConfigProtectConfDir {
		cdir = filepath.Join(rootfs, cdir)

		files, err := ioutil.ReadDir(cdir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			if !regexConfs.MatchString(file.Name()) {
				continue
			}

			content, err := ioutil.ReadFile(path.Join(cdir, file.Name()))
			if err != nil {
				continue
			}

			r, err := loadConfigProtectConfFile(file.Name(), content)
			if err != nil {
				continue
			}

			if r.Name == "" || len(r.Directories) == 0 {
				continue
			}

			c.addProtectFile(r)
		}
	}
	return nil

}

func loadConfigProtectConfFile(filename string, data []byte) (*config.ConfigProtectConfFile, error) {
	ans := config.NewConfigProtectConfFile(filename)
	err := yaml.Unmarshal(data, &ans)
	if err != nil {
		return nil, err
	}
	return ans, nil
}
