package installer

import (
	pkg "github.com/mudler/luet/pkg/package"
)

type System struct {
	Database pkg.PackageDatabase
	Target   string
}

func (s *System) World() ([]pkg.Package, error) {
	return s.Database.World(), nil
}
