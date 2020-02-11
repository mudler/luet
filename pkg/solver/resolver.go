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
	"fmt"
	"strconv"

	"github.com/ecooper/qlearning"
	"github.com/mudler/gophersat/bf"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/pkg/errors"
)

type ActionType int

const (
	Solved        = 1
	NoSolution    = iota
	Going         = iota
	ActionRemoved = iota
	ActionAdded   = iota
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

	Agent *qlearning.SimpleAgent

	debug bool
}

func (resolver *QLearningResolver) Solve(f bf.Formula, s PackageSolver) (PackagesAssertions, error) {
	resolver.Solver = s

	s.SetResolver(&DummyPackageResolver{}) // Set dummy. Otherwise the attempts will run again a QLearning instance.
	defer s.SetResolver(resolver)          // Set back ourselves as resolver

	resolver.Formula = f
	// Our agent has a learning rate of 0.7 and discount of 1.0.
	resolver.Agent = qlearning.NewSimpleAgent(0.7, 1.0)            // FIXME: Remove hardcoded values
	resolver.ToAttempt = len(resolver.Solver.(*Solver).Wanted) - 1 // TODO: type assertions must go away

	resolver.Targets = resolver.Solver.(*Solver).Wanted

	fmt.Println("Targets", resolver.Targets)

	resolver.Attempts = 99
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
		if resolver.Reward(action) > 0.0 {
			resolver.Log("%s was correct", action.Action.String())
			resolver.ToAttempt = 0 // We won. As we had one sat, let's take it
		} else {
			resolver.Log("%s was incorrect", action.Action.String())
		}
	}

	// If we get good result, take it
	if resolver.IsComplete() == Solved {
		resolver.Log("Victory!")
		resolver.Log("removals needed: ", resolver.Correct)
		p := []pkg.Package{}
		fmt.Println("Targets", resolver.Targets)
		// Strip from targets the ones that the agent removed
	TARGET:
		for _, pack := range resolver.Targets {
			for _, w := range resolver.Correct {
				if pack.String() == w.String() {
					fmt.Println("Skipping", pack.String())
					continue TARGET
				}

			}
			fmt.Println("Appending", pack.String())

			p = append(p, pack)
		}
		fmt.Println("Installing")
		for _, pack := range p {
			fmt.Println(pack.String())
		}
		resolver.Solver.(*Solver).Wanted = p
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
	pack := c.String()
	resolver.Attempted[pack+strconv.Itoa(int(c.Action))] = true // increase the count
	s, _ := resolver.Solver.(*Solver)
	var filtered []pkg.Package

	switch c.Action {
	case ActionAdded:
		for _, p := range resolver.Targets {
			if p.String() == pack {
				resolver.Solver.(*Solver).Wanted = append(resolver.Solver.(*Solver).Wanted, p)
			}
		}

	case ActionRemoved:
		for _, p := range s.Wanted {
			if p.String() != pack {
				filtered = append(filtered, p)
			}
		}

		resolver.Solver.(*Solver).Wanted = filtered
	default:
		return errors.New("Nonvalid action")

	}

	_, err := resolver.Solver.Solve()

	return err
}

// Choose applies a pack attempt, returning
// true if the formula returns sat.
//
// Choose updates the resolver's state.
func (resolver *QLearningResolver) Choose(c Choice) bool {
	err := resolver.Try(c)

	if err == nil {
		resolver.Correct = append(resolver.Correct, c)
		//	resolver.Correct[index] = pack
		resolver.ToAttempt--
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
	choice := action.Action.String()

	var filtered []pkg.Package

	//Filter by fingerprint
	for _, p := range resolver.Targets {
		if p.String() != choice {
			filtered = append(filtered, p)
		}
	}

	resolver.Solver.(*Solver).Wanted = filtered
	//resolver.Current = filtered
	_, err := resolver.Solver.Solve()
	//resolver.Solver.(*Solver).Wanted = resolver.Targets

	if err == nil {
		return 24.0 / float32(len(resolver.Attempted))

	}

	return -1000
}

// Next creates a new slice of qlearning.Action instances. A possible
// action is created for each package that could be removed from the formula's target
func (resolver *QLearningResolver) Next() []qlearning.Action {
	actions := make([]qlearning.Action, 0, (len(resolver.Targets)-1)*2)

	fmt.Println("Actions")
	for _, pack := range resolver.Targets {
		//	attempted := resolver.Attempted[pack.String()]
		//	if !attempted {
		actions = append(actions, &Choice{Package: pack.String(), Action: ActionRemoved})
		actions = append(actions, &Choice{Package: pack.String(), Action: ActionAdded})
		fmt.Println(pack.GetName(), " -> Action added: Removed - Added")
		//	}
	}
	fmt.Println("_______")
	return actions
}

// Log is a wrapper of fmt.Printf. If Game.debug is true, Log will print
// to stdout.
func (resolver *QLearningResolver) Log(msg string, args ...interface{}) {
	if resolver.debug {
		logMsg := fmt.Sprintf("(%d moves, %d remaining attempts) %s\n", len(resolver.Attempted), resolver.Attempts, msg)
		fmt.Printf(logMsg, args...)
	}
}

// String returns a consistent hash for the current env state to be
// used in a qlearning.Agent.
func (resolver *QLearningResolver) String() string {
	return fmt.Sprintf("%v", resolver.Correct)
}

// Choice implements qlearning.Action for a package choice for removal from wanted targets
type Choice struct {
	Package string
	Action  ActionType
}

// String returns the character for the current action.
func (choice *Choice) String() string {
	return choice.Package
}

// Apply updates the state of the solver for the package choice.
func (choice *Choice) Apply(state qlearning.State) qlearning.State {
	resolver := state.(*QLearningResolver)
	resolver.Choose(*choice)

	return resolver
}
