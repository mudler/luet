package api

import (
	"bufio"
	"context"
	"io"
	"os"

	"github.com/urfave/cli"
	archive "github.com/containerd/containerd/archive"
	dockerarchive "github.com/docker/docker/pkg/archive"
	docker "github.com/fsouza/go-dockerclient"
	layer "github.com/openSUSE/umoci/oci/layer"
	jww "github.com/spf13/jwalterweatherman"
)

type ExtractOpts struct {
	Source, Destination                string
	Compressed, KeepDirlinks, Rootless bool
	UnpackMode                         string
}

func ExtractLayer(opts *ExtractOpts) error {
	file, err := os.Open(opts.Source)
	if err != nil {
		return err
	}
	var r io.Reader
	r = file

	if opts.Compressed {
		decompressedArchive, err := dockerarchive.DecompressStream(bufio.NewReader(file))
		if err != nil {
			return err
		}
		defer decompressedArchive.Close()
		r = decompressedArchive
	}

	buf := bufio.NewReader(r)
	switch opts.UnpackMode {
	case "umoci": // more fixes are in there
		return layer.UnpackLayer(opts.Destination, buf, &layer.MapOptions{KeepDirlinks: opts.KeepDirlinks, Rootless: opts.Rootless})
	case "containerd": // more cross-compatible
		_, err := archive.Apply(context.Background(), opts.Destination, buf)
		return err
	default: // moby way
		return Untar(buf, opts.Destination, !opts.Compressed)
	}
}

// PullImage pull the specified image
func PullImage(client *docker.Client, image string) error {
	var err error
	// Pulling the image
	jww.INFO.Printf("Pulling the docker image %s\n", image)
	if err = client.PullImage(docker.PullImageOptions{Repository: image}, docker.AuthConfiguration{}); err != nil {
		jww.ERROR.Printf("error pulling %s image: %s\n", image, err)
		return err
	}

	jww.INFO.Println("Image", image, "pulled correctly")

	return nil
}

// NewDocker Creates a new instance of *docker.Client, respecting env settings
func NewDocker() (*docker.Client, error) {
	var err error
	var client *docker.Client
	if os.Getenv("DOCKER_SOCKET") != "" {
		client, err = docker.NewClient(os.Getenv("DOCKER_SOCKET"))
	} else {
		client, err = docker.NewClient("unix:///var/run/docker.sock")
	}
	if err != nil {
		return nil, cli.NewExitError("could not connect to the Docker daemon", 87)
	}
	return client, nil
}
