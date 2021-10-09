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

	"github.com/mudler/luet/pkg/bus"
	artifact "github.com/mudler/luet/pkg/compiler/types/artifact"
	"github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/mudler/luet/pkg/helpers/match"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"

	. "github.com/logrusorgru/aurora"
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
	Ask                                                            bool
	DownloadOnly                                                   bool
	Relaxed                                                        bool
}

type LuetInstaller struct {
	PackageRepositories Repositories

	Options LuetInstallerOptions
}

type ArtifactMatch struct {
	Package    pkg.Package
	Artifact   *artifact.PackageArtifact
	Repository *LuetSystemRepository
}

func NewLuetInstaller(opts LuetInstallerOptions) *LuetInstaller {
	return &LuetInstaller{Options: opts}
}

// computeUpgrade returns the packages to be uninstalled and installed in a system to perform an upgrade
// based on the system repositories
func (l *LuetInstaller) computeUpgrade(syncedRepos Repositories, s *System) (pkg.Packages, pkg.Packages, error) {
	toInstall := pkg.Packages{}
	var uninstall pkg.Packages
	var err error
	// First match packages against repositories by priority
	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)
	// compute a "big" world
	solv := solver.NewResolver(solver.Options{Type: l.Options.SolverOptions.Implementation, Concurrency: l.Options.Concurrency}, s.Database, allRepos, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
	var solution solver.PackagesAssertions

	if l.Options.SolverUpgrade {
		uninstall, solution, err = solv.UpgradeUniverse(l.Options.RemoveUnavailableOnUpgrade)
		if err != nil {
			return uninstall, toInstall, errors.Wrap(err, "Failed solving solution for upgrade")
		}
	} else {
		uninstall, solution, err = solv.Upgrade(l.Options.FullUninstall, true)
		if err != nil {
			return uninstall, toInstall, errors.Wrap(err, "Failed solving solution for upgrade")
		}
	}

	for _, assertion := range solution {
		// Be sure to filter from solutions packages already installed in the system
		if _, err := s.Database.FindPackage(assertion.Package); err != nil && assertion.Value {
			toInstall = append(toInstall, assertion.Package)
		}
	}

	if l.Options.UpgradeNewRevisions {
		for _, p := range s.Database.World() {
			matches := syncedRepos.PackageMatches(pkg.Packages{p})
			if len(matches) == 0 {
				// Package missing. the user should run luet upgrade --universe
				continue
			}
			for _, artefact := range matches[0].Repo.GetIndex() {
				if artefact.CompileSpec.GetPackage() == nil {
					return uninstall, toInstall, errors.New("Package in compilespec empty")

				}
				if artefact.CompileSpec.GetPackage().Matches(p) && artefact.CompileSpec.GetPackage().GetBuildTimestamp() != p.GetBuildTimestamp() {
					toInstall = append(toInstall, matches[0].Package).Unique()
					uninstall = append(uninstall, p).Unique()
				}
			}
		}
	}

	return uninstall, toInstall, nil
}

func packsToList(p pkg.Packages) string {
	var packs []string

	for _, pp := range p {
		packs = append(packs, pp.HumanReadableString())
	}

	sort.Strings(packs)
	return strings.Join(packs, " ")
}

func matchesToList(artefacts map[string]ArtifactMatch) string {
	var packs []string

	for fingerprint, match := range artefacts {
		packs = append(packs, fmt.Sprintf("%s (%s)", fingerprint, match.Repository.GetName()))
	}
	sort.Strings(packs)
	return strings.Join(packs, " ")
}

// Upgrade upgrades a System based on the Installer options. Returns error in case of failure
func (l *LuetInstaller) Upgrade(s *System) error {

	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}

	Info(":thinking: Computing upgrade, please hang tight... :zzz:")
	if l.Options.UpgradeNewRevisions {
		Info(":memo: note: will consider new build revisions while upgrading")
	}

	return l.checkAndUpgrade(syncedRepos, s)
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

	toRemoveFinal := pkg.Packages{}
	for _, p := range toRemove {
		packs, _ := s.Database.FindPackages(p)
		if len(packs) == 0 {
			return errors.New("Package " + p.HumanReadableString() + " not found in the system")
		}
		for _, pp := range packs {
			toRemoveFinal = append(toRemoveFinal, pp)
		}
	}
	o := Option{
		FullUninstall:      false,
		Force:              true,
		CheckConflicts:     false,
		FullCleanUninstall: false,
		NoDeps:             l.Options.NoDeps,
		OnlyDeps:           false,
	}

	return l.swap(o, syncedRepos, toRemoveFinal, toInstall, s)
}

func (l *LuetInstaller) computeSwap(o Option, syncedRepos Repositories, toRemove pkg.Packages, toInstall pkg.Packages, s *System) (map[string]ArtifactMatch, pkg.Packages, solver.PackagesAssertions, pkg.PackageDatabase, error) {

	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)

	toInstall = syncedRepos.ResolveSelectors(toInstall)

	// First check what would have been done
	installedtmp, err := s.Database.Copy()
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "Failed create temporary in-memory db")
	}

	systemAfterChanges := &System{Database: installedtmp}

	packs, err := l.computeUninstall(o, systemAfterChanges, toRemove...)
	if err != nil && !o.Force {
		Error("Failed computing uninstall for ", packsToList(toRemove))
		return nil, nil, nil, nil, errors.Wrap(err, "computing uninstall "+packsToList(toRemove))
	}
	for _, p := range packs {
		err = systemAfterChanges.Database.RemovePackage(p)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed removing package from database")
		}
	}

	match, packages, assertions, allRepos, err := l.computeInstall(o, syncedRepos, toInstall, systemAfterChanges)
	for _, p := range toInstall {
		assertions = append(assertions, solver.PackageAssert{Package: p.(*pkg.DefaultPackage), Value: true})
	}
	return match, packages, assertions, allRepos, err
}

func (l *LuetInstaller) swap(o Option, syncedRepos Repositories, toRemove pkg.Packages, toInstall pkg.Packages, s *System) error {

	match, packages, assertions, allRepos, err := l.computeSwap(o, syncedRepos, toRemove, toInstall, s)
	if err != nil {
		return errors.Wrap(err, "failed computing package replacement")
	}

	if l.Options.Ask {
		if len(toRemove) > 0 {
			Info(":recycle: Packages that are going to be removed from the system:\n ", Yellow(packsToList(toRemove)).BgBlack().String())
		}

		if len(match) > 0 {
			Info("Packages that are going to be installed in the system: \n ", Green(matchesToList(match)).BgBlack().String())
		}

		Info("By going forward, you are also accepting the licenses of the packages that you are going to install in your system.")
		if Ask() {
			l.Options.Ask = false // Don't prompt anymore
		} else {
			return errors.New("Aborted by user")
		}
	}

	// First match packages against repositories by priority
	if err := l.download(syncedRepos, match); err != nil {
		return errors.Wrap(err, "Pre-downloading packages")
	}

	if err := l.checkFileconflicts(match, false, s); err != nil {
		if !l.Options.Force {
			return errors.Wrap(err, "file conflict found")
		} else {
			Warning("file conflict found", err.Error())
		}
	}

	if l.Options.DownloadOnly {
		return nil
	}

	ops := l.getOpsWithOptions(toRemove, match, Option{
		Force:              o.Force,
		NoDeps:             false,
		OnlyDeps:           o.OnlyDeps,
		RunFinalizers:      false,
		CheckFileConflicts: false,
	}, o, syncedRepos, packages, assertions, allRepos)

	err = l.runOps(ops, s)
	if err != nil {
		return errors.Wrap(err, "failed running installer options")
	}

	toFinalize, err := l.getFinalizers(allRepos, assertions, match, o.NoDeps)
	if err != nil {
		return errors.Wrap(err, "failed getting package to finalize")
	}

	return s.ExecuteFinalizers(toFinalize)
}

type Option struct {
	Force              bool
	NoDeps             bool
	CheckConflicts     bool
	FullUninstall      bool
	FullCleanUninstall bool
	OnlyDeps           bool
	RunFinalizers      bool

	CheckFileConflicts bool
}

type operation struct {
	Option  Option
	Package pkg.Package
}

type installOperation struct {
	operation
	Reposiories Repositories
	Packages    pkg.Packages
	Assertions  solver.PackagesAssertions
	Database    pkg.PackageDatabase
	Matches     map[string]ArtifactMatch
}

// installerOp is the operation that is sent to the
// upgradeWorker's channel (todo)
type installerOp struct {
	Uninstall operation
	Install   installOperation
}

func (l *LuetInstaller) runOps(ops []installerOp, s *System) error {
	all := make(chan installerOp)

	wg := new(sync.WaitGroup)

	// Do the real install
	for i := 0; i < l.Options.Concurrency; i++ {
		wg.Add(1)
		go l.installerOpWorker(i, wg, all, s)
	}

	for _, c := range ops {
		all <- c
	}
	close(all)
	wg.Wait()

	return nil
}

// TODO: use installerOpWorker in place of all the other workers.
// This one is general enough to read a list of operations and execute them.
func (l *LuetInstaller) installerOpWorker(i int, wg *sync.WaitGroup, c <-chan installerOp, s *System) error {
	defer wg.Done()

	for p := range c {
		if p.Uninstall.Package != nil {
			Debug("Replacing package inplace")
			toUninstall, uninstall, err := l.generateUninstallFn(p.Uninstall.Option, s, p.Uninstall.Package)
			if err != nil {
				Error("Failed to generate Uninstall function for" + err.Error())
				continue
				//return errors.Wrap(err, "while computing uninstall")
			}

			err = uninstall()
			if err != nil {
				Error("Failed uninstall for ", packsToList(toUninstall))
				continue
				//return errors.Wrap(err, "uninstalling "+packsToList(toUninstall))
			}
		}
		if p.Install.Package != nil {
			artMatch := p.Install.Matches[p.Install.Package.GetFingerPrint()]
			ass := p.Install.Assertions.Search(p.Install.Package.GetFingerPrint())
			packageToInstall, _ := p.Install.Packages.Find(p.Install.Package.GetPackageName())

			err := l.install(
				p.Install.Option,
				p.Install.Reposiories,
				map[string]ArtifactMatch{p.Install.Package.GetFingerPrint(): artMatch},
				pkg.Packages{packageToInstall},
				solver.PackagesAssertions{*ass},
				p.Install.Database,
				s,
			)
			if err != nil {
				Error(err)
			}
		}
	}

	return nil
}

// checks wheter we can uninstall and install in place and compose installer worker ops
func (l *LuetInstaller) getOpsWithOptions(
	toUninstall pkg.Packages, installMatch map[string]ArtifactMatch, installOpt, uninstallOpt Option,
	syncedRepos Repositories, toInstall pkg.Packages, solution solver.PackagesAssertions, allRepos pkg.PackageDatabase) []installerOp {
	resOps := []installerOp{}
	for _, match := range installMatch {
		if pack, err := toUninstall.Find(match.Package.GetPackageName()); err == nil {
			resOps = append(resOps, installerOp{
				Uninstall: operation{Package: pack, Option: uninstallOpt},
				Install: installOperation{
					operation: operation{
						Package: match.Package,
						Option:  installOpt,
					},
					Matches:     installMatch,
					Packages:    toInstall,
					Reposiories: syncedRepos,
					Assertions:  solution,
					Database:    allRepos,
				},
			})
		} else {
			resOps = append(resOps, installerOp{
				Install: installOperation{
					operation:   operation{Package: match.Package, Option: installOpt},
					Matches:     installMatch,
					Reposiories: syncedRepos,
					Packages:    toInstall,
					Assertions:  solution,
					Database:    allRepos,
				},
			})
		}
	}

	for _, p := range toUninstall {
		found := false

		for _, match := range installMatch {
			if match.Package.GetPackageName() == p.GetPackageName() {
				found = true
			}

		}
		if !found {
			resOps = append(resOps, installerOp{
				Uninstall: operation{Package: p, Option: uninstallOpt},
			})
		}
	}
	return resOps
}

func (l *LuetInstaller) checkAndUpgrade(r Repositories, s *System) error {
	Spinner(32)
	uninstall, toInstall, err := l.computeUpgrade(r, s)
	if err != nil {
		return errors.Wrap(err, "failed computing upgrade")
	}
	SpinnerStop()

	if len(uninstall) > 0 {
		Info(":recycle: Packages that are going to be removed from the system:\n ", Yellow(packsToList(uninstall)).BgBlack().String())
	}

	if len(toInstall) > 0 {
		Info(":zap:Packages that are going to be installed in the system:\n ", Green(packsToList(toInstall)).BgBlack().String())
	}

	if len(toInstall) == 0 && len(uninstall) == 0 {
		Info("Nothing to do")
		return nil
	}

	// We don't want any conflict with the installed to raise during the upgrade.
	// In this way we both force uninstalls and we avoid to check with conflicts
	// against the current system state which is pending to deletion
	// E.g. you can't check for conflicts for an upgrade of a new version of A
	// if the old A results installed in the system. This is due to the fact that
	// now the solver enforces the constraints and explictly denies two packages
	// of the same version installed.
	o := Option{
		FullUninstall:      false,
		Force:              true,
		CheckConflicts:     false,
		FullCleanUninstall: false,
		NoDeps:             true,
		OnlyDeps:           false,
	}

	if l.Options.Ask {
		Info("By going forward, you are also accepting the licenses of the packages that you are going to install in your system.")
		if Ask() {
			l.Options.Ask = false // Don't prompt anymore
			return l.swap(o, r, uninstall, toInstall, s)
		} else {
			return errors.New("Aborted by user")
		}
	}

	return l.swap(o, r, uninstall, toInstall, s)
}

func (l *LuetInstaller) Install(cp pkg.Packages, s *System) error {
	syncedRepos, err := l.SyncRepositories(true)
	if err != nil {
		return err
	}

	if len(s.Database.World()) > 0 && !l.Options.Relaxed {
		Info(":thinking: Checking for available upgrades")
		if err := l.checkAndUpgrade(syncedRepos, s); err != nil {
			return errors.Wrap(err, "while checking upgrades before install")
		}
	}

	o := Option{
		NoDeps:             l.Options.NoDeps,
		Force:              l.Options.Force,
		OnlyDeps:           l.Options.OnlyDeps,
		CheckFileConflicts: true,
		RunFinalizers:      true,
	}
	match, packages, assertions, allRepos, err := l.computeInstall(o, syncedRepos, cp, s)
	if err != nil {
		return err
	}

	// Check if we have to process something, or return to the user an error
	if len(match) == 0 {
		Info("No packages to install")
		return nil
	}
	// Resolvers might decide to remove some packages from being installed
	if !l.Options.SolverOptions.ResolverIsSet() {
		for _, p := range cp {
			found := false
			vers, _ := s.Database.FindPackageVersions(p) // If was installed, it is found, as it was filtered
			if len(vers) >= 1 {
				found = true
				continue
			}

			for _, m := range match {
				if m.Package.GetName() == p.GetName() {
					found = true
				}
				for _, pack := range m.Package.GetProvides() {
					if pack.GetName() == p.GetName() {
						found = true
					}
				}
			}

			if !found {
				return fmt.Errorf("Package '%s' not found", p.HumanReadableString())
			}
		}
	}
	Info("Packages that are going to be installed in the system: \n ", Green(matchesToList(match)).BgBlack().String())

	if l.Options.Ask {
		Info("By going forward, you are also accepting the licenses of the packages that you are going to install in your system.")
		if Ask() {
			l.Options.Ask = false // Don't prompt anymore
			return l.install(o, syncedRepos, match, packages, assertions, allRepos, s)
		} else {
			return errors.New("Aborted by user")
		}
	}
	return l.install(o, syncedRepos, match, packages, assertions, allRepos, s)
}

func (l *LuetInstaller) download(syncedRepos Repositories, toDownload map[string]ArtifactMatch) error {

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
				artefact.CompileSpec.GetPackage().HumanReadableString(),
				"from", repo.GetName(), "is installed")
		FILES:
			for _, f := range artefact.Files {
				if fileHelper.Exists(filepath.Join(s.Target, f)) {
					p, err := repo.GetTree().GetDatabase().FindPackage(artefact.CompileSpec.GetPackage())
					if err != nil {
						return err
					}
					Info(":mag: Found package:", p.HumanReadableString())
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
		s.Database.SetPackageFiles(&pkg.PackageFile{PackageFingerprint: pack.GetFingerPrint(), Files: match.Artifact.Files})
		Info(":zap:Reclaimed package:", pack.HumanReadableString())
	}
	Info("Done!")

	return nil
}

func (l *LuetInstaller) computeInstall(o Option, syncedRepos Repositories, cp pkg.Packages, s *System) (map[string]ArtifactMatch, pkg.Packages, solver.PackagesAssertions, pkg.PackageDatabase, error) {
	var p pkg.Packages
	toInstall := map[string]ArtifactMatch{}
	allRepos := pkg.NewInMemoryDatabase(false)
	var solution solver.PackagesAssertions

	// Check if the package is installed first
	for _, pi := range cp {
		vers, _ := s.Database.FindPackageVersions(pi)

		if len(vers) >= 1 {
			//	Warning("Filtering out package " + pi.HumanReadableString() + ", it has other versions already installed. Uninstall one of them first ")
			continue
			//return errors.New("Package " + pi.GetFingerPrint() + " has other versions already installed. Uninstall one of them first: " + strings.Join(vers, " "))

		}
		p = append(p, pi)
	}

	if len(p) == 0 {
		return toInstall, p, solution, allRepos, nil
	}
	// First get metas from all repos (and decodes trees)

	// First match packages against repositories by priority
	//	matches := syncedRepos.PackageMatches(p)

	// compute a "big" world
	syncedRepos.SyncDatabase(allRepos)
	p = syncedRepos.ResolveSelectors(p)
	var packagesToInstall pkg.Packages
	var err error

	if !o.NoDeps {
		solv := solver.NewResolver(solver.Options{Type: l.Options.SolverOptions.Implementation, Concurrency: l.Options.Concurrency}, s.Database, allRepos, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())

		if l.Options.Relaxed {
			solution, err = solv.RelaxedInstall(p)
		} else {
			solution, err = solv.Install(p)
		}
		/// TODO: PackageAssertions needs to be a map[fingerprint]pack so lookup is in O(1)
		if err != nil && !o.Force {
			return toInstall, p, solution, allRepos, errors.Wrap(err, "Failed solving solution for package")
		}
		// Gathers things to install
		for _, assertion := range solution {
			if assertion.Value {
				if _, err := s.Database.FindPackage(assertion.Package); err == nil {
					// skip matching if it is installed already
					continue
				}
				packagesToInstall = append(packagesToInstall, assertion.Package)
			}
		}
	} else if !o.OnlyDeps {
		for _, currentPack := range p {
			if _, err := s.Database.FindPackage(currentPack); err == nil {
				// skip matching if it is installed already
				continue
			}
			packagesToInstall = append(packagesToInstall, currentPack)
		}
	}
	// Gathers things to install
	for _, currentPack := range packagesToInstall {
		// Check if package is already installed.

		matches := syncedRepos.PackageMatches(pkg.Packages{currentPack})
		if len(matches) == 0 {
			return toInstall, p, solution, allRepos, errors.New("Failed matching solutions against repository for " + currentPack.HumanReadableString() + " where are definitions coming from?!")
		}
	A:
		for _, artefact := range matches[0].Repo.GetIndex() {
			if artefact.CompileSpec.GetPackage() == nil {
				return toInstall, p, solution, allRepos, errors.New("Package in compilespec empty")
			}
			if matches[0].Package.Matches(artefact.CompileSpec.GetPackage()) {
				currentPack.SetBuildTimestamp(artefact.CompileSpec.GetPackage().GetBuildTimestamp())
				// Filter out already installed
				if _, err := s.Database.FindPackage(currentPack); err != nil {
					toInstall[currentPack.GetFingerPrint()] = ArtifactMatch{Package: currentPack, Artifact: artefact, Repository: matches[0].Repo}
				}
				break A
			}
		}
	}
	return toInstall, p, solution, allRepos, nil
}

func (l *LuetInstaller) getFinalizers(allRepos pkg.PackageDatabase, solution solver.PackagesAssertions, toInstall map[string]ArtifactMatch, nodeps bool) ([]pkg.Package, error) {
	var toFinalize []pkg.Package
	if !nodeps {
		// TODO: Lower those errors as warning
		for _, w := range toInstall {
			// Finalizers needs to run in order and in sequence.
			ordered, err := solution.Order(allRepos, w.Package.GetFingerPrint())
			if err != nil {
				return toFinalize, errors.Wrap(err, "While order a solution for "+w.Package.HumanReadableString())
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
						return toFinalize, errors.Wrap(err, "Error getting package "+ass.Package.HumanReadableString())
					}

					toFinalize = append(toFinalize, treePackage)
				}
			}

		}
	} else {
		for _, c := range toInstall {
			treePackage, err := c.Repository.GetTree().GetDatabase().FindPackage(c.Package)
			if err != nil {
				return toFinalize, errors.Wrap(err, "Error getting package "+c.Package.HumanReadableString())
			}
			toFinalize = append(toFinalize, treePackage)
		}
	}
	return toFinalize, nil
}

func (l *LuetInstaller) checkFileconflicts(toInstall map[string]ArtifactMatch, checkSystem bool, s *System) error {
	Info("Checking for file conflicts..")
	defer s.Clean() // Release memory

	filesToInstall := []string{}
	for _, m := range toInstall {
		a, err := l.downloadPackage(m)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Failed downloading package")
		}
		files, err := a.FileList()
		if err != nil && !l.Options.Force {
			return errors.Wrapf(err, "Could not get filelist for %s", a.CompileSpec.Package.HumanReadableString())
		}

		for _, f := range files {
			if helpers.Contains(filesToInstall, f) {
				return fmt.Errorf(
					"file conflict between packages to be installed",
				)
			}
			if checkSystem {
				exists, p, err := s.ExistsPackageFile(f)
				if err != nil {
					return errors.Wrap(err, "failed checking into system db")
				}
				if exists {
					return fmt.Errorf(
						"file conflict between '%s' and '%s' ( file: %s )",
						p.HumanReadableString(),
						m.Package.HumanReadableString(),
						f,
					)
				}
			}
		}
		filesToInstall = append(filesToInstall, files...)
	}

	return nil
}

func (l *LuetInstaller) install(o Option, syncedRepos Repositories, toInstall map[string]ArtifactMatch, p pkg.Packages, solution solver.PackagesAssertions, allRepos pkg.PackageDatabase, s *System) error {

	// Download packages in parallel first
	if err := l.download(syncedRepos, toInstall); err != nil {
		return errors.Wrap(err, "Downloading packages")
	}

	if o.CheckFileConflicts {
		// Check file conflicts
		if err := l.checkFileconflicts(toInstall, true, s); err != nil {
			if !l.Options.Force {
				return errors.Wrap(err, "file conflict found")
			} else {
				Warning("file conflict found", err.Error())
			}
		}
	}

	if l.Options.DownloadOnly {
		return nil
	}

	all := make(chan ArtifactMatch)

	wg := new(sync.WaitGroup)

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
		if err != nil && !o.Force {
			return errors.Wrap(err, "Failed creating package")
		}
		bus.Manager.Publish(bus.EventPackageInstall, c)
	}

	if !o.RunFinalizers {
		return nil
	}

	toFinalize, err := l.getFinalizers(allRepos, solution, toInstall, o.NoDeps)
	if err != nil {
		return errors.Wrap(err, "failed getting package to finalize")
	}

	return s.ExecuteFinalizers(toFinalize)
}

func (l *LuetInstaller) downloadPackage(a ArtifactMatch) (*artifact.PackageArtifact, error) {

	artifact, err := a.Repository.Client().DownloadArtifact(a.Artifact)
	if err != nil {
		return nil, errors.Wrap(err, "Error on download artifact")
	}

	err = artifact.Verify()
	if err != nil {
		return nil, errors.Wrap(err, "Artifact integrity check failure")
	}
	return artifact, nil
}

func (l *LuetInstaller) installPackage(m ArtifactMatch, s *System) error {

	a, err := l.downloadPackage(m)
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Failed downloading package")
	}

	files, err := a.FileList()
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Could not open package archive")
	}

	err = a.Unpack(s.Target, true)
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Error met while unpacking rootfs")
	}

	// First create client and download
	// Then unpack to system
	return s.Database.SetPackageFiles(&pkg.PackageFile{PackageFingerprint: m.Package.GetFingerPrint(), Files: files})
}

func (l *LuetInstaller) downloadWorker(i int, wg *sync.WaitGroup, c <-chan ArtifactMatch) error {
	defer wg.Done()

	for p := range c {
		// TODO: Keep trace of what was added from the tar, and save it into system
		_, err := l.downloadPackage(p)
		if err != nil {
			Fatal("Failed downloading package "+p.Package.GetName(), err.Error())
			return errors.Wrap(err, "Failed downloading package "+p.Package.GetName())
		} else {
			Info(":package: Package ", p.Package.HumanReadableString(), "downloaded")
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
			Info(":package: Package ", p.Package.HumanReadableString(), "installed")
		} else if err != nil && l.Options.Force {
			Info(":package: Package ", p.Package.HumanReadableString(), "installed with failures (forced install)")
		}
	}

	return nil
}

func checkAndPrunePath(path string) {
	// check if now the target path is empty
	targetPath := filepath.Dir(path)

	fi, err := os.Lstat(targetPath)
	if err != nil {
		//	Warning("Dir not found (it was before?) ", err.Error())
		return
	}

	switch mode := fi.Mode(); {
	case mode.IsDir():
		files, err := ioutil.ReadDir(targetPath)
		if err != nil {
			Warning("Failed reading folder", targetPath, err.Error())
		}
		if len(files) != 0 {
			Debug("Preserving not-empty folder", targetPath)
			return
		}
	}
	if err = os.Remove(targetPath); err != nil {
		Warning("Failed removing file (maybe not present in the system target anymore ?)", targetPath, err.Error())
	}
}

// We will try to cleanup every path from the file, if the folders left behind are empty
func pruneEmptyFilePath(path string) {
	checkAndPrunePath(path)

	// A path is for e.g. /usr/bin/bar
	// we want to create an array as "/usr", "/usr/bin", "/usr/bin/bar"
	paths := strings.Split(path, string(os.PathSeparator))
	currentPath := filepath.Join(string(os.PathSeparator), paths[0])
	allPaths := []string{currentPath}
	for _, p := range paths[1:] {
		currentPath = filepath.Join(currentPath, p)
		allPaths = append(allPaths, currentPath)
	}
	match.ReverseAny(allPaths)
	for _, p := range allPaths {
		checkAndPrunePath(p)
	}
}

func (l *LuetInstaller) uninstall(p pkg.Package, s *System) error {
	var cp *config.ConfigProtect
	annotationDir := ""

	files, err := s.Database.GetPackageFiles(p)
	if err != nil {
		return errors.Wrap(err, "Failed getting installed files")
	}

	if !config.LuetCfg.ConfigProtectSkip {

		if p.HasAnnotation(string(pkg.ConfigProtectAnnnotation)) {
			dir, ok := p.GetAnnotations()[string(pkg.ConfigProtectAnnnotation)]
			if ok {
				annotationDir = dir
			}
		}

		cp = config.NewConfigProtect(annotationDir)
		cp.Map(files)
	}

	toRemove, notPresent := fileHelper.OrderFiles(s.Target, files)

	// Remove from target
	for _, f := range toRemove {
		target := filepath.Join(s.Target, f)

		if !config.LuetCfg.ConfigProtectSkip && cp.Protected(f) {
			Debug("Preserving protected file:", f)
			continue
		}

		Debug("Removing", target)
		if l.Options.PreserveSystemEssentialData &&
			strings.HasPrefix(f, config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath()) ||
			strings.HasPrefix(f, config.LuetCfg.GetSystem().GetSystemRepoDatabaseDirPath()) {
			Warning("Preserve ", f, " which is required by luet ( you have to delete it manually if you really need to)")
			continue
		}

		fi, err := os.Lstat(target)
		if err != nil {
			Warning("File not found (it was before?) ", err.Error())
			continue
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			files, err := ioutil.ReadDir(target)
			if err != nil {
				Warning("Failed reading folder", target, err.Error())
			}
			if len(files) != 0 {
				Debug("Preserving not-empty folder", target)
				continue
			}
		}

		if err = os.Remove(target); err != nil {
			Warning("Failed removing file (maybe not present in the system target anymore ?)", target, err.Error())
		}

		pruneEmptyFilePath(target)
	}

	for _, f := range notPresent {
		target := filepath.Join(s.Target, f)

		if !config.LuetCfg.ConfigProtectSkip && cp.Protected(f) {
			Debug("Preserving protected file:", f)
			continue
		}

		if err = os.Remove(target); err != nil {
			Debug("Failed removing file (not present in the system target)", target, err.Error())
		}

		pruneEmptyFilePath(target)
	}

	err = s.Database.RemovePackageFiles(p)
	if err != nil {
		return errors.Wrap(err, "Failed removing package files from database")
	}
	err = s.Database.RemovePackage(p)
	if err != nil {
		return errors.Wrap(err, "Failed removing package from database")
	}

	bus.Manager.Publish(bus.EventPackageUnInstall, p)

	Info(":recycle: ", p.GetFingerPrint(), "Removed :heavy_check_mark:")
	return nil
}

func (l *LuetInstaller) computeUninstall(o Option, s *System, packs ...pkg.Package) (pkg.Packages, error) {

	var toUninstall pkg.Packages
	// compute uninstall from all world - remove packages in parallel - run uninstall finalizer (in order) TODO - mark the uninstallation in db
	// Get installed definition
	checkConflicts := o.CheckConflicts
	full := o.FullUninstall
	// if o.Force == true { // IF forced, we want to remove the package and all its requires
	// 	checkConflicts = false
	// 	full = false
	// }

	// Create a temporary DB with the installed packages
	// so the solver is much faster finding the deptree
	// First check what would have been done
	installedtmp, err := s.Database.Copy()
	if err != nil {
		return toUninstall, errors.Wrap(err, "Failed create temporary in-memory db")
	}

	if !o.NoDeps {
		solv := solver.NewResolver(solver.Options{Type: l.Options.SolverOptions.Implementation, Concurrency: l.Options.Concurrency}, installedtmp, installedtmp, pkg.NewInMemoryDatabase(false), l.Options.SolverOptions.Resolver())
		var solution pkg.Packages
		var err error
		if o.FullCleanUninstall {
			solution, err = solv.UninstallUniverse(packs)
			if err != nil {
				return toUninstall, errors.Wrap(err, "Could not solve the uninstall constraints. Tip: try with --solver-type qlearning or with --force, or by removing packages excluding their dependencies with --nodeps")
			}
		} else {
			solution, err = solv.Uninstall(checkConflicts, full, packs...)
			if err != nil && !l.Options.Force {
				return toUninstall, errors.Wrap(err, "Could not solve the uninstall constraints. Tip: try with --solver-type qlearning or with --force, or by removing packages excluding their dependencies with --nodeps")
			}
		}

		for _, p := range solution {
			toUninstall = append(toUninstall, p)
		}
	} else {
		toUninstall = append(toUninstall, packs...)
	}

	return toUninstall, nil
}

func (l *LuetInstaller) generateUninstallFn(o Option, s *System, packs ...pkg.Package) (pkg.Packages, func() error, error) {
	for _, p := range packs {
		if packs, _ := s.Database.FindPackages(p); len(packs) == 0 {
			return nil, nil, errors.New(fmt.Sprintf("Package %s not found in the system", p.HumanReadableString()))
		}
	}

	toUninstall, err := l.computeUninstall(o, s, packs...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "while computing uninstall")
	}

	uninstall := func() error {
		for _, p := range toUninstall {
			err := l.uninstall(p, s)
			if err != nil && !o.Force {
				return errors.Wrap(err, "Uninstall failed")
			}
		}
		return nil
	}

	return toUninstall, uninstall, nil
}

func (l *LuetInstaller) Uninstall(s *System, packs ...pkg.Package) error {

	Spinner(32)
	o := Option{
		FullUninstall:      l.Options.FullUninstall,
		Force:              l.Options.Force,
		CheckConflicts:     l.Options.CheckConflicts,
		FullCleanUninstall: l.Options.FullCleanUninstall,
	}
	toUninstall, uninstall, err := l.generateUninstallFn(o, s, packs...)
	if err != nil {
		return errors.Wrap(err, "while computing uninstall")
	}
	SpinnerStop()

	if len(toUninstall) == 0 {
		Info("Nothing to do")
		return nil
	}

	if l.Options.Ask {
		Info(":recycle: Packages that are going to be removed from the system:\n   ", Yellow(packsToList(toUninstall)).BgBlack().String())
		if Ask() {
			l.Options.Ask = false // Don't prompt anymore
			return uninstall()
		} else {
			return errors.New("Aborted by user")
		}
	}
	return uninstall()
}

func (l *LuetInstaller) Repositories(r []*LuetSystemRepository) { l.PackageRepositories = r }
