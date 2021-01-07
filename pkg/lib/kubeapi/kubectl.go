package kubeapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

func ApplyResource(resource interface{}) error {
	manifest, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	cmd := exec.Command("kubectl", "apply", "--wait", "-f", "-")

	cmd.Stdin = bytes.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}
