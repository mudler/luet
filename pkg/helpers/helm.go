package helpers

import (
	"io/ioutil"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

// RenderHelm renders the template string with helm
func RenderHelm(template string, values, d map[string]interface{}) (string, error) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "",
			Version: "",
		},
		Templates: []*chart.File{
			{Name: "templates", Data: []byte(template)},
		},
		Values: map[string]interface{}{"Values": values},
	}

	v, err := chartutil.CoalesceValues(c, map[string]interface{}{"Values": d})
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}
	out, err := engine.Render(c, v)
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}

	return out["templates"], nil
}

type templatedata map[string]interface{}

// UnMarshalValues unmarshal values files and joins them into a unique templatedata
// the join happens from right to left, so any rightmost value file overwrites the content of the ones before it.
func UnMarshalValues(values []string) (templatedata, error) {
	dst := templatedata{}
	if len(values) > 0 {
		for _, bv := range reverse(values) {
			current := templatedata{}

			defBuild, err := ioutil.ReadFile(bv)
			if err != nil {
				return nil, errors.Wrap(err, "rendering file "+bv)
			}
			err = yaml.Unmarshal(defBuild, &current)
			if err != nil {
				return nil, errors.Wrap(err, "rendering file "+bv)
			}
			if err := mergo.Merge(&dst, current); err != nil {
				return nil, errors.Wrap(err, "merging values file "+bv)
			}
		}
	}
	return dst, nil
}

func reverse(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func RenderFiles(toTemplate, valuesFile string, defaultFile ...string) (string, error) {
	raw, err := ioutil.ReadFile(toTemplate)
	if err != nil {
		return "", errors.Wrap(err, "reading file "+toTemplate)
	}

	if !fileHelper.Exists(valuesFile) {
		return "", errors.Wrap(err, "file not existing "+valuesFile)
	}
	val, err := ioutil.ReadFile(valuesFile)
	if err != nil {
		return "", errors.Wrap(err, "reading file "+valuesFile)
	}

	var values templatedata
	if err = yaml.Unmarshal(val, &values); err != nil {
		return "", errors.Wrap(err, "unmarshalling file "+toTemplate)
	}

	dst, err := UnMarshalValues(defaultFile)
	if err != nil {
		return "", errors.Wrap(err, "unmarshalling values")
	}

	return RenderHelm(string(raw), values, dst)
}
