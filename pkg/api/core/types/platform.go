// Copyright © 2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types

import (
	"fmt"
	"runtime"
	"strings"
)

// Platform identifies a target OS/architecture pair, optionally refined by a
// CPU variant (for example linux/arm/v7).
//
// String() produces the OCI form ("linux/arm/v7"). Use it in YAML, CLI flags,
// image configuration and anything handed to OCI tooling.
//
// The zero Platform means "unspecified" and is reported by IsZero.
type Platform struct {
	OS      string `json:"os,omitempty" yaml:"os,omitempty"`
	Arch    string `json:"arch,omitempty" yaml:"arch,omitempty"`
	Variant string `json:"variant,omitempty" yaml:"variant,omitempty"`
}

// ParsePlatform parses an OCI platform string of the form os/arch or
// os/arch/variant.
//
// It deliberately rejects the empty string rather than defaulting to the host:
// callers that want the host must say so with HostPlatform. An implicit
// default is how luet ended up silently serving amd64 content to arm64 hosts.
func ParsePlatform(s string) (Platform, error) {
	if s == "" {
		return Platform{}, fmt.Errorf("empty platform: expected os/arch or os/arch/variant")
	}

	parts := strings.Split(s, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return Platform{}, fmt.Errorf("invalid platform %q: expected os/arch or os/arch/variant", s)
	}
	for _, p := range parts {
		if p == "" {
			return Platform{}, fmt.Errorf("invalid platform %q: empty component", s)
		}
	}

	p := Platform{OS: parts[0], Arch: parts[1]}
	if len(parts) == 3 {
		p.Variant = parts[2]
	}
	return p, nil
}

// HostPlatform returns the platform luet is currently running on.
//
// Variant is left empty: the Go runtime does not expose the ARM variant it was
// built for, and guessing it would be worse than omitting it.
func HostPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

// String returns the OCI form, or "" for the zero Platform.
func (p Platform) String() string {
	if p.IsZero() {
		return ""
	}
	if p.Variant != "" {
		return fmt.Sprintf("%s/%s/%s", p.OS, p.Arch, p.Variant)
	}
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// IsZero reports whether the platform is unspecified.
func (p Platform) IsZero() bool {
	return p.OS == "" && p.Arch == "" && p.Variant == ""
}
