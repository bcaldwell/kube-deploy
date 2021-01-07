package deploy

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
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

	// ensure we only deploy helm once in the loop
	helmDeployed := false

	for _, folder := range d.DeployFolders {
		if !helmDeployed && folder.Priority >= d.Chart.Priority {
			helmDeployed = true

			err = releaseHelm(d.context, d.ReleaseName, d.Namespace, d.Chart)
			if err != nil {
				return err
			}
		}

		err = deployFolder(d.context, d.Namespace, folder)
		if err != nil {
			return err
		}
	}

	if helmDeployed {
		return nil
	}

	return releaseHelm(d.context, d.ReleaseName, d.Namespace, d.Chart)
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

	repoList, err := helmRepoList()
	if err != nil {
		return helmRepoListItem{}, err
	}

	for _, r := range repoList {
		if r.URL == repo {
			logger.Log("found existing helm repo %s with name %s", repo, r.Name)
			return r, nil
		}
	}

	urlMd5b := md5.Sum([]byte(repo))
	urlMd5 := fmt.Sprintf("%x", urlMd5b)

	logger.Log("adding helm repo %s with name %s", repo, urlMd5)

	err = exec.Command("helm", "repo", "add", urlMd5, repo).Run()
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

func deployFolder(c context, namespace string, folder DeployFolder) error {
	logger.Log("deploying folder %s", folder.Path)

	if _, err := c.fs.Stat(folder.Path); err != nil {
		logger.Log("folder %s not found", folder.Path)
		return nil
	}

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

	// -R for recursive
	return runCommand(exec.Command("kubectl", "apply", "-R", "-f", folderPath))
}

func deployAndDeleteEjsonFiles(c context, namespace string, folder DeployFolder) error {
	fileList, err := listAllFilesInFolder(c.fs, folder.Path)
	if err != nil {
		return err
	}

	ejsonFiles := filterString(fileList, func(path string) bool {
		return filepath.Ext(path) == ".ejson"
	})

	ejsonKey := os.Getenv("EJSON_KEY")

	for _, file := range ejsonFiles {
		err = ejsonsecret.DeploySecret(path.Join(c.rootDir, file), namespace, ejsonKey)
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

func releaseHelm(c context, releaseName, namespace string, chart HelmChart) error {
	logger.Log("Deploying helm chart %s with release %s into %s", chart.Name, releaseName, namespace)

	helmArgs := []string{"upgrade", "--wait", "--install"}

	if chart.Version != "" {
		helmArgs = append(helmArgs, "--version", chart.Version)
	}

	repoItem, err := setupHelmRepo(chart.Repo)
	if err != nil {
		return err
	}

	helmArgs = append(helmArgs, "-n", namespace, releaseName, getHelmChartName(chart, repoItem))

	if _, err := c.fs.Stat(chart.ValuesFolder); err == nil {
		files, err := listAllFilesInFolder(c.fs, chart.ValuesFolder)

		if err != nil {
			return fmt.Errorf("failed to get files in helm config folder %s %w", chart.ValuesFolder, err)
		}

		for _, f := range files {
			// todo copy these files and expand them
			helmArgs = append(helmArgs, "-f", path.Join(c.rootDir, f))
		}
	}

	cmd := exec.Command("helm", helmArgs...)

	return runCommand(cmd)
}
