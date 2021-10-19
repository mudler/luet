/*

Copyright (C) 2021  Daniele Rondina <geaaru@sabayonlinux.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

*/
package specs

import (
	"os"
)

type SpecFile struct {
	File string `yaml:"-" json:"-"`

	// Define the list of prefixes of the path to
	MatchPrefix []string `yaml:"match_prefix,omitempty" json:"match_prefix,omitempty"`
	IgnoreFiles []string `yaml:"ignore_files,omitempty" json:"ignore_files,omitempty"`

	Rename []RenameRule `yaml:"rename,omitempty" json:"rename,omitempty"`

	RemapUids   map[string]string `yaml:"remap_uids,omitempty" json:"remap_uids,omitempty"`
	RemapGids   map[string]string `yaml:"remap_gids,omitempty" json:"remap_gids,omitempty"`
	RemapUsers  map[string]string `yaml:"remap_users,omitempty" json:"remap_users,omitempty"`
	RemapGroups map[string]string `yaml:"remap_groups,omitempty" json:"remap_groups,omitempty"`

	SameOwner        bool `yaml:"same_owner,omitempty" json:"same_owner,omitempty"`
	SameChtimes      bool `yaml:"same_chtimes,omitempty" json:"same_chtimes,omitempty"`
	MapEntities      bool `yaml:"map_entities,omitempty" json:"map_entities,omitempty"`
	BrokenLinksFatal bool `yaml:"broken_links_fatal,omitempty" json:"broken_links_fatal,omitempty"`
}

type RenameRule struct {
	Source string `yaml:"source" json:"source"`
	Dest   string `yaml:"dest" json:"dest"`
}

type Link struct {
	// Contains the path of the link to create (header.Name)
	Name string
	// Contains the path of the path linked to this link (header.Linkname)
	Linkname string
	// Contains the target path merged to the destination path that must be creatd.
	Path     string
	TypeFlag byte
	Mode     os.FileMode
}
