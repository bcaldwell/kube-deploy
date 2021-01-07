package deploy

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
)

const inClusterSAMountPoint = "/var/run/secrets/kubernetes.io/serviceaccount"

func (d *Deploy) getKubeConfig() (string, error) {
	if kubeconfig := os.Getenv("KUBE_CONFIG"); kubeconfig != "" {
		kubeconfig = expandPath(kubeconfig)

		if _, err := os.Stat(kubeconfig); err == nil {
			logger.Log("Using kube config defined in KUBE_CONFIG environment variable %s", kubeconfig)
			return kubeconfig, nil
		}
	}

	if _, err := os.Stat(d.KubeconfigPath); err == nil {
		logger.Log("Using existing kube config found at %s", d.KubeconfigPath)
		return d.KubeconfigPath, nil
	}

	if kubeconfig := os.Getenv(d.KubeconfigEnv); kubeconfig != "" {
		var err error

		logger.Log("Creating kubeconfig from environment variable %s", d.KubeconfigEnv)

		configBytes, err := base64.StdEncoding.DecodeString(kubeconfig)
		if err != nil {
			return "", fmt.Errorf("Failed to decode base64 encode kube config %w", err)
		}

		f, err := ioutil.TempFile(os.TempDir(), "kube-deploy-kubeconfig")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file for downloaded kube config: %w", err)
		}

		_, err = f.Write(configBytes)
		if err != nil {
			return "", fmt.Errorf("failed to write temp file for downloaded kube config: %w", err)
		}

		err = f.Close()
		if err != nil {
			return "", fmt.Errorf("failed to close temp file for downloaded kube config: %w", err)
		}

		return f.Name(), nil
	}

	if _, err := os.Stat(inClusterSAMountPoint); err == nil {
		logger.Log("running in cluster, using in cluster service account kube api access")
		return "", nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	defaultConfig := path.Join(home, ".kube/config")
	if _, err := os.Stat(defaultConfig); err == nil {
		logger.Log("using default user kube config from %s", defaultConfig)
		return defaultConfig, nil
	}

	return "", fmt.Errorf("unable to detect kube config")
}
