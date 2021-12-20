package k8sutil

import (
	"errors"
	"flag"
	"os"
	"path/filepath"

	certmanager "github.com/jetstack/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	kubeconfig *string
)

func CreateKubeconfigFlag() {
	file := filepath.Join(homedir.HomeDir(), ".kube", "config")
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		kubeconfig = flag.String("kubeconfig", file, "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
}

func buildConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	return config.ClientConfig()
}

//NewClientsetFromKubeconfig creates a new kubernetes rest config
func NewClientsetFromKubeconfig() (*kubernetes.Clientset, error) {
	c, err := buildConfig()
	if err != nil {
		return nil, err
	}
	// create the clientset
	return kubernetes.NewForConfig(c)
}

func NewCertManagerClientsetFromKubeconfig() (*certmanager.CertmanagerV1Client, error) {
	c, err := buildConfig()
	if err != nil {
		return nil, err
	}

	return certmanager.NewForConfig(c)
}
