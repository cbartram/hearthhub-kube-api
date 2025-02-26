package service

import (
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"testing"
)

var spec = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test",
	},
	Spec: appsv1.DeploymentSpec{},
}

var pvc = &corev1.PersistentVolumeClaim{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test",
	},
}

func TestDeploymentAction_Apply(t *testing.T) {
	d := &DeploymentAction{
		Deployment: spec,
	}

	deploy, err := d.Apply(fake.NewClientset())
	assert.Nil(t, err)
	assert.Equal(t, deploy, "test")
}

func TestDeploymentAction_Rollback(t *testing.T) {
	d := &DeploymentAction{
		Deployment: spec,
	}

	deploy, err := d.Rollback(fake.NewClientset())
	assert.Nil(t, err)
	assert.Equal(t, deploy, "test")
}

func TestDeploymentAction_Name(t *testing.T) {
	d := &DeploymentAction{
		Deployment: spec,
	}

	deploy := d.Name()
	assert.Equal(t, deploy, "test")
}

func TestPvcAction_Apply(t *testing.T) {
	d := &PVCAction{
		PVC: pvc,
	}

	deploy, err := d.Apply(fake.NewClientset())
	assert.Nil(t, err)
	assert.Equal(t, deploy, "test")
}

func TestPvcAction_Rollback(t *testing.T) {
	d := &PVCAction{
		PVC: pvc,
	}

	deploy, err := d.Rollback(fake.NewClientset())
	assert.Nil(t, err)
	assert.Equal(t, deploy, "test")
}

func TestPvcAction_Name(t *testing.T) {
	d := &PVCAction{
		PVC: pvc,
	}

	deploy := d.Name()
	assert.Equal(t, deploy, "test")
}

func TestMakeKubernetesService(t *testing.T) {
	cfg := &rest.Config{}
	svc := MakeKubernetesService(cfg)
	assert.NotNil(t, svc)
}

func TestGetClient(t *testing.T) {
	cfg := &rest.Config{}
	svc := MakeKubernetesService(cfg)
	client := svc.GetClient()
	assert.NotNil(t, client)
}

func TestAddAction(t *testing.T) {
	cfg := &rest.Config{}
	svc := MakeKubernetesService(cfg)
	svc.AddAction(DeploymentAction{
		Deployment: spec,
	})

	assert.Len(t, svc.GetActions(), 1)
}

func TestApplyResources(t *testing.T) {
	svc := KubernetesServiceImpl{
		Client: fake.NewClientset(spec),
		ResourceActions: []ResourceAction{
			DeploymentAction{
				Deployment: spec,
			},
		},
	}
	assert.Len(t, svc.GetActions(), 1)
	_, err := svc.ApplyResources()
	assert.Nil(t, err)
}
