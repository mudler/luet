package installer

import (
	"github.com/hashicorp/go-multierror"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
)

type System struct {
	Database pkg.PackageDatabase
	Target   string
}

func (s *System) World() (pkg.Packages, error) {
	return s.Database.World(), nil
}

type templatedata map[string]interface{}

func (s *System) ExecuteFinalizers(packs []pkg.Package) error {
	var errs error
	executedFinalizer := map[string]bool{}
	for _, p := range packs {
		if helpers.Exists(p.Rel(tree.FinalizerFile)) {
			out, err := helpers.RenderFiles(p.Rel(tree.FinalizerFile), p.Rel(tree.DefinitionFile), "")
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
