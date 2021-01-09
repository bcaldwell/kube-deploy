package deploy

//go:generate go-enum -f=$GOFILE --marshal --lower

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

	// environment variables to set, these variables are processed in the templates
	Vars map[string]string

	Bastion *Bastion `json:"bastion,omitempty"`

	context
}

type context struct {
	srcFs   afero.Fs
	fs      afero.Fs
	rootDir string
}

/*
ENUM(
Auto
None
Helm
Kustomize
)
*/
type RenderEngine int

type DeployFolder struct {
	// none, helm or kustomize is supported
	RenderEngine RenderEngine
	Path         string
	Order        int
	HelmChart    *HelmChart
}

type HelmChart struct {
	// helm chart repo, can be a url or name. If it is a URL the repo will be added first
	Repo string
	// name of the helm chart to install, will try to install <HelmChartRepo Name>/<HelmChartName>
	Name string
	// helm chart path
	Path string
	// version of the helm chart to install
	Version     string
	ReleaseName string
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
