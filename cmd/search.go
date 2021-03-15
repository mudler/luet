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
	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/v6/list"
	. "github.com/mudler/luet/pkg/config"
	installer "github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/spf13/cobra"
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

var rows table.Row = table.Row{"Package", "Category", "Name", "Version", "Repository", "Description", "License", "URI"}

func packageToRow(repo string, p pkg.Package) table.Row {
	return table.Row{p.HumanReadableString(), p.GetCategory(), p.GetName(), p.GetVersion(), repo, p.GetDescription(), p.GetLicense(), strings.Join(p.GetURI(), "\n")}
}

func packageToList(l list.Writer, repo string, p pkg.Package) {
	l.AppendItem(p.HumanReadableString())
	l.Indent()
	l.AppendItem(fmt.Sprintf("Category: %s", p.GetCategory()))
	l.AppendItem(fmt.Sprintf("Name: %s", p.GetName()))
	l.AppendItem(fmt.Sprintf("Version: %s", p.GetVersion()))
	l.AppendItem(fmt.Sprintf("Description: %s", p.GetDescription()))
	l.AppendItem(fmt.Sprintf("Repository: %s ", repo))
	l.AppendItem(fmt.Sprintf("Uri: %s ", strings.Join(p.GetURI(), "\n")))
	l.UnIndent()
}

func searchLocally(term string, l list.Writer, t table.Writer, label, labelMatch, revdeps, hidden bool) Results {
	var results Results

	system := &installer.System{Database: LuetCfg.GetSystemDB(), Target: LuetCfg.GetSystem().Rootfs}

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
		Fatal("Error: " + err.Error())
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
func searchOnline(term string, l list.Writer, t table.Writer, label, labelMatch, revdeps, hidden bool) Results {
	var results Results

	repos := installer.Repositories{}
	for _, repo := range LuetCfg.SystemRepositories {
		if !repo.Enable {
			continue
		}
		r := installer.NewSystemRepository(repo)
		repos = append(repos, r)
	}

	inst := installer.NewLuetInstaller(
		installer.LuetInstallerOptions{
			Concurrency:   LuetCfg.GetGeneral().Concurrency,
			SolverOptions: *LuetCfg.GetSolverOptions(),
		},
	)
	inst.Repositories(repos)
	synced, err := inst.SyncRepositories(false)
	if err != nil {
		Fatal("Error: " + err.Error())
	}

	Info("--- Search results (" + term + "): ---")

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
					r.Files = m.Artifact.GetFiles()
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
						r.Files = m.Artifact.GetFiles()
					}
					results.Packages = append(results.Packages, *r)
				}
			}
		}
	}
	return results
}
func searchLocalFiles(term string, l list.Writer, t table.Writer) Results {
	var results Results
	Info("--- Search results (" + term + "): ---")

	matches, _ := LuetCfg.GetSystemDB().FindPackageByFile(term)
	for _, pack := range matches {
		t.AppendRow(packageToRow("system", pack))
		packageToList(l, "system", pack)
		f, _ := LuetCfg.GetSystemDB().GetPackageFiles(pack)
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

func searchFiles(term string, l list.Writer, t table.Writer) Results {
	var results Results

	repos := installer.Repositories{}
	for _, repo := range LuetCfg.SystemRepositories {
		if !repo.Enable {
			continue
		}
		r := installer.NewSystemRepository(repo)
		repos = append(repos, r)
	}

	inst := installer.NewLuetInstaller(
		installer.LuetInstallerOptions{
			Concurrency:   LuetCfg.GetGeneral().Concurrency,
			SolverOptions: *LuetCfg.GetSolverOptions(),
		},
	)
	inst.Repositories(repos)
	synced, err := inst.SyncRepositories(false)
	if err != nil {
		Fatal("Error: " + err.Error())
	}

	Info("--- Search results (" + term + "): ---")

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
				Files:      m.Artifact.GetFiles(),
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
		LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
		LuetCfg.Viper.BindPFlag("installed", cmd.Flags().Lookup("installed"))
		LuetCfg.Viper.BindPFlag("solver.type", cmd.Flags().Lookup("solver-type"))
		LuetCfg.Viper.BindPFlag("solver.discount", cmd.Flags().Lookup("solver-discount"))
		LuetCfg.Viper.BindPFlag("system.database_engine", cmd.Flags().Lookup("system-engine"))
		LuetCfg.Viper.BindPFlag("solver.rate", cmd.Flags().Lookup("solver-rate"))
		LuetCfg.Viper.BindPFlag("solver.max_attempts", cmd.Flags().Lookup("solver-attempts"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var results Results
		if len(args) > 1 {
			Fatal("Wrong number of arguments (expected 1)")
		} else if len(args) == 0 {
			args = []string{"."}
		}
		hidden, _ := cmd.Flags().GetBool("hidden")

		installed := LuetCfg.Viper.GetBool("installed")
		stype := LuetCfg.Viper.GetString("solver.type")
		discount := LuetCfg.Viper.GetFloat64("solver.discount")
		rate := LuetCfg.Viper.GetFloat64("solver.rate")
		attempts := LuetCfg.Viper.GetInt("solver.max_attempts")
		searchWithLabel, _ := cmd.Flags().GetBool("by-label")
		searchWithLabelMatch, _ := cmd.Flags().GetBool("by-label-regex")
		revdeps, _ := cmd.Flags().GetBool("revdeps")
		tableMode, _ := cmd.Flags().GetBool("table")
		files, _ := cmd.Flags().GetBool("files")
		dbpath := LuetCfg.Viper.GetString("system.database_path")
		rootfs := LuetCfg.Viper.GetString("system.rootfs")
		engine := LuetCfg.Viper.GetString("system.database_engine")

		LuetCfg.System.DatabaseEngine = engine
		LuetCfg.System.DatabasePath = dbpath
		LuetCfg.System.Rootfs = rootfs
		out, _ := cmd.Flags().GetString("output")
		if out != "terminal" {
			LuetCfg.GetLogging().SetLogLevel("error")
		}

		LuetCfg.GetSolverOptions().Type = stype
		LuetCfg.GetSolverOptions().LearnRate = float32(rate)
		LuetCfg.GetSolverOptions().Discount = float32(discount)
		LuetCfg.GetSolverOptions().MaxAttempts = attempts

		l := list.NewWriter()
		t := table.NewWriter()
		t.AppendHeader(rows)
		Debug("Solver", LuetCfg.GetSolverOptions().CompactString())

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

		t.AppendFooter(rows)
		t.SetStyle(table.StyleColoredBright)

		l.SetStyle(list.StyleConnectedRounded)
		if tableMode {
			Info(t.Render())
		} else {
			Info(l.Render())
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
		}

	},
}

func init() {
	searchCmd.Flags().String("system-dbpath", "", "System db path")
	searchCmd.Flags().String("system-target", "", "System rootpath")
	searchCmd.Flags().String("system-engine", "", "System DB engine")

	searchCmd.Flags().Bool("installed", false, "Search between system packages")
	searchCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+AvailableResolvers+" )")
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
