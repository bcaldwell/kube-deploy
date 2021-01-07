package deploy

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/afero"
)

func (d *Deploy) Run() error {
	var err error
	// returns a FS were the config folder is the root
	d.srcFs, err = d.setupFS()
	if err != nil {
		return err
	}

	err = d.ConfigureFolderFromMetadata("/")
	if err != nil {
		return err
	}

	logger.Log("using config %#v", d)

	d.rootDir, err = ioutil.TempDir(os.TempDir(), "kube-deploy")
	if err != nil {
		return err
	}

	d.fs = afero.NewBasePathFs(afero.NewOsFs(), d.rootDir)

	err = copyAndProcessFolder(d.srcFs, "/", d.fs, "/", func(_, src string) string {
		return os.Expand(src, expandEnvSafe)
	})
	if err != nil {
		return err
	}

	sshTun := d.setupPortForward(d.Bastion.Host, 6443, d.Bastion.RemotePortforwardHost, 6443)
	if sshTun != nil {
		defer sshTun.Stop()
	}

	// sort deploy folder by priority
	sort.Slice(d.DeployFolders, func(i, j int) bool {
		return d.DeployFolders[i].Priority < d.DeployFolders[j].Priority
	})

	kubeConfig, err := d.getKubeConfig()
	if err != nil {
		return err
	}

	// remove the kubeconfig if it is a temp file
	if strings.HasPrefix(kubeConfig, os.TempDir()) {
		defer os.Remove(kubeConfig)
	}

	d.setEnv(kubeConfig)

	err = d.setupHelmChartRepo()
	if err != nil {
		return err
	}

	return d.runDeploy()
}

// setupFS returns a FS were the config folder is the root
// it will clone the config repo if needed
func (d *Deploy) setupFS() (afero.Fs, error) {
	// clone repo if repo is provided
	if d.ConfigRepo != "" {
		logger.Log("Cloning config repo %s", d.ConfigRepo)

		mfs := memfs.New()

		auth, err := getGitAuth()
		if err != nil {
			return nil, err
		}

		_, err = git.Clone(memory.NewStorage(), mfs, &git.CloneOptions{
			URL:  d.ConfigRepo,
			Auth: auth,
		})
		if err != nil {
			return nil, err
		}

		if p, err := mfs.Stat(d.ConfigFolder); err != nil || !p.IsDir() {
			return nil, fmt.Errorf("config folder either doesnt exist or is not a directory: %s", d.ConfigFolder)
		}

		fs := afero.NewMemMapFs()

		err = convertBillyToAfero(mfs, d.ConfigFolder, fs, "/")

		return fs, err
	}

	if p, err := os.Stat(d.ConfigFolder); err != nil || !p.IsDir() {
		return nil, fmt.Errorf("config folder either doesnt exist or is not a directory: %s", d.ConfigFolder)
	}

	return afero.NewBasePathFs(afero.NewOsFs(), d.ConfigFolder), nil
}

func getGitAuth() (transport.AuthMethod, error) {
	// https://github.com/src-d/go-git/issues/637
	return ssh.NewSSHAgentAuth("git")
}

func (d *Deploy) setEnv(kubeconfig string) {
	for k, v := range d.Vars {
		os.Setenv(k, v)
	}

	os.Setenv("KUBECONFIG", kubeconfig)
	os.Setenv("NAMESPACE", d.Namespace)
}

func (d *Deploy) setupHelmChartRepo() error {
	return nil
}

func runCommand(cmd *exec.Cmd) error {
	return timedRun(cmd.Args[0], func() error {
		logger.Log(strings.Join(cmd.Args, " "))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Start()
		if err != nil {
			return err
		}
		err = cmd.Wait()
		return err
	})
}

type Run func() error

func timedRun(name string, function Run) error {
	start := time.Now()

	err := function()

	end := time.Now()
	runTime := end.Sub(start).Round(time.Millisecond).Seconds()

	if err != nil {
		logger.Log("error running %s: %s", name, err)
	}

	logger.Log("%s took %fs", name, runTime)

	return err
}

func filterString(vs []string, f func(string) bool) []string {
	vsf := make([]string, 0)

	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}

	return vsf
}

func convertBillyToAfero(srcFs billy.Filesystem, srcFolder string, destFs afero.Fs, destFolder string) error {
	files, err := srcFs.ReadDir(srcFolder)
	if err != nil {
		return err
	}

	for _, f := range files {
		srcFileName := path.Join(srcFolder, f.Name())
		destFileName := path.Join(destFolder, f.Name())
		if f.IsDir() {
			err = convertBillyToAfero(srcFs, srcFileName, destFs, destFileName)
			if err != nil {
				return err
			}
		}

		srcFile, err := srcFs.Open(srcFileName)
		if err != nil {
			return err
		}

		destFile, err := destFs.Create(destFileName)
		if err != nil {
			return err
		}

		destFs.Chmod(destFileName, f.Mode())

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			return err
		}

		srcFile.Close()
		if err != nil {
			return err
		}

		destFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// expand env but don't change the value if the env variable doesn't exist
func expandEnvSafe(s string) string {
	var expandedVal string
	var ok bool

	if expandedVal, ok = os.LookupEnv(s); !ok {
		return fmt.Sprintf("${%s}", s)
	}
	return expandedVal
}
