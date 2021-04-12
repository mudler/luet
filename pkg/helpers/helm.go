package helpers

import (
	"io/ioutil"
	"sort"

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

func UnMarshalValues(values []string) (templatedata, error) {
	dst := templatedata{}
	if len(values) > 0 {
		allbv := values
		sort.Sort(sort.Reverse(sort.StringSlice(allbv)))
		for _, bv := range allbv {
			current := map[string]interface{}{}

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

func RenderFiles(toTemplate, valuesFile string, defaultFile ...string) (string, error) {
	raw, err := ioutil.ReadFile(toTemplate)
	if err != nil {
		return "", errors.Wrap(err, "reading file "+toTemplate)
	}

	if !Exists(valuesFile) {
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
