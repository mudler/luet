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

package helpers

import (
	"io"
	"os"
	//"os/user"
	//"strconv"

	"github.com/docker/docker/pkg/archive"
	//"github.com/docker/docker/pkg/idtools"
)

func Tar(src, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	fs, err := archive.Tar(src, archive.Uncompressed)
	if err != nil {
		return err
	}
	defer fs.Close()

	_, err = io.Copy(out, fs)
	if err != nil {
		return err
	}

	err = out.Sync()
	if err != nil {
		return err
	}
	return err
}

// Untar just a wrapper around the docker functions
func Untar(src, dest string, sameOwner bool) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	opts := &archive.TarOptions{
		// NOTE: NoLchown boolean is used for chmod of the symlink
		// Probably it's needed set this always to true.
		NoLchown:        true,
		ExcludePatterns: []string{"dev/"}, // prevent 'operation not permitted'
	}

	/*
	u, err := user.Current()
	if err != nil {
		return err
	}
	// TODO: This seems not sufficient for untar with normal user.
	if u.Uid != "0" {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		opts.ChownOpts = &idtools.Identity{UID: uid, GID: gid}
	}
	*/

	return archive.Untar(in, dest, opts)
}
