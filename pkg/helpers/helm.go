package helpers

import (

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"github.com/pkg/errors"
)

// RenderHelm renders the template string with helm
func RenderHelm(template string, values map[string]interface{}) (string,error) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "",
			Version: "",
		},
		Templates: []*chart.File{
			{Name: "templates", Data: []byte(template)},
		},
		Values:    map[string]interface{}{"Values":values},
	}

	v, err := chartutil.CoalesceValues(c, map[string]interface{}{})
	if err != nil {
		return "",errors.Wrap(err,"while rendering template")
	}
	out, err := engine.Render(c, v)
	if err != nil {
		return "",errors.Wrap(err,"while rendering template")
	}

	return out["templates"],nil
}
