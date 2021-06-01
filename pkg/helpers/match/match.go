// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package match

import (
	"reflect"
	"regexp"
)

func ReverseAny(s interface{}) {
	n := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func MapMatchRegex(m *map[string]string, r *regexp.Regexp) bool {
	ans := false

	if m != nil {
		for k, v := range *m {
			if r.MatchString(k + "=" + v) {
				ans = true
				break
			}
		}
	}

	return ans
}

func MapHasKey(m *map[string]string, label string) bool {
	ans := false
	if m != nil {
		for k, _ := range *m {
			if k == label {
				ans = true
				break
			}
		}
	}
	return ans
}
