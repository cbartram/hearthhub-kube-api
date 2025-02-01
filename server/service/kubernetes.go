package service

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
)

// ResourceAction defines an interface for applying and rolling back Kubernetes resources.
type ResourceAction interface {
	Apply(clientset kubernetes.Interface) (string, error)
	Rollback(clientset kubernetes.Interface) (string, error)
	Name() string
}

// DeploymentAction represents a Deployment resource action.
type DeploymentAction struct {
	Deployment *appsv1.Deployment
}

func (d DeploymentAction) Name() string {
	return d.Deployment.Name
}

func (d DeploymentAction) Apply(clientset kubernetes.Interface) (string, error) {
	r, err := clientset.AppsV1().Deployments(d.Deployment.Namespace).Create(context.TODO(), d.Deployment, metav1.CreateOptions{})
	if err != nil {
		return d.Deployment.Name, fmt.Errorf("failed to create deployment: %v", err)
	}
	log.Infof("deployment: %s created successfully", r.GetName())
	return r.GetName(), nil
}

func (d DeploymentAction) Rollback(clientset kubernetes.Interface) (string, error) {
	err := clientset.AppsV1().Deployments(d.Deployment.Namespace).Delete(context.TODO(), d.Deployment.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return d.Deployment.Name, fmt.Errorf("failed to delete deployment: %v", err)
	}
	log.Infof("deployment: %s rolled back successfully", d.Deployment.Name)
	return d.Deployment.Name, nil
}

// PVCAction represents a PersistentVolumeClaim resource action.
type PVCAction struct {
	PVC *corev1.PersistentVolumeClaim
}

func (p PVCAction) Name() string {
	return p.PVC.Name
}

func (p PVCAction) Apply(clientset kubernetes.Interface) (string, error) {
	r, err := clientset.CoreV1().PersistentVolumeClaims(p.PVC.Namespace).Create(context.TODO(), p.PVC, metav1.CreateOptions{})
	if err != nil {
		return p.PVC.Name, fmt.Errorf("failed to create PVC: %v", err)
	}
	log.Infof("PVC: %s created successfully", r.GetName())
	return r.Name, nil
}

func (p PVCAction) Rollback(clientset kubernetes.Interface) (string, error) {
	err := clientset.CoreV1().PersistentVolumeClaims(p.PVC.Namespace).Delete(context.TODO(), p.PVC.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return p.PVC.Name, fmt.Errorf("failed to delete PVC: %v", err)
	}
	log.Infof("PVC: %s rolled back successfully", p.PVC.Name)
	return p.PVC.Name, nil
}

type KubernetesService interface {
	AddAction(action ResourceAction)
	ApplyResources() error
	GetActions() []ResourceAction
	GetClient() kubernetes.Interface
	GetClusterIp() (string, error)
	Rollback() error
}

type KubernetesServiceImpl struct {
	Client          kubernetes.Interface
	ResourceActions []ResourceAction
}

// MakeKubernetesService Creates a new kubernetes service object which intelligently loads configuration from
// either in-cluster or local if in-cluster fails.
func MakeKubernetesService(config *rest.Config) KubernetesService {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating kubernetes client: %v", err)
	}
	return &KubernetesServiceImpl{
		Client: clientset,
	}
}

// GetClusterIp Returns the ipv4 WAN address for the cluster. This will be the address returned to users
// where they can point their Valheim client's to connect.
func (k *KubernetesServiceImpl) GetClusterIp() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "", fmt.Errorf("failed to get public IP: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	return string(ip), nil
}

func (k *KubernetesServiceImpl) GetClient() kubernetes.Interface {
	return k.Client
}

func (k *KubernetesServiceImpl) GetActions() []ResourceAction {
	return k.ResourceActions
}

func (k *KubernetesServiceImpl) AddAction(action ResourceAction) {
	k.ResourceActions = append(k.ResourceActions, action)
}

// ApplyResources applies a list of resources and rolls them back on failure.
func (k *KubernetesServiceImpl) ApplyResources() error {
	for _, resource := range k.ResourceActions {
		if name, err := resource.Apply(k.Client); err != nil {
			log.Errorf("Error applying resource: %s err: %v", name, err)

			// Rollback all previously applied resources
			for _, appliedResource := range k.ResourceActions {
				if name, err := appliedResource.Rollback(k.Client); err != nil {
					log.Errorf("Error rolling back resource: %s err: %v", name, err)
				}
			}

			return fmt.Errorf("failed to apply resource: %s, rolled back changes", name)
		}
	}

	log.Infof("%v resources applied successfully", len(k.ResourceActions))
	return nil
}

func (k *KubernetesServiceImpl) Rollback() error {
	for _, appliedResource := range k.ResourceActions {
		if name, err := appliedResource.Rollback(k.Client); err != nil {
			log.Errorf("Error deleting resource: %s err: %v", name, err)
			return err
		}
	}
	return nil
}
