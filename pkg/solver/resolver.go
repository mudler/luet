// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/crillab/gophersat/bf"
	"github.com/mudler/luet/pkg/helpers"
	. "github.com/mudler/luet/pkg/logger"
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
)

//. "github.com/mudler/luet/pkg/logger"

// PackageResolver assists PackageSolver on unsat cases
type PackageResolver interface {
	Solve(bf.Formula, PackageSolver) (PackagesAssertions, error)
}

type DummyPackageResolver struct {
}

func (*DummyPackageResolver) Solve(bf.Formula, PackageSolver) (PackagesAssertions, error) {
	return nil, errors.New("Could not satisfy the constraints. Try again by removing deps ")
}

type QLearningResolver struct {
	Attempts int

	ToAttempt int
	Attempted map[string]bool
	Correct   []Choice

	Solver  PackageSolver
	Formula bf.Formula

	Targets []pkg.Package
	Current []pkg.Package

	observedDelta       int
	observedDeltaChoice []pkg.Package

	Agent *qlearning.SimpleAgent

	debug bool
}

func (resolver *QLearningResolver) Solve(f bf.Formula, s PackageSolver) (PackagesAssertions, error) {
	resolver.Solver = s

	s.SetResolver(&DummyPackageResolver{}) // Set dummy. Otherwise the attempts will run again a QLearning instance.
	defer s.SetResolver(resolver)          // Set back ourselves as resolver

	resolver.Formula = f
	// Our agent has a learning rate of 0.7 and discount of 1.0.
	resolver.Agent = qlearning.NewSimpleAgent(0.7, 1.0)                                              // FIXME: Remove hardcoded values
	resolver.ToAttempt = int(helpers.Factorial(uint64(len(resolver.Solver.(*Solver).Wanted)-1) * 3)) // TODO: type assertions must go away
	Debug("Attempts:", resolver.ToAttempt)
	resolver.Targets = resolver.Solver.(*Solver).Wanted
	resolver.observedDelta = 999999

	resolver.Attempts = 9000
	resolver.Attempted = make(map[string]bool, len(resolver.Targets))

	resolver.Correct = make([]Choice, len(resolver.Targets), len(resolver.Targets))
	resolver.debug = true
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
		score := resolver.Reward(action)
		Debug("Scored", score)
		if score > 0.0 {
			resolver.Log("%s was correct", action.Action.String())
			//resolver.ToAttempt = 0 // We won. As we had one sat, let's take it
		} else {
			resolver.Log("%s was incorrect", action.Action.String())
		}
	}

	// If we get good result, take it
	// Take the result also if we did  reached overall maximum attempts
	if resolver.IsComplete() == Solved || resolver.IsComplete() == NoSolution {
		Debug("Finished")

		if len(resolver.observedDeltaChoice) != 0 {
			Debug("Taking minimum observed choiceset", resolver.observedDeltaChoice)
			// Take the minimum delta observed choice result, and consume it (Try sets the wanted list)
			resolver.Solver.(*Solver).Wanted = resolver.observedDeltaChoice
		}

		return resolver.Solver.Solve()
	} else {
		resolver.Log("Resolver couldn't find a solution!")
		return nil, errors.New("QLearning resolver failed ")
	}

}

// Returns the current state.
func (resolver *QLearningResolver) IsComplete() int {
	if resolver.Attempts < 1 {
		resolver.Log("Attempts finished!")
		return NoSolution
	}

	if resolver.ToAttempt > 0 {
		resolver.Log("We must continue!")
		return Going
	}

	resolver.Log("we solved it!")
	return Solved
}

func (resolver *QLearningResolver) Try(c Choice) error {
	pack := c.Package
	packtoAdd := pkg.FromString(pack)
	resolver.Attempted[pack+strconv.Itoa(int(c.Action))] = true // increase the count
	s, _ := resolver.Solver.(*Solver)
	var filtered []pkg.Package

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
	case NoAction:
		Debug("Chosen to keep current state")
	}

	Debug("Current test")
	for _, current := range resolver.Solver.(*Solver).Wanted {
		Debug("-", current.GetName())
	}

	_, err := resolver.Solver.Solve()

	return err
}

// Choose applies a pack attempt, returning
// true if the formula returns sat.
//
// Choose updates the resolver's state.
func (resolver *QLearningResolver) Choose(c Choice) bool {
	pack := pkg.FromString(c.Package)
	switch c.Action {
	case ActionRemoved:
		Debug("Chosed to remove ", pack.GetName())
	case ActionAdded:
		Debug("Chosed to add ", pack.GetName())
	}
	err := resolver.Try(c)

	if err == nil {
		resolver.Correct = append(resolver.Correct, c)
		//	resolver.Correct[index] = pack
		resolver.ToAttempt--
		resolver.Attempts-- // Decrease attempts - it's a barrier

	} else {
		resolver.Attempts--
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
	Debug("Observed delta", resolver.observedDelta)
	Debug("Current delta", delta)

	if err == nil {
		// if toBeInstalled == originalTarget { // Base case: all the targets matches (it shouldn't happen, but lets put a higher)
		// 	Debug("Target match, maximum score")
		// 	return 24.0 / float32(len(resolver.Attempted))

		// }
		if DoNoop {
			if noaction && toBeInstalled == 0 { // We decided to stay in the current state, and no targets have been chosen
				Debug("Penalty, noaction and no installed")
				return -100
			}
		}

		if delta <= resolver.observedDelta { // Try to maximise observedDelta
			resolver.observedDelta = delta
			resolver.observedDeltaChoice = resolver.Solver.(*Solver).Wanted // we store it as this is our return value at the end
			Debug("Delta reward", delta)
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
				Debug(pack.GetName(), " -> Action REMOVE")
				continue TARGETS
			}

		}
		actions = append(actions, &Choice{Package: pack.String(), Action: ActionAdded})
		Debug(pack.GetName(), " -> Action ADD")

	}

	if DoNoop {
		actions = append(actions, &Choice{Package: "", Action: NoAction}) // NOOP
	}

	return actions
}

// Log is a wrapper of fmt.Printf. If Game.debug is true, Log will print
// to stdout.
func (resolver *QLearningResolver) Log(msg string, args ...interface{}) {
	if resolver.debug {
		logMsg := fmt.Sprintf("(%d moves, %d remaining attempts) %s\n", len(resolver.Attempted), resolver.Attempts, msg)
		Debug(fmt.Sprintf(logMsg, args...))
	}
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
