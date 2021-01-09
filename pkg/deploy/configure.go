package deploy

import (
	"fmt"
	"os"
	"path"

	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"github.com/spf13/afero"
)

type Metadata struct {
	MetadataConfig
	Targets []Target `json:"targets"`
}

type MetadataConfig struct {
	Helm        *HelmChart `json:"helm"`
	Targets     []Target   `json:"targets"`
	Vars        map[string]string
	Namespace   string
	ReleaseName string
	Folders     []Folder
}

type Target struct {
	Name string `json:"name"`
	MetadataConfig
}

type Folder struct {
	RenderEngine RenderEngine
	Path         string
	HelmChart    *HelmChart `json:"helm"`
}

func (d *Deploy) ConfigureFolderFromMetadata(folder string, targetName string) error {
	metadataFile := path.Join(folder, "metadata.yml")

	if _, err := d.srcFs.Stat(metadataFile); os.IsNotExist(err) {
		logger.Log("skipping configuring from metadata.yml, %s does not exist", metadataFile)
		return nil
	}

	logger.Log("found metadata.yaml, configuring from it")

	content, err := afero.ReadFile(d.srcFs, metadataFile)
	if err != nil {
		return err
	}

	m := Metadata{}

	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return err
	}

	metadataDeploy := convertMetadataToDeploy(d.srcFs, folder, m.MetadataConfig, true)

	target, err := getTargetConfig(targetName, m.Targets)
	if err != nil {
		return err
	}

	mergoOpts := []func(*mergo.Config){mergo.WithOverrideEmptySlice}

	if target != nil {
		logger.Log("found target overrides for %s", targetName)
		targetDeploy := convertMetadataToDeploy(d.srcFs, folder, target.MetadataConfig, false)

		// this a bit stange and flipped so that targetDeploy is the more important one
		err = mergo.Merge(targetDeploy, metadataDeploy, mergoOpts...)
		if err != nil {
			return fmt.Errorf("failed to merge target config with metadata config: %w", err)
		}

		metadataDeploy = targetDeploy
	}

	return mergo.Merge(d, metadataDeploy, mergoOpts...)
}

func getTargetConfig(targetName string, targets []Target) (*Target, error) {
	if targetName == "" {
		return nil, nil
	}

	for _, t := range targets {
		if t.Name == targetName {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unable to find target %s in target list", targetName)
}

func convertMetadataToDeploy(fs afero.Fs, folder string, m MetadataConfig, defaultFolders bool) *Deploy {
	return &Deploy{
		ConfigFolder:  folder,
		Vars:          m.Vars,
		Namespace:     m.Namespace,
		DeployFolders: configureDeployFolders(fs, folder, m.Folders, m.Helm, defaultFolders),
	}
}

func configureDeployFolders(fs afero.Fs, rootFolder string, folders []Folder, helmChart *HelmChart, defaultFolders bool) []DeployFolder {
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

	if len(folders) == 0 && defaultFolders {
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
