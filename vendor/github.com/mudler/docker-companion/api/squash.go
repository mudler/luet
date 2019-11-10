package api

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fsouza/go-dockerclient"
	jww "github.com/spf13/jwalterweatherman"
)

// Squash Squashes a docker image into another one
func Squash(client *docker.Client, image string, toImage string) error {
	var err error
	var Tag = "latest"
	r, w := io.Pipe()

	Imageparts := strings.Split(toImage, ":")
	if len(Imageparts) == 2 {
		Tag = Imageparts[1]
		toImage = Imageparts[0]
	}

	jww.INFO.Println("Creating container")

	container, err := client.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: image,
			Cmd:   []string{"true"},
		},
	})
	if err != nil {
		jww.FATAL.Fatalln("Couldn't create container, sorry", err)
	}
	defer func(*docker.Container) {
		client.RemoveContainer(docker.RemoveContainerOptions{
			ID:    container.ID,
			Force: true,
		})
	}(container)

	signalchan := make(chan os.Signal, 1)
	signal.Notify(signalchan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		for {
			s := <-signalchan
			switch s {

			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				jww.WARN.Println("SIGTERM/SIGINT/SIGQUIT detected, removing pending containers/image")
				client.RemoveContainer(docker.RemoveContainerOptions{
					ID:    container.ID,
					Force: true,
				})
				client.RemoveImage(toImage)

			}
		}
	}()

	// writing without a reader will deadlock so write in a goroutine
	go func() {
		// it is important to close the writer or reading from the other end of the
		// pipe will never finish
		defer w.Close()
		err = client.ExportContainer(docker.ExportContainerOptions{ID: container.ID, OutputStream: w})
		if err != nil {
			jww.FATAL.Fatalln("Couldn't export container, sorry", err)
		}
	}()

	jww.INFO.Println("Importing to", toImage)

	err = client.ImportImage(docker.ImportImageOptions{Repository: toImage,
		Source:      "-",
		InputStream: r,
		Tag:         Tag,
	})

	if err != nil {
		return fmt.Errorf("Could not import docker image")
	}

	return nil
}
