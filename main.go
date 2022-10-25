package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type DockerWebhookRequest struct {
	Callback_url string
	Push_data    struct {
		Pushed_at int    `json:"Pushed_at"`
		Pusher    string `json:"Pusher"`
		Tag       string `json:"Tag"`
	}
	Repository struct {
		Comment_count    int    `json:"Comment_count"`
		Date_created     int    `json:"Date_created"`
		Description      string `json:"Description"`
		Dockerfile       string `json:"Dockerfile"`
		Full_description string `json:"Full_description"`
		Is_official      bool   `json:"Is_official"`
		Is_private       bool   `json:"Is_private"`
		Is_trusted       bool   `json:"Is_trusted"`
		Name             string `json:"Name"`
		Namespace        string `json:"Namespace"`
		Owner            string `json:"Owner"`
		Repo_name        string `json:"Repo_name"`
		Repo_url         string `json:"Repo_url"`
		Star_count       int    `json:"Star_count"`
		Status           string `json:"Status"`
	}
}

var ENV_REPO, ENV_TAG, ENV_SERVICE, ENV_WEBHOOK_TOKEN, ENV_SERVICESYNC_PORT string

func main() {

	// read environment variables
	ENV_REPO = os.Getenv("ENV_REPO")
	ENV_TAG = os.Getenv("ENV_TAG")
	ENV_SERVICE = os.Getenv("ENV_SERVICE")
	ENV_WEBHOOK_TOKEN = os.Getenv("ENV_WEBHOOK_TOKEN")
	ENV_SERVICESYNC_PORT = os.Getenv("ENV_SERVICESYNC_PORT")

	fmt.Println("Wachting for updates on service: " + ENV_SERVICE)
	fmt.Println("Wachting for updates on image: " + ENV_REPO + ENV_TAG)
	fmt.Println("Listening on :" + ENV_SERVICESYNC_PORT + "/webhooks/" + ENV_WEBHOOK_TOKEN)

	// start webserver
	http.HandleFunc("/webhooks/"+ENV_WEBHOOK_TOKEN, syncDockerService)
	http.ListenAndServe(":"+ENV_SERVICESYNC_PORT, nil)
}

func syncDockerService(w http.ResponseWriter, req *http.Request) {
	var params DockerWebhookRequest

	body, _ := ioutil.ReadAll(req.Body)
	json.Unmarshal(body, &params)

	fmt.Println("Incoming request found for image: " + params.Repository.Repo_name + ":" + params.Push_data.Tag)

	if params.Repository.Repo_name+params.Push_data.Tag == ENV_REPO+ENV_TAG {
		err := updateService(ENV_SERVICE, ENV_REPO+ENV_TAG)
		if err != nil {
			http.Get(params.Callback_url)
		}
		fmt.Printf("err: %v\n", err)
	} else {
		fmt.Println("Image is not being watched: " + params.Repository.Repo_name + params.Push_data.Tag + " != " + ENV_REPO + ENV_TAG)
	}
}

func updateService(serviceID, imagename string) error {

	// start new docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Printf("cli: %v\n", cli)
		return err
	}

	// scan running service
	service, _, err := cli.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	if err != nil {
		fmt.Printf("service: %v\n", service)
		return err
	}

	// update service
	service.Spec.TaskTemplate.ContainerSpec.Image = imagename
	service.Spec.TaskTemplate.ForceUpdate = uint64(time.Now().Unix())

	// send update request to docker socket
	serviceResponse, err := cli.ServiceUpdate(
		context.Background(),
		service.ID,
		service.Meta.Version,
		service.Spec,
		types.ServiceUpdateOptions{})

	if err != nil {
		fmt.Printf("serviceResponse: %v\n", serviceResponse)
		return err
	}

	// close client interface
	cli.Close()
	return nil
}
