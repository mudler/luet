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
	"sort"
	"sync"

	compiler "github.com/mudler/luet/pkg/compiler"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"

	"github.com/pkg/errors"
)

type LuetInstaller struct {
	PackageClient       Client
	PackageRepositories Repositories
	Concurrency         int
}

type ArtifactMatch struct {
	Package    pkg.Package
	Artifact   compiler.Artifact
	Repository Repository
}

func NewLuetInstaller(concurrency int) Installer {
	return &LuetInstaller{Concurrency: concurrency}
}

func (l *LuetInstaller) Install(p []pkg.Package, s *System) error {
	// First get metas from all repos (and decodes trees)

	Spinner(32)
	defer SpinnerStop()
	syncedRepos := Repositories{}
	for _, r := range l.PackageRepositories {
		repo, err := r.Sync(l.PackageClient)
		if err != nil {
			return errors.Wrap(err, "Failed syncing repository"+r.GetName())
		}
		syncedRepos = append(syncedRepos, repo)
	}

	// compute what to install and from where
	sort.Sort(syncedRepos)

	// First match packages against repositories by priority
	matches := syncedRepos.PackageMatches(p)

	// Get installed definition
	installed, err := s.World()
	if err != nil {
		return errors.Wrap(err, "Failed generating installed world ")
	}

	// compute a "big" world
	allrepoWorld := syncedRepos.World()

	// If installed exists in the world, we need to use them to make the solver point to them
	realInstalled := []pkg.Package{}
	for _, i := range installed {
		var found pkg.Package
	W:
		for _, p := range allrepoWorld {
			if p.Matches(i) {
				found = p
				break W
			}
		}

		if found != nil {
			realInstalled = append(realInstalled, found)

		} else {
			realInstalled = append(realInstalled, i)
		}
	}

	allwanted := []pkg.Package{}
	for _, wanted := range p {
		var found pkg.Package

	W:
		for _, p := range allrepoWorld {
			if p.Matches(wanted) {
				found = p
				break W
			}
		}

		if found != nil {
			allwanted = append(allwanted, found)

		} else {
			return errors.New("Package requested to install not found")
		}
	}

	s := solver.NewSolver(realInstalled, allrepoWorld, pkg.NewInMemoryDatabase(false))
	solution, err := s.Install(allwanted)

	// Gathers things to install
	toInstall := []ArtifactMatch{}
	for _, assertion := range solution {
		if assertion.Value && assertion.Package.IsFlagged() {
			matches := syncedRepos.PackageMatches([]pkg.Package{assertion.Package})
			if len(matches) != 1 {
				return errors.New("Failed matching solutions against repository - where are definitions coming from?!")
			}
		W:
			for _, artefact := range matches[0].Repo.GetIndex() {
				if matches[0].Package.Matches(artefact.GetCompileSpec().GetPackage()) {
					toInstall = append(toInstall, ArtifactMatch{Package: assertion.Package, Artifact: artefact, Repository: matches[0].Repo})
					break W
				}
			}
		}
	}

	all := make(chan ArtifactMatch)

	var wg = new(sync.WaitGroup)
	for i := 0; i < l.Concurrency; i++ {
		wg.Add(1)
		go t.installerWorker(i, wg, all, s)
	}

	for _, c := range toInstall {
		all <- c
	}
	close(all)
	wg.Wait()

	// Next: Look up for each solution with PackageMatches to check where to install.

	// install (parallel)
	// finalizers(sequential -  generate an ordered list of packagematches of installs from packagematches order for each package. Just make sure to run the finalizers ones.)
	// mark the installation to the system db, along with the files that belongs to the package
	return nil

}

func (l *LuetInstaller) installPackage(a ArtifactMatch, s *System) error {

}

func (l *LuetInstaller) installerWorker(i int, wg *sync.WaitGroup, c <-chan ArtifactMatch, s *System) error {
	defer wg.Done()

	for p := range c {
		err := l.installPackage(p, s)
		if err != nil {
			//TODO: Uninstall, rollback.
			Fatal("Failed installing package" + p.Package.GetName())
			return err
		}
	}

	return nil
}
func (l *LuetInstaller) Uninstall(p []pkg.Package, s *System) error {
	// compute uninstall from all world - remove packages in parallel - run uninstall finalizer (in order) - mark the uninstallation in db
	return nil

}

func (l *LuetInstaller) Client(c Client)             { l.PackageClient = c }
func (l *LuetInstaller) Repositories(r []Repository) { l.PackageRepositories = r }
