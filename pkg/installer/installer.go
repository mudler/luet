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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ghodss/yaml"
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
	SolverOptions               config.LuetSolverOptions
	Concurrency                 int
	NoDeps                      bool
	OnlyDeps                    bool
	Force                       bool
	PreserveSystemEssentialData bool
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

type LuetFinalizer struct {
	Install   []string `json:"install"`
	Uninstall []string `json:"uninstall"` // TODO: Where to store?
}

func (f *LuetFinalizer) RunInstall() error {
	for _, c := range f.Install {
		Debug("finalizer:", "sh", "-c", c)
		cmd := exec.Command("sh", "-c", c)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "Failed running command: "+string(stdoutStderr))
		}
		Info(string(stdoutStderr))
	}
	return nil
}

// TODO: We don't store uninstall finalizers ?!
func (f *LuetFinalizer) RunUnInstall() error {
	for _, c := range f.Install {
		Debug("finalizer:", "sh", "-c", c)
		cmd := exec.Command("sh", "-c", c)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "Failed running command: "+string(stdoutStderr))
		}
		Info(string(stdoutStderr))
	}
	return nil
}

func NewLuetFinalizerFromYaml(data []byte) (*LuetFinalizer, error) {
	var p LuetFinalizer
	err := yaml.Unmarshal(data, &p)
	if err != nil {
		return &p, err
	}
	return &p, err
}

func NewLuetInstaller(opts LuetInstallerOptions) Installer {
	return &LuetInstaller{Options: opts}
}

func (l *LuetInstaller) Upgrade(s *System) error {
	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}
	// First match packages against repositories by priority
	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)
	// compute a "big" world
	solv := solver.NewResolver(s.Database, allRepos, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
	uninstall, solution, err := solv.Upgrade(false)
	if err != nil {
		return errors.Wrap(err, "Failed solving solution for upgrade")
	}

	toInstall := []pkg.Package{}
	for _, assertion := range solution {
		if assertion.Value {
			toInstall = append(toInstall, assertion.Package)
		}
	}

	if err := l.install(syncedRepos, toInstall, s, true); err != nil {
		return errors.Wrap(err, "Pre-downloading packages")
	}

	// We don't want any conflict with the installed to raise during the upgrade.
	// In this way we both force uninstalls and we avoid to check with conflicts
	// against the current system state which is pending to deletion
	// E.g. you can't check for conflicts for an upgrade of a new version of A
	// if the old A results installed in the system. This is due to the fact that
	// now the solver enforces the constraints and explictly denies two packages
	// of the same version installed.
	forced := false
	if l.Options.Force {
		forced = true
	}
	l.Options.Force = true

	for _, u := range uninstall {
		Info(":package: Marked for deletion", u.HumanReadableString())

		err := l.Uninstall(u, s)
		if err != nil && !l.Options.Force {
			Error("Failed uninstall for ", u.HumanReadableString())
			return errors.Wrap(err, "uninstalling "+u.HumanReadableString())
		}

	}
	l.Options.Force = forced

	return l.install(syncedRepos, toInstall, s, false)
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

func (l *LuetInstaller) Install(cp []pkg.Package, s *System, downloadOnly bool) error {
	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}
	return l.install(syncedRepos, cp, s, downloadOnly)
}

func (l *LuetInstaller) install(syncedRepos Repositories, cp []pkg.Package, s *System, downloadOnly bool) error {
	var p []pkg.Package

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
	var packagesToInstall []pkg.Package
	var err error
	var solution solver.PackagesAssertions

	if !l.Options.NoDeps {
		solv := solver.NewResolver(s.Database, allRepos, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
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

	// Gathers things to install
	for _, currentPack := range packagesToInstall {
		matches := syncedRepos.PackageMatches([]pkg.Package{currentPack})
		if len(matches) == 0 {
			return errors.New("Failed matching solutions against repository for " + currentPack.HumanReadableString() + " where are definitions coming from?!")
		}
	A:
		for _, artefact := range matches[0].Repo.GetIndex() {
			if artefact.GetCompileSpec().GetPackage() == nil {
				return errors.New("Package in compilespec empty")

			}
			if matches[0].Package.Matches(artefact.GetCompileSpec().GetPackage()) {
				// Filter out already installed
				if _, err := s.Database.FindPackage(currentPack); err != nil {
					toInstall[currentPack.GetFingerPrint()] = ArtifactMatch{Package: currentPack, Artifact: artefact, Repository: matches[0].Repo}
				}
				break A
			}
		}
	}
	// Install packages into rootfs in parallel.
	all := make(chan ArtifactMatch)

	var wg = new(sync.WaitGroup)

	if !downloadOnly {
		// Download first
		for i := 0; i < l.Options.Concurrency; i++ {
			wg.Add(1)
			go l.installerWorker(i, wg, all, s, true)
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
			go l.installerWorker(i, wg, all, s, false)
		}

		for _, c := range toInstall {
			all <- c
		}
		close(all)
		wg.Wait()
	} else {
		for i := 0; i < l.Options.Concurrency; i++ {
			wg.Add(1)
			go l.installerWorker(i, wg, all, s, downloadOnly)
		}

		for _, c := range toInstall {
			all <- c
		}
		close(all)
		wg.Wait()
	}

	if downloadOnly {
		return nil
	}

	for _, c := range toInstall {
		// Annotate to the system that the package was installed
		_, err := s.Database.CreatePackage(c.Package)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Failed creating package")
		}
	}
	executedFinalizer := map[string]bool{}

	// TODO: Lower those errors as warning
	for _, w := range p {
		// Finalizers needs to run in order and in sequence.
		ordered := solution.Order(allRepos, w.GetFingerPrint())
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
						err = finalizer.RunInstall()
						if err != nil && !l.Options.Force {
							return errors.Wrap(err, "Error executing install finalizer "+treePackage.Rel(tree.FinalizerFile))
						}
						executedFinalizer[ass.Package.GetFingerPrint()] = true
					}
				}
			}
		}

	}
	return nil

}

func (l *LuetInstaller) installPackage(a ArtifactMatch, s *System, downloadOnly bool) error {

	artifact, err := a.Repository.Client().DownloadArtifact(a.Artifact)
	if err != nil {
		return errors.Wrap(err, "Error on download artifact")
	}

	err = artifact.Verify()
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Artifact integrity check failure")
	}
	if downloadOnly {
		return nil
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

func (l *LuetInstaller) installerWorker(i int, wg *sync.WaitGroup, c <-chan ArtifactMatch, s *System, downloadOnly bool) error {
	defer wg.Done()

	for p := range c {
		// TODO: Keep trace of what was added from the tar, and save it into system
		err := l.installPackage(p, s, downloadOnly)
		if err != nil && !l.Options.Force {
			//TODO: Uninstall, rollback.
			Fatal("Failed installing package "+p.Package.GetName(), err.Error())
			return errors.Wrap(err, "Failed installing package "+p.Package.GetName())
		}
		if err == nil && downloadOnly {
			Info(":package: ", p.Package.HumanReadableString(), "downloaded")
		} else if err == nil {
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
	// compute uninstall from all world - remove packages in parallel - run uninstall finalizer (in order) TODO - mark the uninstallation in db
	// Get installed definition

	checkConflicts := true
	if l.Options.Force == true {
		checkConflicts = false
	}

	if !l.Options.NoDeps {
		solv := solver.NewResolver(s.Database, s.Database, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
		solution, err := solv.Uninstall(p, checkConflicts)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Could not solve the uninstall constraints. Tip: try with --solver-type qlearning or with --force, or by removing packages excluding their dependencies with --nodeps")
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
		Info(":package: ", p.HumanReadableString(), "uninstalled")
	}
	return nil

}

func (l *LuetInstaller) Repositories(r []Repository) { l.PackageRepositories = r }
