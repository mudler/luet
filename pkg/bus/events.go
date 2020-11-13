package bus

import (
	"github.com/mudler/go-pluggable"
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
	// EventPackagePreBuildArtifact is the event fired before a package artifact is being built
	EventPackagePreBuildArtifact pluggable.EventType = "package.pre.build_artifact"
	// EventPackagePostBuildArtifact is the event fired after a package artifact was built
	EventPackagePostBuildArtifact pluggable.EventType = "package.post.build_artifact"
	// EventPackagePostBuild is the event fired after a package was built
	EventPackagePostBuild pluggable.EventType = "package.post.build"

	// Repository events

	// EventRepositoryPreBuild is the event fired before a repository is being built
	EventRepositoryPreBuild pluggable.EventType = "repository.pre.build"
	// EventRepositoryPostBuild is the event fired after a repository was built
	EventRepositoryPostBuild pluggable.EventType = "repository.post.build"
)

// Manager is the bus instance manager, which subscribes plugins to events emitted by Luet
var Manager *pluggable.Manager = pluggable.NewManager(
	[]pluggable.EventType{
		EventPackageInstall,
		EventPackageUnInstall,
		EventPackagePreBuild,
		EventPackagePreBuildArtifact,
		EventPackagePostBuildArtifact,
		EventPackagePostBuild,
		EventRepositoryPreBuild,
		EventRepositoryPostBuild,
	},
)
