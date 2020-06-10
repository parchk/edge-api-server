package devicetemplaterevision

import (
	"context"
	"time"

	"github.com/cnrancher/edge-api-server/pkg/apis/edgeapi.cattle.io/v1alpha1"
	controllers "github.com/cnrancher/edge-api-server/pkg/generated/controllers/edgeapi.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/pkg/apply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	name = "device-template-revision-controller"
)

const (
	templateRevisionReference = "edgeapi.cattle.io/device-template-revision-reference"
)

type Controller struct {
	context       context.Context
	dtrController controllers.DeviceTemplateRevisionController
	dtController  controllers.DeviceTemplateController
	apply         apply.Apply
}

func Register(ctx context.Context, apply apply.Apply, revisionController controllers.DeviceTemplateRevisionController, templateController controllers.DeviceTemplateController) {
	ctrl := &Controller{
		context:       ctx,
		dtrController: revisionController,
		dtController:  templateController,
		apply:         apply,
	}
	revisionController.OnChange(ctx, name, ctrl.OnChanged)
	revisionController.OnRemove(ctx, name, ctrl.OnRemoved)
}

func (c *Controller) OnChanged(key string, obj *v1alpha1.DeviceTemplateRevision) (*v1alpha1.DeviceTemplateRevision, error) {
	if key == "" {
		return nil, nil
	}

	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	objCopy := obj.DeepCopy()
	objCopy.Status.UpdatedAt = metav1.Time{Time: time.Now()}

	deviceTemplate, err := SetRevisionOwner(objCopy, c.dtController)
	if err != nil {
		return nil, err
	}

	if err := SyncDeviceTemplateDefaultRevision(objCopy, deviceTemplate, c.dtController, c.dtrController); err != nil {
		return nil, err
	}

	return c.dtrController.Update(objCopy)
}

func (c *Controller) OnRemoved(key string, obj *v1alpha1.DeviceTemplateRevision) (*v1alpha1.DeviceTemplateRevision, error) {
	if key == "" {
		return obj, nil
	}
	return obj, c.dtController.Delete(obj.Namespace, obj.Name, &metav1.DeleteOptions{})
}

func SyncDeviceTemplateDefaultRevision(obj *v1alpha1.DeviceTemplateRevision, deviceTemplate *v1alpha1.DeviceTemplate, templateController controllers.DeviceTemplateController, revisionController controllers.DeviceTemplateRevisionController) error {
	set := labels.Set(map[string]string{templateRevisionReference: obj.Spec.DeviceTemplateName})
	revisionList, err := revisionController.Cache().List(obj.Namespace, set.AsSelector())
	if err != nil {
		return err
	}
	revisionCount := len(revisionList)
	if revisionCount == 0 {
		revList, err := revisionController.List(obj.Namespace, metav1.ListOptions{LabelSelector: set.AsSelector().String()})
		if err != nil {
			return err
		}
		revisionCount = len(revList.Items)
	}

	if revisionCount == 1 || revisionCount == 0 {
		deviceTemplateCopy := deviceTemplate.DeepCopy()
		deviceTemplateCopy.Spec.DefaultRevisionName = obj.Name
		if _, err := templateController.Update(deviceTemplateCopy); err != nil {
			return err
		}
	}

	return nil
}

func SetRevisionOwner(obj *v1alpha1.DeviceTemplateRevision, dtController controllers.DeviceTemplateController) (*v1alpha1.DeviceTemplate, error) {
	deviceTemplate, err := dtController.Get(obj.Namespace, obj.Spec.DeviceTemplateName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var falseValue bool
	falseValue = false
	var trueValue bool
	trueValue = true
	var owner metav1.OwnerReference
	owner.APIVersion = obj.Spec.DeviceTemplateAPIVersion
	owner.BlockOwnerDeletion = &falseValue
	owner.Controller = &trueValue
	owner.Kind = "DeviceTemplate"
	owner.Name = obj.Spec.DeviceTemplateName
	owner.UID = deviceTemplate.UID
	obj.OwnerReferences = append(obj.OwnerReferences[:0], owner)
	return deviceTemplate, nil
}
