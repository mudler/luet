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
	Install(pkg.Packages, *System) error
	Uninstall(*System, ...pkg.Package) error
	Upgrade(s *System) error
	Reclaim(s *System) error

	Repositories([]Repository)
	SyncRepositories(bool) (Repositories, error)
	Swap(pkg.Packages, pkg.Packages, *System) error
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
	SetIndex(i compiler.ArtifactIndex)
	GetTree() tree.Builder
	SetTree(tree.Builder)
	Write(path string, resetRevision, force bool) error
	Sync(bool) (Repository, error)
	GetTreePath() string
	SetTreePath(string)
	GetMetaPath() string
	SetMetaPath(string)
	GetType() string
	SetType(string)
	SetAuthentication(map[string]string)
	GetAuthentication() map[string]string
	GetRevision() int
	IncrementRevision()
	GetLastUpdate() string
	SetLastUpdate(string)
	Client() Client

	SetPriority(int)
	GetRepositoryFile(string) (LuetRepositoryFile, error)
	SetRepositoryFile(string, LuetRepositoryFile)
	SetName(p string)
	Serialize() (*LuetSystemRepositoryMetadata, LuetSystemRepositorySerialized)
	GetBackend() compiler.CompilerBackend
	SetBackend(b compiler.CompilerBackend)
	FileSearch(pattern string) (pkg.Packages, error)
	SearchArtefact(p pkg.Package) (compiler.Artifact, error)

	SetVerify(bool)
	GetVerify() bool
}
