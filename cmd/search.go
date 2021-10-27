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
package cmd

import (
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/mudler/luet/cmd/util"
	"github.com/mudler/luet/pkg/api/core/types"
	installer "github.com/mudler/luet/pkg/installer"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type PackageResult struct {
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Version    string   `json:"version"`
	Repository string   `json:"repository"`
	Target     string   `json:"target"`
	Hidden     bool     `json:"hidden"`
	Files      []string `json:"files"`
}

type Results struct {
	Packages []PackageResult `json:"packages"`
}

func (r PackageResult) String() string {
	return fmt.Sprintf("%s/%s-%s required for %s", r.Category, r.Name, r.Version, r.Target)
}

var rows []string = []string{"Package", "Category", "Name", "Version", "Repository", "License"}

func packageToRow(repo string, p pkg.Package) []string {
	return []string{p.HumanReadableString(), p.GetCategory(), p.GetName(), p.GetVersion(), repo, p.GetLicense()}
}

func packageToList(l *util.ListWriter, repo string, p pkg.Package) {
	l.AppendItem(pterm.BulletListItem{
		Level: 0, Text: p.HumanReadableString(),
		TextStyle: pterm.NewStyle(pterm.FgCyan), Bullet: ">", BulletStyle: pterm.NewStyle(pterm.FgYellow),
	})
	l.AppendItem(pterm.BulletListItem{
		Level: 1, Text: fmt.Sprintf("Category: %s", p.GetCategory()),
		Bullet: "->", BulletStyle: pterm.NewStyle(pterm.FgDarkGray),
	})
	l.AppendItem(pterm.BulletListItem{
		Level: 1, Text: fmt.Sprintf("Name: %s", p.GetName()),
		Bullet: "->", BulletStyle: pterm.NewStyle(pterm.FgDarkGray),
	})
	l.AppendItem(pterm.BulletListItem{
		Level: 1, Text: fmt.Sprintf("Version: %s", p.GetVersion()),
		Bullet: "->", BulletStyle: pterm.NewStyle(pterm.FgDarkGray),
	})
	l.AppendItem(pterm.BulletListItem{
		Level: 1, Text: fmt.Sprintf("Description: %s", p.GetDescription()),
		Bullet: "->", BulletStyle: pterm.NewStyle(pterm.FgDarkGray),
	})
	l.AppendItem(pterm.BulletListItem{
		Level: 1, Text: fmt.Sprintf("Repository: %s ", repo),
		Bullet: "->", BulletStyle: pterm.NewStyle(pterm.FgDarkGray),
	})
	l.AppendItem(pterm.BulletListItem{
		Level: 1, Text: fmt.Sprintf("Uri: %s ", strings.Join(p.GetURI(), " ")),
		Bullet: "->", BulletStyle: pterm.NewStyle(pterm.FgDarkGray),
	})
}

func searchLocally(term string, l *util.ListWriter, t *util.TableWriter, label, labelMatch, revdeps, hidden bool) Results {
	var results Results

	system := &installer.System{Database: util.DefaultContext.Config.GetSystemDB(), Target: util.DefaultContext.Config.GetSystem().Rootfs}

	var err error
	iMatches := pkg.Packages{}
	if label {
		iMatches, err = system.Database.FindPackageLabel(term)
	} else if labelMatch {
		iMatches, err = system.Database.FindPackageLabelMatch(term)
	} else {
		iMatches, err = system.Database.FindPackageMatch(term)
	}

	if err != nil {
		util.DefaultContext.Fatal("Error: " + err.Error())
	}

	for _, pack := range iMatches {
		if !revdeps {
			if !pack.IsHidden() || pack.IsHidden() && hidden {

				t.AppendRow(packageToRow("system", pack))
				packageToList(l, "system", pack)
				f, _ := system.Database.GetPackageFiles(pack)
				results.Packages = append(results.Packages,
					PackageResult{
						Name:       pack.GetName(),
						Version:    pack.GetVersion(),
						Category:   pack.GetCategory(),
						Repository: "system",
						Hidden:     pack.IsHidden(),
						Files:      f,
					})
			}
		} else {

			packs, _ := system.Database.GetRevdeps(pack)
			for _, revdep := range packs {
				if !revdep.IsHidden() || revdep.IsHidden() && hidden {
					t.AppendRow(packageToRow("system", pack))
					packageToList(l, "system", pack)
					f, _ := system.Database.GetPackageFiles(revdep)
					results.Packages = append(results.Packages,
						PackageResult{
							Name:       revdep.GetName(),
							Version:    revdep.GetVersion(),
							Category:   revdep.GetCategory(),
							Repository: "system",
							Hidden:     revdep.IsHidden(),
							Files:      f,
						})
				}
			}
		}
	}
	return results

}
func searchOnline(term string, l *util.ListWriter, t *util.TableWriter, label, labelMatch, revdeps, hidden bool) Results {
	var results Results

	inst := installer.NewLuetInstaller(
		installer.LuetInstallerOptions{
			Concurrency:         util.DefaultContext.Config.GetGeneral().Concurrency,
			SolverOptions:       *util.DefaultContext.Config.GetSolverOptions(),
			PackageRepositories: util.DefaultContext.Config.SystemRepositories,
			Context:             util.DefaultContext,
		},
	)
	synced, err := inst.SyncRepositories()
	if err != nil {
		util.DefaultContext.Fatal("Error: " + err.Error())
	}

	util.DefaultContext.Info("--- Search results (" + term + "): ---")

	matches := []installer.PackageMatch{}
	if label {
		matches = synced.SearchLabel(term)
	} else if labelMatch {
		matches = synced.SearchLabelMatch(term)
	} else {
		matches = synced.Search(term)
	}

	for _, m := range matches {
		if !revdeps {
			if !m.Package.IsHidden() || m.Package.IsHidden() && hidden {
				t.AppendRow(packageToRow(m.Repo.GetName(), m.Package))
				packageToList(l, m.Repo.GetName(), m.Package)
				r := &PackageResult{
					Name:       m.Package.GetName(),
					Version:    m.Package.GetVersion(),
					Category:   m.Package.GetCategory(),
					Repository: m.Repo.GetName(),
					Hidden:     m.Package.IsHidden(),
				}
				if m.Artifact != nil {
					r.Files = m.Artifact.Files
				}
				results.Packages = append(results.Packages, *r)
			}
		} else {
			packs, _ := m.Repo.GetTree().GetDatabase().GetRevdeps(m.Package)
			for _, revdep := range packs {
				if !revdep.IsHidden() || revdep.IsHidden() && hidden {
					t.AppendRow(packageToRow(m.Repo.GetName(), revdep))
					packageToList(l, m.Repo.GetName(), revdep)
					r := &PackageResult{
						Name:       revdep.GetName(),
						Version:    revdep.GetVersion(),
						Category:   revdep.GetCategory(),
						Repository: m.Repo.GetName(),
						Hidden:     revdep.IsHidden(),
					}
					if m.Artifact != nil {
						r.Files = m.Artifact.Files
					}
					results.Packages = append(results.Packages, *r)
				}
			}
		}
	}
	return results
}
func searchLocalFiles(term string, l *util.ListWriter, t *util.TableWriter) Results {
	var results Results
	util.DefaultContext.Info("--- Search results (" + term + "): ---")

	matches, _ := util.DefaultContext.Config.GetSystemDB().FindPackageByFile(term)
	for _, pack := range matches {
		t.AppendRow(packageToRow("system", pack))
		packageToList(l, "system", pack)
		f, _ := util.DefaultContext.Config.GetSystemDB().GetPackageFiles(pack)
		results.Packages = append(results.Packages,
			PackageResult{
				Name:       pack.GetName(),
				Version:    pack.GetVersion(),
				Category:   pack.GetCategory(),
				Repository: "system",
				Hidden:     pack.IsHidden(),
				Files:      f,
			})
	}

	return results
}

func searchFiles(term string, l *util.ListWriter, t *util.TableWriter) Results {
	var results Results

	inst := installer.NewLuetInstaller(
		installer.LuetInstallerOptions{
			Concurrency:         util.DefaultContext.Config.GetGeneral().Concurrency,
			SolverOptions:       *util.DefaultContext.Config.GetSolverOptions(),
			PackageRepositories: util.DefaultContext.Config.SystemRepositories,
			Context:             util.DefaultContext,
		},
	)
	synced, err := inst.SyncRepositories()
	if err != nil {
		util.DefaultContext.Fatal("Error: " + err.Error())
	}

	util.DefaultContext.Info("--- Search results (" + term + "): ---")

	matches := []installer.PackageMatch{}

	matches = synced.SearchPackages(term, installer.FileSearch)

	for _, m := range matches {
		t.AppendRow(packageToRow(m.Repo.GetName(), m.Package))
		packageToList(l, m.Repo.GetName(), m.Package)
		results.Packages = append(results.Packages,
			PackageResult{
				Name:       m.Package.GetName(),
				Version:    m.Package.GetVersion(),
				Category:   m.Package.GetCategory(),
				Repository: m.Repo.GetName(),
				Hidden:     m.Package.IsHidden(),
				Files:      m.Artifact.Files,
			})
	}
	return results
}

var searchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search packages",
	Long: `Search for installed and available packages
	
To search a package in the repositories:

	$ luet search <regex>

To search a package and display results in a table (wide screens):

	$ luet search --table <regex>

To look into the installed packages:

	$ luet search --installed <regex>

Note: the regex argument is optional, if omitted implies "all"

To search a package by label:

	$ luet search --by-label <label>

or by regex against the label:

	$ luet search --by-label-regex <label>

It can also show a package revdeps by:

	$ luet search --revdeps <regex>

Search can also return results in the terminal in different ways: as terminal output, as json or as yaml.

	$ luet search --json <regex> # JSON output
	$ luet search --yaml <regex> # YAML output
`,
	Aliases: []string{"s"},
	PreRun: func(cmd *cobra.Command, args []string) {
		util.BindSystemFlags(cmd)
		util.BindSolverFlags(cmd)
		viper.BindPFlag("installed", cmd.Flags().Lookup("installed"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var results Results
		if len(args) > 1 {
			util.DefaultContext.Fatal("Wrong number of arguments (expected 1)")
		} else if len(args) == 0 {
			args = []string{"."}
		}
		hidden, _ := cmd.Flags().GetBool("hidden")

		installed := viper.GetBool("installed")
		searchWithLabel, _ := cmd.Flags().GetBool("by-label")
		searchWithLabelMatch, _ := cmd.Flags().GetBool("by-label-regex")
		revdeps, _ := cmd.Flags().GetBool("revdeps")
		tableMode, _ := cmd.Flags().GetBool("table")
		files, _ := cmd.Flags().GetBool("files")

		util.SetSystemConfig(util.DefaultContext)
		util.SetSolverConfig(util.DefaultContext)

		out, _ := cmd.Flags().GetString("output")
		if out != "terminal" {
			util.DefaultContext.Config.GetLogging().SetLogLevel(types.FatalLevel)
		}

		l := &util.ListWriter{}
		t := &util.TableWriter{}
		t.AppendRow(rows)
		util.DefaultContext.Debug("Solver", util.DefaultContext.Config.GetSolverOptions().CompactString())

		switch {
		case files && installed:
			results = searchLocalFiles(args[0], l, t)
		case files && !installed:
			results = searchFiles(args[0], l, t)
		case !installed:
			results = searchOnline(args[0], l, t, searchWithLabel, searchWithLabelMatch, revdeps, hidden)
		default:
			results = searchLocally(args[0], l, t, searchWithLabel, searchWithLabelMatch, revdeps, hidden)
		}

		y, err := yaml.Marshal(results)
		if err != nil {
			fmt.Printf("err: %v\n", err)
			return
		}
		switch out {
		case "yaml":
			fmt.Println(string(y))
		case "json":
			j2, err := yaml.YAMLToJSON(y)
			if err != nil {
				fmt.Printf("err: %v\n", err)
				return
			}
			fmt.Println(string(j2))
		default:
			if tableMode {
				t.Render()
			} else {
				l.Render()
			}
		}
	},
}

func init() {
	searchCmd.Flags().String("system-dbpath", "", "System db path")
	searchCmd.Flags().String("system-target", "", "System rootpath")
	searchCmd.Flags().String("system-engine", "", "System DB engine")

	searchCmd.Flags().Bool("installed", false, "Search between system packages")
	searchCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+types.AvailableResolvers+" )")
	searchCmd.Flags().StringP("output", "o", "terminal", "Output format ( Defaults: terminal, available: json,yaml )")
	searchCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	searchCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	searchCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	searchCmd.Flags().Bool("by-label", false, "Search packages through label")
	searchCmd.Flags().Bool("by-label-regex", false, "Search packages through label regex")
	searchCmd.Flags().Bool("revdeps", false, "Search package reverse dependencies")
	searchCmd.Flags().Bool("hidden", false, "Include hidden packages")
	searchCmd.Flags().Bool("table", false, "show output in a table (wider screens)")
	searchCmd.Flags().Bool("files", false, "Search between packages files")

	RootCmd.AddCommand(searchCmd)
}
