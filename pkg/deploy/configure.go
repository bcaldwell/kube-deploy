package deploy

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"github.com/spf13/afero"
)

var fileNotFoundErr = errors.New("file not found")

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
	Folders     []DeployFolder
}

type Target struct {
	Name         string `json:"name"`
	MergeFolders []MergeFolder
	MetadataConfig
}

type MergeFolder struct {
	InsteadOf string
	DeployFolder
}

type GlobalVars struct {
	GlobalVars map[string]string `json:"global_vars"`
}

func (d *Deploy) ConfigureFolderFromMetadata(folder string, targetName string) error {
	metadataFile := path.Join(folder, "metadata.yml")
	m := Metadata{}

	err := readAndUnmarshal(d.srcFs, &m, os.Getenv("KUBE_DEPLOY_METADATA_FILE"), metadataFile)
	if errors.Is(err, fileNotFoundErr) {
		logger.Log("skipping configuring from metadata.yml, %s does not exist", metadataFile)
		return nil
	}

	if err != nil {
		return err
	}

	globalVars, err := d.getGlobalVars(folder)
	if err != nil {
		return err
	}

	if m.Vars == nil {
		m.Vars = make(map[string]string)
	}

	for k, v := range globalVars.GlobalVars {
		// skip existing ones
		if _, ok := m.Vars[k]; ok {
			continue
		}

		m.Vars[k] = v
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

		// only use merge logic if folders is not set in the target
		if len(targetDeploy.DeployFolders) == 0 {
			targetDeploy.DeployFolders, err = mergeDeployFolders(folder, m.Helm, metadataDeploy.DeployFolders, target.MergeFolders)
			if err != nil {
				return fmt.Errorf("failed to merge mergeFolders into existing deploy folders: %w", err)
			}
		} else if len(target.MergeFolders) != 0 {
			return fmt.Errorf("cannot set mergFolders and Folders in the same target in %s", targetName)
		}

		// this a bit strange and flipped so that targetDeploy is the more important one
		err = mergo.Merge(targetDeploy, metadataDeploy, mergoOpts...)
		if err != nil {
			return fmt.Errorf("failed to merge target config with metadata config: %w", err)
		}

		metadataDeploy = targetDeploy
	}

	return mergo.Merge(d, metadataDeploy, mergoOpts...)
}

func (d *Deploy) getGlobalVars(folder string) (GlobalVars, error) {
	globalEnv := GlobalVars{}

	err := readAndUnmarshal(d.srcFs, &globalEnv, os.Getenv("KUBE_DEPLOY_GLOBAL_VARS"), "global_vars.yml", path.Join(folder, "global_vars.yml"))

	return globalEnv, err
}

func readAndUnmarshal(fs afero.Fs, o interface{}, paths ...string) error {
	for _, path := range paths {
		if path == "" {
			continue
		}

		if _, err := fs.Stat(path); os.IsNotExist(err) {
			continue
		}

		logger.Log("loading config from %s", path)

		content, err := afero.ReadFile(fs, path)
		if err != nil {
			return err
		}

		err = yaml.Unmarshal(content, o)
		return err
	}

	return fileNotFoundErr
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

func configureDeployFolders(fs afero.Fs, rootFolder string, folders []DeployFolder, helmChart *HelmChart, defaultFolders bool) []DeployFolder {
	deployFolders := []DeployFolder{}

	kubeFolders := map[string]DeployFolder{
		"predeploy": {
			Order: stringPointer(1),
		},
		"secrets": {
			Order: stringPointer(2),
		},
		"helmvalues": {
			RenderEngine: RenderEngineHelm,
			Order:        stringPointer(100),
		},
		"postdeploy": {
			Order: stringPointer(101),
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
		deployFolders = append(deployFolders, processDeployFolder(rootFolder, i, helmChart, f))
	}

	return deployFolders
}

func mergeDeployFolders(rootFolder string, defaultHelmChart *HelmChart, destFolders []DeployFolder, mergeFolders []MergeFolder) ([]DeployFolder, error) {
	for i, mf := range mergeFolders {
		if mf.Order != nil {
			destFolders = append(destFolders, processDeployFolder(rootFolder, *mf.Order, defaultHelmChart, mf.DeployFolder))
			continue
		}

		if mf.InsteadOf == "" {
			return destFolders, fmt.Errorf("merge folder[%d] must set order or insteadOf", i)
		}

		found := false
		searchPath := path.Join(rootFolder, mf.InsteadOf)

		for j, f := range destFolders {
			if f.Path == searchPath {
				destFolders[j] = processDeployFolder(rootFolder, *f.Order, defaultHelmChart, mf.DeployFolder)
				found = true

				continue
			}
		}

		if !found {
			return destFolders, fmt.Errorf("unable to find referenced path %s", mf.InsteadOf)
		}
	}

	return destFolders, nil
}

func processDeployFolder(rootFolder string, defaultOrder int, defaultHelmChart *HelmChart, f DeployFolder) DeployFolder {
	// todo: add this back
	// f.Path = path.Join(rootFolder, f.Path)

	if f.Order == nil {
		f.Order = &defaultOrder
	}

	if f.HelmChart == nil {
		f.HelmChart = defaultHelmChart
	}

	return f
}
