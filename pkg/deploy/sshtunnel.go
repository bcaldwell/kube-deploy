package deploy

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
	"github.com/kevinburke/ssh_config"
	"github.com/rgzr/sshtun"
)

func (d *Deploy) setupPortForward(host string, localPort int, remoteHost string, remotePort int) *sshtun.SSHTun {
	if !d.Bastion.Enabled || host == "" {
		logger.Log("bastion ssh connection disabled")

		return nil
	}

	f, _ := os.Open(path.Join(os.Getenv("HOME"), ".ssh", "config"))
	sshConfig, _ := ssh_config.Decode(f)

	sshTun := sshtun.New(localPort, host, remotePort)
	sshTun.SetRemoteHost(remoteHost)

	if d.Bastion.User != "" {
		sshTun.SetUser(d.Bastion.User)
	} else {
		user, _ := sshConfig.Get(host, "User")
		if user != "" {
			sshTun.SetUser(user)
		}
	}

	if d.Bastion.KeyFile != "" {
		sshTun.SetKeyFile(d.Bastion.KeyFile)
	} else {
		keyfile, _ := sshConfig.Get(host, "IdentityFile")
		if keyfile != "" {
			sshTun.SetKeyFile(keyfile)
		} else {
			// use ssh key if there is only 1 in ~/.ssh folder
			keys, _ := filepath.Glob(path.Join(os.Getenv("HOME"), ".ssh", "id_*"))
			keys = filterString(keys, func(s string) bool {
				return !strings.HasSuffix(s, ".pub")
			})

			if len(keys) == 1 {
				sshTun.SetKeyFile(keys[0])
			}
		}
	}

	go func() {
		err := sshTun.Start()
		if err != nil {
			logger.Log("error starting ssh tunnel", err.Error())
		}
	}()

	return sshTun
}
