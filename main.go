package main

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
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

var ENV_REPO, ENV_TAG, ENV_SERVICE, ENV_WEBHOOK_TOKEN, ENV_SERVICESYNC_PORT, ENV_USERNAME, ENV_PASSWORD, UPDATE_IMAGE string

func main() {

	// read environment variables
	ENV_REPO = os.Getenv("ENV_REPO")
	ENV_TAG = os.Getenv("ENV_TAG")
	ENV_SERVICE = os.Getenv("ENV_SERVICE")
	ENV_WEBHOOK_TOKEN = os.Getenv("ENV_WEBHOOK_TOKEN")
	ENV_SERVICESYNC_PORT = os.Getenv("ENV_SERVICESYNC_PORT")

	ENV_USERNAME = os.Getenv("ENV_USERNAME")
	ENV_PASSWORD = os.Getenv("ENV_PASSWORD")

	// set image to update
	if ENV_TAG != "" {
		UPDATE_IMAGE = ENV_REPO + ":" + ENV_TAG
	} else {
		UPDATE_IMAGE = ENV_REPO
	}

	log.Printf("ENV_REPO: %v\n", ENV_REPO)
	log.Printf("ENV_TAG: %v\n", ENV_TAG)
	log.Printf("ENV_SERVICE: %v\n", ENV_SERVICE)
	log.Printf("ENV_WEBHOOK_TOKEN: %v\n", ENV_WEBHOOK_TOKEN)
	log.Printf("ENV_SERVICESYNC_PORT: %v\n", ENV_SERVICESYNC_PORT)
	log.Printf("UPDATE_IMAGE: %v\n", UPDATE_IMAGE)

	// start webserver
	http.HandleFunc("/webhooks/"+ENV_WEBHOOK_TOKEN, syncDockerService)
	http.ListenAndServe(":"+ENV_SERVICESYNC_PORT, nil)
}

func syncDockerService(w http.ResponseWriter, req *http.Request) {

	// read params
	var params DockerWebhookRequest
	body, _ := ioutil.ReadAll(req.Body)
	json.Unmarshal(body, &params)
	log.Println("Incoming request found for image: " + params.Repository.Repo_name + ":" + params.Push_data.Tag)

	if params.Repository.Repo_name == ENV_REPO && params.Push_data.Tag == ENV_TAG {
		go updateService()
		http.Get(params.Callback_url)
	} else {
		log.Println("Image is not being watched:")
		log.Println("IMAGE: " + params.Repository.Repo_name + " -> " + ENV_REPO)
		log.Println("TAG: " + params.Push_data.Tag + " -> " + ENV_TAG)
	}
}

func updateService() error {

	// start new docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Printf("cli: %v\n", cli)
		return err
	}
	// login docker hub
	_, err = cli.RegistryLogin(context.Background(), types.AuthConfig{Username: ENV_USERNAME, Password: ENV_PASSWORD})
	if err == nil {
		// pull new image
		log.Println("Pulling new image")
		out, err := cli.ImagePull(context.Background(), UPDATE_IMAGE, types.ImagePullOptions{Platform: "linux/amd64"})
		if err != nil {
			panic(err)
		}
		defer out.Close()
		io.Copy(os.Stdout, out)

	}

	// scan running service
	service, _, err := cli.ServiceInspectWithRaw(context.Background(), ENV_SERVICE, types.ServiceInspectOptions{})
	if err != nil {
		log.Printf("service: %v\n", service)
		return err
	}

	// update service
	service.Spec.TaskTemplate.ContainerSpec.Image = UPDATE_IMAGE
	service.Spec.TaskTemplate.ForceUpdate = uint64(time.Now().Unix())

	log.Println("Updating service")
	// send update request to docker socket
	serviceResponse, err := cli.ServiceUpdate(
		context.Background(),
		service.ID,
		service.Meta.Version,
		service.Spec,
		types.ServiceUpdateOptions{})

	if err != nil {
		log.Printf("serviceResponse: %v\n", serviceResponse)
		return err
	}

	// close client interface
	cli.Close()

	log.Println("Done!")
	return nil
}
