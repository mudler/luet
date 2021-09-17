package installer

import (
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/luet/pkg/helpers"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
)

type System struct {
	Database  pkg.PackageDatabase
	Target    string
	fileIndex map[string]pkg.Package
	sync.Mutex
}

func (s *System) World() (pkg.Packages, error) {
	return s.Database.World(), nil
}

type templatedata map[string]interface{}

func (s *System) ExecuteFinalizers(packs []pkg.Package) error {
	var errs error
	executedFinalizer := map[string]bool{}
	for _, p := range packs {
		if fileHelper.Exists(p.Rel(tree.FinalizerFile)) {
			out, err := helpers.RenderFiles(helpers.ChartFile(p.Rel(tree.FinalizerFile)), p.Rel(pkg.PackageDefinitionFile))
			if err != nil {
				Warning("Failed rendering finalizer for ", p.HumanReadableString(), err.Error())
				errs = multierror.Append(errs, err)
				continue
			}

			if _, exists := executedFinalizer[p.GetFingerPrint()]; !exists {
				executedFinalizer[p.GetFingerPrint()] = true
				Info("Executing finalizer for " + p.HumanReadableString())
				finalizer, err := NewLuetFinalizerFromYaml([]byte(out))
				if err != nil {
					Warning("Failed reading finalizer for ", p.HumanReadableString(), err.Error())
					errs = multierror.Append(errs, err)
					continue
				}
				err = finalizer.RunInstall(s)
				if err != nil {
					Warning("Failed running finalizer for ", p.HumanReadableString(), err.Error())
					errs = multierror.Append(errs, err)
					continue
				}
			}
		}
	}
	return errs
}

func (s *System) buildFileIndex() {
	s.Lock()
	defer s.Unlock()
	// Check if cache is empty or if it got modified
	if s.fileIndex == nil { //|| len(s.Database.GetPackages()) != len(s.fileIndex) {
		s.fileIndex = make(map[string]pkg.Package)
		for _, p := range s.Database.World() {
			files, _ := s.Database.GetPackageFiles(p)
			for _, f := range files {
				s.fileIndex[f] = p
			}
		}
	}
}

func (s *System) Clean() {
	s.Lock()
	defer s.Unlock()
	s.fileIndex = nil
}

func (s *System) ExistsPackageFile(file string) (bool, pkg.Package, error) {
	Debug("Checking if file ", file, "belongs to any package")
	s.buildFileIndex()
	s.Lock()
	defer s.Unlock()
	if p, exists := s.fileIndex[file]; exists {
		Debug(file, "belongs already to", p.HumanReadableString())

		return exists, p, nil
	}
	Debug(file, "doesn't belong to any package")
	return false, nil, nil
}
