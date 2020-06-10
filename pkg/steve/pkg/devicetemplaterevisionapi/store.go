package devicetemplaterevisionapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/cnrancher/edge-api-server/pkg/apis/edgeapi.cattle.io/v1alpha1"
	"github.com/cnrancher/edge-api-server/pkg/auth"
	controller "github.com/cnrancher/edge-api-server/pkg/generated/controllers/edgeapi.cattle.io/v1alpha1"
	"github.com/cnrancher/edge-api-server/pkg/util"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Store struct {
	types.Store
	asl                              accesscontrol.AccessSetLookup
	client                           dynamic.Interface
	ctx                              context.Context
	auth                             auth.Authenticator
	deviceTemplateController         controller.DeviceTemplateController
	deviceTemplateRevisionController controller.DeviceTemplateRevisionController
}

const (
	templateRevisionDeviceTypeName = "edgeapi.cattle.io/device-template-revision-device-type"
	templateRevisionDeviceVersion  = "edgeapi.cattle.io/device-template-revision-device-version"
	templateRevisionDeviceGroup    = "edgeapi.cattle.io/device-template-revision-device-group"
	templateRevisionDeviceResource = "edgeapi.cattle.io/device-template-revision-device-resource"
	templateRevisionOwnerName      = "edgeapi.cattle.io/device-template-revision-owner"
	templateRevisionReference      = "edgeapi.cattle.io/device-template-revision-reference"
)

func (s *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	var deviceTemplateRevision v1alpha1.DeviceTemplateRevision
	err := convert.ToObj(data.Data(), &deviceTemplateRevision)
	if err != nil {
		logrus.Errorf("failed to convert device template revision data, error: %s", err.Error())
		return data, err
	}

	if err := ValidateTemplateRequest(&deviceTemplateRevision.Spec); err != nil {
		logrus.Errorf("invalid device template revision request, error: %s", err.Error())
		return data, err
	}

	if err := ValidTemplateSpec(s.ctx, &deviceTemplateRevision, s.client); err != nil {
		logrus.Errorf("valid template spec error: %s", err.Error())
		return data, err
	}

	deviceTemplate, err := ValidateDeviceTemplateIsExist(s.ctx, &deviceTemplateRevision, s.deviceTemplateController)
	if err != nil {
		logrus.Errorf("device template is not exist, error: %s", err.Error())
		return data, err
	}

	authed, user, err := s.auth.Authenticate(apiOp.Request)
	if !authed || err != nil {
		logrus.Error("Invalid user error:", err.Error())
		return data, err
	}

	deviceTemplateRevision.Labels = map[string]string{
		templateRevisionDeviceTypeName: deviceTemplate.Spec.DeviceKind,
		templateRevisionDeviceVersion:  deviceTemplate.Spec.DeviceVersion,
		templateRevisionDeviceGroup:    deviceTemplate.Spec.DeviceGroup,
		templateRevisionDeviceResource: deviceTemplate.Spec.DeviceResource,
		templateRevisionReference:      deviceTemplateRevision.Spec.DeviceTemplateName,
		templateRevisionOwnerName:      user,
	}

	err = convert.ToObj(deviceTemplateRevision, &data.Object)
	if err != nil {
		logrus.Errorf("failed to convert device template revision data, error: %s", err.Error())
		return data, err
	}

	return s.Store.Create(apiOp, schema, data)
}

func (s *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	var deviceTemplateRevision v1alpha1.DeviceTemplateRevision
	err := convert.ToObj(data.Data(), &deviceTemplateRevision)
	if err != nil {
		logrus.Errorf("failed to convert device template revision data, error: %s", err.Error())
		return data, err
	}

	if err := ValidateTemplateRequest(&deviceTemplateRevision.Spec); err != nil {
		logrus.Errorf("invalid device template revision request, error: %s", err.Error())
		return data, err
	}

	if err := ValidTemplateSpec(s.ctx, &deviceTemplateRevision, s.client); err != nil {
		logrus.Errorf("valid template spec error: %s", err.Error())
		return data, err
	}

	deviceTemplate, err := ValidateDeviceTemplateIsExist(s.ctx, &deviceTemplateRevision, s.deviceTemplateController)
	if err != nil {
		logrus.Errorf("device template is not exist, error: %s", err.Error())
		return data, err
	}

	deviceTemplateRevision.Labels = map[string]string{
		templateRevisionDeviceTypeName: deviceTemplate.Spec.DeviceKind,
		templateRevisionDeviceVersion:  deviceTemplate.Spec.DeviceVersion,
		templateRevisionDeviceGroup:    deviceTemplate.Spec.DeviceGroup,
		templateRevisionDeviceResource: deviceTemplate.Spec.DeviceResource,
		templateRevisionReference:      deviceTemplateRevision.Spec.DeviceTemplateName,
	}

	err = convert.ToObj(deviceTemplateRevision, &data.Object)
	if err != nil {
		logrus.Errorf("failed to convert device template revision data, error: %s", err.Error())
		return data, err
	}

	return s.Store.Update(apiOp, schema, data, id)
}

func ValidateTemplateRequest(spec *v1alpha1.DeviceTemplateRevisionSpec) error {
	if spec.DisplayName == "" {
		return errors.New("displayName is required of DeviceTemplateRevision")
	}
	if spec.DeviceTemplateName == "" {
		return errors.New("deviceTemplateName is required of DeviceTemplateRevision")
	}
	if spec.DeviceTemplateAPIVersion == "" {
		return errors.New("deviceTemplateAPIVersion is required of DeviceTemplateRevision")
	}
	if spec.TemplateSpec == nil {
		return errors.New("templateSpec is required of DeviceTemplateRevision")
	}
	return nil
}

func ValidTemplateSpec(ctx context.Context, obj *v1alpha1.DeviceTemplateRevision, client dynamic.Interface) error {
	deviceGroup := obj.Labels[templateRevisionDeviceGroup]
	deviceVersion := obj.Labels[templateRevisionDeviceVersion]
	deviceResource := obj.Labels[templateRevisionDeviceResource]
	deviceType := obj.Labels[templateRevisionDeviceTypeName]

	tempStr := util.GenerateRandomTempKey(7)
	device := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", deviceGroup, deviceVersion),
			"kind":       deviceType,
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("devicetemplate-%s", tempStr),
				"namespace": obj.Namespace,
			},
			"spec": obj.Spec.TemplateSpec,
		},
	}

	opt := metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}

	resource := schema.GroupVersionResource{
		Group:    deviceGroup,
		Version:  deviceVersion,
		Resource: deviceResource,
	}

	crdClient := client.Resource(resource)
	if _, err := crdClient.Namespace(obj.Namespace).Create(ctx, &device, opt); err != nil {
		return err
	}

	return nil
}

func ValidateDeviceTemplateIsExist(ctx context.Context, obj *v1alpha1.DeviceTemplateRevision, controller controller.DeviceTemplateController) (*v1alpha1.DeviceTemplate, error) {
	deviceTemplate, err := controller.Get(obj.Namespace, obj.Spec.DeviceTemplateName, metav1.GetOptions{})
	if err != nil {
		return deviceTemplate, err
	}

	return deviceTemplate, nil
}
