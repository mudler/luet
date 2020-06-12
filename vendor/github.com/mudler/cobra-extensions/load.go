// Copyright © 2020 Ettore Di Giacinto
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

package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type Extension struct {
	AbsPath   string
	ShortName string
}

func (e Extension) String() string {
	return e.ShortName
}

func (e Extension) Short() string {
	return e.ShortName
}

func (e Extension) Path() string {
	return e.AbsPath
}

func (e Extension) CobraCommand() *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s --help", e.Short()),
		Short: fmt.Sprintf("extension: %s (run to show the extension helper)", e.Short()),
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.Exec(args)
		}}
}

// Discover returns extensions found in the paths specified and in PATH
// Extensions must start with the project tag (e.g. 'myawesomecli-' )
func Discover(project string, extensionpath ...string) []ExtensionInterface {
	var result []ExtensionInterface

	// by convention, extensions paths must have a prefix with the name of the project
	// e.g. 'foo-ext1' 'foo-ext2'
	projPrefix := fmt.Sprintf("%s-", project)
	paths := strings.Split(os.Getenv("PATH"), ":")

	for _, path := range extensionpath {
		if filepath.IsAbs(path) {
			paths = append(paths, path)
			continue
		}

		rel, err := RelativeToCwd(path)
		if err != nil {
			continue
		}
		paths = append(paths, rel)
	}

	for _, p := range paths {
		matches, err := filepath.Glob(filepath.Join(p, fmt.Sprintf("%s*", projPrefix)))
		if err != nil {
			continue
		}
		for _, m := range matches {
			short := strings.TrimPrefix(filepath.Base(m), projPrefix)
			result = append(result, Extension{AbsPath: m, ShortName: short})
		}
	}
	return result
}
