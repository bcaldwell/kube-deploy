package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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

var errNoGitDirFound = errors.New("no git repository found")

func (d *Deploy) Run(target string) error {
	var err error
	// returns a FS were the config folder is the root
	d.srcFs, err = d.setupFS()
	if err != nil {
		return err
	}

	err = d.ConfigureFolderFromMetadata(d.ConfigFolder, target)
	if err != nil {
		return err
	}

	b, _ := json.Marshal(d)
	logger.Log("using config %s", b)

	kubeConfig, err := d.getKubeConfig()
	if err != nil {
		return err
	}

	// remove the kubeconfig if it is a temp file
	if strings.HasPrefix(kubeConfig, os.TempDir()) {
		defer os.Remove(kubeConfig)
	}

	d.setEnv(kubeConfig)

	d.rootDir, err = ioutil.TempDir(os.TempDir(), "kube-deploy")
	if err != nil {
		return err
	}

	d.fs = afero.NewBasePathFs(afero.NewOsFs(), d.rootDir)

	err = copyAndProcessFolder(d.srcFs, d.ConfigFolder, d.fs, "/", func(_, src string) string {
		return os.Expand(src, expandEnvSafe)
	})
	if err != nil {
		return err
	}

	if d.Bastion != nil {
		sshTun := d.setupPortForward(d.Bastion.Host, 6443, d.Bastion.RemotePortforwardHost, 6443)
		if sshTun != nil {
			defer sshTun.Stop()
		}
	}

	// sort deploy folder by priority
	sort.Slice(d.DeployFolders, func(i, j int) bool {
		var a, b int
		if d.DeployFolders[i].Order != nil {
			a = *d.DeployFolders[i].Order
		}

		if d.DeployFolders[j].Order != nil {
			b = *d.DeployFolders[j].Order
		}

		return a < b
	})

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

	// setup fs at the root of the git repo, to support repo level config
	gitRoot, err := findTopLevelGitDir(d.ConfigFolder)

	if errors.Is(err, errNoGitDirFound) {
		gitRoot = d.ConfigFolder
	} else if err != nil {
		return nil, err
	}

	return afero.NewBasePathFs(afero.NewOsFs(), gitRoot), nil
}

// based on https://github.com/GoogleContainerTools/skaffold/pull/275/files
func findTopLevelGitDir(workingDir string) (string, error) {
	dir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("invalid working dir %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errNoGitDirFound
		}
		dir = parent
	}
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
