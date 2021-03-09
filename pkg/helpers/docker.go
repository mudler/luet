// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"os"

	"github.com/mudler/luet/pkg/helpers/imgworker"
	"github.com/pkg/errors"
)

// DownloadAndExtractDockerImage is a re-adaption
// from genuinetools/img https://github.com/genuinetools/img/blob/54d0ca981c1260546d43961a538550eef55c87cf/pull.go
func DownloadAndExtractDockerImage(temp, image, dest string) (*imgworker.ListedImage, error) {
	defer os.RemoveAll(temp)
	c, err := imgworker.New(temp)
	if err != nil {
		return nil, errors.Wrapf(err, "failed creating client")
	}
	defer c.Close()

	listedImage, err := c.Pull(image)
	if err != nil {
		return nil, errors.Wrapf(err, "failed listing images")

	}

	os.RemoveAll(dest)
	err = c.Unpack(image, dest)
	return listedImage, err
}
