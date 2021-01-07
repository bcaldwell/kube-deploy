package deploy

import (
	"os"
	"path"

	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"github.com/spf13/afero"
)

type Metadata struct {
	Helm        HelmChart `json:"helm"`
	Targets     []Target  `json:"targets"`
	Vars        map[string]string
	Namespace   string
	ReleaseName string
}

type Target struct {
	Name            string `json:"name"`
	FolderOverrides map[string]string
}

func (d *Deploy) ConfigureFolderFromMetadata(folder string) error {
	metadataFile := path.Join(folder, "metadata.yml")
	defaultFolders := map[string]int{"predeploy": 1, "secrets": 2, "postdeploy": 101}
	helmOrder := 100

	if _, err := d.srcFs.Stat(metadataFile); os.IsNotExist(err) {
		logger.Log("skipping configuring from metadata, %s does not exist", metadataFile)
		return nil
	}

	content, err := afero.ReadFile(d.srcFs, metadataFile)
	if err != nil {
		return err
	}

	m := Metadata{}
	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return err
	}

	metadataDeploy := Deploy{
		ConfigFolder: folder,
		Vars:         m.Vars,
		Namespace:    m.Namespace,
		Chart:        m.Helm,
		ReleaseName:  m.ReleaseName,
	}

	if metadataDeploy.Chart.Priority == 0 {
		metadataDeploy.Chart.Priority = helmOrder
	}

	if metadataDeploy.Chart.ValuesFolder == "" {
		metadataDeploy.Chart.ValuesFolder = "helmvalues"
	}

	for n, p := range defaultFolders {
		file := path.Join(folder, n)

		if _, err := d.srcFs.Stat(file); os.IsNotExist(err) {
			continue
		}

		d.DeployFolders = append(d.DeployFolders, DeployFolder{
			Path:     file,
			Priority: p,
		})
	}

	return mergo.Merge(d, &metadataDeploy)
}
