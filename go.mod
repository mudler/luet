module github.com/mudler/luet

go 1.12

require (
	github.com/DataDog/zstd v1.4.4 // indirect
	github.com/Sabayon/pkgs-checker v0.7.2
	github.com/asdine/storm v0.0.0-20190418133842-e0f77eada154
	github.com/briandowns/spinner v1.7.0
	github.com/cavaliercoder/grab v2.0.0+incompatible
	github.com/crillab/gophersat v1.3.2-0.20201023142334-3fc2ac466765
	github.com/docker/docker v17.12.0-ce-rc1.0.20200417035958-130b0bc6032c+incompatible
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/ecooper/qlearning v0.0.0-20160612200101-3075011a69fd
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-version v1.2.0
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3
	github.com/klauspost/pgzip v1.2.1
	github.com/knqyf263/go-deb-version v0.0.0-20190517075300-09fca494f03d
	github.com/kr/pretty v0.2.0 // indirect
	github.com/kyokomi/emoji v2.1.0+incompatible
	github.com/logrusorgru/aurora v0.0.0-20190417123914-21d75270181e
	github.com/marcsauter/single v0.0.0-20181104081128-f8bf46f26ec0
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/moby/sys/mount v0.1.1-0.20200320164225-6154f11e6840 // indirect
	github.com/mudler/cobra-extensions v0.0.0-20200612154940-31a47105fe3d
	github.com/mudler/docker-companion v0.4.6-0.20200418093252-41846f112d87
	github.com/mudler/topsort v0.0.0-20201103161459-db5c7901c290
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.0
	github.com/otiai10/copy v1.2.1-0.20200916181228-26f84a0b1578
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/philopon/go-toposort v0.0.0-20170620085441-9be86dbd762f
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.6.3
	go.etcd.io/bbolt v1.3.4
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f // indirect
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553 // indirect
	gopkg.in/yaml.v2 v2.2.8
	gotest.tools/v3 v3.0.2 // indirect
	helm.sh/helm/v3 v3.3.4
	mvdan.cc/sh/v3 v3.0.0-beta1
)

replace github.com/docker/docker => github.com/Luet-lab/moby v17.12.0-ce-rc1.0.20200605210607-749178b8f80d+incompatible
