// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package solver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/crillab/gophersat/bf"
	"github.com/crillab/gophersat/explain"
	types "github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

type Explainer struct{}

func decodeDimacs(vars map[string]string, dimacs string) (string, error) {
	res := ""
	sc := bufio.NewScanner(bytes.NewBufferString(dimacs))
	lines := strings.Split(dimacs, "\n")
	linenum := 1
SCAN:
	for sc.Scan() {

		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "p":
			continue SCAN
		default:
			for i := 0; i < len(fields)-1; i++ {
				v := fields[i]
				negative := false
				if strings.HasPrefix(fields[i], "-") {
					v = strings.TrimLeft(fields[i], "-")
					negative = true
				}
				variable := vars[v]
				if negative {
					res += fmt.Sprintf("!(%s)", variable)
				} else {
					res += variable
				}

				if i != len(fields)-2 {
					res += fmt.Sprintf(" or ")
				}
			}
			if linenum != len(lines)-1 {
				res += fmt.Sprintf(" and \n")
			}
		}
		linenum++
	}
	if err := sc.Err(); err != nil {
		return res, fmt.Errorf("could not parse problem: %v", err)
	}
	return res, nil
}

func parseVars(r io.Reader) (map[string]string, error) {
	sc := bufio.NewScanner(r)
	res := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "c":
			data := strings.Split(fields[1], "=")
			res[data[1]] = data[0]

		default:
			continue

		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("could not parse problem: %v", err)
	}
	return res, nil
}

// maxExplainClauses bounds the problem size for which an explanation is
// computed.
//
// MUS extraction is roughly quadratic in clause count, and it runs on the
// FAILURE path - so an unsatisfiable request on a large tree spent far longer
// explaining itself than solving. The solver proves UNSAT quickly and linearly;
// only the explanation blows up. Measured on generated worlds:
//
//	families   clauses   solve    with MUS
//	     100    15,169   244ms       1.3s
//	     200    29,698   419ms       5.6s
//	     400    59,001   949ms      19.9s
//	     800   117,673    1.9s     >90s (abandoned)
//
// A real repository is well past the last row, so a genuine conflict would look
// like a hang rather than an error. Below the bound the explanation is worth
// having and costs little; above it, reporting the failure promptly matters
// more than describing it.
const maxExplainClauses = 10000

// clauseCount reads the clause count from a DIMACS header ("p cnf <vars>
// <clauses>"), without consuming the buffer. Returns false if absent.
func clauseCount(dimacs string) (int, bool) {
	for _, line := range strings.Split(dimacs, "\n") {
		if !strings.HasPrefix(line, "p cnf ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return 0, false
		}
		n, err := strconv.Atoi(fields[3])
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// Solve tries to find the MUS (minimum unsat) formula from the original problem.
// it returns an error with the decoded dimacs
func (*Explainer) Solve(f bf.Formula, s types.PackageSolver) (types.PackagesAssertions, error) {
	buf := bytes.NewBufferString("")
	if err := bf.Dimacs(f, buf); err != nil {
		return nil, errors.Wrap(err, "cannot extract dimacs from formula")
	}

	// String() does not consume the buffer, so the parsing below is unaffected.
	if n, ok := clauseCount(buf.String()); ok && n > maxExplainClauses {
		return nil, fmt.Errorf(
			"could not satisfy the constraints: the problem has %d clauses, "+
				"above the %d limit for computing an explanation", n, maxExplainClauses)
	}

	copy := *buf

	pb, err := explain.ParseCNF(&copy)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse problem")
	}
	pb2, err := pb.MUS()
	if err != nil {
		return nil, errors.Wrap(err, "could not extract subset")
	}

	variables, err := parseVars(buf)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse variables")
	}

	res, err := decodeDimacs(variables, pb2.CNF())
	if err != nil {
		return nil, errors.Wrap(err, "could not parse dimacs")
	}

	return nil, fmt.Errorf("could not satisfy the constraints: \n%s", res)
}
