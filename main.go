package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/DrBushytop/AzJsonator/templates"
	"github.com/a-h/templ"
)

func main() {
	// Create a new AzClient
	azClient, err := NewAzClient("")
	if err != nil {
		panic(err)
	}

	component := templates.Index()
	// component.Render(context.Background(), os.Stdout)
	http.Handle("/", templ.Handler(component))
	http.HandleFunc("/resourceGroup/List", resourceGroupListHander(azClient))
	http.HandleFunc("/resource/ListByResourceGroup", resourceListHandler(azClient))
	http.HandleFunc("/resource/GetByResourceId", resourceByIdHanlder(azClient))
	fmt.Println("Listening on http://localhost:3000")
	http.ListenAndServe(":3000", nil)

	// fmt.Println(azClient.GetLatestApiVersion("Microsoft.Network", "virtualNetworks"))

	// fmt.Println(azClient.GetSubresourceTypes("Microsoft.Network", "virtualNetworks"))

	// fmt.Println(azClient.GetResourceByResourceId("/subscriptions//resourceGroups/tfstate-dwf/providers/Microsoft.Network/virtualNetworks/hub"))
}

func resourceGroupListHander(c *AzClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rgs, err := c.GetResourceGroupNames()
		if err != nil {
			log.Println(err)
			w.Write([]byte(fmt.Sprintf("<option value='error'>%s</option>", err)))
		}
		template := templates.ResourceGroupList(rgs)
		template.Render(r.Context(), w)
	}
}

func resourceListHandler(c *AzClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		resourceGroup := queryParams.Get("groupName")
		resources, err := c.GetResourcesInResourceGroup(resourceGroup)
		if err != nil {
			log.Println(err)
			w.Write([]byte(fmt.Sprintf("<ul><li>%s</li></ul>", err)))
		}
		template := templates.ResourceList(resources)
		template.Render(r.Context(), w)
	}
}

func resourceByIdHanlder(c *AzClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()
		resourceId := queryParams.Get("id")

		unescapedResourceId, err := url.QueryUnescape(resourceId)
		if err != nil {
			log.Println(err)
			w.Write([]byte(fmt.Sprintf("Unable to parse resourceId: %s", err)))
		}

		resource, err := c.GetResourceByResourceId(unescapedResourceId)
		if err != nil {
			log.Println(err)
			w.Write([]byte(fmt.Sprintf("Unable to get resource: %s", err)))
		}

		jsonString, err := json.MarshalIndent(resource, "", "  ")
		if err != nil {
			fmt.Println(err)
		}

		template := templates.ResourceJson(string(jsonString))
		template.Render(r.Context(), w)
	}
}
