package bus

import (
	"github.com/mudler/go-pluggable"
	"github.com/sirupsen/logrus"
)

var (
	// Package events

	// EventPackageInstall is the event fired when a new package is being installed
	EventPackageInstall pluggable.EventType = "package.install"
	// EventPackageUnInstall is the event fired when a new package is being uninstalled
	EventPackageUnInstall pluggable.EventType = "package.uninstall"

	// Package build

	// EventPackagePreBuild is the event fired before a package is being built
	EventPackagePreBuild pluggable.EventType = "package.pre.build"
	// EventPackagePreBuildArtifact is the event fired before a package tarball is being generated
	EventPackagePreBuildArtifact pluggable.EventType = "package.pre.build_artifact"
	// EventPackagePostBuildArtifact is the event fired after a package tarball is generated
	EventPackagePostBuildArtifact pluggable.EventType = "package.post.build_artifact"
	// EventPackagePostBuild is the event fired after a package was built
	EventPackagePostBuild pluggable.EventType = "package.post.build"

	// Image build

	// EventImagePreBuild is the event fired before a image is being built
	EventImagePreBuild pluggable.EventType = "image.pre.build"
	// EventImagePrePull is the event fired before a image is being pulled
	EventImagePrePull pluggable.EventType = "image.pre.pull"
	// EventImagePrePush is the event fired before a image is being pushed
	EventImagePrePush pluggable.EventType = "image.pre.push"
	// EventImagePostBuild is the event fired after an image is being built
	EventImagePostBuild pluggable.EventType = "image.post.build"
	// EventImagePostPull is the event fired after an image is being pulled
	EventImagePostPull pluggable.EventType = "image.post.pull"
	// EventImagePostPush is the event fired after an image is being pushed
	EventImagePostPush pluggable.EventType = "image.post.push"

	// Repository events

	// EventRepositoryPreBuild is the event fired before a repository is being built
	EventRepositoryPreBuild pluggable.EventType = "repository.pre.build"
	// EventRepositoryPostBuild is the event fired after a repository was built
	EventRepositoryPostBuild pluggable.EventType = "repository.post.build"

	// Image unpack

	// EventImagePreUnPack is the event fired before unpacking an image to a local dir
	EventImagePreUnPack pluggable.EventType = "image.pre.unpack"
	// EventImagePostUnPack is the event fired after unpacking an image to a local dir
	EventImagePostUnPack pluggable.EventType = "image.post.unpack"
)

// Manager is the bus instance manager, which subscribes plugins to events emitted by Luet
var Manager *Bus = &Bus{
	Manager: pluggable.NewManager(
		[]pluggable.EventType{
			EventPackageInstall,
			EventPackageUnInstall,
			EventPackagePreBuild,
			EventPackagePreBuildArtifact,
			EventPackagePostBuildArtifact,
			EventPackagePostBuild,
			EventRepositoryPreBuild,
			EventRepositoryPostBuild,
			EventImagePreBuild,
			EventImagePrePull,
			EventImagePrePush,
			EventImagePostBuild,
			EventImagePostPull,
			EventImagePostPush,
			EventImagePreUnPack,
			EventImagePostUnPack,
		},
	),
}

type Bus struct {
	*pluggable.Manager
}

func (b *Bus) Initialize(plugin ...string) {
	b.Manager.Load(plugin...).Register()

	for _, e := range b.Manager.Events {
		b.Manager.Response(e, func(p *pluggable.Plugin, r *pluggable.EventResponse) {
			if r.Errored() {
				logrus.Fatal("Plugin", p.Name, "at", p.Executable, "Error", r.Error)
			}
			logrus.Debug(
				"plugin_event",
				"received from",
				p.Name,
				"at",
				p.Executable,
				r,
			)
		})
	}
}
