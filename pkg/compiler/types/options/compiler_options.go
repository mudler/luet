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

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/compiler/types/compression"
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
	SolverOptions   types.LuetSolverOptions
	BuildValuesFile []string
	BuildValues     []map[string]interface{}

	PackageTargetOnly bool
	Rebuild           bool

	BackendArgs []string

	BackendType string

	// TemplatesFolder. should default to tree/templates
	TemplatesFolder []string

	// Tells wether to push final container images after building
	PushFinalImages      bool
	PushFinalImagesForce bool
	// Image repository to push to
	PushFinalImagesRepository string

	Context types.Context
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
		SolverOptions:       types.LuetSolverOptions{SolverOptions: types.SolverOptions{Concurrency: 1, Type: types.SolverSingleCoreSimple}},
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

// WithFinalRepository Sets the final repository where to push
// images of built artifacts
func WithFinalRepository(r string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.PushFinalImagesRepository = r
		return nil
	}
}

func EnablePushFinalImages(cfg *Compiler) error {
	cfg.PushFinalImages = true
	return nil
}

func ForcePushFinalImages(cfg *Compiler) error {
	cfg.PushFinalImagesForce = true
	return nil
}

func WithBackendType(r string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.BackendType = r
		return nil
	}
}

func WithTemplateFolder(r []string) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.TemplatesFolder = r
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

// WithPushRepository Sets the image reference where to push
// cache images
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

func WithSolverOptions(c types.LuetSolverOptions) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.SolverOptions = c
		return nil
	}
}

func WithContext(c types.Context) func(cfg *Compiler) error {
	return func(cfg *Compiler) error {
		cfg.Context = c
		return nil
	}
}
