// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>,
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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

package version

import (
	"errors"
	"sort"
	"strings"

	semver "github.com/hashicorp/go-version"
	debversion "github.com/knqyf263/go-deb-version"
)

// WrappedVersioner uses different means to return unique result that is understendable by Luet
// It tries different approaches to sort, validate, and sanitize to a common versioning format
// that is understendable by the whole code
type WrappedVersioner struct{}

func (w *WrappedVersioner) Validate(version string) error {
	if !debversion.Valid(version) {
		return errors.New("Invalid version")
	}
	return nil
}

func (w *WrappedVersioner) Sanitize(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

func (w *WrappedVersioner) Sort(toSort []string) []string {
	if len(toSort) == 0 {
		return toSort
	}
	var versionsMap map[string]string = make(map[string]string)
	versionsRaw := []string{}
	result := []string{}
	for _, v := range toSort {
		sanitizedVersion := w.Sanitize(v)
		versionsMap[v] = sanitizedVersion
		versionsRaw = append(versionsRaw, sanitizedVersion)
	}

	versions := make([]*semver.Version, len(versionsRaw))

	for i, raw := range versionsRaw {
		v, _ := semver.NewVersion(raw)
		versions[i] = v
	}

	// Try first semver sorting
	sort.Sort(semver.Collection(versions))
	if len(versions) > 0 {
		for _, v := range versions {
			result = append(result, versionsMap[v.Original()])

		}
		return result
	}

	// Try with debian sorting
	vs := make([]debversion.Version, len(versionsRaw))
	for i, r := range versionsRaw {
		v, _ := debversion.NewVersion(r)
		vs[i] = v
	}

	sort.Slice(vs, func(i, j int) bool {
		return vs[i].LessThan(vs[j])
	})
	for _, v := range vs {
		result = append(result, versionsMap[v.String()])
	}
	return result
}
