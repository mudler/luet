package installer

import (
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
)

type System struct {
	Database pkg.PackageDatabase
	Target   string
}

func (s *System) World() ([]pkg.Package, error) {
	t := tree.NewDefaultTree()
	t.SetPackageSet(s.Database)
	return t.World()
}
