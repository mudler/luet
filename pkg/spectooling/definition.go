// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>,
//                       Daniele Rondina <geaaru@sabayonlinux.org>
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

package spectooling

import (
	pkg "github.com/mudler/luet/pkg/package"

	"gopkg.in/yaml.v2"
)

type DefaultPackageSanitized struct {
	Name             string                     `json:"name" yaml:"name"`
	Version          string                     `json:"version" yaml:"version"`
	Category         string                     `json:"category" yaml:"category"`
	UseFlags         []string                   `json:"use_flags,omitempty" yaml:"use_flags,omitempty"`
	PackageRequires  []*DefaultPackageSanitized `json:"requires,omitempty" yaml:"requires,omitempty"`
	PackageConflicts []*DefaultPackageSanitized `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
	Provides         []*DefaultPackageSanitized `json:"provides,omitempty" yaml:"provides,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

	// Path is set only internally when tree is loaded from disk
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Uri         []string `json:"uri,omitempty" yaml:"uri,omitempty"`
	License     string   `json:"license,omitempty" yaml:"license,omitempty"`
	Hidden      bool     `json:"hidden,omitempty" yaml:"hidden,omitempty"`

	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

func NewDefaultPackageSanitizedFromYaml(data []byte) (*DefaultPackageSanitized, error) {
	ans := &DefaultPackageSanitized{}
	if err := yaml.Unmarshal(data, ans); err != nil {
		return nil, err
	}
	return ans, nil
}

func NewDefaultPackageSanitized(p pkg.Package) *DefaultPackageSanitized {
	ans := &DefaultPackageSanitized{
		Name:        p.GetName(),
		Version:     p.GetVersion(),
		Category:    p.GetCategory(),
		UseFlags:    p.GetUses(),
		Hidden:      p.IsHidden(),
		Path:        p.GetPath(),
		Description: p.GetDescription(),
		Uri:         p.GetURI(),
		License:     p.GetLicense(),
		Labels:      p.GetLabels(),
		Annotations: p.GetAnnotations(),
	}

	if p.GetRequires() != nil && len(p.GetRequires()) > 0 {
		ans.PackageRequires = []*DefaultPackageSanitized{}
		for _, r := range p.GetRequires() {
			// I avoid recursive call of NewDefaultPackageSanitized
			ans.PackageRequires = append(ans.PackageRequires,
				&DefaultPackageSanitized{
					Name:     r.Name,
					Version:  r.Version,
					Category: r.Category,
					Hidden:   r.IsHidden(),
				},
			)
		}
	}

	if p.GetConflicts() != nil && len(p.GetConflicts()) > 0 {
		ans.PackageConflicts = []*DefaultPackageSanitized{}
		for _, c := range p.GetConflicts() {
			// I avoid recursive call of NewDefaultPackageSanitized
			ans.PackageConflicts = append(ans.PackageConflicts,
				&DefaultPackageSanitized{
					Name:     c.Name,
					Version:  c.Version,
					Category: c.Category,
					Hidden:   c.IsHidden(),
				},
			)
		}
	}

	if p.GetProvides() != nil && len(p.GetProvides()) > 0 {
		ans.Provides = []*DefaultPackageSanitized{}
		for _, prov := range p.GetProvides() {
			// I avoid recursive call of NewDefaultPackageSanitized
			ans.Provides = append(ans.Provides,
				&DefaultPackageSanitized{
					Name:     prov.Name,
					Version:  prov.Version,
					Category: prov.Category,
					Hidden:   prov.IsHidden(),
				},
			)
		}
	}

	return ans
}

func (p *DefaultPackageSanitized) Yaml() ([]byte, error) {
	return yaml.Marshal(p)
}

func (p *DefaultPackageSanitized) Clone() (*DefaultPackageSanitized, error) {
	data, err := p.Yaml()
	if err != nil {
		return nil, err
	}

	return NewDefaultPackageSanitizedFromYaml(data)
}
