// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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

package installer

import (
	compiler "github.com/mudler/luet/pkg/compiler"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	//"github.com/mudler/luet/pkg/solver"
)

type Installer interface {
	Install([]pkg.Package, *System) error
	Uninstall(pkg.Package, *System) error
	Upgrade(s *System) error
	Repositories([]Repository)
	SyncRepositories(bool) (Repositories, error)
}

type Client interface {
	DownloadArtifact(compiler.Artifact) (compiler.Artifact, error)
	DownloadFile(string) (string, error)
}

type Repositories []Repository

type Repository interface {
	GetName() string
	GetDescription() string
	GetUrls() []string
	SetUrls([]string)
	AddUrl(string)
	GetPriority() int
	GetIndex() compiler.ArtifactIndex
	GetTree() tree.Builder
	SetTree(tree.Builder)
	Write(path string) error
	Sync() (Repository, error)
	GetTreePath() string
	SetTreePath(string)
	GetType() string
	SetType(string)
	Client() Client
}
