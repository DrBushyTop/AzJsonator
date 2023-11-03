package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type AzClient struct {
	credential            *azidentity.ChainedTokenCredential
	resourceGroupsClient  *armresources.ResourceGroupsClient
	resourceClient        *armresources.Client
	providersClient       *armresources.ProvidersClient
	CurrentSubscriptionID string
}

func NewAzClient(subId string) (*AzClient, error) {
	azCLI, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		// TODO: handle error
		return nil, err
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

	provClient, err := armresources.NewProvidersClient(subId, credential, nil)
	if err != nil {
		return nil, err
	}

	return &AzClient{
		credential:            credential,
		resourceClient:        resClient,
		resourceGroupsClient:  rgClient,
		providersClient:       provClient,
		CurrentSubscriptionID: subId,
	}, nil
}

func (c *AzClient) GetResourceGroupNames() ([]string, error) {
	resourceGroups, err := c.GetResourceGroups()
	if err != nil {
		return nil, err
	}
	var resourceGroupNames []string
	for _, rg := range resourceGroups {
		resourceGroupNames = append(resourceGroupNames, *rg.Name)
	}

	return resourceGroupNames, nil
}

func (c *AzClient) GetResourceGroups() ([]*armresources.ResourceGroup, error) {
	rgs := c.resourceGroupsClient.NewListPager(nil)
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
	resources := c.resourceClient.NewListPager(&armresources.ClientListOptions{
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

func (c *AzClient) GetResourceByResourceId(resourceId string) (*armresources.GenericResource, error) {
	// split resource type from resource id
	split := strings.Split(resourceId, "/")
	resourceProvider := split[6]
	resourceType := split[7]
	apiVersion, err := c.GetLatestApiVersion(resourceProvider, resourceType)
	if err != nil {
		return nil, err
	}

	// get latest api version for resource type
	resource, err := c.resourceClient.GetByID(context.Background(), resourceId, apiVersion, nil)
	if err != nil {
		return nil, err
	}
	return &resource.GenericResource, nil
}

func (c *AzClient) GetLatestApiVersion(resourceProvider string, resourceType string) (string, error) {
	provider, err := c.providersClient.Get(context.Background(), resourceProvider, nil)
	if err != nil {
		return "", err
	}
	for _, rt := range provider.Provider.ResourceTypes {
		if *rt.ResourceType != resourceType {
			continue
		}

		if len(rt.APIVersions) == 0 {
			return "", fmt.Errorf("no api versions found for resource type %s", resourceType)
		}

		return *rt.APIVersions[0], nil
	}
	return "", fmt.Errorf("no resource type %s found for provider %s", resourceType, resourceProvider)
}

func (c *AzClient) GetSubresourcesByResourceId(resourceId string) []*armresources.GenericResourceExpanded {
	// split resource type from resource id
	resourceTypeIndex := 7
	split := strings.Split(resourceId, "/")
	resourceProvider := split[6]
	resourceType := split[resourceTypeIndex]

	// Get subresource type
	for len(split) > resourceTypeIndex+2 {
		resourceTypeIndex = resourceTypeIndex + 2
		resourceType = resourceType + "/" + split[resourceTypeIndex]
	}

	// Get all resource types
	subResourceTypes, err := c.GetSubresourceTypes(resourceProvider, resourceType)
	if err != nil {
		log.Printf("Unable to get subresource types for %s: %s", resourceType, err)
		return nil
	}

	type result struct {
		subResources []*armresources.GenericResourceExpanded
		err          error
	}

	// Get all resources of subresource type
	var resources []*armresources.GenericResourceExpanded
	results := make(chan result)
	var wg sync.WaitGroup // Use WaitGroup to wait for all goroutines to finish

	for _, subResourceType := range subResourceTypes {
		wg.Add(1) // Increment the WaitGroup counter
		go func(srType string) {
			defer wg.Done() // Decrement the counter when the goroutine completes
			srList, err := c.GetAllSubresourcesWithSpecificParent(resourceId, srType)
			results <- result{srList, err}
		}(subResourceType)
	}

	// Start a goroutine to close the results channel once all goroutines are done
	go func() {
		wg.Wait()      // Wait for all goroutines to finish
		close(results) // Close the channel so the loop below can terminate
	}()

	for r := range results {
		if r.err != nil {
			log.Printf("Error: %s", r.err)
			continue // If there's an error, skip this set of subresources
		}
		resources = append(resources, r.subResources...)
	}

	return resources
}

func (c *AzClient) GetSubresourceTypes(resourceProvider string, resourceType string) ([]string, error) {
	provider, err := c.providersClient.Get(context.Background(), resourceProvider, nil)
	if err != nil {
		return nil, err
	}

	var res []string

	for _, rt := range provider.Provider.ResourceTypes {
		if !strings.HasPrefix(*rt.ResourceType, fmt.Sprintf("%s/", resourceType)) {
			continue
		}
		res = append(res, *rt.ResourceType)
	}

	return res, nil
}

func (c *AzClient) GetAllSubresourcesWithSpecificParent(parent string, resourceType string) ([]*armresources.GenericResourceExpanded, error) {
	split := strings.Split(parent, "/")
	resourceGroup := split[4]
	// provider := split[6]

	resources := c.resourceClient.NewListByResourceGroupPager(resourceGroup, &armresources.ClientListByResourceGroupOptions{
		// Filter: to.Ptr("resourceType eq '" + provider + "/" + resourceType + "'"),
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
