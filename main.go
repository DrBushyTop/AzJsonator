package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profile/p20200901/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"strings"
)

func main() {
	// Create a new AzClient
	azClient, err := NewAzClient("")
	if err != nil {
		panic(err)
	}
	rgs, err := azClient.GetResourceGroups()
	if err != nil {
		fmt.Println(err)
	}
	for _, rg := range rgs[:3] { // TODO: remove [:1] to get all resource groups
		rgJson, _ := json.MarshalIndent(rg, "", "  ")
		fmt.Printf("RG:\n%s\n", rgJson)

		resources, err := azClient.GetResourcesInResourceGroup(*rg.Name)
		if err != nil {
			fmt.Println(err)
		}

		for _, res := range resources {
			resJson, _ := json.MarshalIndent(res, "", "  ")
			fmt.Printf("Resource:\n%s\n", resJson)
		}
	}
}

type AzClient struct {
	Credential            *azidentity.ChainedTokenCredential
	SubscriptionsClient   *armsubscriptions.SubscriptionClient
	ResourceGroupsClient  *armresources.ResourceGroupsClient
	ResourceClient        *armresources.Client
	CurrentSubscriptionID string
}

func NewAzClient(subId string) (*AzClient, error) {
	azCLI, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		// TODO: handle error
	}

	credential, err := azidentity.NewChainedTokenCredential(
		[]azcore.TokenCredential{
			azCLI,
		}, nil,
	)
	if err != nil {
		return nil, err
	}

	rgClient, err := armresources.NewResourceGroupsClient(subId, credential, nil)
	if err != nil {
		return nil, err
	}

	resClient, err := armresources.NewClient(subId, credential, nil)
	if err != nil {
		return nil, err
	}

	return &AzClient{
		Credential:            credential,
		ResourceClient:        resClient,
		ResourceGroupsClient:  rgClient,
		CurrentSubscriptionID: "",
	}, nil
}

func (c *AzClient) GetResourceGroups() ([]*armresources.ResourceGroup, error) {
	rgs := c.ResourceGroupsClient.NewListPager(nil)
	var resourceGroups []*armresources.ResourceGroup
	for rgs.More() {
		page, err := rgs.NextPage(context.Background())
		if err != nil {
			break
		}
		resourceGroups = append(resourceGroups, page.Value...)
	}
	return resourceGroups, nil
}

func (c *AzClient) GetResourcesInResourceGroup(resourceGroupName string) ([]*armresources.GenericResourceExpanded, error) {
	resources := c.ResourceClient.NewListPager(&armresources.ClientListOptions{
		Filter: to.Ptr("resourceGroup eq '" + resourceGroupName + "'"),
	})
	var resourcesList []*armresources.GenericResourceExpanded
	for resources.More() {
		page, err := resources.NextPage(context.Background())
		if err != nil {
			break
		}
		resourcesList = append(resourcesList, page.Value...)
	}
	return resourcesList, nil
}

func (c AzClient) GetResourceByResourceId(resourceId string) (*armresources.GenericResource, error) {
	// split resource type from resource id
	resourceType := strings.Split(resourceId, "/")[6]

	// get latest api version for resource type

	resource, err := c.ResourceClient.GetByID(context.Background(), resourceId, apiVersion)
	if err != nil {
		return nil, err
	}
	return &resource.GenericResource, nil
}