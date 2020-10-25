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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	compiler "github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"
	"github.com/mudler/luet/pkg/tree"

	"github.com/pkg/errors"
)

type LuetInstallerOptions struct {
	SolverOptions                                                  config.LuetSolverOptions
	Concurrency                                                    int
	NoDeps                                                         bool
	OnlyDeps                                                       bool
	Force                                                          bool
	PreserveSystemEssentialData                                    bool
	FullUninstall, FullCleanUninstall                              bool
	CheckConflicts                                                 bool
	SolverUpgrade, RemoveUnavailableOnUpgrade, UpgradeNewRevisions bool
}

type LuetInstaller struct {
	PackageRepositories Repositories

	Options LuetInstallerOptions
}

type ArtifactMatch struct {
	Package    pkg.Package
	Artifact   compiler.Artifact
	Repository Repository
}

func NewLuetInstaller(opts LuetInstallerOptions) Installer {
	return &LuetInstaller{Options: opts}
}

func (l *LuetInstaller) Upgrade(s *System) error {
	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}

	Info(":thinking: Computing upgrade, please hang tight")

	Spinner(32)
	defer SpinnerStop()
	// First match packages against repositories by priority
	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)
	// compute a "big" world
	solv := solver.NewResolver(solver.Options{Type: l.Options.SolverOptions.Implementation, Concurrency: l.Options.Concurrency}, s.Database, allRepos, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
	var uninstall pkg.Packages
	var solution solver.PackagesAssertions

	if l.Options.SolverUpgrade {

		uninstall, solution, err = solv.UpgradeUniverse(l.Options.RemoveUnavailableOnUpgrade)
		if err != nil {
			return errors.Wrap(err, "Failed solving solution for upgrade")
		}
	} else {
		uninstall, solution, err = solv.Upgrade(!l.Options.FullUninstall, l.Options.NoDeps)
		if err != nil {
			return errors.Wrap(err, "Failed solving solution for upgrade")
		}

	}
	SpinnerStop()

	if len(uninstall) > 0 {
		Info("Packages marked for uninstall:")
	}

	for _, p := range uninstall {
		Info(fmt.Sprintf("- %s", p.HumanReadableString()))
	}

	if len(solution) > 0 {
		Info("Packages marked for upgrade:")
	}

	toInstall := pkg.Packages{}
	for _, assertion := range solution {
		// Be sure to filter from solutions packages already installed in the system
		if _, err := s.Database.FindPackage(assertion.Package); err != nil && assertion.Value {
			Info(fmt.Sprintf("- %s", assertion.Package.HumanReadableString()))
			toInstall = append(toInstall, assertion.Package)
		}
	}

	if l.Options.UpgradeNewRevisions {
		Info("Checking packages with new revisions available")
		for _, p := range s.Database.World() {
			matches := syncedRepos.PackageMatches(pkg.Packages{p})
			if len(matches) == 0 {
				// Package missing. the user should run luet upgrade --universe
				Info("Installed packages seems to be missing from remote repositories.")
				Info("It is suggested to run 'luet upgrade --universe'")
				continue
			}
			for _, artefact := range matches[0].Repo.GetIndex() {
				if artefact.GetCompileSpec().GetPackage() == nil {
					return errors.New("Package in compilespec empty")

				}
				if artefact.GetCompileSpec().GetPackage().Matches(p) && artefact.GetCompileSpec().GetPackage().GetBuildTimestamp() != p.GetBuildTimestamp() {
					toInstall = append(toInstall, matches[0].Package).Unique()
					uninstall = append(uninstall, p).Unique()
					Info(
						fmt.Sprintf("- %s ( %s vs %s ) repo: %s (date: %s)",
							p.HumanReadableString(),
							artefact.GetCompileSpec().GetPackage().GetBuildTimestamp(),
							p.GetBuildTimestamp(),
							matches[0].Repo.GetName(),
							matches[0].Repo.GetLastUpdate(),
						))
				}
			}
		}
	}

	return l.swap(syncedRepos, uninstall, toInstall, s)
}

func (l *LuetInstaller) SyncRepositories(inMemory bool) (Repositories, error) {
	Spinner(32)
	defer SpinnerStop()
	syncedRepos := Repositories{}
	for _, r := range l.PackageRepositories {
		repo, err := r.Sync(false)
		if err != nil {
			return nil, errors.Wrap(err, "Failed syncing repository: "+r.GetName())
		}
		syncedRepos = append(syncedRepos, repo)
	}

	// compute what to install and from where
	sort.Sort(syncedRepos)

	if !inMemory {
		l.PackageRepositories = syncedRepos
	}

	return syncedRepos, nil
}

func (l *LuetInstaller) Swap(toRemove pkg.Packages, toInstall pkg.Packages, s *System) error {
	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}
	return l.swap(syncedRepos, toRemove, toInstall, s)
}

func (l *LuetInstaller) swap(syncedRepos Repositories, toRemove pkg.Packages, toInstall pkg.Packages, s *System) error {
	// First match packages against repositories by priority
	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)
	toInstall = syncedRepos.ResolveSelectors(toInstall)

	if err := l.download(syncedRepos, toInstall); err != nil {
		return errors.Wrap(err, "Pre-downloading packages")
	}

	// We don't want any conflict with the installed to raise during the upgrade.
	// In this way we both force uninstalls and we avoid to check with conflicts
	// against the current system state which is pending to deletion
	// E.g. you can't check for conflicts for an upgrade of a new version of A
	// if the old A results installed in the system. This is due to the fact that
	// now the solver enforces the constraints and explictly denies two packages
	// of the same version installed.
	forced := l.Options.Force

	l.Options.Force = true

	for _, u := range toRemove {
		Info(":package:", u.HumanReadableString(), "Marked for deletion")

		err := l.Uninstall(u, s)
		if err != nil && !l.Options.Force {
			Error("Failed uninstall for ", u.HumanReadableString())
			return errors.Wrap(err, "uninstalling "+u.HumanReadableString())
		}

	}
	l.Options.Force = forced

	return l.install(syncedRepos, toInstall, s)
}

func (l *LuetInstaller) Install(cp pkg.Packages, s *System) error {
	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}
	return l.install(syncedRepos, cp, s)
}

func (l *LuetInstaller) download(syncedRepos Repositories, cp pkg.Packages) error {
	toDownload := map[string]ArtifactMatch{}

	// FIXME: This can be optimized. We don't need to re-match this to the repository
	// But we could just do it once

	// Gathers things to download
	for _, currentPack := range cp {
		matches := syncedRepos.PackageMatches(pkg.Packages{currentPack})
		if len(matches) == 0 {
			return errors.New("Failed matching solutions against repository for " + currentPack.HumanReadableString() + " where are definitions coming from?!")
		}
	A:
		for _, artefact := range matches[0].Repo.GetIndex() {
			if artefact.GetCompileSpec().GetPackage() == nil {
				return errors.New("Package in compilespec empty")

			}
			if matches[0].Package.Matches(artefact.GetCompileSpec().GetPackage()) {

				toDownload[currentPack.GetFingerPrint()] = ArtifactMatch{Package: currentPack, Artifact: artefact, Repository: matches[0].Repo}

				break A
			}
		}
	}
	// Download packages into cache in parallel.
	all := make(chan ArtifactMatch)

	var wg = new(sync.WaitGroup)

	// Download
	for i := 0; i < l.Options.Concurrency; i++ {
		wg.Add(1)
		go l.downloadWorker(i, wg, all)
	}
	for _, c := range toDownload {
		all <- c
	}
	close(all)
	wg.Wait()

	return nil
}

// Reclaim adds packages to the system database
// if files from artifacts in the repositories are found
// in the system target
func (l *LuetInstaller) Reclaim(s *System) error {
	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}

	var toMerge []ArtifactMatch = []ArtifactMatch{}

	for _, repo := range syncedRepos {
		for _, artefact := range repo.GetIndex() {
			Debug("Checking if",
				artefact.GetCompileSpec().GetPackage().HumanReadableString(),
				"from", repo.GetName(), "is installed")
		FILES:
			for _, f := range artefact.GetFiles() {
				if helpers.Exists(filepath.Join(s.Target, f)) {
					p, err := repo.GetTree().GetDatabase().FindPackage(artefact.GetCompileSpec().GetPackage())
					if err != nil {
						return err
					}
					Info("Found package:", p.HumanReadableString())
					toMerge = append(toMerge, ArtifactMatch{Artifact: artefact, Package: p})
					break FILES
				}
			}
		}
	}

	for _, match := range toMerge {
		pack := match.Package
		vers, _ := s.Database.FindPackageVersions(pack)

		if len(vers) >= 1 {
			Warning("Filtering out package " + pack.HumanReadableString() + ", already reclaimed")
			continue
		}
		_, err := s.Database.CreatePackage(pack)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Failed creating package")
		}
		s.Database.SetPackageFiles(&pkg.PackageFile{PackageFingerprint: pack.GetFingerPrint(), Files: match.Artifact.GetFiles()})
		Info("Reclaimed package:", pack.HumanReadableString())
	}
	Info("Done!")

	return nil
}

func (l *LuetInstaller) install(syncedRepos Repositories, cp pkg.Packages, s *System) error {
	var p pkg.Packages

	// Check if the package is installed first
	for _, pi := range cp {

		vers, _ := s.Database.FindPackageVersions(pi)

		if len(vers) >= 1 {
			Warning("Filtering out package " + pi.HumanReadableString() + ", it has other versions already installed. Uninstall one of them first ")
			continue
			//return errors.New("Package " + pi.GetFingerPrint() + " has other versions already installed. Uninstall one of them first: " + strings.Join(vers, " "))

		}
		p = append(p, pi)
	}

	if len(p) == 0 {
		Warning("No package to install, bailing out with no errors")
		return nil
	}
	// First get metas from all repos (and decodes trees)

	// First match packages against repositories by priority
	//	matches := syncedRepos.PackageMatches(p)

	// compute a "big" world
	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)
	p = syncedRepos.ResolveSelectors(p)
	toInstall := map[string]ArtifactMatch{}
	var packagesToInstall pkg.Packages
	var err error
	var solution solver.PackagesAssertions

	if !l.Options.NoDeps {
		solv := solver.NewResolver(solver.Options{Type: l.Options.SolverOptions.Implementation, Concurrency: l.Options.Concurrency}, s.Database, allRepos, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
		solution, err = solv.Install(p)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Failed solving solution for package")
		}
		// Gathers things to install
		for _, assertion := range solution {
			if assertion.Value {
				packagesToInstall = append(packagesToInstall, assertion.Package)
			}
		}
	} else if !l.Options.OnlyDeps {
		for _, currentPack := range p {
			packagesToInstall = append(packagesToInstall, currentPack)
		}
	}
	Info(":deciduous_tree: Finding packages to install")
	// Gathers things to install
	for _, currentPack := range packagesToInstall {
		// Check if package is already installed.
		if _, err := s.Database.FindPackage(currentPack); err == nil {
			// skip matching if it is installed already
			continue
		}
		matches := syncedRepos.PackageMatches(pkg.Packages{currentPack})
		if len(matches) == 0 {
			return errors.New("Failed matching solutions against repository for " + currentPack.HumanReadableString() + " where are definitions coming from?!")
		}
	A:
		for _, artefact := range matches[0].Repo.GetIndex() {
			if artefact.GetCompileSpec().GetPackage() == nil {
				return errors.New("Package in compilespec empty")

			}
			if matches[0].Package.Matches(artefact.GetCompileSpec().GetPackage()) {
				currentPack.SetBuildTimestamp(artefact.GetCompileSpec().GetPackage().GetBuildTimestamp())
				// Filter out already installed
				if _, err := s.Database.FindPackage(currentPack); err != nil {
					toInstall[currentPack.GetFingerPrint()] = ArtifactMatch{Package: currentPack, Artifact: artefact, Repository: matches[0].Repo}
					Info("\t:package:", currentPack.HumanReadableString(), "from repository", matches[0].Repo.GetName())
				}
				break A
			}
		}
	}
	// Install packages into rootfs in parallel.
	all := make(chan ArtifactMatch)

	var wg = new(sync.WaitGroup)

	// Download first
	for i := 0; i < l.Options.Concurrency; i++ {
		wg.Add(1)
		go l.downloadWorker(i, wg, all)
	}

	for _, c := range toInstall {
		all <- c
	}
	close(all)
	wg.Wait()

	all = make(chan ArtifactMatch)

	wg = new(sync.WaitGroup)

	// Do the real install
	for i := 0; i < l.Options.Concurrency; i++ {
		wg.Add(1)
		go l.installerWorker(i, wg, all, s)
	}

	for _, c := range toInstall {
		all <- c
	}
	close(all)
	wg.Wait()

	for _, c := range toInstall {
		// Annotate to the system that the package was installed
		_, err := s.Database.CreatePackage(c.Package)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Failed creating package")
		}
	}
	executedFinalizer := map[string]bool{}

	if !l.Options.NoDeps {
		// TODO: Lower those errors as warning
		for _, w := range p {
			// Finalizers needs to run in order and in sequence.
			ordered, err := solution.Order(allRepos, w.GetFingerPrint())
			if err != nil {
				return errors.Wrap(err, "While order a solution for "+w.HumanReadableString())
			}
		ORDER:
			for _, ass := range ordered {
				if ass.Value {

					installed, ok := toInstall[ass.Package.GetFingerPrint()]
					if !ok {
						// It was a dep already installed in the system, so we can skip it safely
						continue ORDER
					}

					treePackage, err := installed.Repository.GetTree().GetDatabase().FindPackage(ass.Package)
					if err != nil {
						return errors.Wrap(err, "Error getting package "+ass.Package.HumanReadableString())
					}
					if helpers.Exists(treePackage.Rel(tree.FinalizerFile)) {
						Info("Executing finalizer for " + ass.Package.HumanReadableString())
						finalizerRaw, err := ioutil.ReadFile(treePackage.Rel(tree.FinalizerFile))
						if err != nil && !l.Options.Force {
							return errors.Wrap(err, "Error reading file "+treePackage.Rel(tree.FinalizerFile))
						}
						if _, exists := executedFinalizer[ass.Package.GetFingerPrint()]; !exists {
							finalizer, err := NewLuetFinalizerFromYaml(finalizerRaw)
							if err != nil && !l.Options.Force {
								return errors.Wrap(err, "Error reading finalizer "+treePackage.Rel(tree.FinalizerFile))
							}
							err = finalizer.RunInstall(s)
							if err != nil && !l.Options.Force {
								return errors.Wrap(err, "Error executing install finalizer "+treePackage.Rel(tree.FinalizerFile))
							}
							executedFinalizer[ass.Package.GetFingerPrint()] = true
						}
					}

				}
			}

		}
	} else {
		for _, c := range toInstall {
			treePackage, err := c.Repository.GetTree().GetDatabase().FindPackage(c.Package)
			if err != nil {
				return errors.Wrap(err, "Error getting package "+c.Package.HumanReadableString())
			}
			if helpers.Exists(treePackage.Rel(tree.FinalizerFile)) {
				Info("Executing finalizer for " + c.Package.HumanReadableString())
				finalizerRaw, err := ioutil.ReadFile(treePackage.Rel(tree.FinalizerFile))
				if err != nil && !l.Options.Force {
					return errors.Wrap(err, "Error reading file "+treePackage.Rel(tree.FinalizerFile))
				}
				if _, exists := executedFinalizer[c.Package.GetFingerPrint()]; !exists {
					finalizer, err := NewLuetFinalizerFromYaml(finalizerRaw)
					if err != nil && !l.Options.Force {
						return errors.Wrap(err, "Error reading finalizer "+treePackage.Rel(tree.FinalizerFile))
					}
					err = finalizer.RunInstall(s)
					if err != nil && !l.Options.Force {
						return errors.Wrap(err, "Error executing install finalizer "+treePackage.Rel(tree.FinalizerFile))
					}
					executedFinalizer[c.Package.GetFingerPrint()] = true
				}
			}
		}
	}

	return nil

}

func (l *LuetInstaller) downloadPackage(a ArtifactMatch) (compiler.Artifact, error) {

	artifact, err := a.Repository.Client().DownloadArtifact(a.Artifact)
	if err != nil {
		return nil, errors.Wrap(err, "Error on download artifact")
	}

	err = artifact.Verify()
	if err != nil && !l.Options.Force {
		return nil, errors.Wrap(err, "Artifact integrity check failure")
	}
	return artifact, nil
}

func (l *LuetInstaller) installPackage(a ArtifactMatch, s *System) error {

	artifact, err := l.downloadPackage(a)
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Failed downloading package")
	}

	files, err := artifact.FileList()
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Could not open package archive")
	}

	err = artifact.Unpack(s.Target, true)
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Error met while unpacking rootfs")
	}

	// First create client and download
	// Then unpack to system
	return s.Database.SetPackageFiles(&pkg.PackageFile{PackageFingerprint: a.Package.GetFingerPrint(), Files: files})
}

func (l *LuetInstaller) downloadWorker(i int, wg *sync.WaitGroup, c <-chan ArtifactMatch) error {
	defer wg.Done()

	for p := range c {
		// TODO: Keep trace of what was added from the tar, and save it into system
		_, err := l.downloadPackage(p)
		if err != nil && !l.Options.Force {
			//TODO: Uninstall, rollback.
			Fatal("Failed installing package "+p.Package.GetName(), err.Error())
			return errors.Wrap(err, "Failed installing package "+p.Package.GetName())
		}
		if err == nil {
			Info(":package: ", p.Package.HumanReadableString(), "downloaded")
		} else if err != nil && l.Options.Force {
			Info(":package: ", p.Package.HumanReadableString(), "downloaded with failures (force download)")
		}
	}

	return nil
}

func (l *LuetInstaller) installerWorker(i int, wg *sync.WaitGroup, c <-chan ArtifactMatch, s *System) error {
	defer wg.Done()

	for p := range c {
		// TODO: Keep trace of what was added from the tar, and save it into system
		err := l.installPackage(p, s)
		if err != nil && !l.Options.Force {
			//TODO: Uninstall, rollback.
			Fatal("Failed installing package "+p.Package.GetName(), err.Error())
			return errors.Wrap(err, "Failed installing package "+p.Package.GetName())
		}
		if err == nil {
			Info(":package: ", p.Package.HumanReadableString(), "installed")
		} else if err != nil && l.Options.Force {
			Info(":package: ", p.Package.HumanReadableString(), "installed with failures (force install)")
		}
	}

	return nil
}

func (l *LuetInstaller) uninstall(p pkg.Package, s *System) error {
	files, err := s.Database.GetPackageFiles(p)
	if err != nil {
		return errors.Wrap(err, "Failed getting installed files")
	}

	// Remove from target
	for _, f := range files {
		target := filepath.Join(s.Target, f)
		Debug("Removing", target)

		if l.Options.PreserveSystemEssentialData &&
			strings.HasPrefix(f, config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath()) ||
			strings.HasPrefix(f, config.LuetCfg.GetSystem().GetSystemRepoDatabaseDirPath()) {
			Warning("Preserve ", f, " which is required by luet ( you have to delete it manually if you really need to)")
			continue
		}

		err := os.Remove(target)
		if err != nil {
			Warning("Failed removing file (not present in the system target ?)", target)
		}
	}
	err = s.Database.RemovePackageFiles(p)
	if err != nil {
		return errors.Wrap(err, "Failed removing package files from database")
	}
	err = s.Database.RemovePackage(p)
	if err != nil {
		return errors.Wrap(err, "Failed removing package from database")
	}

	Info(p.GetFingerPrint(), "Removed")
	return nil
}

func (l *LuetInstaller) Uninstall(p pkg.Package, s *System) error {
	Spinner(32)
	defer SpinnerStop()

	Info("Uninstalling :package:", p.HumanReadableString(), "hang tight")

	// compute uninstall from all world - remove packages in parallel - run uninstall finalizer (in order) TODO - mark the uninstallation in db
	// Get installed definition
	checkConflicts := l.Options.CheckConflicts
	full := l.Options.FullUninstall
	if l.Options.Force == true { // IF forced, we want to remove the package and all its requires
		checkConflicts = false
		full = false
	}

	// Create a temporary DB with the installed packages
	// so the solver is much faster finding the deptree
	installedtmp := pkg.NewInMemoryDatabase(false)

	for _, i := range s.Database.World() {
		_, err := installedtmp.CreatePackage(i)
		if err != nil {
			return errors.Wrap(err, "Failed create temporary in-memory db")
		}
	}

	if !l.Options.NoDeps {
		Info("Finding :package:", p.HumanReadableString(), "dependency graph :deciduous_tree:")
		solv := solver.NewResolver(solver.Options{Type: l.Options.SolverOptions.Implementation, Concurrency: l.Options.Concurrency}, installedtmp, installedtmp, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
		var solution pkg.Packages
		var err error
		if l.Options.FullCleanUninstall {
			solution, err = solv.UninstallUniverse(pkg.Packages{p})
			if err != nil {
				return errors.Wrap(err, "Could not solve the uninstall constraints. Tip: try with --solver-type qlearning or with --force, or by removing packages excluding their dependencies with --nodeps")
			}
		} else {
			solution, err = solv.Uninstall(p, checkConflicts, full)
			if err != nil && !l.Options.Force {
				return errors.Wrap(err, "Could not solve the uninstall constraints. Tip: try with --solver-type qlearning or with --force, or by removing packages excluding their dependencies with --nodeps")
			}
		}

		for _, p := range solution {
			Info("Uninstalling", p.HumanReadableString())
			err := l.uninstall(p, s)
			if err != nil && !l.Options.Force {
				return errors.Wrap(err, "Uninstall failed")
			}
		}
	} else {
		Info("Uninstalling", p.HumanReadableString(), "without deps")
		err := l.uninstall(p, s)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Uninstall failed")
		}
		Info(":package:", p.HumanReadableString(), "uninstalled")
	}
	return nil

}

func (l *LuetInstaller) Repositories(r []Repository) { l.PackageRepositories = r }
