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
	Helm        *HelmChart `json:"helm"`
	Targets     []Target   `json:"targets"`
	Vars        map[string]string
	Namespace   string
	ReleaseName string
	Folders     []Folder
}

type Target struct {
	Name string `json:"name"`
}

type Folder struct {
	RenderEngine RenderEngine
	Path         string
	HelmChart    *HelmChart `json:"helm"`
}

func (d *Deploy) ConfigureFolderFromMetadata(folder string) error {
	metadataFile := path.Join(folder, "metadata.yml")

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
		ConfigFolder:  folder,
		Vars:          m.Vars,
		Namespace:     m.Namespace,
		DeployFolders: configureDeployFolders(d.srcFs, folder, m.Folders, m.Helm),
	}

	return mergo.Merge(d, &metadataDeploy)
}

func configureDeployFolders(fs afero.Fs, rootFolder string, folders []Folder, helmChart *HelmChart) []DeployFolder {
	deployFolders := []DeployFolder{}

	kubeFolders := map[string]DeployFolder{
		"predeploy": {
			Order: 1,
		},
		"secrets": {
			Order: 2,
		},
		"helmvalues": {
			RenderEngine: RenderEngineHelm,
			Order:        100,
		},
		"postdeploy": {
			Order: 101,
		},
	}

	if len(folders) == 0 {
		for n, f := range kubeFolders {
			file := path.Join(rootFolder, n)

			if _, err := fs.Stat(file); os.IsNotExist(err) {
				continue
			}

			if f.RenderEngine == RenderEngineHelm && f.HelmChart == nil {
				f.HelmChart = helmChart
			}

			f.Path = file

			deployFolders = append(deployFolders, f)
		}

		return deployFolders
	}

	for i, f := range folders {
		file := path.Join(rootFolder, f.Path)

		deployFolder := DeployFolder{
			Path:      file,
			Order:     i,
			HelmChart: f.HelmChart,
		}

		if deployFolder.HelmChart == nil {
			deployFolder.HelmChart = helmChart
		}

		deployFolders = append(deployFolders, deployFolder)
	}

	return deployFolders
}
