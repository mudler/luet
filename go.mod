module github.com/mudler/luet

go 1.16

require (
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/Sabayon/pkgs-checker v0.8.4
	github.com/apex/log v1.9.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200907205600-7a23bdc65eef
	github.com/asdine/storm v0.0.0-20190418133842-e0f77eada154
	github.com/briandowns/spinner v1.12.1-0.20201220203425-e201aaea0a31
	github.com/cavaliercoder/grab v1.0.1-0.20201108051000-98a5bfe305ec
	github.com/containerd/cgroups v0.0.0-20200217135630-d732e370d46d // indirect
	github.com/containerd/containerd v1.4.1-0.20201117152358-0edc412565dc
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/crillab/gophersat v1.3.2-0.20210701121804-72b19f5b6b38
	github.com/docker/cli v20.10.0-beta1.0.20201029214301-1d20b15adc38+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.0-beta1.0.20201110211921-af34b94a78a1+incompatible
	github.com/docker/go-units v0.4.0
	github.com/ecooper/qlearning v0.0.0-20160612200101-3075011a69fd
	github.com/fatih/color v1.12.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/geaaru/tar-formers v0.1.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/go-containerregistry v0.2.1
	github.com/google/renameio v1.0.0
	github.com/google/uuid v1.3.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-version v1.3.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/jedib0t/go-pretty/v6 v6.0.5
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3
	github.com/klauspost/compress v1.12.2
	github.com/klauspost/pgzip v1.2.1
	github.com/knqyf263/go-deb-version v0.0.0-20190517075300-09fca494f03d
	github.com/kyokomi/emoji v2.2.4+incompatible
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/marcsauter/single v0.0.0-20181104081128-f8bf46f26ec0
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.1
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/mudler/cobra-extensions v0.0.0-20200612154940-31a47105fe3d
	github.com/mudler/docker-companion v0.4.6-0.20200418093252-41846f112d87
	github.com/mudler/go-pluggable v0.0.0-20210513155700-54c6443073af
	github.com/mudler/topsort v0.0.0-20201103161459-db5c7901c290
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc9.0.20200221051241-688cf6d43cc4 // indirect
	github.com/otiai10/copy v1.2.1-0.20200916181228-26f84a0b1578
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/philopon/go-toposort v0.0.0-20170620085441-9be86dbd762f
	github.com/pkg/errors v0.9.1
	github.com/rancher-sandbox/gofilecache v0.0.0-20210330135715-becdeff5df15
	github.com/schollz/progressbar/v3 v3.7.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
	github.com/theupdateframework/notary v0.7.0
	go.etcd.io/bbolt v1.3.5
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.19.1
	golang.org/x/mod v0.4.2
	golang.org/x/sys v0.0.0-20210915083310-ed5796bab164 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools/v3 v3.0.2 // indirect
	helm.sh/helm/v3 v3.3.4

)

replace github.com/docker/docker => github.com/Luet-lab/moby v17.12.0-ce-rc1.0.20200605210607-749178b8f80d+incompatible
