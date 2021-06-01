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

package docker

import (
	"context"
	"encoding/hex"
	"github.com/mudler/luet/pkg/helpers/imgworker"
	"os"
	"strings"

	"github.com/docker/cli/cli/trust"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/theupdateframework/notary/tuf/data"
)

// See also https://github.com/docker/cli/blob/88c6089300a82d3373892adf6845a4fed1a4ba8d/cli/command/image/trust.go#L171

func verifyImage(image string, authConfig *types.AuthConfig) (string, error) {
	ref, err := reference.ParseAnyReference(image)
	if err != nil {
		return "", errors.Wrapf(err, "invalid reference %s", image)
	}

	// only check if image ref doesn't contain hashes
	if _, ok := ref.(reference.Digested); !ok {
		namedRef, ok := ref.(reference.Named)
		if !ok {
			return "", errors.New("failed to resolve image digest using content trust: reference is not named")
		}
		namedRef = reference.TagNameOnly(namedRef)
		taggedRef, ok := namedRef.(reference.NamedTagged)
		if !ok {
			return "", errors.New("failed to resolve image digest using content trust: reference is not tagged")
		}

		resolvedImage, err := trustedResolveDigest(context.Background(), taggedRef, authConfig, "luet")
		if err != nil {
			return "", errors.Wrap(err, "failed to resolve image digest using content trust")
		}
		resolvedFamiliar := reference.FamiliarString(resolvedImage)
		return resolvedFamiliar, nil
	}

	return "", nil
}

func trustedResolveDigest(ctx context.Context, ref reference.NamedTagged, authConfig *types.AuthConfig, useragent string) (reference.Canonical, error) {
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, err
	}

	notaryRepo, err := trust.GetNotaryRepository(os.Stdin, os.Stdout, useragent, repoInfo, authConfig, "pull")
	if err != nil {
		return nil, errors.Wrap(err, "error establishing connection to trust repository")
	}

	t, err := notaryRepo.GetTargetByName(ref.Tag(), trust.ReleasesRole, data.CanonicalTargetsRole)
	if err != nil {
		return nil, trust.NotaryError(repoInfo.Name.Name(), err)
	}
	// Only get the tag if it's in the top level targets role or the releases delegation role
	// ignore it if it's in any other delegation roles
	if t.Role != trust.ReleasesRole && t.Role != data.CanonicalTargetsRole {
		return nil, trust.NotaryError(repoInfo.Name.Name(), errors.Errorf("No trust data for %s", reference.FamiliarString(ref)))
	}

	h, ok := t.Hashes["sha256"]
	if !ok {
		return nil, errors.New("no valid hash, expecting sha256")
	}

	dgst := digest.NewDigestFromHex("sha256", hex.EncodeToString(h))

	// Allow returning canonical reference with tag
	return reference.WithDigest(ref, dgst)
}

// DownloadAndExtractDockerImage is a re-adaption
// from genuinetools/img https://github.com/genuinetools/img/blob/54d0ca981c1260546d43961a538550eef55c87cf/pull.go
func DownloadAndExtractDockerImage(temp, image, dest string, auth *types.AuthConfig, verify bool) (*imgworker.ListedImage, error) {

	if verify {
		img, err := verifyImage(image, auth)
		if err != nil {
			return nil, errors.Wrapf(err, "failed verifying image")
		}
		image = img
	}

	defer os.RemoveAll(temp)
	c, err := imgworker.New(temp, auth)
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

func StripInvalidStringsFromImage(s string) string {
	return strings.ReplaceAll(s, "+", "-")
}
