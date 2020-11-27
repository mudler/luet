package installer

import (
	. "github.com/mudler/luet/pkg/logger"

	"github.com/mudler/luet/pkg/helpers"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	"github.com/pkg/errors"
)

type System struct {
	Database pkg.PackageDatabase
	Target   string
}

func (s *System) World() (pkg.Packages, error) {
	return s.Database.World(), nil
}

type templatedata map[string]interface{}

func (s *System) ExecuteFinalizers(packs []pkg.Package, force bool) error {
	executedFinalizer := map[string]bool{}
	for _, p := range packs {
		if helpers.Exists(p.Rel(tree.FinalizerFile)) {
			out, err := helpers.RenderFiles(p.Rel(tree.FinalizerFile), p.Rel(tree.DefinitionFile), "")
			if err != nil && !force {
				return errors.Wrap(err, "reading file "+p.Rel(tree.FinalizerFile))
			}

			if _, exists := executedFinalizer[p.GetFingerPrint()]; !exists {
				Info("Executing finalizer for " + p.HumanReadableString())
				finalizer, err := NewLuetFinalizerFromYaml([]byte(out))
				if err != nil && !force {
					return errors.Wrap(err, "Error reading finalizer "+p.Rel(tree.FinalizerFile))
				}
				err = finalizer.RunInstall(s)
				if err != nil && !force {
					return errors.Wrap(err, "Error executing install finalizer "+p.Rel(tree.FinalizerFile))
				}
				executedFinalizer[p.GetFingerPrint()] = true
			}
		}
	}
	return nil
}
