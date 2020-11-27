package helpers

import (
	"io/ioutil"

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

func RenderFiles(toTemplate, valuesFile string, defaultFile string) (string, error) {
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
	d := templatedata{}
	if len(defaultFile) > 0 {
		def, err := ioutil.ReadFile(defaultFile)
		if err != nil {
			return "", errors.Wrap(err, "reading file "+valuesFile)
		}
		if err = yaml.Unmarshal(def, &d); err != nil {
			return "", errors.Wrap(err, "unmarshalling file "+toTemplate)
		}
	}

	if err = yaml.Unmarshal(val, &values); err != nil {
		return "", errors.Wrap(err, "unmarshalling file "+toTemplate)
	}

	return RenderHelm(string(raw), values, d)
}
