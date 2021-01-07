package deploy

import "github.com/spf13/afero"

type Deploy struct {
	// github repo to clone to access the config
	ConfigRepo string
	// path to the deployment config folder: relative to the github repo root or absolute if local
	ConfigFolder string

	// env variable to get kubeconfig from
	KubeconfigEnv string
	// kube config path
	KubeconfigPath string

	// namespace to deploy everything to
	Namespace string

	// folders inside config folder to deploy, along with metadata for how to deploy them
	DeployFolders []DeployFolder

	// helm chart configuration
	Chart HelmChart

	// release name, mainly used as the helm release name
	ReleaseName string

	// environment variables to set, these variables are processed in the templates
	Vars map[string]string

	Bastion Bastion

	context
}

type context struct {
	srcFs   afero.Fs
	fs      afero.Fs
	rootDir string
}

type DeployFolder struct {
	// none or kustomize is supported
	RenderEngine string
	Path         string
	Priority     int
}

type HelmChart struct {
	// helm chart configuration folder
	ValuesFolder string
	// helm chart repo, can be a url or name. If it is a URL the repo will be added first
	Repo string
	// name of the helm chart to install, will try to install <HelmChartRepo Name>/<HelmChartName>
	Name string
	// helm chart path
	Path string
	// version of the helm chart to install
	Version  string
	Priority int
}

type Bastion struct {
	Enabled bool
	// host to port forward kubernetes api server from
	Host    string
	User    string
	KeyFile string
	//
	RemotePortforwardHost string
}
