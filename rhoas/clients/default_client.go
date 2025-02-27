package clients

import (
	"context"
	"fmt"
	kafkainstance "github.com/redhat-developer/app-services-sdk-go/kafkainstance/apiv1"
	kafkainstanceclient "github.com/redhat-developer/app-services-sdk-go/kafkainstance/apiv1/client"
	kafkamgmtclient "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/client"
	kafkamgmtv1errors "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/error"
	serviceAccounts "github.com/redhat-developer/app-services-sdk-go/serviceaccountmgmt/apiv1/client"
	"net/http"
)

type ServiceStatus = string

// accepted, preparing, provisioning, ready, failed, deprovision, deleting
const (
	StatusAccepted     ServiceStatus = "accepted"
	StatusPreparing    ServiceStatus = "preparing"
	StatusProvisioning ServiceStatus = "provisioning"
	StatusFailed       ServiceStatus = "failed"
	StatusDeprovision  ServiceStatus = "deprovision"
	StatusDeleting     ServiceStatus = "deleting"
)

type DefaultClient struct {
	kafkaClient          *kafkamgmtclient.APIClient
	serviceAccountClient *serviceAccounts.APIClient
	httpClient           *http.Client
}

func NewDefaultClient(kafkaClient *kafkamgmtclient.APIClient, serviceAccountClient *serviceAccounts.APIClient, httpClient *http.Client) *DefaultClient {
	return &DefaultClient{
		kafkaClient:          kafkaClient,
		serviceAccountClient: serviceAccountClient,
		httpClient:           httpClient,
	}
}

func (c *DefaultClient) KafkaMgmt() kafkamgmtclient.DefaultApi {
	return c.kafkaClient.DefaultApi
}

func (c *DefaultClient) ServiceAccountMgmt() serviceAccounts.ServiceAccountsApi {
	return c.serviceAccountClient.ServiceAccountsApi
}

func (c *DefaultClient) KafkaAdmin(ctx *context.Context, instanceID string) (*kafkainstanceclient.APIClient, *kafkamgmtclient.KafkaRequest, error) {
	kafkaAPI := c.KafkaMgmt()

	kafkaInstance, resp, err := kafkaAPI.GetKafkaById(*ctx, instanceID).Execute()
	if resp != nil {
		defer resp.Body.Close()
	}

	if kafkamgmtv1errors.IsAPIError(err, kafkamgmtv1errors.ERROR_7) {
		return nil, nil, fmt.Errorf("%w", err)
	}

	kafkaStatus := kafkaInstance.GetStatus()

	switch kafkaStatus {
	case StatusProvisioning, StatusAccepted:
		err = fmt.Errorf(`Kafka instance "%v" is not ready yet`, kafkaInstance.GetName())
		return nil, nil, err
	case StatusFailed:
		err = fmt.Errorf(`Kafka instance "%v" has failed`, kafkaInstance.GetName())
		return nil, nil, err
	case StatusDeprovision:
		err = fmt.Errorf(`Kafka instance "%v" is being deprovisioned`, kafkaInstance.GetName())
		return nil, nil, err
	case StatusDeleting:
		err = fmt.Errorf(`Kafka instance "%v" is being deleted`, kafkaInstance.GetName())
		return nil, nil, err
	}

	bootstrapURL := kafkaInstance.GetBootstrapServerHost()
	if bootstrapURL == "" {
		err = fmt.Errorf(`bootstrap URL is missing for Kafka instance "%v"`, kafkaInstance.GetName())

		return nil, nil, err
	}

	apiURL := kafkaInstance.GetAdminApiServerUrl()

	client := kafkainstance.NewAPIClient(&kafkainstance.Config{
		BaseURL:    apiURL,
		HTTPClient: c.httpClient,
	})

	return client, &kafkaInstance, nil
}

func (c *DefaultClient) HTTPClient() *http.Client {
	return c.httpClient
}
