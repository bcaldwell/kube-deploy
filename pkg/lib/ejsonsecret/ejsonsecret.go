package ejsonsecret

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Shopify/ejson"
	"github.com/bcaldwell/kube-deploy/pkg/lib/kubeapi"
	"github.com/bcaldwell/kube-deploy/pkg/lib/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ejsonSecret struct {
	Name      string                 `json:"_name"`
	Namespace string                 `json:"_namespace"`
	Data      map[string]interface{} `json:"data"`
}

func DeploySecret(secretsFile string, namespace string, ejsonKey string) error {
	logger.Log("Create kubernetes secret from %s", secretsFile)

	decryptedSource, err := ejson.DecryptFile(secretsFile, "/opt/ejson/keys", ejsonKey)
	if err != nil {
		fmt.Printf("Error: failed to decrypt ejson file %s\n", err)
		os.Exit(1)
	}

	var inputSecret ejsonSecret

	if err := json.Unmarshal(decryptedSource, &inputSecret); err != nil {
		return fmt.Errorf("Failed to unmarshal decrypted json file %w", err)
	}

	if inputSecret.Name == "" {
		return fmt.Errorf("Error parsing ejson secret: _name can not be blank")
	}

	if inputSecret.Namespace == "" {
		return fmt.Errorf("Error parsing ejson secret: _namespace can not be blank")
	}

	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      inputSecret.Name,
			Namespace: inputSecret.Namespace,
		},
		Data: make(map[string][]byte),
		Type: v1.SecretTypeOpaque,
	}

	if namespace != "" {
		secret.ObjectMeta.Namespace = namespace
	}

	// convert secrets to base64
	for key, value := range inputSecret.Data {
		var bytes []byte
		if s, ok := value.(string); ok {
			bytes = []byte(s)
		} else {
			bytes, _ = json.Marshal(value)
		}

		secret.Data[key] = bytes
	}

	logger.Log("creating secret %s in %s", inputSecret.Name, inputSecret.Namespace)

	return kubeapi.ApplyResource(secret)
}
