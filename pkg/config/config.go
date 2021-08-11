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
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	pkg "github.com/mudler/luet/pkg/package"
	solver "github.com/mudler/luet/pkg/solver"

	v "github.com/spf13/viper"
)

var LuetCfg = NewLuetConfig(v.GetViper())
var AvailableResolvers = strings.Join([]string{solver.QLearningResolverType}, " ")

type LuetLoggingConfig struct {
	// Path of the logfile
	Path string `mapstructure:"path"`
	// Enable/Disable logging to file
	EnableLogFile bool `mapstructure:"enable_logfile"`
	// Enable JSON format logging in file
	JsonFormat bool `mapstructure:"json_format"`

	// Log level
	Level string `mapstructure:"level"`

	// Enable emoji
	EnableEmoji bool `mapstructure:"enable_emoji"`
	// Enable/Disable color in logging
	Color bool `mapstructure:"color"`
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

type LuetSolverOptions struct {
	solver.Options
	Type           string            `mapstructure:"type"`
	LearnRate      float32           `mapstructure:"rate"`
	Discount       float32           `mapstructure:"discount"`
	MaxAttempts    int               `mapstructure:"max_attempts"`
	Implementation solver.SolverType `mapstructure:"implementation"`
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
	TreePath       string            `json:"treepath,omitempty" yaml:"treepath,omitempty" mapstructure:"treepath"`
	MetaPath       string            `json:"metapath,omitempty" yaml:"metapath,omitempty" mapstructure:"metapath"`
	Verify         bool              `json:"verify,omitempty" yaml:"verify,omitempty" mapstructure:"verify"`

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
		MetaPath:       "",
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
		MetaPath:       "",
		Enable:         false,
		Cached:         false,
		Authentication: make(map[string]string, 0),
	}
}

func (r *LuetRepository) String() string {
	return fmt.Sprintf("[%s] prio: %d, type: %s, enable: %t, cached: %t",
		r.Name, r.Priority, r.Type, r.Enable, r.Cached)
}

type LuetKV struct {
	Key   string `json:"key" yaml:"key" mapstructure:"key"`
	Value string `json:"value" yaml:"value" mapstructure:"value"`
}

type LuetConfig struct {
	Viper *v.Viper

	Logging LuetLoggingConfig `mapstructure:"logging"`
	General LuetGeneralConfig `mapstructure:"general"`
	System  LuetSystemConfig  `mapstructure:"system"`
	Solver  LuetSolverOptions `mapstructure:"solver"`

	RepositoriesConfDir  []string         `mapstructure:"repos_confdir"`
	ConfigProtectConfDir []string         `mapstructure:"config_protect_confdir"`
	ConfigProtectSkip    bool             `mapstructure:"config_protect_skip"`
	ConfigFromHost       bool             `mapstructure:"config_from_host"`
	CacheRepositories    []LuetRepository `mapstructure:"repetitors"`
	SystemRepositories   []LuetRepository `mapstructure:"repositories"`

	FinalizerEnvs []LuetKV `json:"finalizer_envs,omitempty" yaml:"finalizer_envs,omitempty" mapstructure:"finalizer_envs,omitempty"`

	ConfigProtectConfFiles []ConfigProtectConfFile
}

func NewLuetConfig(viper *v.Viper) *LuetConfig {
	if viper == nil {
		viper = v.New()
	}

	GenDefault(viper)
	return &LuetConfig{Viper: viper, ConfigProtectConfFiles: nil}
}

func GenDefault(viper *v.Viper) {
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.enable_logfile", false)
	viper.SetDefault("logging.path", "/var/log/luet.log")
	viper.SetDefault("logging.json_format", false)
	viper.SetDefault("logging.enable_emoji", true)
	viper.SetDefault("logging.color", true)

	viper.SetDefault("general.concurrency", runtime.NumCPU())
	viper.SetDefault("general.debug", false)
	viper.SetDefault("general.show_build_output", false)
	viper.SetDefault("general.spinner_ms", 100)
	viper.SetDefault("general.spinner_charset", 22)
	viper.SetDefault("general.fatal_warnings", false)

	u, err := user.Current()
	// os/user doesn't work in from scratch environments
	if err != nil || (u != nil && u.Uid == "0") {
		viper.SetDefault("general.same_owner", true)
	} else {
		viper.SetDefault("general.same_owner", false)
	}

	viper.SetDefault("system.database_engine", "boltdb")
	viper.SetDefault("system.database_path", "/var/cache/luet")
	viper.SetDefault("system.rootfs", "/")
	viper.SetDefault("system.tmpdir_base", filepath.Join(os.TempDir(), "tmpluet"))
	viper.SetDefault("system.pkgs_cache_path", "packages")

	viper.SetDefault("repos_confdir", []string{"/etc/luet/repos.conf.d"})
	viper.SetDefault("config_protect_confdir", []string{"/etc/luet/config.protect.d"})
	viper.SetDefault("config_protect_skip", false)
	// TODO: Set default to false when we are ready for migration.
	viper.SetDefault("config_from_host", true)
	viper.SetDefault("cache_repositories", []string{})
	viper.SetDefault("system_repositories", []string{})
	viper.SetDefault("finalizer_envs", make(map[string]string, 0))

	viper.SetDefault("solver.type", "")
	viper.SetDefault("solver.rate", 0.7)
	viper.SetDefault("solver.discount", 1.0)
	viper.SetDefault("solver.max_attempts", 9000)
}

func (c *LuetConfig) GetSystemDB() pkg.PackageDatabase {
	switch LuetCfg.GetSystem().DatabaseEngine {
	case "boltdb":
		return pkg.NewBoltDatabase(
			filepath.Join(LuetCfg.GetSystem().GetSystemRepoDatabaseDirPath(), "luet.db"))
	default:
		return pkg.NewInMemoryDatabase(true)
	}
}

func (c *LuetConfig) AddSystemRepository(r LuetRepository) {
	c.SystemRepositories = append(c.SystemRepositories, r)
}

func (c *LuetConfig) GetFinalizerEnvsMap() map[string]string {
	ans := make(map[string]string, 0)

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

func (c *LuetConfig) GetConfigProtectConfFiles() []ConfigProtectConfFile {
	return c.ConfigProtectConfFiles
}

func (c *LuetConfig) AddConfigProtectConfFile(file *ConfigProtectConfFile) {
	if c.ConfigProtectConfFiles == nil {
		c.ConfigProtectConfFiles = []ConfigProtectConfFile{*file}
	} else {
		c.ConfigProtectConfFiles = append(c.ConfigProtectConfFiles, *file)
	}
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

func (c *LuetSolverOptions) String() string {
	ans := fmt.Sprintf(`
solver:
  type: %s
  rate: %f
  discount: %f
  max_attempts: %d`, c.Type, c.LearnRate, c.Discount,
		c.MaxAttempts)

	return ans
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

func (c *LuetLoggingConfig) SetLogLevel(s string) {
	c.Level = s
}

func (c *LuetLoggingConfig) String() string {
	ans := fmt.Sprintf(`
logging:
  enable_logfile: %t
  path: %s
  json_format: %t
  color: %t
  enable_emoji: %t
  level: %s`, c.EnableLogFile, c.Path, c.JsonFormat,
		c.Color, c.EnableEmoji, c.Level)

	return ans
}

func (c *LuetSystemConfig) String() string {
	ans := fmt.Sprintf(`
system:
  database_engine: %s
  database_path: %s
  pkgs_cache_path: %s
  tmpdir_base: %s
  rootfs: %s`,
		c.DatabaseEngine, c.DatabasePath, c.PkgsCachePath,
		c.TmpDirBase, c.Rootfs)

	return ans
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
