// Copyright © 2020-2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/crillab/gophersat/bf"
	"github.com/crillab/gophersat/explain"
	"github.com/mudler/luet/pkg/helpers"
	"gopkg.in/yaml.v2"

	"github.com/ecooper/qlearning"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/pkg/errors"
)

type ActionType int

const (
	NoAction      = 0
	Solved        = iota
	NoSolution    = iota
	Going         = iota
	ActionRemoved = iota
	ActionAdded   = iota

	DoNoop = false

	ActionDomains = 3 // Bump it if you increase the number of actions

	DefaultMaxAttempts     = 9000
	DefaultLearningRate    = 0.7
	DefaultDiscount        = 1.0
	DefaultInitialObserved = 999999

	QLearningResolverType = "qlearning"
)

// PackageResolver assists PackageSolver on unsat cases
type PackageResolver interface {
	Solve(bf.Formula, PackageSolver) (PackagesAssertions, error)
}

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

// Solve tries to find the MUS (minimum unsat) formula from the original problem.
// it returns an error with the decoded dimacs
func (*Explainer) Solve(f bf.Formula, s PackageSolver) (PackagesAssertions, error) {
	buf := bytes.NewBufferString("")
	if err := bf.Dimacs(f, buf); err != nil {
		return nil, errors.Wrap(err, "cannot extract dimacs from formula")
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

type QLearningResolver struct {
	Attempts int

	ToAttempt int

	attempts int

	Attempted map[string]bool

	Solver  PackageSolver
	Formula bf.Formula

	Targets pkg.Packages
	Current pkg.Packages

	observedDelta       int
	observedDeltaChoice pkg.Packages

	Agent *qlearning.SimpleAgent
}

func SimpleQLearningSolver() PackageResolver {
	return NewQLearningResolver(DefaultLearningRate, DefaultDiscount, DefaultMaxAttempts, DefaultInitialObserved)
}

// Defaults LearningRate 0.7, Discount 1.0
func NewQLearningResolver(LearningRate, Discount float32, MaxAttempts, initialObservedDelta int) PackageResolver {
	return &QLearningResolver{
		Agent:         qlearning.NewSimpleAgent(LearningRate, Discount),
		observedDelta: initialObservedDelta,
		Attempts:      MaxAttempts,
	}
}

func (resolver *QLearningResolver) Solve(f bf.Formula, s PackageSolver) (PackagesAssertions, error) {
	//	Info("Using QLearning solver to resolve conflicts. Please be patient.")
	resolver.Solver = s

	s.SetResolver(&Explainer{})   // Set dummy. Otherwise the attempts will run again a QLearning instance.
	defer s.SetResolver(resolver) // Set back ourselves as resolver

	resolver.Formula = f

	// Our agent by default has a learning rate of 0.7 and discount of 1.0.
	if resolver.Agent == nil {
		resolver.Agent = qlearning.NewSimpleAgent(DefaultLearningRate, DefaultDiscount) // FIXME: Remove hardcoded values
	}

	// 3 are the action domains, counting noop regardless if enabled or not
	// get the permutations to attempt
	resolver.ToAttempt = int(helpers.Factorial(uint64(len(resolver.Solver.(*Solver).Wanted)-1) * ActionDomains)) // TODO: type assertions must go away
	resolver.Targets = resolver.Solver.(*Solver).Wanted

	resolver.attempts = resolver.Attempts

	resolver.Attempted = make(map[string]bool, len(resolver.Targets))

	for resolver.IsComplete() == Going {
		// Pick the next move, which is going to be a letter choice.
		action := qlearning.Next(resolver.Agent, resolver)

		// Whatever that choice is, let's update our model for its
		// impact. If the package chosen makes the formula sat,
		// then this action will be positive. Otherwise, it will be
		// negative.
		resolver.Agent.Learn(action, resolver)

		// Reward doesn't change state so we can check what the
		// reward would be for this action, and report how the
		// env changed.
		//	score := resolver.Reward(action)
		//	if score > 0.0 {
		//	resolver.Log("%s was correct", action.Action.String())
		//	} else {
		//	resolver.Log("%s was incorrect", action.Action.String())
		//	}
	}

	// If we get good result, take it
	// Take the result also if we did  reached overall maximum attempts
	if resolver.IsComplete() == Solved || resolver.IsComplete() == NoSolution {

		if len(resolver.observedDeltaChoice) != 0 {
			// Take the minimum delta observed choice result, and consume it (Try sets the wanted list)
			resolver.Solver.(*Solver).Wanted = resolver.observedDeltaChoice
		}

		return resolver.Solver.Solve()
	} else {
		return nil, errors.New("QLearning resolver failed ")
	}

}

// Returns the current state.
func (resolver *QLearningResolver) IsComplete() int {
	if resolver.attempts < 1 {
		return NoSolution
	}

	if resolver.ToAttempt > 0 {
		return Going
	}

	return Solved
}

func (resolver *QLearningResolver) Try(c Choice) error {
	pack := c.Package
	packtoAdd := pkg.FromString(pack)
	resolver.Attempted[pack+strconv.Itoa(int(c.Action))] = true // increase the count
	s, _ := resolver.Solver.(*Solver)
	var filtered pkg.Packages

	switch c.Action {
	case ActionAdded:
		found := false
		for _, p := range s.Wanted {
			if p.String() == pack {
				found = true
				break
			}
		}
		if !found {
			resolver.Solver.(*Solver).Wanted = append(resolver.Solver.(*Solver).Wanted, packtoAdd)
		}

	case ActionRemoved:
		for _, p := range s.Wanted {
			if p.String() != pack {
				filtered = append(filtered, p)
			}
		}

		resolver.Solver.(*Solver).Wanted = filtered
	}

	_, err := resolver.Solver.Solve()

	return err
}

// Choose applies a pack attempt, returning
// true if the formula returns sat.
//
// Choose updates the resolver's state.
func (resolver *QLearningResolver) Choose(c Choice) bool {
	//pack := pkg.FromString(c.Package)

	err := resolver.Try(c)

	if err == nil {
		resolver.ToAttempt--
		resolver.attempts-- // Decrease attempts - it's a barrier. We could also do not decrease it here, allowing more attempts to be made
	} else {
		resolver.attempts--
		return false
	}

	return true
}

// Reward returns a score for a given qlearning.StateAction. Reward is a
// member of the qlearning.Rewarder interface. If the choice will make sat the formula, a positive score is returned.
// Otherwise, a static -1000 is returned.
func (resolver *QLearningResolver) Reward(action *qlearning.StateAction) float32 {
	choice := action.Action.(*Choice)

	//_, err := resolver.Solver.Solve()
	err := resolver.Try(*choice)

	toBeInstalled := len(resolver.Solver.(*Solver).Wanted)
	originalTarget := len(resolver.Targets)
	noaction := choice.Action == NoAction
	delta := originalTarget - toBeInstalled

	if err == nil {
		// if toBeInstalled == originalTarget { // Base case: all the targets matches (it shouldn't happen, but lets put a higher)
		// 	Debug("Target match, maximum score")
		// 	return 24.0 / float32(len(resolver.Attempted))

		// }
		if DoNoop {
			if noaction && toBeInstalled == 0 { // We decided to stay in the current state, and no targets have been chosen
				return -100
			}
		}

		if delta <= resolver.observedDelta { // Try to maximise observedDelta
			resolver.observedDelta = delta
			resolver.observedDeltaChoice = resolver.Solver.(*Solver).Wanted // we store it as this is our return value at the end
			return 24.0 / float32(len(resolver.Attempted))
		} else if toBeInstalled > 0 { // If we installed something, at least give a good score
			return 24.0 / float32(len(resolver.Attempted))
		}

	}

	return -1000
}

// Next creates a new slice of qlearning.Action instances. A possible
// action is created for each package that could be removed from the formula's target
func (resolver *QLearningResolver) Next() []qlearning.Action {
	actions := make([]qlearning.Action, 0, (len(resolver.Targets)-1)*3)

TARGETS:
	for _, pack := range resolver.Targets {
		for _, current := range resolver.Solver.(*Solver).Wanted {
			if current.String() == pack.String() {
				actions = append(actions, &Choice{Package: pack.String(), Action: ActionRemoved})
				continue TARGETS
			}

		}
		actions = append(actions, &Choice{Package: pack.String(), Action: ActionAdded})
	}

	if DoNoop {
		actions = append(actions, &Choice{Package: "", Action: NoAction}) // NOOP
	}

	return actions
}

// Log is a wrapper of fmt.Printf. If Game.debug is true, Log will print
// to stdout.
func (resolver *QLearningResolver) Log(msg string, args ...interface{}) {
	logMsg := fmt.Sprintf("(%d moves, %d remaining attempts) %s\n", len(resolver.Attempted), resolver.attempts, msg)
	fmt.Println(fmt.Sprintf(logMsg, args...))
}

// String returns a consistent hash for the current env state to be
// used in a qlearning.Agent.
func (resolver *QLearningResolver) String() string {
	return fmt.Sprintf("%v", resolver.Solver.(*Solver).Wanted)
}

// Choice implements qlearning.Action for a package choice for removal from wanted targets
type Choice struct {
	Package string     `json:"pack"`
	Action  ActionType `json:"action"`
}

func ChoiceFromString(s string) (*Choice, error) {
	var p *Choice
	err := yaml.Unmarshal([]byte(s), &p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// String returns the character for the current action.
func (choice *Choice) String() string {
	data, err := json.Marshal(choice)
	if err != nil {
		return ""
	}
	return string(data)
}

// Apply updates the state of the solver for the package choice.
func (choice *Choice) Apply(state qlearning.State) qlearning.State {
	resolver := state.(*QLearningResolver)
	resolver.Choose(*choice)

	return resolver
}
