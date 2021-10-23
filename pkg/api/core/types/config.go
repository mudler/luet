// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
//             2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
	Path string `mapstructure:"path"`
	// Enable/Disable logging to file
	EnableLogFile bool `mapstructure:"enable_logfile"`
	// Enable JSON format logging in file
	JsonFormat bool `mapstructure:"json_format"`

	// Log level
	Level LogLevel `mapstructure:"level"`

	// Enable emoji
	EnableEmoji bool `mapstructure:"enable_emoji"`
	// Enable/Disable color in logging
	Color bool `mapstructure:"color"`
}

type LuetGeneralConfig struct {
	SameOwner       bool `yaml:"same_owner,omitempty" mapstructure:"same_owner"`
	Concurrency     int  `yaml:"concurrency,omitempty" mapstructure:"concurrency"`
	Debug           bool `yaml:"debug,omitempty" mapstructure:"debug"`
	ShowBuildOutput bool `yaml:"show_build_output,omitempty" mapstructure:"show_build_output"`
	FatalWarns      bool `yaml:"fatal_warnings,omitempty" mapstructure:"fatal_warnings"`
	HTTPTimeout     int  `yaml:"http_timeout,omitempty" mapstructure:"http_timeout"`
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

func (s *LuetSystemConfig) SetRootFS(path string) error {
	p, err := fileHelper.Rel2Abs(path)
	if err != nil {
		return err
	}

	s.Rootfs = p
	return nil
}

func (sc *LuetSystemConfig) GetRepoDatabaseDirPath(name string) string {
	dbpath := filepath.Join(sc.Rootfs, sc.DatabasePath)
	dbpath = filepath.Join(dbpath, "repos/"+name)
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		panic(err)
	}
	return dbpath
}

func (sc *LuetSystemConfig) GetSystemRepoDatabaseDirPath() string {
	dbpath := filepath.Join(sc.Rootfs,
		sc.DatabasePath)
	err := os.MkdirAll(dbpath, os.ModePerm)
	if err != nil {
		panic(err)
	}
	return dbpath
}

func (sc *LuetSystemConfig) GetSystemPkgsCacheDirPath() (ans string) {
	var cachepath string
	if sc.PkgsCachePath != "" {
		cachepath = sc.PkgsCachePath
	} else {
		// Create dynamic cache for test suites
		cachepath, _ = ioutil.TempDir(os.TempDir(), "cachepkgs")
	}

	if filepath.IsAbs(cachepath) {
		ans = cachepath
	} else {
		ans = filepath.Join(sc.GetSystemRepoDatabaseDirPath(), cachepath)
	}

	return
}

func (sc *LuetSystemConfig) GetRootFsAbs() (string, error) {
	return filepath.Abs(sc.Rootfs)
}

type LuetKV struct {
	Key   string `json:"key" yaml:"key" mapstructure:"key"`
	Value string `json:"value" yaml:"value" mapstructure:"value"`
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

	FinalizerEnvs []LuetKV `json:"finalizer_envs,omitempty" yaml:"finalizer_envs,omitempty" mapstructure:"finalizer_envs,omitempty"`

	ConfigProtectConfFiles []config.ConfigProtectConfFile `yaml:"-" mapstructure:"-"`
}

func (c *LuetConfig) GetSystemDB() pkg.PackageDatabase {
	switch c.GetSystem().DatabaseEngine {
	case "boltdb":
		return pkg.NewBoltDatabase(
			filepath.Join(c.GetSystem().GetSystemRepoDatabaseDirPath(), "luet.db"))
	default:
		return pkg.NewInMemoryDatabase(true)
	}
}

func (c *LuetConfig) AddSystemRepository(r LuetRepository) {
	c.SystemRepositories = append(c.SystemRepositories, r)
}

func (c *LuetConfig) GetFinalizerEnvsMap() map[string]string {
	ans := make(map[string]string)

	for _, kv := range c.FinalizerEnvs {
		ans[kv.Key] = kv.Value
	}
	return ans
}

func (c *LuetConfig) SetFinalizerEnv(k, v string) {
	keyPresent := false
	envs := []LuetKV{}

	for _, kv := range c.FinalizerEnvs {
		if kv.Key == k {
			keyPresent = true
			envs = append(envs, LuetKV{Key: kv.Key, Value: v})
		} else {
			envs = append(envs, kv)
		}
	}
	if !keyPresent {
		envs = append(envs, LuetKV{Key: k, Value: v})
	}

	c.FinalizerEnvs = envs
}

func (c *LuetConfig) GetFinalizerEnvs() []string {
	ans := []string{}
	for _, kv := range c.FinalizerEnvs {
		ans = append(ans, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
	}
	return ans
}

func (c *LuetConfig) GetFinalizerEnv(k string) (string, error) {
	keyNotPresent := true
	ans := ""
	for _, kv := range c.FinalizerEnvs {
		if kv.Key == k {
			keyNotPresent = false
			ans = kv.Value
		}
	}

	if keyNotPresent {
		return "", errors.New("Finalizer key " + k + " not found")
	}
	return ans, nil
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

func (c *LuetConfig) GetSolverOptions() *LuetSolverOptions {
	return &c.Solver
}

func (c *LuetConfig) YAML() ([]byte, error) {
	return yaml.Marshal(c)
}

func (c *LuetConfig) GetConfigProtectConfFiles() []config.ConfigProtectConfFile {
	return c.ConfigProtectConfFiles
}

func (c *LuetConfig) AddConfigProtectConfFile(file *config.ConfigProtectConfFile) {
	if c.ConfigProtectConfFiles == nil {
		c.ConfigProtectConfFiles = []config.ConfigProtectConfFile{*file}
	} else {
		c.ConfigProtectConfFiles = append(c.ConfigProtectConfFiles, *file)
	}
}

func (c *LuetConfig) LoadRepositories(ctx *Context) error {
	var regexRepo = regexp.MustCompile(`.yml$|.yaml$`)
	var err error
	rootfs := ""

	// Respect the rootfs param on read repositories
	if !c.ConfigFromHost {
		rootfs, err = c.GetSystem().GetRootFsAbs()
		if err != nil {
			return err
		}
	}

	for _, rdir := range c.RepositoriesConfDir {

		rdir = filepath.Join(rootfs, rdir)

		ctx.Debug("Parsing Repository Directory", rdir, "...")

		files, err := ioutil.ReadDir(rdir)
		if err != nil {
			ctx.Debug("Skip dir", rdir, ":", err.Error())
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			if !regexRepo.MatchString(file.Name()) {
				ctx.Debug("File", file.Name(), "skipped.")
				continue
			}

			content, err := ioutil.ReadFile(path.Join(rdir, file.Name()))
			if err != nil {
				ctx.Warning("On read file", file.Name(), ":", err.Error())
				ctx.Warning("File", file.Name(), "skipped.")
				continue
			}

			r, err := LoadRepository(content)
			if err != nil {
				ctx.Warning("On parse file", file.Name(), ":", err.Error())
				ctx.Warning("File", file.Name(), "skipped.")
				continue
			}

			if r.Name == "" || len(r.Urls) == 0 || r.Type == "" {
				ctx.Warning("Invalid repository ", file.Name())
				ctx.Warning("File", file.Name(), "skipped.")
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

func (c *LuetConfig) LoadConfigProtect(ctx *Context) error {
	var regexConfs = regexp.MustCompile(`.yml$`)
	var err error

	rootfs := ""

	// Respect the rootfs param on read repositories
	if !c.ConfigFromHost {
		rootfs, err = c.GetSystem().GetRootFsAbs()
		if err != nil {
			return err
		}
	}

	for _, cdir := range c.ConfigProtectConfDir {
		cdir = filepath.Join(rootfs, cdir)

		ctx.Debug("Parsing Config Protect Directory", cdir, "...")

		files, err := ioutil.ReadDir(cdir)
		if err != nil {
			ctx.Debug("Skip dir", cdir, ":", err.Error())
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			if !regexConfs.MatchString(file.Name()) {
				ctx.Debug("File", file.Name(), "skipped.")
				continue
			}

			content, err := ioutil.ReadFile(path.Join(cdir, file.Name()))
			if err != nil {
				ctx.Warning("On read file", file.Name(), ":", err.Error())
				ctx.Warning("File", file.Name(), "skipped.")
				continue
			}

			r, err := loadConfigProtectConFile(file.Name(), content)
			if err != nil {
				ctx.Warning("On parse file", file.Name(), ":", err.Error())
				ctx.Warning("File", file.Name(), "skipped.")
				continue
			}

			if r.Name == "" || len(r.Directories) == 0 {
				ctx.Warning("Invalid config protect file", file.Name())
				ctx.Warning("File", file.Name(), "skipped.")
				continue
			}

			c.AddConfigProtectConfFile(r)
		}
	}
	return nil

}

func loadConfigProtectConFile(filename string, data []byte) (*config.ConfigProtectConfFile, error) {
	ans := config.NewConfigProtectConfFile(filename)
	err := yaml.Unmarshal(data, &ans)
	if err != nil {
		return nil, err
	}
	return ans, nil
}

func (c *LuetLoggingConfig) SetLogLevel(s LogLevel) {
	c.Level = s
}

func (c *LuetSystemConfig) InitTmpDir() error {
	if !filepath.IsAbs(c.TmpDirBase) {
		abs, err := fileHelper.Rel2Abs(c.TmpDirBase)
		if err != nil {
			return errors.Wrap(err, "while converting relative path to absolute path")
		}
		c.TmpDirBase = abs
	}

	if _, err := os.Stat(c.TmpDirBase); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(c.TmpDirBase, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *LuetSystemConfig) CleanupTmpDir() error {
	return os.RemoveAll(c.TmpDirBase)
}

func (c *LuetSystemConfig) TempDir(pattern string) (string, error) {
	err := c.InitTmpDir()
	if err != nil {
		return "", err
	}
	return ioutil.TempDir(c.TmpDirBase, pattern)
}

func (c *LuetSystemConfig) TempFile(pattern string) (*os.File, error) {
	err := c.InitTmpDir()
	if err != nil {
		return nil, err
	}
	return ioutil.TempFile(c.TmpDirBase, pattern)
}
