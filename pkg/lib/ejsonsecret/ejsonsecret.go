package ejsonsecret

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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

var InvalidEjsonSecret = errors.New("ejson secret is invalid")

func DeploySecret(secretsFile string, namespace string, ejsonKey string) error {
	logger.Log("create kubernetes secret from %s", secretsFile)

	var inputSecret ejsonSecret

	encryptedFile, err := ioutil.ReadFile(secretsFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", secretsFile, err)
	}

	if err := json.Unmarshal(encryptedFile, &inputSecret); err != nil {
		return fmt.Errorf("Failed to unmarshal decrypted json file %w", err)
	}

	if inputSecret.Name == "" {
		logger.Log("skipping creating ejson secret: _name can not be blank")
		return fmt.Errorf("%w: _name can not be blank", InvalidEjsonSecret)
	}

	// set namespace to default value if no namespace is set in the secret
	if inputSecret.Namespace == "" {
		inputSecret.Namespace = namespace
	}

	if inputSecret.Namespace == "" {
		logger.Log("skipping creating ejson secret: _namespace can not be blank")
		return fmt.Errorf("%w: _namespace can not be blank", InvalidEjsonSecret)
	}

	decryptedSource, err := ejson.DecryptFile(secretsFile, "/opt/ejson/keys", ejsonKey)
	if err != nil {
		fmt.Printf("Error: failed to decrypt ejson file %s\n", err)
		os.Exit(1)
	}

	if err := json.Unmarshal(decryptedSource, &inputSecret); err != nil {
		return fmt.Errorf("Failed to unmarshal decrypted json file %w", err)
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
