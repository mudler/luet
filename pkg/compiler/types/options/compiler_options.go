// Copyright © 2019-2021 Ettore Di Giacinto <mudler@sabayon.org>
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

package options

import (
	"runtime"

	"github.com/mudler/luet/pkg/compiler/types/compression"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/solver"
)

type Compiler struct {
	PushImageRepository      string
	PullImageRepository      []string
	PullFirst, KeepImg, Push bool
	Concurrency              int
	CompressionType          compression.Implementation

	Wait            bool
	OnlyDeps        bool
	NoDeps          bool
	SolverOptions   config.LuetSolverOptions
	BuildValuesFile []string
	BuildValues     []map[string]interface{}

	PackageTargetOnly bool
	Rebuild           bool

	BackendArgs []string

	BackendType string
}

func NewDefaultCompiler() *Compiler {
	return &Compiler{
		PushImageRepository: "luet/cache",
		PullFirst:           false,
		Push:                false,
		CompressionType:     compression.None,
		KeepImg:             true,
		Concurrency:         runtime.NumCPU(),
		OnlyDeps:            false,
		NoDeps:              false,
		SolverOptions:       config.LuetSolverOptions{Options: solver.Options{Concurrency: 1, Type: solver.SingleCoreSimple}},
	}
}

type Option func(cfg *Compiler) error

func (cfg *Compiler) Apply(opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return err
		}
	}
	return nil
}

func WithOptions(opt *Compiler) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg = opt
		return nil
	}
}

func WithBackendType(r string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.BackendType = r
		return nil
	}
}

func WithBuildValues(r []string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.BuildValuesFile = r
		return nil
	}
}

func WithPullRepositories(r []string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.PullImageRepository = r
		return nil
	}
}

func WithPushRepository(r string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		if len(cfg.PullImageRepository) == 0 {
			cfg.PullImageRepository = []string{cfg.PushImageRepository}
		}
		cfg.PushImageRepository = r
		return nil
	}
}

func BackendArgs(r []string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.BackendArgs = r
		return nil
	}
}

func PullFirst(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.PullFirst = b
		return nil
	}
}

func KeepImg(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.KeepImg = b
		return nil
	}
}

func Rebuild(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.Rebuild = b
		return nil
	}
}

func PushImages(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.Push = b
		return nil
	}
}

func Wait(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.Wait = b
		return nil
	}
}

func OnlyDeps(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.OnlyDeps = b
		return nil
	}
}

func OnlyTarget(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.PackageTargetOnly = b
		return nil
	}
}

func NoDeps(b bool) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.NoDeps = b
		return nil
	}
}

func Concurrency(i int) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		if i == 0 {
			i = runtime.NumCPU()
		}
		cfg.Concurrency = i
		return nil
	}
}

func WithCompressionType(t compression.Implementation) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.CompressionType = t
		return nil
	}
}

func WithSolverOptions(c config.LuetSolverOptions) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.SolverOptions = c
		return nil
	}
}
