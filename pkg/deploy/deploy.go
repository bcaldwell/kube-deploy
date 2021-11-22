package deploy

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/bcaldwell/kube-deploy/pkg/lib/ejsonsecret"
	"github.com/bcaldwell/kube-deploy/pkg/lib/kubeapi"
	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
	"github.com/spf13/afero"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type helmRepoListItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (d *Deploy) runDeploy() error {
	err := createNamespace(d.Namespace)
	if err != nil {
		return fmt.Errorf("error while creating namespace %s %w", d.Namespace, err)
	}

	for _, folder := range d.DeployFolders {
		if _, err := d.fs.Stat(folder.Path); err != nil {
			logger.Log("folder %s not found", folder.Path)
			return nil
		}

		logger.Log("deploying folder %s using %s as the render engine", folder.Path, folder.RenderEngine)

		switch getRenderEngineWithDefault(d.fs, folder) {
		case RenderEngineHelm:
			// todo: move this to a valdiation function
			if folder.HelmChart == nil {
				return fmt.Errorf("helm chart can not be nil when helm render engine is set. Found in %v", folder)
			}

			err = releaseHelm(d.context, d.Namespace, folder.Path, *folder.HelmChart)
		case RenderEngineNone:
			err = kubectlDeployFolder(d.context, d.Namespace, folder, []string{"-R", "-f"})
		case RenderEngineKustomize:
			err = kubectlDeployFolder(d.context, d.Namespace, folder, []string{"-k"})
		default:
			return fmt.Errorf("%v has invald deploy type %s", folder, folder.RenderEngine)
		}
	}

	return err
}

func getRenderEngineWithDefault(fs afero.Fs, folder DeployFolder) RenderEngine {
	if folder.RenderEngine != RenderEngineAuto {
		return folder.RenderEngine
	}

	if folder.HelmChart != nil {
		return RenderEngineHelm
	}

	kustomizeFiles := []string{"kustomization.yaml", "kustomization.yml"}
	for _, f := range kustomizeFiles {
		if _, err := fs.Stat(path.Join(folder.Path, f)); err == nil {
			return RenderEngineKustomize
		}
	}

	return RenderEngineNone
}

func getHelmChartName(chart HelmChart, repoList helmRepoListItem) string {
	if chart.Path != "" {
		return chart.Path
	}

	if chart.Repo == "" {
		return chart.Name
	}

	return path.Join(repoList.Name, chart.Name)
}

func helmRepoList() ([]helmRepoListItem, error) {
	out, err := exec.Command("helm", "repo", "list", "-o", "json").Output()
	if bytes.Contains(out, []byte("no repositories to show")) {
		return []helmRepoListItem{}, nil
	}

	if err != nil {
		return nil, err
	}

	repoList := make([]helmRepoListItem, 0)
	err = json.Unmarshal(out, &repoList)

	return repoList, err
}

func setupHelmRepo(repo string) (helmRepoListItem, error) {
	if repo == "" {
		return helmRepoListItem{}, nil
	}

	repoList, _ := helmRepoList()

	for _, r := range repoList {
		if r.URL == repo {
			logger.Log("found existing helm repo %s with name %s", repo, r.Name)
			return r, nil
		}
	}

	urlMd5b := md5.Sum([]byte(repo))
	urlMd5 := fmt.Sprintf("%x", urlMd5b)

	logger.Log("adding helm repo %s with name %s", repo, urlMd5)

	err := exec.Command("helm", "repo", "add", urlMd5, repo).Run()
	if err != nil {
		return helmRepoListItem{}, err
	}

	err = exec.Command("helm", "repo", "update").Run()

	return helmRepoListItem{
		Name: urlMd5,
		URL:  repo,
	}, err
}

func createNamespace(namespace string) error {
	ns := v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	logger.Log("creating namespace %s", namespace)

	return kubeapi.ApplyResource(ns)
}

func kubectlDeployFolder(c context, namespace string, folder DeployFolder, applyArgs []string) error {
	err := deployAndDeleteEjsonFiles(c, namespace, folder)
	if err != nil {
		return err
	}

	files, err := afero.ReadDir(c.fs, folder.Path)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	folderPath := path.Join(c.rootDir, folder.Path)

	args := []string{"apply"}
	args = append(args, applyArgs...)
	args = append(args, folderPath)

	// -R for recursive
	return runCommand(exec.Command("kubectl", args...))
}

func deployAndDeleteEjsonFiles(c context, namespace string, folder DeployFolder) error {
	fileList, err := listAllFilesInFolder(c.fs, folder.Path)
	if err != nil {
		return err
	}

	ejsonFiles := filterString(fileList, func(path string) bool {
		return filepath.Ext(path) == ".ejson"
	})

	ejsonKeyPath := os.Getenv("EJSON_KEY_PATH")
	ejsonKey := os.Getenv("EJSON_KEY")

	if ejsonKeyPath != "" && ejsonKey == "" {
		b, err := ioutil.ReadFile(ejsonKeyPath)
		if err != nil {
			return err
		}

		ejsonKey = string(b)
	}

	for _, file := range ejsonFiles {
		err = ejsonsecret.DeploySecret(path.Join(c.rootDir, file), namespace, ejsonKey)
		if errors.Is(err, ejsonsecret.InvalidEjsonSecret) {
			continue
		}

		if err != nil {
			return err
		}

		// remove ejson file because it will cause issues with kubectl
		err = c.fs.Remove(file)
		if err != nil {
			return err
		}
	}

	return nil
}

func releaseHelm(c context, namespace string, folder string, chart HelmChart) error {
	logger.Log("Deploying helm chart %s with release %s into %s", chart.Name, chart.ReleaseName, namespace)

	helmArgs := []string{"upgrade", "--wait", "--install"}

	if chart.Version != "" {
		helmArgs = append(helmArgs, "--version", chart.Version)
	}

	repoItem, err := setupHelmRepo(chart.Repo)
	if err != nil {
		return err
	}

	helmArgs = append(helmArgs, "-n", namespace, chart.ReleaseName, getHelmChartName(chart, repoItem))

	valuesFiles := chart.ValuesFiles
	for i, f := range valuesFiles {
		valuesFiles[i] = path.Join(folder, f)
	}

	if _, err := c.fs.Stat(folder); len(valuesFiles) == 0 && err == nil {
		valuesFiles, err = listAllFilesInFolder(c.fs, folder)
		if err != nil {
			return fmt.Errorf("failed to get files in helm config folder %s %w", folder, err)
		}
	}

	for _, f := range valuesFiles {
		// todo copy these files and expand them
		helmArgs = append(helmArgs, "-f", path.Join(c.rootDir, f))
	}

	if chart.PostRenderer != "" {
		helmArgs = append(helmArgs, "--post-renderer", chart.PostRenderer)
	}

	cmd := exec.Command("helm", helmArgs...)
	cmd.Dir = path.Join(c.rootDir, folder)

	return runCommand(cmd)
}
