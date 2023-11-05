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
	azClient, err := NewAzClient("")
	if err != nil {
		panic(err)
	}

	component := templates.Index()
	http.Handle("/", templ.Handler(component))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	http.HandleFunc("/resource-groups/list", resourceGroupListHander(azClient))
	http.HandleFunc("/resources/list-by-resource-group", resourceListHandler(azClient))
	http.HandleFunc("/resources/get-by-resource-id", resourceByIdHandler(azClient))
	http.HandleFunc("/resources/get-subresources-by-id", subResourcesHandler(azClient))
	fmt.Println("Listening on http://localhost:3000")
	http.ListenAndServe("localhost:3000", nil)
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
		resourceGroup := r.URL.Query().Get("groupName")
		resources, err := c.GetResourcesInResourceGroup(resourceGroup)
		if err != nil {
			log.Println(err)
			w.Write([]byte(fmt.Sprintf("<ul><li>%s</li></ul>", err)))
		}
		template := templates.ResourceList(resources)
		template.Render(r.Context(), w)
	}
}

func resourceByIdHandler(c *AzClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceId := r.URL.Query().Get("id")

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

func subResourcesHandler(c *AzClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceId := r.URL.Query().Get("id")

		unescapedResourceId, err := url.QueryUnescape(resourceId)
		if err != nil {
			log.Println(err)
			w.Write([]byte(fmt.Sprintf("Unable to parse resourceId: %s", err)))
		}

		subResources := c.GetSubresourcesByResourceId(unescapedResourceId)

		template := templates.ResourceList(subResources)
		template.Render(r.Context(), w)
	}
}
